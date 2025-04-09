package pull_request

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	appsetutils "github.com/argoproj/argo-cd/v3/applicationset/utils"
)

type GithubService struct {
	v4Client *githubv4.Client
	owner    string
	repo     string
	labels   []string
}

var _ PullRequestService = (*GithubService)(nil)

type githubLabel struct {
	Name githubv4.String
}

func NewGithubService(token, url, owner, repo string, labels []string, optionalHTTPClient ...*http.Client) (PullRequestService, error) {
	// Undocumented environment variable to set a default token, to be used in testing to dodge anonymous rate limits.
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	var v4Client *githubv4.Client
	httpClient := appsetutils.GetOptionalHTTPClient(optionalHTTPClient...)

	if token != "" {
		httpClient.Transport = &oauth2.Transport{
			Source: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
			Base:   httpClient.Transport,
		}
	}

	if url == "" {
		v4Client = githubv4.NewClient(httpClient)
	} else {
		url = strings.TrimSuffix(url, "/")
		v4Client = githubv4.NewEnterpriseClient(url+"/graphql", httpClient)
	}
	return &GithubService{
		v4Client: v4Client,
		owner:    owner,
		repo:     repo,
		labels:   labels,
	}, nil
}

func (g *GithubService) List(ctx context.Context) ([]*PullRequest, error) {
	var query struct {
		Repository *struct {
			ID githubv4.ID
		} `graphql:"repository(owner: $owner, name: $repo)"`
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
					} `graphql:"labels(first: 100)"`
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
		queryString += fmt.Sprintf(" label:%q", label)
	}

	variables := map[string]any{
		"owner": githubv4.String(g.owner),
		"repo":  githubv4.String(g.repo),
		"query": githubv4.String(queryString),
		"after": (*githubv4.String)(nil),
	}

	pullRequests := []*PullRequest{}
	for {
		err := g.v4Client.Query(ctx, &query, variables)
		if err != nil {
			if strings.Contains(err.Error(), "Could not resolve to a Repository") {
				// return a custom error indicating that the repository is not found,
				// but also returning the empty result since the decision to continue or not in this case is made by the caller
				return pullRequests, NewRepositoryNotFoundError(err)
			}
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
