package pull_request

import (
	"context"
	"net/http"
	"os"
	"strconv"

	"github.com/argoproj/argo-cd/v3/util/sourcecraft"
)

type SourceCraftService struct {
	client           *sourcecraft.Client
	organizationSlug string
	repoSlug         string
}

var _ PullRequestService = (*SourceCraftService)(nil)

func NewSourceCraftService(token, url, organizationSlug, repoSlug string, insecure bool) (PullRequestService, error) {
	if token == "" {
		token = os.Getenv("SOURCECRAFT_TOKEN")
	}
	client, err := sourcecraft.NewClient(url, sourcecraft.SetToken(token), sourcecraft.WithHTTPClient(insecure))
	if err != nil {
		return nil, err
	}
	return &SourceCraftService{
		client:           client,
		organizationSlug: organizationSlug,
		repoSlug:         repoSlug,
	}, nil
}

func (g *SourceCraftService) List(ctx context.Context) ([]*PullRequest, error) {
	opts := sourcecraft.ListRepoPullRequestsOptions{}
	list := []*PullRequest{}
	prsResp, status, err := g.client.ListRepoPullRequests(ctx, g.organizationSlug, g.repoSlug, opts)
	if err != nil {
		if status != nil && status.StatusCode == http.StatusNotFound {
			// return a custom error indicating that the repository is not found,
			// but also returning the empty result since the decision to continue or not in this case is made by the caller
			return list, NewRepositoryNotFoundError(err)
		}
		return nil, err
	}

	for _, pr := range prsResp.PullRequests {
		if pr.Status != "open" {
			continue
		}
		n, err := strconv.Atoi(pr.Slug)
		if err != nil {
			return nil, err
		}
		branch, _, err := g.client.GetRepoBranch(ctx, g.organizationSlug, g.repoSlug, pr.SourceBranch)
		if branch == nil || err != nil {
			continue
		}
		list = append(list, &PullRequest{
			Number:       int64(n),
			Title:        pr.Title,
			Branch:       pr.SourceBranch,
			TargetBranch: pr.TargetBranch,
			Author:       pr.Author.Slug,
			HeadSHA:      branch.Commit.Hash,
		})
	}
	return list, nil
}
