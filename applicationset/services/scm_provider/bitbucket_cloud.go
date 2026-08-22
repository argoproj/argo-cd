package scm_provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	bitbucket "github.com/ktrysmt/go-bitbucket"
)

// bitbucketCloudMaxPageLen is the largest page length the Bitbucket Cloud API documents for
// paginated collections: "The default value is 10 with 100 being the maximum allowed value."
// (https://developer.atlassian.com/cloud/bitbucket/rest/intro/#pagination). Requesting the maximum
// only reduces the number of round trips -- correctness comes from following the `next` link, so a
// smaller page size enforced by the API would still return every branch.
const bitbucketCloudMaxPageLen = 100

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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, urlStr, body)
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

// bitbucketCloudBranchPage is the part of a paginated `refs/branches` response that pagination
// needs. go-bitbucket cannot request a page by URL -- RepositoryBranchOptions only exposes an
// integer page number -- so pages after the first are fetched and decoded here instead.
// bitbucket.RepositoryBranch carries no JSON tags, but its field names match the API's snake_case
// keys under encoding/json's case-insensitive matching.
type bitbucketCloudBranchPage struct {
	Next   string                       `json:"next"`
	Values []bitbucket.RepositoryBranch `json:"values"`
}

// getBranchPage fetches one page of a paginated `refs/branches` response by following the absolute
// URL Bitbucket returned in `next`, rather than by reconstructing it from a page number.
//
// The URL must point at the configured API host. The request carries the provider's credentials,
// so following an arbitrary host named in a response body would hand them to that host.
func (c *ExtendedClient) getBranchPage(ctx context.Context, pageURL string) (*bitbucketCloudBranchPage, error) {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing next page url %q: %w", pageURL, err)
	}
	if apiHost := c.GetApiHostnameURL(); parsed.Scheme+"://"+parsed.Host != apiHost {
		return nil, fmt.Errorf("refusing to follow next page url %q: expected the Bitbucket API host %s", pageURL, apiHost)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	// Only send credentials when both halves are set, matching go-bitbucket's own
	// authenticateRequest so that anonymous access to a public repository keeps working.
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error fetching next page %q: %s", pageURL, resp.Status)
	}

	page := &bitbucketCloudBranchPage{}
	if err := json.NewDecoder(resp.Body).Decode(page); err != nil {
		return nil, fmt.Errorf("error decoding next page %q: %w", pageURL, err)
	}
	return page, nil
}

var _ SCMProviderService = &BitBucketCloudProvider{}

func NewBitBucketCloudProvider(owner string, user string, password string, allBranches bool) (*BitBucketCloudProvider, error) {
	bitbucketClient, err := bitbucket.NewBasicAuth(user, password)
	if err != nil {
		return nil, fmt.Errorf("error creating BitBucket Cloud client with basic auth: %w", err)
	}
	client := &ExtendedClient{
		bitbucketClient,
		user,
		password,
		owner,
	}
	return &BitBucketCloudProvider{client: client, owner: owner, allBranches: allBranches}, nil
}

func (g *BitBucketCloudProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := []*Repository{}
	branches, err := g.listBranches(ctx, repo)
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
		Role:  "member",
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
	contents, err := g.client.GetContents(repo, path)
	if err != nil {
		return false, err
	}
	if contents {
		return true, nil
	}
	return false, nil
}

func (g *BitBucketCloudProvider) listBranches(ctx context.Context, repo *Repository) ([]bitbucket.RepositoryBranch, error) {
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

	// ListBranches only returns the first page. Without walking the rest, only the API's default
	// page length is returned and the generator silently produces Applications for a subset of
	// the branches.
	response, err := g.client.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{
		Owner:    g.owner,
		RepoSlug: repo.Repository,
		Pagelen:  bitbucketCloudMaxPageLen,
	})
	if err != nil {
		return nil, err
	}

	// Follow the `next` link verbatim instead of incrementing a page number. Bitbucket Cloud
	// documents two pagination styles: list-based collections expose a numeric `page`, while
	// iterator-based ones put an unpredictable token in `next`. Following `next` is correct for
	// both, and go-bitbucket cannot express a token page anyway (PageNum is an int).
	// https://developer.atlassian.com/cloud/bitbucket/rest/intro/#pagination
	branches := response.Branches
	visited := map[string]bool{}
	for next := response.Next; next != ""; {
		// A `next` we have already followed cannot make progress, and would otherwise spin
		// against the API forever.
		if visited[next] {
			return nil, fmt.Errorf("bitbucket returned a repeated next page url %q", next)
		}
		visited[next] = true

		page, err := g.client.getBranchPage(ctx, next)
		if err != nil {
			return nil, err
		}
		branches = append(branches, page.Values...)
		next = page.Next
	}
	return branches, nil
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
