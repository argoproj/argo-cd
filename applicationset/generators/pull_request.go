package generators

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gosimple/slug"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	pullrequest "github.com/argoproj/argo-cd/v3/applicationset/services/pull_request"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
)

const (
	DefaultPullRequestRequeueAfter = 30 * time.Minute
)

type PullRequestGenerator struct {
	client                    client.Client
	selectServiceProviderFunc func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
	// newAzureTokenProvider creates the token provider used when an Azure DevOps generator sets
	// useAzureWorkloadIdentity. It can be replaced in tests.
	newAzureTokenProvider func() workloadidentity.TokenProvider
	// azureTokenProvider is created on first use and then shared by all reconciliations, so that tokens are
	// cached by the provider between reconciliations.
	azureTokenProvider     workloadidentity.TokenProvider
	azureTokenProviderOnce sync.Once
	SCMConfig
}

func NewPullRequestGenerator(client client.Client, scmConfig SCMConfig) Generator {
	g := &PullRequestGenerator{
		client:                client,
		SCMConfig:             scmConfig,
		newAzureTokenProvider: workloadidentity.NewStrictWorkloadIdentityTokenProvider,
	}
	g.selectServiceProviderFunc = g.selectServiceProvider
	return g
}

func (g *PullRequestGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	// Return a requeue default of 30 minutes, if no default is specified.

	if appSetGenerator.PullRequest.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.PullRequest.RequeueAfterSeconds) * time.Second
	}

	return DefaultPullRequestRequeueAfter
}

func (g *PullRequestGenerator) GetContinueOnRepoNotFoundError(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) bool {
	return appSetGenerator.PullRequest.ContinueOnRepoNotFoundError
}

func (g *PullRequestGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.PullRequest.Template
}

