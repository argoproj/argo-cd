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
	"github.com/argoproj/argo-cd/v3/util/configbus"
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
	configProvider configbus.Provider
	// Deprecated: use configProvider.ApplicationsetScmRootCAPath.
	scmRootCAPath string
	// Deprecated: use configProvider.ApplicationsetAllowedScmProviders.
	allowedSCMProviders []string
	// Deprecated: use configProvider.ApplicationsetEnableScmProviders.
	enableSCMProviders bool
	// Deprecated: use configProvider.ApplicationsetEnableGitHubAPIMetrics.
	enableGitHubAPIMetrics bool
	GitHubApps             github_app_auth.Credentials
	// Deprecated: use configProvider.ApplicationsetTokenRefStrictMode.
	tokenRefStrictMode bool
	// Deprecated: use configProvider.ApplicationsetScmProxyURL.
	scmProxyURL string
	// Deprecated: use configProvider.ApplicationsetScmNoProxy.
	scmNoProxy string
}

// SetConfigProvider attaches the configbus Provider used for SCM setting reads.
func (c *SCMConfig) SetConfigProvider(p configbus.Provider) {
	c.configProvider = p
}

func (c *SCMConfig) enableSCMProvidersResolved() (bool, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetEnableScmProviders(context.Background())
	}
	return c.LegacyEnableSCMProviders(), nil
}

func (c *SCMConfig) allowedSCMProvidersResolved() ([]string, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetAllowedScmProviders(context.Background())
	}
	return c.LegacyAllowedSCMProviders(), nil
}

func (c *SCMConfig) tokenRefStrictModeResolved() (bool, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetTokenRefStrictMode(context.Background())
	}
	return c.LegacyTokenRefStrictMode(), nil
}

func (c *SCMConfig) scmRootCAPathResolved() (string, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetScmRootCAPath(context.Background())
	}
	return c.LegacyScmRootCAPath(), nil
}

func (c *SCMConfig) scmProxyURLResolved() (string, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetScmProxyURL(context.Background())
	}
	return c.LegacyScmProxyURL(), nil
}

func (c *SCMConfig) scmNoProxyResolved() (string, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetScmNoProxy(context.Background())
	}
	return c.LegacyScmNoProxy(), nil
}

func (c *SCMConfig) enableGitHubAPIMetricsResolved() (bool, error) {
	if c.configProvider != nil {
		return c.configProvider.ApplicationsetEnableGitHubAPIMetrics(context.Background())
	}
	return c.LegacyEnableGitHubAPIMetrics(), nil
}

// scmResolvedSettings holds SCM settings resolved once per generator invocation.
type scmResolvedSettings struct {
	enableSCMProviders     bool
	allowedSCMProviders    []string
	tokenRefStrictMode     bool
	scmRootCAPath          string
	scmProxyURL            string
	scmNoProxy             string
	enableGitHubAPIMetrics bool
}

