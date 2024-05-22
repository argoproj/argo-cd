package scm_provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"

	"code.gitea.io/sdk/gitea"
)

type GiteaProvider struct {
	client      *gitea.Client
	owner       string
	allBranches bool
}

var _ SCMProviderService = &GiteaProvider{}

func NewGiteaProvider(ctx context.Context, owner, token, url string, allBranches, insecure bool) (*GiteaProvider, error) {
	if token == "" {
		token = os.Getenv("GITEA_TOKEN")
	}
	httpClient := &http.Client{}
	if insecure {
		cookieJar, _ := cookiejar.New(nil)

		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		httpClient = &http.Client{
			Jar:       cookieJar,
			Transport: tr,
		}
	}
	client, err := gitea.NewClient(url, gitea.SetToken(token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error creating a new gitea client: %w", err)
	}
	return &GiteaProvider{
		client:      client,
		owner:       owner,
		allBranches: allBranches,
	}, nil
}

func (g *GiteaProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	if !g.allBranches {
		branch, status, err := g.client.GetRepoBranch(g.owner, repo.Repository, repo.Branch)
		if status.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("got 404 while getting default branch %q for repo %q - check your repo config: %w", repo.Branch, repo.Repository, err)
		}
		if err != nil {
			return nil, err
		}
		return []*Repository{
			{
				Organization: repo.Organization,
				Repository:   repo.Repository,
				Branch:       repo.Branch,
				URL:          repo.URL,
				SHA:          branch.Commit.ID,
				Labels:       repo.Labels,
				RepositoryId: repo.RepositoryId,
			},
		}, nil
	}
	repos := []*Repository{}
	opts := gitea.ListRepoBranchesOptions{}
	branches, _, err := g.client.ListRepoBranches(g.owner, repo.Repository, opts)
	if err != nil {
		return nil, err
	}
	for _, branch := range branches {
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			Branch:       branch.Name,
			URL:          repo.URL,
			SHA:          branch.Commit.ID,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *GiteaProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	repos := []*Repository{}
	repoOpts := gitea.ListOrgReposOptions{}
	giteaRepos, _, err := g.client.ListOrgRepos(g.owner, repoOpts)
	if err != nil {
		return nil, err
	}
	for _, repo := range giteaRepos {
		var url string
		switch cloneProtocol {
		// Default to SSH if unspecified (i.e. if "").
		case "", "ssh":
			url = repo.SSHURL
		case "https":
			url = repo.HTMLURL
		default:
			return nil, fmt.Errorf("unknown clone protocol for GitHub %v", cloneProtocol)
		}
		labelOpts := gitea.ListLabelsOptions{}
		giteaLabels, _, err := g.client.ListRepoLabels(g.owner, repo.Name, labelOpts)
		if err != nil {
			return nil, err
		}
		labels := []string{}
		for _, label := range giteaLabels {
			labels = append(labels, label.Name)
		}
		repos = append(repos, &Repository{
			Organization: g.owner,
			Repository:   repo.Name,
			Branch:       repo.DefaultBranch,
			URL:          url,
			Labels:       labels,
			RepositoryId: int(repo.ID),
		})
	}
	return repos, nil
}

func (g *GiteaProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	_, resp, err := g.client.GetContents(repo.Organization, repo.Repository, repo.Branch, path)
	if resp != nil && resp.StatusCode == 404 {
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
