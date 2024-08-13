package pull_request

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v63/github"
	"golang.org/x/oauth2"
)

type GithubService struct {
	client *github.Client
	owner  string
	repo   string
	labels []string
}

var _ PullRequestService = (*GithubService)(nil)

func NewGithubService(ctx context.Context, token, url, owner, repo string, labels []string) (PullRequestService, error) {
	var ts oauth2.TokenSource
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token != "" {
		ts = oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	}
	httpClient := oauth2.NewClient(ctx, ts)
	var client *github.Client
	if url == "" {
		client = github.NewClient(httpClient)
	} else {
		var err error
		client, err = github.NewClient(httpClient).WithEnterpriseURLs(url, url)
		if err != nil {
			return nil, err
		}
	}
	return &GithubService{
		client: client,
		owner:  owner,
		repo:   repo,
		labels: labels,
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
			return nil, fmt.Errorf("error listing pull requests for %s/%s: %w", g.owner, g.repo, err)
		}
		for _, pull := range pulls {
			if !containLabels(g.labels, pull.Labels) {
				continue
			}
			pullRequests = append(pullRequests, &PullRequest{
				Number:       *pull.Number,
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

// containLabels returns true if gotLabels contains expectedLabels
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

// Get the Github pull request label names.
func getGithubPRLabelNames(gitHubLabels []*github.Label) []string {
	var labelNames []string
	for _, gitHubLabel := range gitHubLabels {
		labelNames = append(labelNames, *gitHubLabel.Name)
	}
	return labelNames
}
