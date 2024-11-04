package scm_provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
)

type BitbucketServerProvider struct {
	client      *bitbucketv1.APIClient
	projectKey  string
	allBranches bool
}

var _ SCMProviderService = &BitbucketServerProvider{}

func NewBitbucketServerProviderBasicAuth(ctx context.Context, username, password, url, projectKey string, allBranches bool, scmRootCAPath string, insecure bool, caCerts []byte) (*BitbucketServerProvider, error) {
	bitbucketConfig := bitbucketv1.NewConfiguration(url)
	// Avoid the XSRF check
	bitbucketConfig.AddDefaultHeader("x-atlassian-token", "no-check")
	bitbucketConfig.AddDefaultHeader("x-requested-with", "XMLHttpRequest")

	ctx = context.WithValue(ctx, bitbucketv1.ContextBasicAuth, bitbucketv1.BasicAuth{
		UserName: username,
		Password: password,
	})
	return newBitbucketServerProvider(ctx, bitbucketConfig, projectKey, allBranches, scmRootCAPath, insecure, caCerts)
}

func NewBitbucketServerProviderBearerToken(ctx context.Context, bearerToken, url, projectKey string, allBranches bool, scmRootCAPath string, insecure bool, caCerts []byte) (*BitbucketServerProvider, error) {
	bitbucketConfig := bitbucketv1.NewConfiguration(url)
	// Avoid the XSRF check
	bitbucketConfig.AddDefaultHeader("x-atlassian-token", "no-check")
	bitbucketConfig.AddDefaultHeader("x-requested-with", "XMLHttpRequest")

	ctx = context.WithValue(ctx, bitbucketv1.ContextAccessToken, bearerToken)
	return newBitbucketServerProvider(ctx, bitbucketConfig, projectKey, allBranches, scmRootCAPath, insecure, caCerts)
}

func NewBitbucketServerProviderNoAuth(ctx context.Context, url, projectKey string, allBranches bool, scmRootCAPath string, insecure bool, caCerts []byte) (*BitbucketServerProvider, error) {
	return newBitbucketServerProvider(ctx, bitbucketv1.NewConfiguration(url), projectKey, allBranches, scmRootCAPath, insecure, caCerts)
}

func newBitbucketServerProvider(ctx context.Context, bitbucketConfig *bitbucketv1.Configuration, projectKey string, allBranches bool, scmRootCAPath string, insecure bool, caCerts []byte) (*BitbucketServerProvider, error) {
	bitbucketConfig.BasePath = utils.NormalizeBitbucketBasePath(bitbucketConfig.BasePath)
	tlsConfig := utils.GetTlsConfig(scmRootCAPath, insecure, caCerts)
	bitbucketConfig.HTTPClient = &http.Client{Transport: &http.Transport{
		TLSClientConfig: tlsConfig,
	}}
	bitbucketClient := bitbucketv1.NewAPIClient(ctx, bitbucketConfig)

	return &BitbucketServerProvider{
		client:      bitbucketClient,
		projectKey:  projectKey,
		allBranches: allBranches,
	}, nil
}

