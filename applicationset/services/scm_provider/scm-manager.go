package scm_provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"

	scmm "github.com/scm-manager/goscm"
)

type ScmManagerProvider struct {
	client      *scmm.Client
	allBranches bool
}

var _ SCMProviderService = &ScmManagerProvider{}

func NewScmManagerProvider(ctx context.Context, token, url string, allBranches, insecure bool, scmRootCAPath string, caCerts []byte) (*ScmManagerProvider, error) {
	if token == "" {
		token = os.Getenv("SCMM_TOKEN")
	}
	httpClient := &http.Client{}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = utils.GetTlsConfig(scmRootCAPath, insecure, caCerts)
	httpClient.Transport = tr

	client, err := scmm.NewClient(url, token)
	if err != nil {
		return nil, fmt.Errorf("error creating a new SCM-Manager client: %w", err)
	}

	client.SetHttpClient(httpClient)

	return &ScmManagerProvider{
		client:      client,
		allBranches: allBranches,
	}, nil
}

func (g *ScmManagerProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	scmmRepo, err := g.client.GetRepo(repo.Organization, repo.Repository)
	if err != nil {
		return nil, err
	}

	if !g.allBranches {
		defaultBranch, err := g.client.GetDefaultBranch(repo.Organization, repo.Repository)
		if err != nil {
			return nil, err
		}

		return []*Repository{
			{
				Organization: repo.Organization,
				Repository:   repo.Repository,
				Branch:       defaultBranch.Name,
				URL:          repo.URL,
				SHA:          defaultBranch.Revision,
				Labels:       make([]string, 0),
				RepositoryId: scmmRepo.Namespace + "/" + scmmRepo.Name,
			},
		}, nil
	}
	repos := []*Repository{}
	branches, err := g.client.ListRepoBranches(repo.Organization, repo.Repository)
	if err != nil {
		return nil, err
	}
	for _, branch := range branches.Embedded.Branches {
		repos = append(repos, &Repository{
			Organization: scmmRepo.Namespace,
			Repository:   scmmRepo.Name,
			Branch:       branch.Name,
			URL:          scmmRepo.Links.ProtocolUrl[0].Href,
			SHA:          branch.Revision,
			Labels:       make([]string, 0),
			RepositoryId: scmmRepo.Namespace + "/" + scmmRepo.Name,
		})
	}
	return repos, nil
}

func (g *ScmManagerProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	repos := []*Repository{}
	filter := g.client.NewRepoListFilter()
	filter.Limit = 9999
	scmmRepos, err := g.client.ListRepos(filter)
	if err != nil {
		return nil, err
	}
	for _, scmmRepo := range scmmRepos.Embedded.Repositories {
		var url string
		switch cloneProtocol {
		// Default to SSH if unspecified (i.e. if ""). SSH Plugin needs to be installed
		case "", "ssh":
			url = getProtocolUrlByName(scmmRepo.Links.ProtocolUrl, "ssh")
		case "https":
			url = getProtocolUrlByName(scmmRepo.Links.ProtocolUrl, "http")
		default:
			return nil, fmt.Errorf("unknown clone protocol %v", cloneProtocol)
		}

		if url == "" {
			return nil, errors.New("could not find valid repository protocol url")
		}

		defaultBranch, err := g.client.GetDefaultBranch(scmmRepo.Namespace, scmmRepo.Name)
		if err != nil {
			if errors.Is(err, scmm.ErrEmptyRepository) || errors.Is(err, scmm.ErrNoDefaultBranchFound) {
				continue
			} else {
				return nil, err
			}
		}

		repos = append(repos, &Repository{
			Organization: scmmRepo.Namespace,
			Repository:   scmmRepo.Name,
			Branch:       defaultBranch.Name,
			URL:          url,
			RepositoryId: scmmRepo.Namespace + "/" + scmmRepo.Name,
		})
	}
	return repos, nil
}

func getProtocolUrlByName(urls []scmm.ProtocolUrl, name string) string {
	for _, url := range urls {
		if url.Name == name {
			return url.Href
		}
	}
	return ""
}

func (g *ScmManagerProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	_, resp, err := g.client.GetContent(repo.Organization, repo.Repository, repo.Branch, path)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if fmt.Sprint(err) == "expect file, got directory" {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
