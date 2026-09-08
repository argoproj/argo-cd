package pull_request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ktrysmt/go-bitbucket"
	log "github.com/sirupsen/logrus"
)

type BitbucketCloudService struct {
	client         *bitbucket.Client
	owner          string
	repositorySlug string
	hints          *PRHintStore
}

type BitbucketCloudPullRequest struct {
	ID          int                                  `json:"id"`
	Title       string                               `json:"title"`
	Source      BitbucketCloudPullRequestSource      `json:"source"`
	Author      BitbucketCloudPullRequestAuthor      `json:"author"`
	Destination BitbucketCloudPullRequestDestination `json:"destination"`
}

type BitbucketCloudPullRequestDestination struct {
	Branch BitbucketCloudPullRequestDestinationBranch `json:"branch"`
}

type BitbucketCloudPullRequestDestinationBranch struct {
	Name string `json:"name"`
}

type BitbucketCloudPullRequestSource struct {
	Branch BitbucketCloudPullRequestSourceBranch `json:"branch"`
	Commit BitbucketCloudPullRequestSourceCommit `json:"commit"`
}

type BitbucketCloudPullRequestSourceBranch struct {
	Name string `json:"name"`
}

type BitbucketCloudPullRequestSourceCommit struct {
	Hash string `json:"hash"`
}

// Also have display_name and uuid, but don't plan to use them.
type BitbucketCloudPullRequestAuthor struct {
	Nickname string `json:"nickname"`
}

type PullRequestResponse struct {
	Page     int32         `json:"page"`
	Size     int32         `json:"size"`
	Pagelen  int32         `json:"pagelen"`
	Next     string        `json:"next"`
	Previous string        `json:"previous"`
	Items    []PullRequest `json:"values"`
}

var _ PullRequestService = (*BitbucketCloudService)(nil)

func parseURL(uri string) (*url.URL, error) {
	if uri == "" {
		uri = "https://api.bitbucket.org/2.0"
	}

	url, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func NewBitbucketCloudServiceBasicAuth(baseURL, username, password, owner, repositorySlug string, hints *PRHintStore) (PullRequestService, error) {
	url, err := parseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url of %s for %s/%s: %w", baseURL, owner, repositorySlug, err)
	}

	bitbucketClient, err := bitbucket.NewBasicAuth(username, password)
	if err != nil {
		return nil, fmt.Errorf("error creating BitBucket Cloud client with basic auth: %w", err)
	}
	bitbucketClient.SetApiBaseURL(*url)

	return &BitbucketCloudService{
		client:         bitbucketClient,
		owner:          owner,
		repositorySlug: repositorySlug,
		hints:          hints,
	}, nil
}

func NewBitbucketCloudServiceBearerToken(baseURL, bearerToken, owner, repositorySlug string, hints *PRHintStore) (PullRequestService, error) {
	url, err := parseURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url of %s for %s/%s: %w", baseURL, owner, repositorySlug, err)
	}

	bitbucketClient, err := bitbucket.NewOAuthbearerToken(bearerToken)
	if err != nil {
		return nil, fmt.Errorf("error creating BitBucket Cloud client with oauth bearer token: %w", err)
	}
	bitbucketClient.SetApiBaseURL(*url)

	return &BitbucketCloudService{client: bitbucketClient, owner: owner, repositorySlug: repositorySlug, hints: hints}, nil
}

func NewBitbucketCloudServiceNoAuth(baseURL, owner, repositorySlug string, hints *PRHintStore) (PullRequestService, error) {
	// There is currently no method to explicitly not require auth
	return NewBitbucketCloudServiceBearerToken(baseURL, "", owner, repositorySlug, hints)
}

