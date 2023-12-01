package generators

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gosimple/slug"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pullrequest "github.com/argoproj/argo-cd/v2/applicationset/services/pull_request"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*PullRequestGenerator)(nil)

const (
	DefaultPullRequestRequeueAfterSeconds = 30 * time.Minute
)

type PullRequestGenerator struct {
	client                    client.Client
	selectServiceProviderFunc func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
	auth                      SCMAuthProviders
	scmRootCAPath             string
	allowedSCMProviders       []string
	enableSCMProviders        bool
}

func NewPullRequestGenerator(client client.Client, auth SCMAuthProviders, scmRootCAPath string, allowedScmProviders []string, enableSCMProviders bool) Generator {
	g := &PullRequestGenerator{
		client:              client,
		auth:                auth,
		scmRootCAPath:       scmRootCAPath,
		allowedSCMProviders: allowedScmProviders,
		enableSCMProviders:  enableSCMProviders,
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

func (g *PullRequestGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.PullRequest == nil {
		return nil, EmptyAppSetGeneratorError
	}

	ctx := context.Background()
	svc, err := g.selectServiceProviderFunc(ctx, appSetGenerator.PullRequest, applicationSetInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to select pull request service provider: %w", err)
	}

	pulls, err := pullrequest.ListPullRequests(ctx, svc, appSetGenerator.PullRequest.Filters)
	if err != nil {
		return nil, fmt.Errorf("error listing repos: %v", err)
	}
	params := make([]map[string]interface{}, 0, len(pulls))

	// In order to follow the DNS label standard as defined in RFC 1123,
	// we need to limit the 'branch' to 50 to give room to append/suffix-ing it
	// with 13 more characters. Also, there is the need to clean it as recommended
	// here https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names
	slug.MaxLength = 50

	// Converting underscores to dashes
	slug.CustomSub = map[string]string{
		"_": "-",
	}

	var shortSHALength int
	var shortSHALength7 int
	for _, pull := range pulls {
		shortSHALength = 8
		if len(pull.HeadSHA) < 8 {
			shortSHALength = len(pull.HeadSHA)
		}

		shortSHALength7 = 7
		if len(pull.HeadSHA) < 7 {
			shortSHALength7 = len(pull.HeadSHA)
		}

		paramMap := map[string]interface{}{
			"number":             strconv.Itoa(pull.Number),
			"branch":             pull.Branch,
			"branch_slug":        slug.Make(pull.Branch),
			"target_branch":      pull.TargetBranch,
			"target_branch_slug": slug.Make(pull.TargetBranch),
			"head_sha":           pull.HeadSHA,
			"head_short_sha":     pull.HeadSHA[:shortSHALength],
			"head_short_sha_7":   pull.HeadSHA[:shortSHALength7],
		}

		// PR labels will only be supported for Go Template appsets, since fasttemplate will be deprecated.
		if applicationSetInfo != nil && applicationSetInfo.Spec.GoTemplate {
			paramMap["labels"] = pull.Labels
		}

		err = appendTemplatedValues(appSetGenerator.PullRequest.Values, paramMap, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to append templated values: %w", err)
		}

		params = append(params, paramMap)
	}
	return params, nil
}

// selectServiceProvider selects the provider to get pull requests from the configuration
func (g *PullRequestGenerator) selectServiceProvider(ctx context.Context, generatorConfig *argoprojiov1alpha1.PullRequestGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
	if !g.enableSCMProviders {
		return nil, ErrSCMProvidersDisabled
	}
	if err := ScmProviderAllowed(applicationSetInfo, generatorConfig, g.allowedSCMProviders); err != nil {
		return nil, fmt.Errorf("scm provider not allowed: %w", err)
	}

	if generatorConfig.Github != nil {
		return g.github(ctx, generatorConfig.Github, applicationSetInfo)
	}
	if generatorConfig.GitLab != nil {
		providerConfig := generatorConfig.GitLab
		token, err := g.getSecretRef(ctx, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %v", err)
		}
		return pullrequest.NewGitLabService(ctx, token, providerConfig.API, providerConfig.Project, providerConfig.Labels, providerConfig.PullRequestState, g.scmRootCAPath, providerConfig.Insecure)
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
	if generatorConfig.Bitbucket != nil {
		providerConfig := generatorConfig.Bitbucket
		if providerConfig.BearerToken != nil {
			appToken, err := g.getSecretRef(ctx, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %v", err)
			}
			return pullrequest.NewBitbucketCloudServiceBearerToken(providerConfig.API, appToken, providerConfig.Owner, providerConfig.Repo)
		} else if providerConfig.BasicAuth != nil {
			password, err := g.getSecretRef(ctx, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %v", err)
			}
			return pullrequest.NewBitbucketCloudServiceBasicAuth(providerConfig.API, providerConfig.BasicAuth.Username, password, providerConfig.Owner, providerConfig.Repo)
		} else {
			return pullrequest.NewBitbucketCloudServiceNoAuth(providerConfig.API, providerConfig.Owner, providerConfig.Repo)
		}
	}
	if generatorConfig.AzureDevOps != nil {
		providerConfig := generatorConfig.AzureDevOps
		token, err := g.getSecretRef(ctx, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %v", err)
		}
		return pullrequest.NewAzureDevOpsService(ctx, token, providerConfig.API, providerConfig.Organization, providerConfig.Project, providerConfig.Repo, providerConfig.Labels)
	}
	return nil, fmt.Errorf("no Pull Request provider implementation configured")
}

func (g *PullRequestGenerator) github(ctx context.Context, cfg *argoprojiov1alpha1.PullRequestGeneratorGithub, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
	// use an app if it was configured
	if cfg.AppSecretName != "" {
		auth, err := g.auth.GitHubApps.GetAuthSecret(ctx, cfg.AppSecretName)
		if err != nil {
			return nil, fmt.Errorf("error getting GitHub App secret: %v", err)
		}
		return pullrequest.NewGithubAppService(*auth, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels)
	}

	// always default to token, even if not set (public access)
	token, err := g.getSecretRef(ctx, cfg.TokenRef, applicationSetInfo.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error fetching Secret token: %v", err)
	}
	return pullrequest.NewGithubService(ctx, token, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels)
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
