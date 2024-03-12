package gitlab_environments

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
	environmentState string
}

var _ EnvironmentService = (*GitLabService)(nil)

func NewGitLabService(ctx context.Context, token, url, project string, environmentState string, scmRootCAPath string, insecure bool) (EnvironmentService, error) {
	var clientOptionFns []gitlab.ClientOptionFunc

	// Set a custom Gitlab base URL if one is provided
	if url != "" {
		clientOptionFns = append(clientOptionFns, gitlab.WithBaseURL(url))
	}

	if token == "" {
		token = os.Getenv("GITLAB_TOKEN")
	}

	tr := &http.Transport{
		TLSClientConfig: utils.GetTlsConfig(scmRootCAPath, insecure),
	}
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
		environmentState: environmentState,
	}, nil
}

func (g *GitLabService) List(ctx context.Context) ([]*Environment, error) {

	opts := &gitlab.ListEnvironmentsOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if g.environmentState != "" {
		opts.States = &g.environmentState
	}

	environments := []*Environment{}
	for {
		envs, resp, err := g.client.Environments.ListEnvironments(g.project, opts)
		if err != nil {
			return nil, fmt.Errorf("error listing environments for project '%s': %v", g.project, err)
		}
		for _, env := range envs {
			environments = append(environments, &Environment{
				ID:          env.ID,
				Name:        env.Name,
				Slug:        env.Slug,
				State:       env.State,
				Tier:        env.Tier,
				ExternalURL: env.ExternalURL,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return environments, nil
}
