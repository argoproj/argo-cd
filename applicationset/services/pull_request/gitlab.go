package pull_request

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	"github.com/hashicorp/go-retryablehttp"
	gitlab "github.com/xanzy/go-gitlab"
)

type GitLabService struct {
	client           *gitlab.Client
	project          string
	labels           []string
	pullRequestState string
}

var _ PullRequestService = (*GitLabService)(nil)

func NewGitLabService(ctx context.Context, token, url, project string, labels []string, pullRequestState string, scmRootCAPath string, insecure bool) (PullRequestService, error) {
	var clientOptionFns []gitlab.ClientOptionFunc

	// Set a custom Gitlab base URL if one is provided
	if url != "" {
		clientOptionFns = append(clientOptionFns, gitlab.WithBaseURL(url))
	}

	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = utils.GetTlsConfig(scmRootCAPath, insecure)

	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient.Transport = tr

	clientOptionFns = append(clientOptionFns, gitlab.WithHTTPClient(retryClient.HTTPClient))

	client, err := gitlab.NewClient(token, clientOptionFns...)
	if err != nil {
		return nil, fmt.Errorf("error creating Gitlab client: %v", err)
	}

	return &GitLabService{
		client:           client,
		project:          project,
		labels:           labels,
		pullRequestState: pullRequestState,
	}, nil
}

func (g *GitLabService) List(ctx context.Context) ([]*PullRequest, error) {

	// Filter the merge requests on labels, if they are specified.
	var labels *gitlab.Labels
	if len(g.labels) > 0 {
		labels = (*gitlab.Labels)(&g.labels)
	}

	opts := &gitlab.ListProjectMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Labels: labels,
	}

	if g.pullRequestState != "" {
		opts.State = &g.pullRequestState
	}

	pullRequests := []*PullRequest{}
	for {
		mrs, resp, err := g.client.MergeRequests.ListProjectMergeRequests(g.project, opts)
		if err != nil {
			return nil, fmt.Errorf("error listing merge requests for project '%s': %v", g.project, err)
		}
		for _, mr := range mrs {
			pullRequests = append(pullRequests, &PullRequest{
				Number:       mr.IID,
				Branch:       mr.SourceBranch,
				TargetBranch: mr.TargetBranch,
				HeadSHA:      mr.SHA,
				Labels:       mr.Labels,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return pullRequests, nil
}
