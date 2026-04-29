package generators

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/util/proxy"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/applicationset/services"
	"github.com/argoproj/argo-cd/v3/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider"
	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/common"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*SCMProviderGenerator)(nil)

const (
	DefaultSCMProviderRequeueAfter = 30 * time.Minute
)

type SCMProviderGenerator struct {
	client client.Client
	// Testing hooks.
	overrideProvider scm_provider.SCMProviderService
	SCMConfig
}
type SCMConfig struct {
	scmRootCAPath          string
	allowedSCMProviders    []string
	enableSCMProviders     bool
	enableGitHubAPIMetrics bool
	GitHubApps             github_app_auth.Credentials
	tokenRefStrictMode     bool
	scmProxyURL            string
	scmNoProxy             string
}

func NewSCMConfig(scmRootCAPath string, allowedSCMProviders []string, enableSCMProviders bool, enableGitHubAPIMetrics bool, gitHubApps github_app_auth.Credentials, tokenRefStrictMode bool, opts ...SCMConfigOpts) SCMConfig {
	c := SCMConfig{
		scmRootCAPath:          scmRootCAPath,
		allowedSCMProviders:    allowedSCMProviders,
		enableSCMProviders:     enableSCMProviders,
		enableGitHubAPIMetrics: enableGitHubAPIMetrics,
		GitHubApps:             gitHubApps,
		tokenRefStrictMode:     tokenRefStrictMode,
	}

	for _, opt := range opts {
		opt(&c)
	}
	return c
}

type SCMConfigOpts func(c *SCMConfig)

func WithNoProxyList(noProxyList string) SCMConfigOpts {
	return func(config *SCMConfig) {
		config.scmNoProxy = noProxyList
	}
}

func WithProxyURL(scmProxyURL string) SCMConfigOpts {
	return func(config *SCMConfig) {
		config.scmProxyURL = scmProxyURL
	}
}

func NewSCMProviderGenerator(client client.Client, scmConfig SCMConfig) Generator {
	return &SCMProviderGenerator{
		client:    client,
		SCMConfig: scmConfig,
	}
}

// Testing generator
func NewTestSCMProviderGenerator(overrideProvider scm_provider.SCMProviderService) Generator {
	return &SCMProviderGenerator{overrideProvider: overrideProvider, SCMConfig: SCMConfig{
		enableSCMProviders: true,
	}}
}

func (g *SCMProviderGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	// Return a requeue default of 30 minutes, if no default is specified.

	if appSetGenerator.SCMProvider.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.SCMProvider.RequeueAfterSeconds) * time.Second
	}

	return DefaultSCMProviderRequeueAfter
}

func (g *SCMProviderGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.SCMProvider.Template
}

var ErrSCMProvidersDisabled = errors.New("scm providers are disabled")

type ErrDisallowedSCMProvider struct {
	Provider string
	Allowed  []string
}

func NewErrDisallowedSCMProvider(provider string, allowed []string) ErrDisallowedSCMProvider {
	return ErrDisallowedSCMProvider{
		Provider: provider,
		Allowed:  allowed,
	}
}

func (e ErrDisallowedSCMProvider) Error() string {
	return fmt.Sprintf("scm provider %q not allowed, must use one of the following: %s", e.Provider, strings.Join(e.Allowed, ", "))
}

func ScmProviderAllowed(applicationSetInfo *argoprojiov1alpha1.ApplicationSet, generator SCMGeneratorWithCustomApiUrl, allowedScmProviders []string) error {
	url := generator.CustomApiUrl()

	if url == "" || len(allowedScmProviders) == 0 {
		return nil
	}

	if slices.Contains(allowedScmProviders, url) {
		return nil
	}

	log.WithFields(log.Fields{
		common.SecurityField: common.SecurityMedium,
		"applicationset":     applicationSetInfo.Name,
		"appSetNamespace":    applicationSetInfo.Namespace,
	}).Debugf("attempted to use disallowed SCM %q, must use one of the following: %s", url, strings.Join(allowedScmProviders, ", "))

	return NewErrDisallowedSCMProvider(url, allowedScmProviders)
}

