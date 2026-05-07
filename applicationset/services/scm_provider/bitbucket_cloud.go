package scm_provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bitbucket "github.com/ktrysmt/go-bitbucket"
)

type BitBucketCloudProvider struct {
	client      *ExtendedClient
	allBranches bool
	owner       string
	// bearer tokens (workspace access tokens, OAuth client credentials) have no
	// user membership context, so the role=member filter must be omitted.
	omitRoleFilter bool
}

type ExtendedClient struct {
	*bitbucket.Client
}

var _ SCMProviderService = &BitBucketCloudProvider{}

func NewBitBucketCloudProvider(owner string, user string, password string, allBranches bool) (*BitBucketCloudProvider, error) {
	bitbucketClient, err := bitbucket.NewBasicAuth(user, password)
	if err != nil {
		return nil, fmt.Errorf("error creating BitBucket Cloud client with basic auth: %w", err)
	}
	client := &ExtendedClient{
		Client: bitbucketClient,
	}
	return &BitBucketCloudProvider{client: client, owner: owner, allBranches: allBranches}, nil
}

func NewBitBucketCloudProviderBearerToken(owner string, token string, allBranches bool) (*BitBucketCloudProvider, error) {
	bitbucketClient, err := bitbucket.NewOAuthbearerToken(token)
	if err != nil {
		return nil, fmt.Errorf("error creating BitBucket Cloud client with bearer token: %w", err)
	}
	client := &ExtendedClient{
		Client: bitbucketClient,
	}
	return &BitBucketCloudProvider{client: client, owner: owner, allBranches: allBranches, omitRoleFilter: true}, nil
}

func (g *BitBucketCloudProvider) GetBranches(_ context.Context, repo *Repository) ([]*Repository, error) {
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

func (g *BitBucketCloudProvider) ListRepos(_ context.Context, cloneProtocol string) ([]*Repository, error) {
	if cloneProtocol == "" {
		cloneProtocol = "ssh"
	}
	opt := &bitbucket.RepositoriesOptions{
		Owner: g.owner,
	}
	if !g.omitRoleFilter {
		opt.Role = "member"
	}
	repos := []*Repository{}
	accountReposResp, err := g.client.Repositories.ListForAccount(opt)
	if err != nil {
		return nil, fmt.Errorf("error listing repositories for %s: %w", g.owner, err)
	}
	for _, bitBucketRepo := range accountReposResp.Items {
		cloneURL, err := findCloneURL(cloneProtocol, &bitBucketRepo)
		if err != nil {
			return nil, fmt.Errorf("error fetching clone url for repo %s: %w", bitBucketRepo.Slug, err)
		}
		repos = append(repos, &Repository{
			Organization: g.owner,
			Repository:   bitBucketRepo.Slug,
			Branch:       bitBucketRepo.Mainbranch.Name,
			URL:          *cloneURL,
			Labels:       []string{},
			RepositoryId: bitBucketRepo.Uuid,
		})
	}
	return repos, nil
}

func (g *BitBucketCloudProvider) RepoHasPath(_ context.Context, repo *Repository, path string) (bool, error) {
	_, err := g.client.Repositories.Repository.GetFileContent(&bitbucket.RepositoryFilesOptions{
		Owner:    g.owner,
		RepoSlug: repo.Repository,
		Ref:      repo.SHA,
		Path:     path,
	})
	if err != nil {
		var statusErr *bitbucket.UnexpectedResponseStatusError
		if errors.As(err, &statusErr) && strings.HasPrefix(statusErr.Status, "404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
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

	var allBranches []bitbucket.RepositoryBranch
	for pageNum := 1; ; pageNum++ {
		page, err := g.client.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{
			Owner:    g.owner,
			RepoSlug: repo.Repository,
			Pagelen:  100,
			PageNum:  pageNum,
		})
		if err != nil {
			return nil, err
		}
		allBranches = append(allBranches, page.Branches...)
		if page.Next == "" {
			break
		}
	}
	return allBranches, nil
}

func findCloneURL(cloneProtocol string, repo *bitbucket.Repository) (*string, error) {
	cloneLinks, ok := repo.Links["clone"].([]any)
	if !ok {
		return nil, errors.New("unknown type returned from repo links")
	}
	for _, link := range cloneLinks {
		linkEntry, ok := link.(map[string]any)
		if !ok {
			return nil, errors.New("unknown type returned from clone link")
		}
		if linkEntry["name"] == cloneProtocol {
			url, ok := linkEntry["href"].(string)
			if !ok {
				return nil, errors.New("could not find href for clone link")
			}
			return &url, nil
		}
	}
	return nil, fmt.Errorf("unknown clone protocol for Bitbucket cloud %v", cloneProtocol)
}
