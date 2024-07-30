package pull_request

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/ktrysmt/go-bitbucket"
)

type BitbucketCloudService struct {
	client         *bitbucket.Client
	owner          string
	repositorySlug string
}

type BitbucketCloudPullRequest struct {
	ID     int                             `json:"id"`
	Title  string                          `json:"title"`
	Source BitbucketCloudPullRequestSource `json:"source"`
	Author string                          `json:"author"`
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

type PullRequestResponse struct {
	Page     int32         `json:"page"`
	Size     int32         `json:"size"`
	Pagelen  int32         `json:"pagelen"`
	Next     string        `json:"next"`
	Previous string        `json:"previous"`
	Items    []PullRequest `json:"values"`
}

var _ PullRequestService = (*BitbucketCloudService)(nil)

func parseUrl(uri string) (*url.URL, error) {
	if uri == "" {
		uri = "https://api.bitbucket.org/2.0"
	}

	url, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func NewBitbucketCloudServiceBasicAuth(baseUrl, username, password, owner, repositorySlug string) (PullRequestService, error) {
	url, err := parseUrl(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url of %s for %s/%s: %w", baseUrl, owner, repositorySlug, err)
	}

	bitbucketClient := bitbucket.NewBasicAuth(username, password)
	bitbucketClient.SetApiBaseURL(*url)

	return &BitbucketCloudService{
		client:         bitbucketClient,
		owner:          owner,
		repositorySlug: repositorySlug,
	}, nil
}

func NewBitbucketCloudServiceBearerToken(baseUrl, bearerToken, owner, repositorySlug string) (PullRequestService, error) {
	url, err := parseUrl(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url of %s for %s/%s: %w", baseUrl, owner, repositorySlug, err)
	}

	bitbucketClient := bitbucket.NewOAuthbearerToken(bearerToken)
	bitbucketClient.SetApiBaseURL(*url)

	return &BitbucketCloudService{
		client:         bitbucketClient,
		owner:          owner,
		repositorySlug: repositorySlug,
	}, nil
}

func NewBitbucketCloudServiceNoAuth(baseUrl, owner, repositorySlug string) (PullRequestService, error) {
	// There is currently no method to explicitly not require auth
	return NewBitbucketCloudServiceBearerToken(baseUrl, "", owner, repositorySlug)
}

func (b *BitbucketCloudService) List(_ context.Context) ([]*PullRequest, error) {
	opts := &bitbucket.PullRequestsOptions{
		Owner:    b.owner,
		RepoSlug: b.repositorySlug,
	}

	response, err := b.client.Repositories.PullRequests.Gets(opts)
	if err != nil {
		return nil, fmt.Errorf("error listing pull requests for %s/%s: %w", b.owner, b.repositorySlug, err)
	}

	resp, ok := response.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unknown type returned from bitbucket pull requests")
	}

	repoArray, ok := resp["values"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unknown type returned from response values")
	}

	jsonStr, err := json.Marshal(repoArray)
	if err != nil {
		return nil, fmt.Errorf("error marshalling response body to json: %w", err)
	}

	var pulls []BitbucketCloudPullRequest
	if err := json.Unmarshal(jsonStr, &pulls); err != nil {
		return nil, fmt.Errorf("error unmarshalling json to type '[]BitbucketCloudPullRequest': %w", err)
	}

	pullRequests := []*PullRequest{}
	for _, pull := range pulls {
		pullRequests = append(pullRequests, &PullRequest{
			Number:  pull.ID,
			Title:   pull.Title,
			Branch:  pull.Source.Branch.Name,
			HeadSHA: pull.Source.Commit.Hash,
			Author:  pull.Author,
		})
	}

	return pullRequests, nil
}
