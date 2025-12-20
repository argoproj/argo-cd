package pull_request

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/v69/github"

	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
)

type GithubService struct {
	client         *github.Client
	owner          string
	repo           string
	labels         []string
	excludedLabels []string
}

var _ PullRequestService = (*GithubService)(nil)

func NewGithubService(token, url, owner, repo string, labels []string, excludedLabels []string, optionalHTTPClient ...*http.Client) (PullRequestService, error) {
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	var client *github.Client
	httpClient := appsetutils.GetOptionalHTTPClient(optionalHTTPClient...)

	if url == "" {
		if token == "" {
			client = github.NewClient(httpClient)
		} else {
			client = github.NewClient(httpClient).WithAuthToken(token)
		}
	} else {
		var err error
		if token == "" {
			client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		} else {
			client, err = github.NewClient(httpClient).WithAuthToken(token).WithEnterpriseURLs(url, url)
		}
		if err != nil {
			return nil, err
		}
	}
	return &GithubService{
		client:         client,
		owner:          owner,
		repo:           repo,
		labels:         labels,
		excludedLabels: excludedLabels,
	}, nil
}

func (g *GithubService) List(ctx context.Context) ([]*PullRequest, error) {
	opts := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	pullRequests := []*PullRequest{}
	for {
		pulls, resp, err := g.client.PullRequests.List(ctx, g.owner, g.repo, opts)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				// return a custom error indicating that the repository is not found,
				// but also returning the empty result since the decision to continue or not in this case is made by the caller
				return pullRequests, NewRepositoryNotFoundError(err)
			}
			return nil, fmt.Errorf("error listing pull requests for %s/%s: %w", g.owner, g.repo, err)
		}
		for _, pull := range pulls {
			if !containLabels(g.labels, pull.Labels) {
				continue
			}
			if containsAnyLabel(g.excludedLabels, pull.Labels) {
				continue
			}
			pullRequests = append(pullRequests, &PullRequest{
				Number:       int64(*pull.Number),
				Title:        *pull.Title,
				Branch:       *pull.Head.Ref,
				TargetBranch: *pull.Base.Ref,
				HeadSHA:      *pull.Head.SHA,
				Labels:       getGithubPRLabelNames(pull.Labels),
				Author:       *pull.User.Login,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return pullRequests, nil
}

// containLabels returns true if gotLabels contains all expectedLabels
func containLabels(expectedLabels []string, gotLabels []*github.Label) bool {
	for _, expected := range expectedLabels {
		found := false
		for _, got := range gotLabels {
			if got.Name == nil {
				continue
			}
			if expected == *got.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// containsAnyLabel returns true if gotLabels contains any of the excludedLabels
func containsAnyLabel(excludedLabels []string, gotLabels []*github.Label) bool {
	for _, excluded := range excludedLabels {
		for _, got := range gotLabels {
			if got.Name != nil && excluded == *got.Name {
				return true
			}
		}
	}
	return false
}

// Get the Github pull request label names.
func getGithubPRLabelNames(gitHubLabels []*github.Label) []string {
	var labelNames []string
	for _, gitHubLabel := range gitHubLabels {
		labelNames = append(labelNames, *gitHubLabel.Name)
	}
	return labelNames
}
