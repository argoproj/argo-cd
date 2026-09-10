package scm_provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"

	"code.gitea.io/sdk/gitea"

	"github.com/argoproj/argo-cd/v3/util/proxy"
)

type GiteaProvider struct {
	client               *gitea.Client
	owner                string
	allBranches          bool
	excludeArchivedRepos bool
}

var _ SCMProviderService = &GiteaProvider{}

const (
	// giteaPageSize is the number of items requested per Gitea API call. Gitea
	// clamps the page size to its MAX_RESPONSE_ITEMS setting, so a short page
	// does not mean the last page.
	giteaPageSize = 50
	// giteaMaxPages bounds the paging loops so that a server which never reports
	// the end of a list fails loudly instead of looping forever.
	giteaMaxPages = 1000
)

// giteaAllCollected reports whether every item of a list endpoint has been
// collected, based on the X-Total-Count header Gitea sets on its list
// responses. When the header is absent the caller keeps paging until it gets an
// empty page.
func giteaAllCollected(resp *gitea.Response, collected int) bool {
	if resp == nil {
		return false
	}
	total, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
	if err != nil {
		return false
	}
	return collected >= total
}

func NewGiteaProvider(owner, token, url string, allBranches, insecure, excludeArchivedRepos bool, proxyURL, noProxy string) (*GiteaProvider, error) {
	if token == "" {
		token = os.Getenv("GITEA_TOKEN")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	transport.Proxy = proxy.GetCallback(proxyURL, noProxy)

	cookieJar, _ := cookiejar.New(nil)
	httpClient := &http.Client{Jar: cookieJar, Transport: transport}
	client, err := gitea.NewClient(url, gitea.SetToken(token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("error creating a new gitea client: %w", err)
	}
	return &GiteaProvider{
		client:               client,
		owner:                owner,
		allBranches:          allBranches,
		excludeArchivedRepos: excludeArchivedRepos,
	}, nil
}

func (g *GiteaProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	g.client.SetContext(ctx)

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
	firstOfPreviousPage := ""
	for page := 1; ; page++ {
		opts := gitea.ListRepoBranchesOptions{
			Page:     page,
			PageSize: giteaPageSize,
		}
		branches, resp, err := g.client.ListRepoBranches(g.owner, repo.Repository, opts)
		if err != nil {
			return nil, err
		}
		if len(branches) == 0 {
			return repos, nil
		}
		if page > giteaMaxPages {
			return nil, fmt.Errorf("gitea returned more than %d pages of branches for repo %q", giteaMaxPages, repo.Repository)
		}
		if page > 1 && branches[0].Name == firstOfPreviousPage {
			return nil, fmt.Errorf("gitea returned the same branches on pages %d and %d for repo %q, the server is not honouring the page parameter", page-1, page, repo.Repository)
		}
		firstOfPreviousPage = branches[0].Name
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
		if giteaAllCollected(resp, len(repos)) {
			return repos, nil
		}
	}
}

func (g *GiteaProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	g.client.SetContext(ctx)

	repos := []*Repository{}
	fetched := 0
	firstOfPreviousPage := int64(0)
	for page := 1; ; page++ {
		repoOpts := gitea.ListOrgReposOptions{
			Page:     page,
			PageSize: giteaPageSize,
		}
		giteaRepos, resp, err := g.client.ListOrgRepos(g.owner, repoOpts)
		if err != nil {
			return nil, err
		}
		if len(giteaRepos) == 0 {
			return repos, nil
		}
		if page > giteaMaxPages {
			return nil, fmt.Errorf("gitea returned more than %d pages of repositories for org %q", giteaMaxPages, g.owner)
		}
		if page > 1 && giteaRepos[0].ID == firstOfPreviousPage {
			return nil, fmt.Errorf("gitea returned the same repositories on pages %d and %d for org %q, the server is not honouring the page parameter", page-1, page, g.owner)
		}
		firstOfPreviousPage = giteaRepos[0].ID
		fetched += len(giteaRepos)
		for _, repo := range giteaRepos {
			if g.excludeArchivedRepos && repo.Archived {
				continue
			}

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
			labels, err := g.listRepoLabels(repo.Name)
			if err != nil {
				return nil, fmt.Errorf("error listing labels for repo %q: %w", repo.Name, err)
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
		if giteaAllCollected(resp, fetched) {
			return repos, nil
		}
	}
}

// listRepoLabels returns every label defined on the given repository. The
// caller is responsible for setting the client context.
func (g *GiteaProvider) listRepoLabels(repo string) ([]string, error) {
	labels := []string{}
	firstOfPreviousPage := int64(0)
	for page := 1; ; page++ {
		labelOpts := gitea.ListLabelsOptions{
			Page:     page,
			PageSize: giteaPageSize,
		}
		giteaLabels, resp, err := g.client.ListRepoLabels(g.owner, repo, labelOpts)
		if err != nil {
			return nil, err
		}
		if len(giteaLabels) == 0 {
			return labels, nil
		}
		if page > giteaMaxPages {
			return nil, fmt.Errorf("gitea returned more than %d pages of labels for repo %q", giteaMaxPages, repo)
		}
		if page > 1 && giteaLabels[0].ID == firstOfPreviousPage {
			return nil, fmt.Errorf("gitea returned the same labels on pages %d and %d for repo %q, the server is not honouring the page parameter", page-1, page, repo)
		}
		firstOfPreviousPage = giteaLabels[0].ID
		for _, label := range giteaLabels {
			labels = append(labels, label.Name)
		}
		if giteaAllCollected(resp, len(labels)) {
			return labels, nil
		}
	}
}

func (g *GiteaProvider) RepoHasPath(_ context.Context, repo *Repository, path string) (bool, error) {
	_, resp, err := g.client.GetContents(repo.Organization, repo.Repository, repo.Branch, path)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		if err.Error() == "expect file, got directory" {
			return true, nil
		}
		return false, err
	}
	return true, nil
}
