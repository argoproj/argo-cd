package pull_request

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"
	"os"

	"code.gitea.io/sdk/gitea"
)

type GiteaService struct {
	client *gitea.Client
	owner  string
	repo   string
	labels []string
}

var _ PullRequestService = (*GiteaService)(nil)

func NewGiteaService(token, url, owner, repo string, labels []string, insecure bool) (PullRequestService, error) {
	if token == "" {
		token = os.Getenv("GITEA_TOKEN")
	}
	httpClient := &http.Client{}
	if insecure {
		cookieJar, _ := cookiejar.New(nil)

		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

		httpClient = &http.Client{
			Jar:       cookieJar,
			Transport: tr,
		}
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
	opts := gitea.ListPullRequestsOptions{
		State: gitea.StateOpen,
	}
	g.client.SetContext(ctx)
	prs, _, err := g.client.ListRepoPullRequests(g.owner, g.repo, opts)
	if err != nil {
		return nil, err
	}
	list := []*PullRequest{}
	for _, pr := range prs {
		if !giteaContainLabels(g.labels, pr.Labels) {
			continue
		}
		list = append(list, &PullRequest{
			Number:       int(pr.Index),
			Title:        pr.Title,
			Branch:       pr.Head.Ref,
			TargetBranch: pr.Base.Ref,
			HeadSHA:      pr.Head.Sha,
			Labels:       getGiteaPRLabelNames(pr.Labels),
			Author:       pr.Poster.UserName,
		})
	}
	return list, nil
}

// containLabels returns true if gotLabels contains expectedLabels
func giteaContainLabels(expectedLabels []string, gotLabels []*gitea.Label) bool {
	gotLabelNamesMap := make(map[string]bool)
	for i := 0; i < len(gotLabels); i++ {
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
