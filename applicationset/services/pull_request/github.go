package pull_request

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/shurcooL/githubv4"
)

type GithubService struct {
	client *githubv4.Client
	owner  string
	repo   string
	labels []string
}

var _ PullRequestService = (*GithubService)(nil)

type githubLabel struct {
	Name githubv4.String
}

func NewGithubService(token, url, owner, repo string, labels []string) (PullRequestService, error) {
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	httpClient := &http.Client{}
	var client *githubv4.Client
	if url == "" {
		httpClient.Transport = roundTripperFunc(
			func(req *http.Request) (*http.Response, error) {
				req = req.Clone(req.Context())
				req.Header.Set("Authorization", "Bearer "+token)
				return http.DefaultTransport.RoundTrip(req)
			},
		)
		client = githubv4.NewClient(httpClient)
	} else {
		client = githubv4.NewEnterpriseClient(url, httpClient)
	}
	return &GithubService{
		client: client,
		owner:  owner,
		repo:   repo,
		labels: labels,
	}, nil
}

func (g *GithubService) List(ctx context.Context) ([]*PullRequest, error) {
	var query struct {
		Search struct {
			Nodes []struct {
				PullRequest struct {
					Number      githubv4.Int
					Title       githubv4.String
					HeadRefName githubv4.String
					BaseRefName githubv4.String
					HeadRefOid  githubv4.String
					Labels      struct {
						Nodes []githubLabel
					} `graphql:"labels(first: 10)"`
					Author struct {
						Login githubv4.String
					}
				} `graphql:"... on PullRequest"`
			}
			PageInfo struct {
				EndCursor   githubv4.String
				HasNextPage bool
			}
		} `graphql:"search(query: $query, type: ISSUE, first: 100, after: $after)"`
	}

	queryString := fmt.Sprintf("repo:%s/%s is:pr is:open", g.owner, g.repo)
	for _, label := range g.labels {
		queryString += fmt.Sprintf(" label:\"%s\"", label)
	}

	variables := map[string]any{
		"query": githubv4.String(queryString),
		"after": (*githubv4.String)(nil),
	}

	pullRequests := []*PullRequest{}
	for {
		err := g.client.Query(ctx, &query, variables)
		if err != nil {
			return nil, fmt.Errorf("error listing pull requests for %s/%s: %w", g.owner, g.repo, err)
		}

		for _, node := range query.Search.Nodes {
			pull := node.PullRequest
			pullRequests = append(pullRequests, &PullRequest{
				Number:       int(pull.Number),
				Title:        string(pull.Title),
				Branch:       string(pull.HeadRefName),
				TargetBranch: string(pull.BaseRefName),
				HeadSHA:      string(pull.HeadRefOid),
				Labels:       getGithubPRLabelNames(pull.Labels.Nodes),
				Author:       string(pull.Author.Login),
			})
		}

		if !query.Search.PageInfo.HasNextPage {
			break
		}
		variables["after"] = githubv4.String(query.Search.PageInfo.EndCursor)
	}

	return pullRequests, nil
}

// Get the Github pull request label names.
func getGithubPRLabelNames(gitHubLabels []githubLabel) []string {
	var labelNames []string
	for _, gitHubLabel := range gitHubLabels {
		labelNames = append(labelNames, string(gitHubLabel.Name))
	}
	return labelNames
}

// roundTripperFunc creates a RoundTripper (transport).
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