func (g *SCMProviderGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.SCMProvider == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if !g.enableSCMProviders {
		return nil, ErrSCMProvidersDisabled
	}

	// Create the SCM provider helper.
	providerConfig := appSetGenerator.SCMProvider

	if err := ScmProviderAllowed(applicationSetInfo, providerConfig, g.allowedSCMProviders); err != nil {
		return nil, fmt.Errorf("scm provider not allowed: %w", err)
	}

	ctx := context.Background()
	scmHTTPClient := g.newSCMHTTPClient()
	var provider scm_provider.SCMProviderService
	switch {
	case g.overrideProvider != nil:
		provider = g.overrideProvider
	case providerConfig.Github != nil:
		var err error
		provider, err = g.githubProvider(ctx, providerConfig.Github, applicationSetInfo, scmHTTPClient)
		if err != nil {
			return nil, fmt.Errorf("scm provider: %w", err)
		}
	case providerConfig.Gitlab != nil:
		providerConfig := providerConfig.Gitlab
		var caCerts []byte
		var scmError error
		if providerConfig.CARef != nil {
			caCerts, scmError = utils.GetConfigMapData(ctx, g.client, providerConfig.CARef, applicationSetInfo.Namespace)
			if scmError != nil {
				return nil, fmt.Errorf("error fetching CA certificates from ConfigMap: %w", scmError)
			}
		}
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Gitlab token: %w", err)
		}
		provider, err = scm_provider.NewGitlabProvider(providerConfig.Group, token, providerConfig.API, providerConfig.AllBranches, providerConfig.IncludeSubgroups, providerConfig.WillIncludeSharedProjects(), providerConfig.IncludeArchivedRepos, providerConfig.Insecure, g.scmRootCAPath, providerConfig.Topic, caCerts, g.scmProxyURL, g.scmNoProxy)
		if err != nil {
			return nil, fmt.Errorf("error initializing Gitlab service: %w", err)
		}
	case providerConfig.Gitea != nil:
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.Gitea.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Gitea token: %w", err)
		}
		provider, err = scm_provider.NewGiteaProvider(providerConfig.Gitea.Owner, token, providerConfig.Gitea.API, providerConfig.Gitea.AllBranches, providerConfig.Gitea.Insecure, providerConfig.Gitea.ExcludeArchivedRepos, g.scmProxyURL, g.scmNoProxy)
		if err != nil {
			return nil, fmt.Errorf("error initializing Gitea service: %w", err)
		}
	case providerConfig.BitbucketServer != nil:
		providerConfig := providerConfig.BitbucketServer
		var caCerts []byte
		var scmError error
		if providerConfig.CARef != nil {
			caCerts, scmError = utils.GetConfigMapData(ctx, g.client, providerConfig.CARef, applicationSetInfo.Namespace)
			if scmError != nil {
				return nil, fmt.Errorf("error fetching CA certificates from ConfigMap: %w", scmError)
			}
		}
		switch {
		case providerConfig.BearerToken != nil:
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			provider, scmError = scm_provider.NewBitbucketServerProviderBearerToken(ctx, appToken, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
		case providerConfig.BasicAuth != nil:
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			provider, scmError = scm_provider.NewBitbucketServerProviderBasicAuth(ctx, providerConfig.BasicAuth.Username, password, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
		default:
			provider, scmError = scm_provider.NewBitbucketServerProviderNoAuth(ctx, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, g.scmRootCAPath, providerConfig.Insecure, caCerts, g.scmProxyURL, g.scmNoProxy)
		}
		if scmError != nil {
			return nil, fmt.Errorf("error initializing Bitbucket Server service: %w", scmError)
		}
	case providerConfig.AzureDevOps != nil:
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.AzureDevOps.AccessTokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Azure Devops access token: %w", err)
		}
		provider, err = scm_provider.NewAzureDevOpsProvider(token, providerConfig.AzureDevOps.Organization, providerConfig.AzureDevOps.API, providerConfig.AzureDevOps.TeamProject, providerConfig.AzureDevOps.AllBranches)
		if err != nil {
			return nil, fmt.Errorf("error initializing Azure Devops service: %w", err)
		}
	case providerConfig.Bitbucket != nil:
		switch {
		case providerConfig.Bitbucket.BearerToken != nil && providerConfig.Bitbucket.AppPasswordRef != nil:
			return nil, errors.New("bitbucket scm provider bearerToken and appPasswordRef are mutually exclusive")
		case providerConfig.Bitbucket.BearerToken != nil:
			token, err := utils.GetSecretRef(ctx, g.client, providerConfig.Bitbucket.BearerToken.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Bitbucket cloud bearer token: %w", err)
			}
			provider, err = scm_provider.NewBitBucketCloudProviderBearerToken(providerConfig.Bitbucket.Owner, token, providerConfig.Bitbucket.AllBranches)
			if err != nil {
				return nil, fmt.Errorf("error initializing Bitbucket cloud service: %w", err)
			}
		case providerConfig.Bitbucket.AppPasswordRef != nil:
			appPassword, err := utils.GetSecretRef(ctx, g.client, providerConfig.Bitbucket.AppPasswordRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Bitbucket cloud appPassword: %w", err)
			}
			provider, err = scm_provider.NewBitBucketCloudProvider(providerConfig.Bitbucket.Owner, providerConfig.Bitbucket.User, appPassword, providerConfig.Bitbucket.AllBranches)
			if err != nil {
				return nil, fmt.Errorf("error initializing Bitbucket cloud service: %w", err)
			}
		default:
			return nil, errors.New("bitbucket scm provider requires either bearerToken or appPasswordRef")
		}
	case providerConfig.AWSCodeCommit != nil:
		var awsErr error
		provider, awsErr = scm_provider.NewAWSCodeCommitProvider(ctx, providerConfig.AWSCodeCommit.TagFilters, providerConfig.AWSCodeCommit.Role, providerConfig.AWSCodeCommit.Region, providerConfig.AWSCodeCommit.AllBranches)
		if awsErr != nil {
			return nil, fmt.Errorf("error initializing AWS codecommit service: %w", awsErr)
		}
	default:
		return nil, errors.New("no SCM provider implementation configured")
	}

	// Find all the available repos.
	repos, err := scm_provider.ListRepos(ctx, provider, providerConfig.Filters, providerConfig.CloneProtocol)
	if err != nil {
		return nil, fmt.Errorf("error listing repos: %w", err)
	}
	paramsArray := make([]map[string]any, 0, len(repos))
	var shortSHALength int
	var shortSHALength7 int
	for _, repo := range repos {
		shortSHALength = min(len(repo.SHA), 8)

		shortSHALength7 = min(len(repo.SHA), 7)

		params := map[string]any{
			"organization":     repo.Organization,
			"repository":       repo.Repository,
			"repository_id":    repo.RepositoryId,
			"url":              repo.URL,
			"branch":           repo.Branch,
			"sha":              repo.SHA,
			"short_sha":        repo.SHA[:shortSHALength],
			"short_sha_7":      repo.SHA[:shortSHALength7],
			"labels":           strings.Join(repo.Labels, ","),
			"branchNormalized": utils.SanitizeName(repo.Branch),
		}

		err := appendTemplatedValues(appSetGenerator.SCMProvider.Values, params, applicationSetInfo.Spec.GoTemplate, applicationSetInfo.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("failed to append templated values: %w", err)
		}

		paramsArray = append(paramsArray, params)
	}
	return paramsArray, nil
}

