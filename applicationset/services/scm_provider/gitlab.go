package scm_provider

import (
	"context"
	"fmt"
	"os"
	pathpkg "path"

	gitlab "github.com/xanzy/go-gitlab"
)

type GitlabProvider struct {
	client           *gitlab.Client
	organization     string
	allBranches      bool
	includeSubgroups bool
}

var _ SCMProviderService = &GitlabProvider{}

func NewGitlabProvider(ctx context.Context, organization string, token string, url string, allBranches, includeSubgroups bool) (*GitlabProvider, error) {
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}
	var client *gitlab.Client
	if url == "" {
		var err error
		client, err = gitlab.NewClient(token)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(url))
		if err != nil {
			return nil, err
		}
	}
	return &GitlabProvider{client: client, organization: organization, allBranches: allBranches, includeSubgroups: includeSubgroups}, nil
}

func (g *GitlabProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	branches, err := g.listBranches(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("error listing branches for %s/%s: %v", repo.Organization, repo.Repository, err)
	}

	for _, branch := range branches {
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       branch.Name,
			SHA:          branch.Commit.ID,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *GitlabProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	opt := &gitlab.ListGroupProjectsOptions{
		ListOptions:      gitlab.ListOptions{PerPage: 100},
		IncludeSubgroups: &g.includeSubgroups,
	}
	repos := []*Repository{}
	for {
		gitlabRepos, resp, err := g.client.Groups.ListGroupProjects(g.organization, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing projects for %s: %v", g.organization, err)
		}
		for _, gitlabRepo := range gitlabRepos {
			var url string
			switch cloneProtocol {
			// Default to SSH if unspecified (i.e. if "").
			case "", "ssh":
				url = gitlabRepo.SSHURLToRepo
			case "https":
				url = gitlabRepo.HTTPURLToRepo
			default:
				return nil, fmt.Errorf("unknown clone protocol for Gitlab %v", cloneProtocol)
			}

			repos = append(repos, &Repository{
				Organization: gitlabRepo.Namespace.FullPath,
				Repository:   gitlabRepo.Path,
				URL:          url,
				Branch:       gitlabRepo.DefaultBranch,
				Labels:       gitlabRepo.TagList,
				RepositoryId: gitlabRepo.ID,
			})
		}
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opt.Page = resp.NextPage
	}
	return repos, nil
}

func (g *GitlabProvider) RepoHasPath(_ context.Context, repo *Repository, path string) (bool, error) {
	p, _, err := g.client.Projects.GetProject(repo.Organization+"/"+repo.Repository, nil)
	if err != nil {
		return false, err
	}
	directories := []string{
		path,
		pathpkg.Dir(path),
	}
	for _, directory := range directories {
		options := gitlab.ListTreeOptions{
			Path: &directory,
			Ref:  &repo.Branch,
		}
		for {
			treeNode, resp, err := g.client.Repositories.ListTree(p.ID, &options)
			if err != nil {
				return false, err
			}
			if path == directory {
				if resp.TotalItems > 0 {
					return true, nil
				}
			}
			for i := range treeNode {
				if treeNode[i].Path == path {
					return true, nil
				}
			}
			if resp.NextPage == 0 {
				// no future pages
				break
			}
			options.Page = resp.NextPage
		}
	}
	return false, nil
}

func (g *GitlabProvider) listBranches(_ context.Context, repo *Repository) ([]gitlab.Branch, error) {
	branches := []gitlab.Branch{}
	// If we don't specifically want to query for all branches, just use the default branch and call it a day.
	if !g.allBranches {
		gitlabBranch, _, err := g.client.Branches.GetBranch(repo.RepositoryId, repo.Branch, nil)
		if err != nil {
			return nil, err
		}
		branches = append(branches, *gitlabBranch)
		return branches, nil
	}
	// Otherwise, scrape the ListBranches API.
	opt := &gitlab.ListBranchesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	for {
		gitlabBranches, resp, err := g.client.Branches.ListBranches(repo.RepositoryId, opt)
		if err != nil {
			return nil, err
		}
		for _, gitlabBranch := range gitlabBranches {
			branches = append(branches, *gitlabBranch)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return branches, nil
}