func (b *BitbucketServerProvider) ListRepos(_ context.Context, cloneProtocol string) ([]*Repository, error) {
	paged := map[string]interface{}{
		"limit": 100,
	}
	repos := []*Repository{}
	for {
		response, err := b.client.DefaultApi.GetRepositoriesWithOptions(b.projectKey, paged)
		if err != nil {
			return nil, fmt.Errorf("error listing repositories for %s: %w", b.projectKey, err)
		}
		repositories, err := bitbucketv1.GetRepositoriesResponse(response)
		if err != nil {
			log.Errorf("error parsing repositories response '%v'", response.Values)
			return nil, fmt.Errorf("error parsing repositories response %s: %w", b.projectKey, err)
		}
		for _, bitbucketRepo := range repositories {
			var url string
			switch cloneProtocol {
			// Default to SSH if unspecified (i.e. if "").
			case "", "ssh":
				url = getCloneURLFromLinks(bitbucketRepo.Links.Clone, "ssh")
			case "https":
				url = getCloneURLFromLinks(bitbucketRepo.Links.Clone, "http")
			default:
				return nil, fmt.Errorf("unknown clone protocol for Bitbucket Server %v", cloneProtocol)
			}

			org := bitbucketRepo.Project.Key
			repo := bitbucketRepo.Name
			// Bitbucket doesn't return the default branch in the repo query, fetch it here
			branch, err := b.getDefaultBranch(org, repo)
			if err != nil {
				return nil, err
			}
			if branch == nil {
				log.Debugf("%s/%s does not have a default branch, skipping", org, repo)
				continue
			}

			repos = append(repos, &Repository{
				Organization: org,
				Repository:   repo,
				URL:          url,
				Branch:       branch.DisplayID,
				SHA:          branch.LatestCommit,
				Labels:       []string{}, // Not supported by library
				RepositoryId: bitbucketRepo.ID,
			})
		}
		hasNextPage, nextPageStart := bitbucketv1.HasNextPage(response)
		if !hasNextPage {
			break
		}
		paged["start"] = nextPageStart
	}
	return repos, nil
}

func (b *BitbucketServerProvider) RepoHasPath(_ context.Context, repo *Repository, path string) (bool, error) {
	opts := map[string]interface{}{
		"limit": 100,
		"at":    repo.Branch,
		"type_": true,
	}
	// No need to query for all pages here
	response, err := b.client.DefaultApi.GetContent_0(repo.Organization, repo.Repository, path, opts)
	if response != nil && response.StatusCode == 404 {
		// File/directory not found
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *BitbucketServerProvider) GetBranches(_ context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	branches, err := b.listBranches(repo)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %w", repo.Organization, repo.Repository, err)
	}

	for _, branch := range branches {
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       branch.DisplayID,
			SHA:          branch.LatestCommit,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (b *BitbucketServerProvider) listBranches(repo *Repository) ([]bitbucketv1.Branch, error) {
	// If we don't specifically want to query for all branches, just use the default branch and call it a day.
	if !b.allBranches {
		branch, err := b.getDefaultBranch(repo.Organization, repo.Repository)
		if err != nil {
			return nil, err
		}
		if branch == nil {
			return []bitbucketv1.Branch{}, nil
		}
		return []bitbucketv1.Branch{*branch}, nil
	}
	// Otherwise, scrape the GetBranches API.
	branches := []bitbucketv1.Branch{}
	paged := map[string]interface{}{
		"limit": 100,
	}
	for {
		response, err := b.client.DefaultApi.GetBranches(repo.Organization, repo.Repository, paged)
		if err != nil {
			return nil, fmt.Errorf("error listing branches for %s/%s: %w", repo.Organization, repo.Repository, err)
		}
		bitbucketBranches, err := bitbucketv1.GetBranchesResponse(response)
		if err != nil {
			log.Errorf("error parsing branches response '%v'", response.Values)
			return nil, fmt.Errorf("error parsing branches response for %s/%s: %w", repo.Organization, repo.Repository, err)
		}

		branches = append(branches, bitbucketBranches...)

		hasNextPage, nextPageStart := bitbucketv1.HasNextPage(response)
		if !hasNextPage {
			break
		}
		paged["start"] = nextPageStart
	}
	return branches, nil
}

func (b *BitbucketServerProvider) getDefaultBranch(org string, repo string) (*bitbucketv1.Branch, error) {
	response, err := b.client.DefaultApi.GetDefaultBranch(org, repo)
	// The API will return 404 if a default branch is set but doesn't exist. In case the repo is empty and default branch is unset,
	// we will get an EOF and a nil response.
	if (response != nil && response.StatusCode == 404) || (response == nil && err != nil && errors.Is(err, io.EOF)) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	branch, err := bitbucketv1.GetBranchResponse(response)
	if err != nil {
		return nil, err
	}
	return &branch, nil
}

func getCloneURLFromLinks(links []bitbucketv1.CloneLink, name string) string {
	for _, link := range links {
		if link.Name == name {
			return link.Href
		}
	}
	return ""
}
