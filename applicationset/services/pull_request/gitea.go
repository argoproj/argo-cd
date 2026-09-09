package pull_request

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strconv"

	"code.gitea.io/sdk/gitea"

	"github.com/argoproj/argo-cd/v3/util/proxy"
)

type GiteaService struct {
	client *gitea.Client
	owner  string
	repo   string
	labels []string
}

var _ PullRequestService = (*GiteaService)(nil)

const (
	// giteaPageSize is the number of pull requests requested per API call. Gitea
	// clamps the page size to its MAX_RESPONSE_ITEMS setting, so a short page
	// does not mean the last page.
	giteaPageSize = 50
	// giteaMaxPages bounds the paging loop so that a server which ignores the
	// page parameter fails loudly instead of looping forever.
	giteaMaxPages = 1000
)

// giteaAllCollected reports whether every pull request has been collected,
// based on the X-Total-Count header Gitea sets on its list responses. When the
// header is absent the caller keeps paging until it gets an empty page.
func giteaAllCollected(resp *gitea.Response, collected int) bool {
	if resp == nil {
		return false
	}
	total, err := strconv.Atoi(resp.Header.Get("X-Total-Count"))
	if err != nil {
		return false
	}
	return collected >= total
}

func NewGiteaService(token, url, owner, repo string, labels []string, insecure bool, proxyURL, noProxy string) (PullRequestService, error) {
	if token == "" {
		token = os.Getenv("GITEA_TOKEN")
	}
	cookieJar, _ := cookiejar.New(nil)

	tr := http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	tr.Proxy = proxy.GetCallback(proxyURL, noProxy)

	httpClient := &http.Client{
		Jar:       cookieJar,
		Transport: tr,
	}
	client, err := gitea.NewClient(url, gitea.SetToken(token), gitea.SetHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}
	return &GiteaService{
		client: client,
		owner:  owner,
		repo:   repo,
		labels: labels,
	}, nil
}

func (g *GiteaService) List(ctx context.Context) ([]*PullRequest, error) {
	g.client.SetContext(ctx)
	list := []*PullRequest{}
	fetched := 0
	for page := 1; page <= giteaMaxPages; page++ {
		opts := gitea.ListPullRequestsOptions{
			ListOptions: gitea.ListOptions{
				Page:     page,
				PageSize: giteaPageSize,
			},
			State: gitea.StateOpen,
		}
		prs, resp, err := g.client.ListRepoPullRequests(g.owner, g.repo, opts)
		if err != nil {
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				// return a custom error indicating that the repository is not found,
				// but also returning the empty result since the decision to continue or not in this case is made by the caller
				return []*PullRequest{}, NewRepositoryNotFoundError(err)
			}
			return nil, err
		}
		if len(prs) == 0 {
			return list, nil
		}
		fetched += len(prs)

		for _, pr := range prs {
			if !giteaContainLabels(g.labels, pr.Labels) {
				continue
			}
			list = append(list, &PullRequest{
				Number:       int64(pr.Index),
				Title:        pr.Title,
				Branch:       pr.Head.Ref,
				TargetBranch: pr.Base.Ref,
				HeadSHA:      pr.Head.Sha,
				Labels:       getGiteaPRLabelNames(pr.Labels),
				Author:       pr.Poster.UserName,
			})
		}
		if giteaAllCollected(resp, fetched) {
			return list, nil
		}
	}
	return nil, fmt.Errorf("gitea returned more than %d pages of pull requests for repo %q", giteaMaxPages, g.repo)
}

// containLabels returns true if gotLabels contains expectedLabels
func giteaContainLabels(expectedLabels []string, gotLabels []*gitea.Label) bool {
	gotLabelNamesMap := make(map[string]bool)
	for i := range gotLabels {
		gotLabelNamesMap[gotLabels[i].Name] = true
	}
	for _, expected := range expectedLabels {
		v, ok := gotLabelNamesMap[expected]
		if !v || !ok {
			return false
		}
	}
	return true
}

// Get the Gitea pull request label names.
func getGiteaPRLabelNames(giteaLabels []*gitea.Label) []string {
	var labelNames []string
	for _, giteaLabel := range giteaLabels {
		labelNames = append(labelNames, giteaLabel.Name)
	}
	return labelNames
}
