package generators

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gosimple/slug"

	pullrequest "github.com/argoproj/argo-cd/v2/applicationset/services/pull_request"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*PullRequestGenerator)(nil)

const (
	DefaultPullRequestRequeueAfterSeconds = 30 * time.Minute
)

type PullRequestGenerator struct {
	client                    client.Client
	selectServiceProviderFunc func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
	SCMConfig
}

func NewPullRequestGenerator(client client.Client, scmConfig SCMConfig) Generator {
	g := &PullRequestGenerator{
		client:    client,
		SCMConfig: scmConfig,
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

func (g *PullRequestGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]interface{}, error) {
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
		return nil, fmt.Errorf("error listing repos: %w", err)
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
			"title":              pull.Title,
			"branch":             pull.Branch,
			"branch_slug":        slug.Make(pull.Branch),
			"target_branch":      pull.TargetBranch,
			"target_branch_slug": slug.Make(pull.TargetBranch),
			"head_sha":           pull.HeadSHA,
			"head_short_sha":     pull.HeadSHA[:shortSHALength],
			"head_short_sha_7":   pull.HeadSHA[:shortSHALength7],
			"author":             pull.Author,
		}

		// PR lables will only be supported for Go Template appsets, since fasttemplate will be deprecated.
		if applicationSetInfo != nil && applicationSetInfo.Spec.GoTemplate {
			paramMap["labels"] = pull.Labels
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
		var caCerts []byte
		var prErr error
		if providerConfig.CARef != nil {
			caCerts, prErr = utils.GetConfigMapData(ctx, g.client, providerConfig.CARef, applicationSetInfo.Namespace)
			if prErr != nil {
				return nil, fmt.Errorf("error fetching CA certificates from ConfigMap: %w", prErr)
			}
		}
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}
		return pullrequest.NewGitLabService(ctx, token, providerConfig.API, providerConfig.Project, providerConfig.Labels, providerConfig.PullRequestState, g.scmRootCAPath, providerConfig.Insecure, caCerts)
	}
	if generatorConfig.Gitea != nil {
		providerConfig := generatorConfig.Gitea
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}
		return pullrequest.NewGiteaService(ctx, token, providerConfig.API, providerConfig.Owner, providerConfig.Repo, providerConfig.Insecure)
	}
	if generatorConfig.BitbucketServer != nil {
		providerConfig := generatorConfig.BitbucketServer
		var caCerts []byte
		var prErr error
		if providerConfig.CARef != nil {
			caCerts, prErr = utils.GetConfigMapData(ctx, g.client, providerConfig.CARef, applicationSetInfo.Namespace)
			if prErr != nil {
				return nil, fmt.Errorf("error fetching CA certificates from ConfigMap: %w", prErr)
			}
		}
		if providerConfig.BearerToken != nil {
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			return pullrequest.NewBitbucketServiceBearerToken(ctx, providerConfig.API, appToken, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts)
		} else if providerConfig.BasicAuth != nil {
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			return pullrequest.NewBitbucketServiceBasicAuth(ctx, providerConfig.BasicAuth.Username, password, providerConfig.API, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts)
		} else {
			return pullrequest.NewBitbucketServiceNoAuth(ctx, providerConfig.API, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts)
		}
	}
	if generatorConfig.Bitbucket != nil {
		providerConfig := generatorConfig.Bitbucket
		if providerConfig.BearerToken != nil {
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			return pullrequest.NewBitbucketCloudServiceBearerToken(providerConfig.API, appToken, providerConfig.Owner, providerConfig.Repo)
		} else if providerConfig.BasicAuth != nil {
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			return pullrequest.NewBitbucketCloudServiceBasicAuth(providerConfig.API, providerConfig.BasicAuth.Username, password, providerConfig.Owner, providerConfig.Repo)
		} else {
			return pullrequest.NewBitbucketCloudServiceNoAuth(providerConfig.API, providerConfig.Owner, providerConfig.Repo)
		}
	}
	if generatorConfig.AzureDevOps != nil {
		providerConfig := generatorConfig.AzureDevOps
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}
		return pullrequest.NewAzureDevOpsService(ctx, token, providerConfig.API, providerConfig.Organization, providerConfig.Project, providerConfig.Repo, providerConfig.Labels)
	}
	return nil, fmt.Errorf("no Pull Request provider implementation configured")
}

func (g *PullRequestGenerator) github(ctx context.Context, cfg *argoprojiov1alpha1.PullRequestGeneratorGithub, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
	// use an app if it was configured
	if cfg.AppSecretName != "" {
		auth, err := g.GitHubApps.GetAuthSecret(ctx, cfg.AppSecretName)
		if err != nil {
			return nil, fmt.Errorf("error getting GitHub App secret: %w", err)
		}
		return pullrequest.NewGithubAppService(*auth, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels)
	}

	// always default to token, even if not set (public access)
	token, err := utils.GetSecretRef(ctx, g.client, cfg.TokenRef, applicationSetInfo.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error fetching Secret token: %w", err)
	}
	return pullrequest.NewGithubService(ctx, token, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels)
}
