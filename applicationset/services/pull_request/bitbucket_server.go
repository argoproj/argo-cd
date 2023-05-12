package pull_request

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	bitbucketv1 "github.com/gfleury/go-bitbucket-v1"
	log "github.com/sirupsen/logrus"
)

type BitbucketService struct {
	client         *bitbucketv1.APIClient
	projectKey     string
	repositorySlug string
	// Not supported for PRs by Bitbucket Server
	// labels         []string
}

var _ PullRequestService = (*BitbucketService)(nil)

func NewBitbucketServiceBasicAuth(ctx context.Context, username, password, url, projectKey, repositorySlug string) (PullRequestService, error) {
	bitbucketConfig := bitbucketv1.NewConfiguration(url)
	// Avoid the XSRF check
	bitbucketConfig.AddDefaultHeader("x-atlassian-token", "no-check")
	bitbucketConfig.AddDefaultHeader("x-requested-with", "XMLHttpRequest")

	ctx = context.WithValue(ctx, bitbucketv1.ContextBasicAuth, bitbucketv1.BasicAuth{
		UserName: username,
		Password: password,
	})
	return newBitbucketService(ctx, bitbucketConfig, projectKey, repositorySlug)
}

func NewBitbucketServiceNoAuth(ctx context.Context, url, projectKey, repositorySlug string) (PullRequestService, error) {
	return newBitbucketService(ctx, bitbucketv1.NewConfiguration(url), projectKey, repositorySlug)
}

func newBitbucketService(ctx context.Context, bitbucketConfig *bitbucketv1.Configuration, projectKey, repositorySlug string) (PullRequestService, error) {
	bitbucketConfig.BasePath = utils.NormalizeBitbucketBasePath(bitbucketConfig.BasePath)
	bitbucketClient := bitbucketv1.NewAPIClient(ctx, bitbucketConfig)

	return &BitbucketService{
		client:         bitbucketClient,
		projectKey:     projectKey,
		repositorySlug: repositorySlug,
	}, nil
}

func (b *BitbucketService) List(_ context.Context) ([]*PullRequest, error) {
	paged := map[string]interface{}{
		"limit": 100,
	}

	pullRequests := []*PullRequest{}
	for {
		response, err := b.client.DefaultApi.GetPullRequestsPage(b.projectKey, b.repositorySlug, paged)
		if err != nil {
			return nil, fmt.Errorf("error listing pull requests for %s/%s: %v", b.projectKey, b.repositorySlug, err)
		}
		pulls, err := bitbucketv1.GetPullRequestsResponse(response)
		if err != nil {
			log.Errorf("error parsing pull request response '%v'", response.Values)
			return nil, fmt.Errorf("error parsing pull request response for %s/%s: %v", b.projectKey, b.repositorySlug, err)
		}

		for _, pull := range pulls {
			pullRequests = append(pullRequests, &PullRequest{
				Number:  pull.ID,
				Branch:  pull.FromRef.DisplayID,    // ID: refs/heads/main DisplayID: main
				HeadSHA: pull.FromRef.LatestCommit, // This is not defined in the official docs, but works in practice
				Labels:  []string{},                // Not supported by library
			})
		}

		hasNextPage, nextPageStart := bitbucketv1.HasNextPage(response)
		if !hasNextPage {
			break
		}
		paged["start"] = nextPageStart
	}
	return pullRequests, nil
}
