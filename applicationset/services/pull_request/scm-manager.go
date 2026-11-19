package pull_request

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	scmm "github.com/scm-manager/goscm"

	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/scmmanager"
)

type ScmManagerService struct {
	client    *scmmanager.Client
	namespace string
	name      string
}

var _ PullRequestService = (*ScmManagerService)(nil)

func NewScmManagerService(_ context.Context, token, apiURL, namespace, name string, insecure bool, scmRootCAPath string, caCerts []byte) (PullRequestService, error) {
	return &ScmManagerService{
		client:    scmmanager.NewClient(apiURL, token, insecure, scmRootCAPath, caCerts),
		namespace: namespace,
		name:      name,
	}, nil
}

type scmManagerPullRequest struct {
	scmm.PullRequest
	Author struct {
		ID string `json:"id"`
	} `json:"author"`
}

type scmManagerPullRequestPage struct {
	Page      int `json:"page"`
	PageTotal int `json:"pageTotal"`
	Embedded  struct {
		PullRequests []scmManagerPullRequest `json:"pullRequests"`
	} `json:"_embedded"`
}

func (g *ScmManagerService) List(ctx context.Context) ([]*PullRequest, error) {
	path := "/api/v2/pull-requests/" + url.PathEscape(g.namespace) + "/" + url.PathEscape(g.name)
	prs, err := scmmanager.ListPages(ctx, g.client, path, url.Values{"status": {"OPEN"}, "pageSize": {"10"}}, func(page scmManagerPullRequestPage) ([]scmManagerPullRequest, int, int) {
		return page.Embedded.PullRequests, page.Page, page.PageTotal
	})
	if err != nil {
		return nil, err
	}
	list := make([]*PullRequest, 0, len(prs))
	for _, pr := range prs {
		number, err := strconv.ParseInt(pr.Id, 10, 64)
		if err != nil {
			return nil, err
		}
		path := scmmanager.RepositoryPath(g.namespace, g.name) + "/branches/" + url.PathEscape(pr.Source) + "/changesets"
		var changesets scmm.ChangesetContainer
		if err := g.client.GetJSON(ctx, path, url.Values{"pageSize": {"1"}}, &changesets); err != nil {
			return nil, err
		}
		if len(changesets.Embedded.Changesets) == 0 {
			return nil, fmt.Errorf("SCM-Manager pull request %d has no changesets for branch %q", number, pr.Source)
		}
		list = append(list, &PullRequest{
			Number:       number,
			Branch:       pr.Source,
			TargetBranch: pr.Target,
			Title:        pr.Title,
			Author:       pr.Author.ID,
			HeadSHA:      changesets.Embedded.Changesets[0].Id,
			Labels:       []string{},
		})
	}
	return list, nil
}