// listBranchNames returns live branch names; refs are strongly consistent unlike commit objects.
func (b *BitbucketCloudService) listBranchNames() (map[string]struct{}, error) {
	names := map[string]struct{}{}
	for page := 1; ; page++ {
		resp, err := b.client.Repositories.Repository.ListBranches(&bitbucket.RepositoryBranchOptions{
			Owner:    b.owner,
			RepoSlug: b.repositorySlug,
			Pagelen:  100,
			PageNum:  page,
		})
		if err != nil {
			return nil, err
		}
		for _, br := range resp.Branches {
			names[br.Name] = struct{}{}
		}
		if resp.Next == "" {
			break
		}
	}
	return names, nil
}

func (b *BitbucketCloudService) List(_ context.Context) ([]*PullRequest, error) {
	// Drain hints before the API call so they are consumed regardless of API outcome.
	var hinted []*PullRequest
	if b.hints != nil {
		hinted = b.hints.Take(b.owner, b.repositorySlug)
	}

	// Fetch live branches once; PRs whose source branch is gone are skipped.
	// Fail-open on listing error: a transient API failure should not block previews.
	branches, err := b.listBranchNames()
	if err != nil {
		log.WithError(err).Warnf("could not list branches for %s/%s; skipping deleted-branch filter", b.owner, b.repositorySlug)
		branches = nil
	}

	opts := &bitbucket.PullRequestsOptions{
		Owner:    b.owner,
		RepoSlug: b.repositorySlug,
	}

	pullRequests := []*PullRequest{}

	response, err := b.client.Repositories.PullRequests.Gets(opts)
	if err != nil {
		// A standard Http 404 error is not returned for Bitbucket Cloud,
		// so checking the error message for a specific pattern
		if strings.Contains(err.Error(), "404 Not Found") {
			// return a custom error indicating that the repository is not found,
			// but also return the empty result since the decision to continue or not in this case is made by the caller
			return pullRequests, NewRepositoryNotFoundError(err)
		}
		return nil, fmt.Errorf("error listing pull requests for %s/%s: %w", b.owner, b.repositorySlug, err)
	}

	resp, ok := response.(map[string]any)
	if !ok {
		return nil, errors.New("unknown type returned from bitbucket pull requests")
	}

	repoArray, ok := resp["values"].([]any)
	if !ok {
		return nil, errors.New("unknown type returned from response values")
	}

	jsonStr, err := json.Marshal(repoArray)
	if err != nil {
		return nil, fmt.Errorf("error marshalling response body to json: %w", err)
	}

	var pulls []BitbucketCloudPullRequest
	if err := json.Unmarshal(jsonStr, &pulls); err != nil {
		return nil, fmt.Errorf("error unmarshalling json to type '[]BitbucketCloudPullRequest': %w", err)
	}

	for _, pull := range pulls {
		if branches != nil {
			if _, ok := branches[pull.Source.Branch.Name]; !ok {
				log.WithFields(log.Fields{
					"owner":  b.owner,
					"repo":   b.repositorySlug,
					"pr":     pull.ID,
					"branch": pull.Source.Branch.Name,
				}).Warn("skipping PR: source branch deleted")
				continue
			}
		}
		pullRequests = append(pullRequests, &PullRequest{
			Number:       int64(pull.ID),
			Title:        pull.Title,
			Branch:       pull.Source.Branch.Name,
			TargetBranch: pull.Destination.Branch.Name,
			HeadSHA:      pull.Source.Commit.Hash,
			Author:       pull.Author.Nickname,
		})
	}

	// Merge hinted PRs not yet visible in the API (eventual-consistency lag).
	// Apply the same branch filter so a stale hint for a deleted branch is dropped.
	if len(hinted) > 0 {
		seen := make(map[int64]struct{}, len(pullRequests))
		for _, pr := range pullRequests {
			seen[pr.Number] = struct{}{}
		}
		for _, pr := range hinted {
			if _, exists := seen[pr.Number]; exists {
				continue
			}
			if branches != nil {
				if _, ok := branches[pr.Branch]; !ok {
					continue
				}
			}
			pullRequests = append(pullRequests, pr)
		}
	}

	return pullRequests, nil
}
