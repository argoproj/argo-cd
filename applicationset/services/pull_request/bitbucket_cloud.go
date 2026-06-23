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

func (b *BitbucketCloudService) List(_ context.Context) ([]*PullRequest, error) {
	// Consume any PR injected directly from the webhook payload (bypasses the
	// eventually-consistent Bitbucket Cloud list API on webhook-triggered reconciles).
	if b.hints != nil {
		if hinted := b.hints.Take(b.owner, b.repositorySlug); len(hinted) > 0 {
			return hinted, nil
		}
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
		// Bitbucket has no persistent PR refs; a deleted source branch makes the SHA unreachable.
		if !b.isCommitReachable(pull.Source.Commit.Hash) {
			log.WithFields(log.Fields{
				"owner":   b.owner,
				"repo":    b.repositorySlug,
				"pr":      pull.ID,
				"headSHA": pull.Source.Commit.Hash,
				"branch":  pull.Source.Branch.Name,
			}).Warn("skipping PR: head commit unreachable (source branch likely deleted)")
			continue
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

	return pullRequests, nil
}

// isCommitReachable checks whether the given SHA is accessible in the remote repository.
// Returns false when Bitbucket returns 404 (branch deleted, commit dangling), true otherwise.
// On non-404 errors (network, auth) it returns true so the PR is not silently dropped.
func (b *BitbucketCloudService) isCommitReachable(sha string) bool {
	if sha == "" {
		return false
	}
	_, err := b.client.Repositories.Commits.GetCommit(&bitbucket.CommitsOptions{
		Owner:    b.owner,
		RepoSlug: b.repositorySlug,
		Revision: sha,
	})
	if err == nil {
		return true
	}
	// Only treat 404 as "unreachable"; surface all other errors as reachable so
	// transient failures don't silently drop PRs.
	if strings.Contains(err.Error(), "404 Not Found") {
		return false
	}
	log.WithError(err).WithFields(log.Fields{
		"owner": b.owner, "repo": b.repositorySlug, "sha": sha,
	}).Warn("error checking commit reachability; treating as reachable")
	return true
}
