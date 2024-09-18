package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	bitbucket "github.com/ktrysmt/go-bitbucket"
)

type BitBucketCloudProvider struct {
	client      *ExtendedClient
	allBranches bool
	owner       string
}

type ExtendedClient struct {
	*bitbucket.Client
	username string
	password string
	owner    string
}

func (c *ExtendedClient) GetContents(repo *Repository, path string) (bool, error) {
	urlStr := c.GetApiBaseURL()

	// Getting file contents from V2 defined at https://developer.atlassian.com/cloud/bitbucket/rest/api-group-source/#api-repositories-workspace-repo-slug-src-commit-path-get
	urlStr += fmt.Sprintf("/repositories/%s/%s/src/%s/%s?format=meta", c.owner, repo.Repository, repo.SHA, path)
	body := strings.NewReader("")

	req, err := http.NewRequest(http.MethodGet, urlStr, body)
	if err != nil {
		return false, err
	}
	req.SetBasicAuth(c.username, c.password)
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, fmt.Errorf("%s", resp.Status)
}

var _ SCMProviderService = &BitBucketCloudProvider{}

func NewBitBucketCloudProvider(ctx context.Context, owner string, user string, password string, allBranches bool) (*BitBucketCloudProvider, error) {
	client := &ExtendedClient{
		bitbucket.NewBasicAuth(user, password),
		user,
		password,
		owner,
	}
	return &BitBucketCloudProvider{client: client, owner: owner, allBranches: allBranches}, nil
}

func (g *BitBucketCloudProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	branches, err := g.listBranches(repo)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %w", repo.Organization, repo.Repository, err)
	}

	for _, branch := range branches {
		hash, ok := branch.Target["hash"].(string)
		if !ok {
			return nil, fmt.Errorf("error getting SHA for branch for %s/%s/%s: %w", g.owner, repo.Repository, branch.Name, err)
		}
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       branch.Name,
			SHA:          hash,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *BitBucketCloudProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	if cloneProtocol == "" {
		cloneProtocol = "ssh"
	}
	opt := &bitbucket.RepositoriesOptions{
		Owner: g.owner,
		Role:  "member",
	}
	repos := []*Repository{}
	accountReposResp, err := g.client.Repositories.ListForAccount(opt)
	if err != nil {
		return nil, fmt.Errorf("error listing repositories for %s: %w", g.owner, err)
	}
	for _, bitBucketRepo := range accountReposResp.Items {
		cloneUrl, err := findCloneURL(cloneProtocol, &bitBucketRepo)
		if err != nil {
			return nil, fmt.Errorf("error fetching clone url for repo %s: %w", bitBucketRepo.Slug, err)
		}
		repos = append(repos, &Repository{
			Organization: g.owner,
			Repository:   bitBucketRepo.Slug,
			Branch:       bitBucketRepo.Mainbranch.Name,
			URL:          *cloneUrl,
			Labels:       []string{},
			RepositoryId: bitBucketRepo.Uuid,
		})
	}
	return repos, nil
}

func (g *BitBucketCloudProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	contents, err := g.client.GetContents(repo, path)
	if err != nil {
		return false, err
	}
	if contents {
		return true, nil
	}
	return false, nil
}

func (g *BitBucketCloudProvider) listBranches(repo *Repository) ([]bitbucket.RepositoryBranch, error) {
	if !g.allBranches {
		repoBranch, err := g.client.Repositories.Repository.GetBranch(&bitbucket.RepositoryBranchOptions{
			Owner:      g.owner,
			RepoSlug:   repo.Repository,
			BranchName: repo.Branch,
		})
		if err != nil {
			return nil, err
		}
		return []bitbucket.RepositoryBranch{
			*repoBranch,
		}, nil
	}

	branches, err := g.client.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{
		Owner:    g.owner,
		RepoSlug: repo.Repository,
	})
	if err != nil {
		return nil, err
	}
	return branches.Branches, nil
}

func findCloneURL(cloneProtocol string, repo *bitbucket.Repository) (*string, error) {
	cloneLinks, ok := repo.Links["clone"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unknown type returned from repo links")
	}
	for _, link := range cloneLinks {
		linkEntry, ok := link.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unknown type returned from clone link")
		}
		if linkEntry["name"] == cloneProtocol {
			url, ok := linkEntry["href"].(string)
			if !ok {
				return nil, fmt.Errorf("could not find href for clone link")
			}
			return &url, nil
		}
	}
	return nil, fmt.Errorf("unknown clone protocol for Bitbucket cloud %v", cloneProtocol)
}
