package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

type GithubProvider struct {
	client       *github.Client
	organization string
	allBranches  bool
}

var _ SCMProviderService = &GithubProvider{}

func NewGithubProvider(ctx context.Context, organization string, token string, url string, allBranches bool) (*GithubProvider, error) {
	var ts oauth2.TokenSource
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		ts = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	}
	httpClient := oauth2.NewClient(ctx, ts)
	var client *github.Client
	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		var err error
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, err
		}
	}
	return &GithubProvider{client: client, organization: organization, allBranches: allBranches}, nil
}

func (g *GithubProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	branches, err := g.listBranches(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %w", repo.Organization, repo.Repository, err)
	}

	for _, branch := range branches {
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       branch.GetName(),
			SHA:          branch.GetCommit().GetSHA(),
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *GithubProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	repos := []*Repository{}
	for {
		githubRepos, resp, err := g.client.Repositories.ListByOrg(ctx, g.organization, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing repositories for %s: %w", g.organization, err)
		}
		for _, githubRepo := range githubRepos {
			var url string
			switch cloneProtocol {
			// Default to SSH if unspecified (i.e. if "").
			case "", "ssh":
				url = githubRepo.GetSSHURL()
			case "https":
				url = githubRepo.GetCloneURL()
			default:
				return nil, fmt.Errorf("unknown clone protocol for GitHub %v", cloneProtocol)
			}
			repos = append(repos, &Repository{
				Organization: githubRepo.Owner.GetLogin(),
				Repository:   githubRepo.GetName(),
				Branch:       githubRepo.GetDefaultBranch(),
				URL:          url,
				Labels:       githubRepo.Topics,
				RepositoryId: githubRepo.ID,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}

func (g *GithubProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	_, _, resp, err := g.client.Repositories.GetContents(ctx, repo.Organization, repo.Repository, path, &github.RepositoryContentGetOptions{
		Ref: repo.Branch,
	})
	// 404s are not an error here, just a normal false.
	if resp != nil && resp.StatusCode == 404 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (g *GithubProvider) listBranches(ctx context.Context, repo *Repository) ([]github.Branch, error) {
	// If we don't specifically want to query for all branches, just use the default branch and call it a day.
	if !g.allBranches {
		defaultBranch, resp, err := g.client.Repositories.GetBranch(ctx, repo.Organization, repo.Repository, repo.Branch, 0)
		if err != nil {
			if resp.StatusCode == http.StatusNotFound {
				// Default branch doesn't exist, so the repo is empty.
				return []github.Branch{}, nil
			}
			return nil, err
		}
		return []github.Branch{*defaultBranch}, nil
	}
	// Otherwise, scrape the ListBranches API.
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	branches := []github.Branch{}
	for {
		githubBranches, resp, err := g.client.Repositories.ListBranches(ctx, repo.Organization, repo.Repository, opt)
		if err != nil {
			return nil, err
		}
		for _, githubBranch := range githubBranches {
			branches = append(branches, *githubBranch)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return branches, nil
}