func (g *SCMProviderGenerator) githubProvider(ctx context.Context, github *argoprojiov1alpha1.SCMProviderGeneratorGithub, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, baseHTTPClient *http.Client) (scm_provider.SCMProviderService, error) {
	httpClient := baseHTTPClient
	if g.enableGitHubAPIMetrics {
		metricsCtx := &services.MetricsContext{
			AppSetNamespace: applicationSetInfo.Namespace,
			AppSetName:      applicationSetInfo.Name,
		}
		httpClient = services.NewGitHubMetricsClientFrom(httpClient, metricsCtx)
	}

	if github.AppSecretName != "" {
		auth, err := g.GitHubApps.GetAuthSecret(ctx, github.AppSecretName)
		if err != nil {
			return nil, fmt.Errorf("error fetching Github app secret: %w", err)
		}
		return scm_provider.NewGithubAppProviderFor(ctx, *auth, github.Organization, github.API, github.AllBranches, github.ExcludeArchivedRepos, httpClient)
	}

	token, err := utils.GetSecretRef(ctx, g.client, github.TokenRef, applicationSetInfo.Namespace, g.tokenRefStrictMode)
	if err != nil {
		return nil, fmt.Errorf("error fetching Github token: %w", err)
	}
	return scm_provider.NewGithubProvider(github.Organization, token, github.API, github.AllBranches, github.ExcludeArchivedRepos, httpClient)
}

func (g *SCMConfig) newSCMHTTPClient() *http.Client {
	if g.scmProxyURL == "" {
		return &http.Client{}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = proxy.GetCallback(g.scmProxyURL, g.scmNoProxy)
	return &http.Client{Transport: tr}
}