func (c *SCMConfig) resolveSCMSettings() (scmResolvedSettings, error) {
	var s scmResolvedSettings
	var err error
	if s.enableSCMProviders, err = c.enableSCMProvidersResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve enable SCM providers: %w", err)
	}
	if s.allowedSCMProviders, err = c.allowedSCMProvidersResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve allowed SCM providers: %w", err)
	}
	if s.tokenRefStrictMode, err = c.tokenRefStrictModeResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve token ref strict mode: %w", err)
	}
	if s.scmRootCAPath, err = c.scmRootCAPathResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve SCM root CA path: %w", err)
	}
	if s.scmProxyURL, err = c.scmProxyURLResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve SCM proxy URL: %w", err)
	}
	if s.scmNoProxy, err = c.scmNoProxyResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve SCM no-proxy: %w", err)
	}
	if s.enableGitHubAPIMetrics, err = c.enableGitHubAPIMetricsResolved(); err != nil {
		return s, fmt.Errorf("failed to resolve enable GitHub API metrics: %w", err)
	}
	return s, nil
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

	scmSettings, err := g.resolveSCMSettings()
	if err != nil {
		return nil, err
	}

	if !scmSettings.enableSCMProviders {
		return nil, ErrSCMProvidersDisabled
	}

	// Create the SCM provider helper.
	providerConfig := appSetGenerator.SCMProvider

	if err := ScmProviderAllowed(applicationSetInfo, providerConfig, scmSettings.allowedSCMProviders); err != nil {
		return nil, fmt.Errorf("scm provider not allowed: %w", err)
	}

	ctx := context.Background()
	scmHTTPClient := newSCMHTTPClient(scmSettings.scmProxyURL, scmSettings.scmNoProxy)
	var provider scm_provider.SCMProviderService
	switch {
	case g.overrideProvider != nil:
		provider = g.overrideProvider
	case providerConfig.Github != nil:
		provider, err = g.githubProvider(ctx, providerConfig.Github, applicationSetInfo, scmHTTPClient, scmSettings)
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
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.TokenRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Gitlab token: %w", err)
		}
		provider, err = scm_provider.NewGitlabProvider(providerConfig.Group, token, providerConfig.API, providerConfig.AllBranches, providerConfig.IncludeSubgroups, providerConfig.WillIncludeSharedProjects(), providerConfig.IncludeArchivedRepos, providerConfig.Insecure, scmSettings.scmRootCAPath, providerConfig.Topic, caCerts, scmSettings.scmProxyURL, scmSettings.scmNoProxy)
		if err != nil {
			return nil, fmt.Errorf("error initializing Gitlab service: %w", err)
		}
	case providerConfig.Gitea != nil:
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.Gitea.TokenRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Gitea token: %w", err)
		}
		provider, err = scm_provider.NewGiteaProvider(providerConfig.Gitea.Owner, token, providerConfig.Gitea.API, providerConfig.Gitea.AllBranches, providerConfig.Gitea.Insecure, providerConfig.Gitea.ExcludeArchivedRepos, scmSettings.scmProxyURL, scmSettings.scmNoProxy)
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
			appToken, err := utils.GetSecretRef(ctx, g.client, providerConfig.BearerToken.TokenRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret Bearer token: %w", err)
			}
			provider, scmError = scm_provider.NewBitbucketServerProviderBearerToken(ctx, appToken, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, scmSettings.scmRootCAPath, providerConfig.Insecure, caCerts, scmSettings.scmProxyURL, scmSettings.scmNoProxy)
		case providerConfig.BasicAuth != nil:
			password, err := utils.GetSecretRef(ctx, g.client, providerConfig.BasicAuth.PasswordRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
			if err != nil {
				return nil, fmt.Errorf("error fetching Secret token: %w", err)
			}
			provider, scmError = scm_provider.NewBitbucketServerProviderBasicAuth(ctx, providerConfig.BasicAuth.Username, password, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, scmSettings.scmRootCAPath, providerConfig.Insecure, caCerts, scmSettings.scmProxyURL, scmSettings.scmNoProxy)
		default:
			provider, scmError = scm_provider.NewBitbucketServerProviderNoAuth(ctx, providerConfig.API, providerConfig.Project, providerConfig.AllBranches, scmSettings.scmRootCAPath, providerConfig.Insecure, caCerts, scmSettings.scmProxyURL, scmSettings.scmNoProxy)
		}
		if scmError != nil {
			return nil, fmt.Errorf("error initializing Bitbucket Server service: %w", scmError)
		}
	case providerConfig.AzureDevOps != nil:
		token, err := utils.GetSecretRef(ctx, g.client, providerConfig.AzureDevOps.AccessTokenRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Azure Devops access token: %w", err)
		}
		provider, err = scm_provider.NewAzureDevOpsProvider(token, providerConfig.AzureDevOps.Organization, providerConfig.AzureDevOps.API, providerConfig.AzureDevOps.TeamProject, providerConfig.AzureDevOps.AllBranches)
		if err != nil {
			return nil, fmt.Errorf("error initializing Azure Devops service: %w", err)
		}
	case providerConfig.Bitbucket != nil:
		appPassword, err := utils.GetSecretRef(ctx, g.client, providerConfig.Bitbucket.AppPasswordRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
		if err != nil {
			return nil, fmt.Errorf("error fetching Bitbucket cloud appPassword: %w", err)
		}
		provider, err = scm_provider.NewBitBucketCloudProvider(providerConfig.Bitbucket.Owner, providerConfig.Bitbucket.User, appPassword, providerConfig.Bitbucket.AllBranches)
		if err != nil {
			return nil, fmt.Errorf("error initializing Bitbucket cloud service: %w", err)
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

func (g *SCMProviderGenerator) githubProvider(ctx context.Context, github *argoprojiov1alpha1.SCMProviderGeneratorGithub, applicationSetInfo *argoprojiov1alpha1.ApplicationSet, baseHTTPClient *http.Client, scmSettings scmResolvedSettings) (scm_provider.SCMProviderService, error) {
	httpClient := baseHTTPClient
	if scmSettings.enableGitHubAPIMetrics {
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

	token, err := utils.GetSecretRef(ctx, g.client, github.TokenRef, applicationSetInfo.Namespace, scmSettings.tokenRefStrictMode)
	if err != nil {
		return nil, fmt.Errorf("error fetching Github token: %w", err)
	}
	return scm_provider.NewGithubProvider(github.Organization, token, github.API, github.AllBranches, github.ExcludeArchivedRepos, httpClient)
}

func newSCMHTTPClient(scmProxyURL, scmNoProxy string) *http.Client {
	if scmProxyURL == "" {
		return &http.Client{}
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = proxy.GetCallback(scmProxyURL, scmNoProxy)
	return &http.Client{Transport: tr}
}
