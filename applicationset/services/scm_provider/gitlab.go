package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"os"

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