func (g *PullRequestGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.PullRequest == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	ctx := context.Background()
	svc, err := g.selectServiceProviderFunc(ctx, appSetGenerator.PullRequest, applicationSetInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to select pull request service provider: %w", err)
	}

	pulls, err := pullrequest.ListPullRequests(ctx, svc, appSetGenerator.PullRequest.Filters)
	params := make([]map[string]any, 0, len(pulls))
	if err != nil {
		if pullrequest.IsRepositoryNotFoundError(err) && g.GetContinueOnRepoNotFoundError(appSetGenerator) {
			log.WithError(err).WithField("generator", g).
				Warn("Skipping params generation for this repository since it was not found.")
			return params, nil
		}
		return nil, fmt.Errorf("error listing repos: %w", err)
	}

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
		shortSHALength = min(len(pull.HeadSHA), 8)

		shortSHALength7 = min(len(pull.HeadSHA), 7)

		paramMap := map[string]any{
			"number":             strconv.FormatInt(pull.Number, 10),
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

		// PR labels will only be supported for Go Template appsets, since fasttemplate will be deprecated.
		if applicationSetInfo != nil && applicationSetInfo.Spec.GoTemplate {
			paramMap["labels"] = pull.Labels
		}

		err := appendTemplatedValues(appSetGenerator.PullRequest.Values, paramMap, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
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
		var caCerts []byte
		var prErr error
		if providerConfig.CARef != nil {
			caCerts, prErr = utils.GetConfigMapData(ctx, g.client, providerConfig.CARef, applicationSetInfo.Namespace)
			if prErr != nil {
				return nil, fmt.Errorf("error fetching CA certificates from ConfigMap: %w", prErr)
			}
		}
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}
		return pullrequest.NewGitLabService(token, providerConfig.API, providerConfig.Project, providerConfig.Labels, providerConfig.ExcludedLabels, providerConfig.PullRequestState, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
	}
	if generatorConfig.Gitea != nil {
		providerConfig := generatorConfig.Gitea
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}

		return pullrequest.NewGiteaService(token, providerConfig.API, providerConfig.Owner, providerConfig.Repo, providerConfig.Labels, providerConfig.ExcludedLabels, providerConfig.Insecure, g.scmProxyURL, g.scmNoProxy)
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
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			return pullrequest.NewBitbucketServiceBearerToken(ctx, appToken, providerConfig.API, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
		} else if providerConfig.BasicAuth != nil {
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			return pullrequest.NewBitbucketServiceBasicAuth(ctx, providerConfig.BasicAuth.Username, password, providerConfig.API, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
		}
		return pullrequest.NewBitbucketServiceNoAuth(ctx, providerConfig.API, providerConfig.Project, providerConfig.Repo, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
	}
	if generatorConfig.Bitbucket != nil {
		providerConfig := generatorConfig.Bitbucket
		if providerConfig.BearerToken != nil {
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			return pullrequest.NewBitbucketCloudServiceBearerToken(providerConfig.API, appToken, providerConfig.Owner, providerConfig.Repo)
		} else if providerConfig.BasicAuth != nil {
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			return pullrequest.NewBitbucketCloudServiceBasicAuth(providerConfig.API, providerConfig.BasicAuth.Username, password, providerConfig.Owner, providerConfig.Repo)
		}
		return pullrequest.NewBitbucketCloudServiceNoAuth(providerConfig.API, providerConfig.Owner, providerConfig.Repo)
	}
	if generatorConfig.AzureDevOps != nil {
		providerConfig := generatorConfig.AzureDevOps
		if providerConfig.UseAzureWorkloadIdentity {
			if providerConfig.TokenRef != nil {
				return nil, errors.New("azureDevOps pull request generator: tokenRef and useAzureWorkloadIdentity are mutually exclusive")
			}
			// The workload identity token is a credential of the controller, not of the ApplicationSet author, so it
			// must not be sent to an arbitrary host. A custom api URL is only accepted when the administrator has
			// explicitly allowed it; ScmProviderAllowed above already enforces the list when it is not empty.
			if !isDefaultAzureDevOpsAPI(providerConfig.API) && len(g.allowedSCMProviders) == 0 {
				return nil, fmt.Errorf("azureDevOps pull request generator: useAzureWorkloadIdentity with a custom api URL (%q) requires the URL to be listed in the allowed SCM providers", providerConfig.API)
			}
			return pullrequest.NewAzureDevOpsServiceWithWorkloadIdentity(g.getAzureTokenProvider(), providerConfig.API, providerConfig.Organization, providerConfig.Project, providerConfig.Repo, providerConfig.Labels, providerConfig.ExcludedLabels)
		}
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Secret token: %w", err)
		}
		return pullrequest.NewAzureDevOpsService(token, providerConfig.API, providerConfig.Organization, providerConfig.Project, providerConfig.Repo, providerConfig.Labels, providerConfig.ExcludedLabels)
	}
	return nil, errors.New("no Pull Request provider implementation configured")
}

// getAzureTokenProvider returns the Azure Workload Identity token provider, creating it on first use.
func (g *PullRequestGenerator) getAzureTokenProvider() workloadidentity.TokenProvider {
	g.azureTokenProviderOnce.Do(func() {
		g.azureTokenProvider = g.newAzureTokenProvider()
	})
	return g.azureTokenProvider
}

// isDefaultAzureDevOpsAPI reports whether api is empty or the public Azure DevOps service URL.
func isDefaultAzureDevOpsAPI(api string) bool {
	return api == "" || strings.EqualFold(strings.TrimSuffix(api, "/"), pullrequest.AZURE_DEVOPS_DEFAULT_URL)
}

func (g *PullRequestGenerator) github(ctx context.Context, cfg *argoprojiov1alpha1.PullRequestGeneratorGithub, applicationSetInfo *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
	httpClient := g.newSCMHTTPClient()
	if g.enableGitHubAPIMetrics {
		metricsCtx := &services.MetricsContext{
			AppSetNamespace: applicationSetInfo.Namespace,
			AppSetName:      applicationSetInfo.Name,
		}
		httpClient = services.NewGitHubMetricsClientFrom(httpClient, metricsCtx)
	}

	// use an app if it was configured
	if cfg.AppSecretName != "" {
		auth, err := g.GitHubApps.GetAuthSecret(ctx, cfg.AppSecretName)
		if err != nil {
			return nil, fmt.Errorf("error getting GitHub App secret: %w", err)
		}
		return pullrequest.NewGithubAppService(ctx, *auth, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels, cfg.ExcludedLabels, httpClient)
	}

	// always default to token, even if not set (public access)
	token, err := utils.GetSecretRef(ctx, g.client, cfg.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
	if err != nil {
		return nil, fmt.Errorf("error fetching Secret token: %w", err)
	}
	return pullrequest.NewGithubService(token, cfg.API, cfg.Owner, cfg.Repo, cfg.Labels, cfg.ExcludedLabels, httpClient)
}
