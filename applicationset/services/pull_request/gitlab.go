package pull_request

import (
	"context"
	"fmt"
	"os"

	gitlab "github.com/xanzy/go-gitlab"
)

type GitLabService struct {
	client           *gitlab.Client
	project          string
	group            string
	labels           []string
	pullRequestState string
}

var _ PullRequestService = (*GitLabService)(nil)

func NewGitLabService(ctx context.Context, token, url, project string, labels []string, pullRequestState string, group string) (PullRequestService, error) {
	var clientOptionFns []gitlab.ClientOptionFunc

	// Set a custom Gitlab base URL if one is provided
	if url != "" {
		clientOptionFns = append(clientOptionFns, gitlab.WithBaseURL(url))
	}

	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}

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
	if g.group != "" {
		return ListGroupMRs(g)
	} else {
		return ListProjectMRs(g)
	}
}

func ListProjectMRs(g *GitLabService) ([]*PullRequest, error) {

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
	
	projOpts := &gitlab.GetProjectOptions{}

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
			
			proj, _, err := g.client.Projects.GetProject(mr.SourceProjectID, projOpts)
			if err != nil {
				return nil, fmt.Errorf("error getting project name for project id '%d': %v", mr.SourceProjectID, err)
			}
			
			pullRequests = append(pullRequests, &PullRequest{
				Number:  mr.IID,
				Branch:  mr.SourceBranch,
				HeadSHA: mr.SHA,
				Url:     proj.HTTPURLToRepo,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return pullRequests, nil
}

func ListGroupMRs(g *GitLabService) ([]*PullRequest, error) {

	// Filter the merge requests on labels, if they are specified.
	var labels *gitlab.Labels
	if len(g.labels) > 0 {
		labels = (*gitlab.Labels)(&g.labels)
	}

	opts := &gitlab.ListGroupMergeRequestsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
		Labels: labels,
	}
	
	projOpts := &gitlab.GetProjectOptions{}

	if g.pullRequestState != "" {
		opts.State = &g.pullRequestState
	}

	pullRequests := []*PullRequest{}
	for {
		mrs, resp, err := g.client.MergeRequests.ListGroupMergeRequests(g.group, opts)
		if err != nil {
			return nil, fmt.Errorf("error listing merge requests for group '%s': %v", g.group, err)
		}
		for _, mr := range mrs {
			
			proj, _, err := g.client.Projects.GetProject(mr.SourceProjectID, projOpts)
			if err != nil {
				return nil, fmt.Errorf("error getting project name for project id '%d': %v", mr.SourceProjectID, err)
			}
			
			pullRequests = append(pullRequests, &PullRequest{
				Number:  mr.IID,
				Branch:  mr.SourceBranch,
				HeadSHA: mr.SHA,
				Url:     proj.HTTPURLToRepo,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return pullRequests, nil
}
