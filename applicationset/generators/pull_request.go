package generators

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/services/pull_request"
	pullrequest "github.com/argoproj/argo-cd/v2/applicationset/services/pull_request"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

var _ Generator = (*PullRequestGenerator)(nil)

const (
	DefaultPullRequestRequeueAfterSeconds = 30 * time.Minute
)

type PullRequestGenerator struct {
	client                    client.Client
	selectServiceProviderFunc func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
}

func NewPullRequestGenerator(client client.Client) Generator {
	g := &PullRequestGenerator{
		client: client,
	}
	g.selectServiceProviderFunc = g.selectServiceProvider
	return g
}

func (g *PullRequestGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	// Return a requeue default of 30 minutes, if no default is specified.

	if appSetGenerator.PullRequest.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.PullRequest.RequeueAfterSeconds) * time.Second
	}

	return DefaultPullRequestRequeueAfterSeconds
}

func (g *PullRequestGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.PullRequest.Template
}

func (g *PullRequestGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) ([]map[string]string, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.PullRequest == nil {
		return nil, EmptyAppSetGeneratorError
	}

	ctx := context.Background()
	svc, err := g.selectServiceProviderFunc(ctx, appSetGenerator.PullRequest, applicationSetInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to select pull request service provider: %v", err)
	}

	pulls, err := pull_request.ListPullRequests(ctx, svc, appSetGenerator.PullRequest.Filters)
	if err != nil {
		return nil, fmt.Errorf("error listing repos: %v", err)
	}
	params := make([]map[string]string, 0, len(pulls))
	for _, pull := range pulls {
		params = append(params, map[string]string{
			"number":   strconv.Itoa(pull.Number),
			"branch":   pull.Branch,
			"head_sha": pull.HeadSHA,
		})
	}
	return params, nil
}

// selectServiceProvider selects the provider to get pull requests from the configuration
func (g *PullRequestGenerator) selectServiceProvider(ctx context.Context, generatorConfig *argoprojiov1alpha1.PullRequestGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
	if generatorConfig.Github != nil {
		providerConfig := generatorConfig.Github
		token, err := g.getSecretRef(ctx, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %v", err)
		}
		return pullrequest.NewGithubService(ctx, token, providerConfig.API, providerConfig.Owner, providerConfig.Repo, providerConfig.Labels)
	}
	if generatorConfig.Gitea != nil {
		providerConfig := generatorConfig.Gitea
		token, err := g.getSecretRef(ctx, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %v", err)
		}
		return pullrequest.NewGiteaService(ctx, token, providerConfig.API, providerConfig.Owner, providerConfig.Repo, providerConfig.Insecure)
	}
	if generatorConfig.BitbucketServer != nil {
		providerConfig := generatorConfig.BitbucketServer
		if providerConfig.BasicAuth != nil {
			password, err := g.getSecretRef(ctx, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %v", err)
			}
			return pullrequest.NewBitbucketServiceBasicAuth(ctx, providerConfig.BasicAuth.Username, password, providerConfig.API, providerConfig.Project, providerConfig.Repo)
		} else {
			return pullrequest.NewBitbucketServiceNoAuth(ctx, providerConfig.API, providerConfig.Project, providerConfig.Repo)
		}
	}
	return nil, fmt.Errorf("no Pull Request provider implementation configured")
}

// getSecretRef gets the value of the key for the specified Secret resource.
func (g *PullRequestGenerator) getSecretRef(ctx context.Context, ref *argoprojiov1alpha1.SecretRef, namespace string) (string, error) {
	if ref == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	err := g.client.Get(
		ctx,
		client.ObjectKey{
			Name:      ref.SecretName,
			Namespace: namespace,
		},
		secret)
	if err != nil {
		return "", fmt.Errorf("error fetching secret %s/%s: %v", namespace, ref.SecretName, err)
	}
	tokenBytes, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %q in secret %s/%s not found", ref.Key, namespace, ref.SecretName)
	}
	return string(tokenBytes), nil
}
