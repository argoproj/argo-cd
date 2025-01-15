package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	pathpkg "path"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/xanzy/go-gitlab"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
)

type GitlabProvider struct {
	client                *gitlab.Client
	organization          string
	allBranches           bool
	includeSubgroups      bool
	includeSharedProjects bool
	topic                 string
}

var _ SCMProviderService = &GitlabProvider{}

func NewGitlabProvider(ctx context.Context, organization string, token string, url string, allBranches, includeSubgroups, includeSharedProjects, insecure bool, scmRootCAPath, topic string, caCerts []byte) (*GitlabProvider, error) {
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}
	var client *gitlab.Client

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = utils.GetTlsConfig(scmRootCAPath, insecure, caCerts)

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient.Transport = tr

	if url == "" {
		var err error
		client, err = gitlab.NewClient(token, gitlab.WithHTTPClient(retryClient.HTTPClient))
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		client, err = gitlab.NewClient(token, gitlab.WithBaseURL(url), gitlab.WithHTTPClient(retryClient.HTTPClient))
		if err != nil {
			return nil, err
		}
	}

	return &GitlabProvider{client: client, organization: organization, allBranches: allBranches, includeSubgroups: includeSubgroups, includeSharedProjects: includeSharedProjects, topic: topic}, nil
}

func (g *GitlabProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
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
		IncludeSubGroups: &g.includeSubgroups,
		WithShared:       &g.includeSharedProjects,
		Topic:            &g.topic,
	}

	repos := []*Repository{}
	for {
		gitlabRepos, resp, err := g.client.Groups.ListGroupProjects(g.organization, opt)
		if err != nil {
			return nil, fmt.Errorf("error listing projects for %s: %w", g.organization, err)
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

			var repoLabels []string
			if len(gitlabRepo.Topics) == 0 {
				// fallback to for gitlab prior to 14.5
				repoLabels = gitlabRepo.TagList
			} else {
				repoLabels = gitlabRepo.Topics
			}

			repos = append(repos, &Repository{
				Organization: gitlabRepo.Namespace.FullPath,
				Repository:   gitlabRepo.Path,
				URL:          url,
				Branch:       gitlabRepo.DefaultBranch,
				Labels:       repoLabels,
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
		gitlabBranch, resp, err := g.client.Branches.GetBranch(repo.RepositoryId, repo.Branch, nil)
		// 404s are not an error here, just a normal false.
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return []gitlab.Branch{}, nil
		}
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
		// 404s are not an error here, just a normal false.
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return []gitlab.Branch{}, nil
		}
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
