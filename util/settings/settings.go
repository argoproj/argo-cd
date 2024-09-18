package settings

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	enginecache "github.com/argoproj/gitops-engine/pkg/cache"
	timeutil "github.com/argoproj/pkg/time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/settings/oidc"
	"github.com/argoproj/argo-cd/v2/util"
	"github.com/argoproj/argo-cd/v2/util/crypto"
	"github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/password"
	tlsutil "github.com/argoproj/argo-cd/v2/util/tls"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// URL is the externally facing URL users will visit to reach Argo CD.
	// The value here is used when configuring SSO. Omitting this value will disable SSO.
	URL string `json:"url,omitempty"`
	// URLs is a list of externally facing URLs users will visit to reach Argo CD.
	// The value here is used when configuring SSO reachable from multiple domains.
	AdditionalURLs []string `json:"additionalUrls,omitempty"`
	// Indicates if status badge is enabled or not.
	StatusBadgeEnabled bool `json:"statusBadgeEnable"`
	// Indicates if status badge custom root URL should be used.
	StatusBadgeRootUrl string `json:"statusBadgeRootUrl,omitempty"`
	// DexConfig contains portions of a dex config yaml
	DexConfig string `json:"dexConfig,omitempty"`
	// OIDCConfigRAW holds OIDC configuration as a raw string
	OIDCConfigRAW string `json:"oidcConfig,omitempty"`
	// ServerSignature holds the key used to generate JWT tokens.
	ServerSignature []byte `json:"serverSignature,omitempty"`
	// Certificate holds the certificate/private key for the Argo CD API server.
	// If nil, will run insecure without TLS.
	Certificate *tls.Certificate `json:"-"`
	// CertificateIsExternal indicates whether Certificate was loaded from external secret
	CertificateIsExternal bool `json:"-"`
	// WebhookGitLabSecret holds the shared secret for authenticating GitHub webhook events
	WebhookGitHubSecret string `json:"webhookGitHubSecret,omitempty"`
	// WebhookGitLabSecret holds the shared secret for authenticating GitLab webhook events
	WebhookGitLabSecret string `json:"webhookGitLabSecret,omitempty"`
	// WebhookBitbucketUUID holds the UUID for authenticating Bitbucket webhook events
	WebhookBitbucketUUID string `json:"webhookBitbucketUUID,omitempty"`
	// WebhookBitbucketServerSecret holds the shared secret for authenticating BitbucketServer webhook events
	WebhookBitbucketServerSecret string `json:"webhookBitbucketServerSecret,omitempty"`
	// WebhookGogsSecret holds the shared secret for authenticating Gogs webhook events
	WebhookGogsSecret string `json:"webhookGogsSecret,omitempty"`
	// WebhookAzureDevOpsUsername holds the username for authenticating Azure DevOps webhook events
	WebhookAzureDevOpsUsername string `json:"webhookAzureDevOpsUsername,omitempty"`
	// WebhookAzureDevOpsPassword holds the password for authenticating Azure DevOps webhook events
	WebhookAzureDevOpsPassword string `json:"webhookAzureDevOpsPassword,omitempty"`
	// Secrets holds all secrets in argocd-secret as a map[string]string
	Secrets map[string]string `json:"secrets,omitempty"`
	// KustomizeBuildOptions is a string of kustomize build parameters
	KustomizeBuildOptions string `json:"kustomizeBuildOptions,omitempty"`
	// Indicates if anonymous user is enabled or not
	AnonymousUserEnabled bool `json:"anonymousUserEnabled,omitempty"`
	// Specifies token expiration duration
	UserSessionDuration time.Duration `json:"userSessionDuration,omitempty"`
	// UiCssURL local or remote path to user-defined CSS to customize ArgoCD UI
	UiCssURL string `json:"uiCssURL,omitempty"`
	// Content of UI Banner
	UiBannerContent string `json:"uiBannerContent,omitempty"`
	// URL for UI Banner
	UiBannerURL string `json:"uiBannerURL,omitempty"`
	// Make Banner permanent and not closeable
	UiBannerPermanent bool `json:"uiBannerPermanent,omitempty"`
	// Position of UI Banner
	UiBannerPosition string `json:"uiBannerPosition,omitempty"`
	// PasswordPattern for password regular expression
	PasswordPattern string `json:"passwordPattern,omitempty"`
	// BinaryUrls contains the URLs for downloading argocd binaries
	BinaryUrls map[string]string `json:"binaryUrls,omitempty"`
	// InClusterEnabled indicates whether to allow in-cluster server address
	InClusterEnabled bool `json:"inClusterEnabled"`
	// ServerRBACLogEnforceEnable temporary var indicates whether rbac will be enforced on logs
	ServerRBACLogEnforceEnable bool `json:"serverRBACLogEnforceEnable"`
	// MaxPodLogsToRender the maximum number of pod logs to render
	MaxPodLogsToRender int64 `json:"maxPodLogsToRender"`
	// ExecEnabled indicates whether the UI exec feature is enabled
	ExecEnabled bool `json:"execEnabled"`
	// ExecShells restricts which shells are allowed for `exec` and in which order they are tried
	ExecShells []string `json:"execShells"`
	// TrackingMethod defines the resource tracking method to be used
	TrackingMethod string `json:"application.resourceTrackingMethod,omitempty"`
	// OIDCTLSInsecureSkipVerify determines whether certificate verification is skipped when verifying tokens with the
	// configured OIDC provider (either external or the bundled Dex instance). Setting this to `true` will cause JWT
	// token verification to pass despite the OIDC provider having an invalid certificate. Only set to `true` if you
	// understand the risks.
	OIDCTLSInsecureSkipVerify bool `json:"oidcTLSInsecureSkipVerify"`
	// AppsInAnyNamespaceEnabled indicates whether applications are allowed to be created in any namespace
	AppsInAnyNamespaceEnabled bool `json:"appsInAnyNamespaceEnabled"`
	// ExtensionConfig configurations related to ArgoCD proxy extensions. The value
	// is a yaml string defined in extension.ExtensionConfigs struct.
	ExtensionConfig string `json:"extensionConfig,omitempty"`
	// ImpersonationEnabled indicates whether Application sync privileges can be decoupled from control plane
	// privileges using impersonation
	ImpersonationEnabled bool `json:"impersonationEnabled"`
}

type GoogleAnalytics struct {
	TrackingID     string `json:"trackingID,omitempty"`
	AnonymizeUsers bool   `json:"anonymizeUsers,omitempty"`
}

type GlobalProjectSettings struct {
	ProjectName   string               `json:"projectName,omitempty"`
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// Help settings
type Help struct {
	// the URL for getting chat help, this will typically be your Slack channel for support
	ChatURL string `json:"chatUrl,omitempty"`
	// the text for getting chat help, defaults to "Chat now!"
	ChatText string `json:"chatText,omitempty"`
	// the URLs for downloading argocd binaries
	BinaryURLs map[string]string `json:"binaryUrl,omitempty"`
}

// oidcConfig is the same as the public OIDCConfig, except the public one excludes the AllowedAudiences and the
// SkipAudienceCheckWhenTokenHasNoAudience fields.
// AllowedAudiences should be accessed via ArgoCDSettings.OAuth2AllowedAudiences.
// SkipAudienceCheckWhenTokenHasNoAudience should be accessed via ArgoCDSettings.SkipAudienceCheckWhenTokenHasNoAudience.
type oidcConfig struct {
	OIDCConfig
	AllowedAudiences                        []string `json:"allowedAudiences,omitempty"`
	SkipAudienceCheckWhenTokenHasNoAudience *bool    `json:"skipAudienceCheckWhenTokenHasNoAudience,omitempty"`
}

func (o *oidcConfig) toExported() *OIDCConfig {
	if o == nil {
		return nil
	}
	return &OIDCConfig{
		Name:                     o.Name,
		Issuer:                   o.Issuer,
		ClientID:                 o.ClientID,
		ClientSecret:             o.ClientSecret,
		CLIClientID:              o.CLIClientID,
		UserInfoPath:             o.UserInfoPath,
		EnableUserInfoGroups:     o.EnableUserInfoGroups,
		UserInfoCacheExpiration:  o.UserInfoCacheExpiration,
		RequestedScopes:          o.RequestedScopes,
		RequestedIDTokenClaims:   o.RequestedIDTokenClaims,
		LogoutURL:                o.LogoutURL,
		RootCA:                   o.RootCA,
		EnablePKCEAuthentication: o.EnablePKCEAuthentication,
		DomainHint:               o.DomainHint,
	}
}

type OIDCConfig struct {
	Name                     string                 `json:"name,omitempty"`
	Issuer                   string                 `json:"issuer,omitempty"`
	ClientID                 string                 `json:"clientID,omitempty"`
	ClientSecret             string                 `json:"clientSecret,omitempty"`
	CLIClientID              string                 `json:"cliClientID,omitempty"`
	EnableUserInfoGroups     bool                   `json:"enableUserInfoGroups,omitempty"`
	UserInfoPath             string                 `json:"userInfoPath,omitempty"`
	UserInfoCacheExpiration  string                 `json:"userInfoCacheExpiration,omitempty"`
	RequestedScopes          []string               `json:"requestedScopes,omitempty"`
	RequestedIDTokenClaims   map[string]*oidc.Claim `json:"requestedIDTokenClaims,omitempty"`
	LogoutURL                string                 `json:"logoutURL,omitempty"`
	RootCA                   string                 `json:"rootCA,omitempty"`
	EnablePKCEAuthentication bool                   `json:"enablePKCEAuthentication,omitempty"`
	DomainHint               string                 `json:"domainHint,omitempty"`
}

// DEPRECATED. Helm repository credentials are now managed using RepoCredentials
type HelmRepoCredentials struct {
	URL            string                   `json:"url,omitempty"`
	Name           string                   `json:"name,omitempty"`
	UsernameSecret *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	PasswordSecret *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	CertSecret     *apiv1.SecretKeySelector `json:"certSecret,omitempty"`
	KeySecret      *apiv1.SecretKeySelector `json:"keySecret,omitempty"`
}

// KustomizeVersion holds information about additional Kustomize version
type KustomizeVersion struct {
	// Name holds Kustomize version name
	Name string
	// Path holds corresponding binary path
	Path string
	// BuildOptions that are specific to Kustomize version
	BuildOptions string
}

// KustomizeSettings holds kustomize settings
type KustomizeSettings struct {
	BuildOptions string
	Versions     []KustomizeVersion
}

var (
	ByClusterURLIndexer     = "byClusterURL"
	byClusterURLIndexerFunc = func(obj interface{}) ([]string, error) {
		s, ok := obj.(*apiv1.Secret)
		if !ok {
			return nil, nil
		}
		if s.Labels == nil || s.Labels[common.LabelKeySecretType] != common.LabelValueSecretTypeCluster {
			return nil, nil
		}
		if s.Data == nil {
			return nil, nil
		}
		if url, ok := s.Data["server"]; ok {
			return []string{strings.TrimRight(string(url), "/")}, nil
		}
		return nil, nil
	}
	ByClusterNameIndexer     = "byClusterName"
	byClusterNameIndexerFunc = func(obj interface{}) ([]string, error) {
		s, ok := obj.(*apiv1.Secret)
		if !ok {
			return nil, nil
		}
		if s.Labels == nil || s.Labels[common.LabelKeySecretType] != common.LabelValueSecretTypeCluster {
			return nil, nil
		}
		if s.Data == nil {
			return nil, nil
		}
		if name, ok := s.Data["name"]; ok {
			return []string{string(name)}, nil
		}
		return nil, nil
	}
	ByProjectClusterIndexer = "byProjectCluster"
	ByProjectRepoIndexer    = "byProjectRepo"
	byProjectIndexerFunc    = func(secretType string) func(obj interface{}) ([]string, error) {
		return func(obj interface{}) ([]string, error) {
			s, ok := obj.(*apiv1.Secret)
			if !ok {
				return nil, nil
			}
			if s.Labels == nil || s.Labels[common.LabelKeySecretType] != secretType {
				return nil, nil
			}
			if s.Data == nil {
				return nil, nil
			}
			if project, ok := s.Data["project"]; ok {
				return []string{string(project)}, nil
			}
			return nil, nil
		}
	}
)

func (ks *KustomizeSettings) GetOptions(source v1alpha1.ApplicationSource) (*v1alpha1.KustomizeOptions, error) {
	binaryPath := ""
	buildOptions := ""
	if source.Kustomize != nil && source.Kustomize.Version != "" {
		for _, ver := range ks.Versions {
			if ver.Name == source.Kustomize.Version {
				// add version specific path and build options
				binaryPath = ver.Path
				buildOptions = ver.BuildOptions
				break
			}
		}
		if binaryPath == "" {
			return nil, fmt.Errorf("kustomize version %s is not registered", source.Kustomize.Version)
		}
	} else {
		// add build options for the default version
		buildOptions = ks.BuildOptions
	}
	return &v1alpha1.KustomizeOptions{
		BuildOptions: buildOptions,
		BinaryPath:   binaryPath,
	}, nil
}

// Credentials for accessing a Git repository
type Repository struct {
	// The URL to the repository
	URL string `json:"url,omitempty"`
	// the type of the repo, "git" or "helm", assumed to be "git" if empty or absent
	Type string `json:"type,omitempty"`
	// helm only
	Name string `json:"name,omitempty"`
	// Name of the secret storing the username used to access the repo
	UsernameSecret *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	// Name of the secret storing the password used to access the repo
	PasswordSecret *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	// Name of the secret storing the SSH private key used to access the repo. Git only
	SSHPrivateKeySecret *apiv1.SecretKeySelector `json:"sshPrivateKeySecret,omitempty"`
	// Whether to connect the repository in an insecure way (deprecated)
	InsecureIgnoreHostKey bool `json:"insecureIgnoreHostKey,omitempty"`
	// Whether to connect the repository in an insecure way
	Insecure bool `json:"insecure,omitempty"`
	// Whether the repo is git-lfs enabled. Git only.
	EnableLFS bool `json:"enableLfs,omitempty"`
	// Name of the secret storing the TLS client cert data
	TLSClientCertDataSecret *apiv1.SecretKeySelector `json:"tlsClientCertDataSecret,omitempty"`
	// Name of the secret storing the TLS client cert's key data
	TLSClientCertKeySecret *apiv1.SecretKeySelector `json:"tlsClientCertKeySecret,omitempty"`
	// Whether the repo is helm-oci enabled. Git only.
	EnableOci bool `json:"enableOci,omitempty"`
	// Github App Private Key PEM data
	GithubAppPrivateKeySecret *apiv1.SecretKeySelector `json:"githubAppPrivateKeySecret,omitempty"`
	// Github App ID of the app used to access the repo
	GithubAppId int64 `json:"githubAppID,omitempty"`
	// Github App Installation ID of the installed GitHub App
	GithubAppInstallationId int64 `json:"githubAppInstallationID,omitempty"`
	// Github App Enterprise base url if empty will default to https://api.github.com
	GithubAppEnterpriseBaseURL string `json:"githubAppEnterpriseBaseUrl,omitempty"`
	// Proxy specifies the HTTP/HTTPS proxy used to access the repo
	Proxy string `json:"proxy,omitempty"`
	// NoProxy specifies a list of targets where the proxy isn't used, applies only in cases where the proxy is applied
	NoProxy string `json:"noProxy,omitempty"`
	// GCPServiceAccountKey specifies the service account key in JSON format to be used for getting credentials to Google Cloud Source repos
	GCPServiceAccountKey *apiv1.SecretKeySelector `json:"gcpServiceAccountKey,omitempty"`
	// ForceHttpBasicAuth determines whether Argo CD should force use of basic auth for HTTP connected repositories
	ForceHttpBasicAuth bool `json:"forceHttpBasicAuth,omitempty"`
}

// Credential template for accessing repositories
type RepositoryCredentials struct {
	// The URL pattern the repository URL has to match
	URL string `json:"url,omitempty"`
	// Name of the secret storing the username used to access the repo
	UsernameSecret *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	// Name of the secret storing the password used to access the repo
	PasswordSecret *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	// Name of the secret storing the SSH private key used to access the repo. Git only
	SSHPrivateKeySecret *apiv1.SecretKeySelector `json:"sshPrivateKeySecret,omitempty"`
	// Name of the secret storing the TLS client cert data
	TLSClientCertDataSecret *apiv1.SecretKeySelector `json:"tlsClientCertDataSecret,omitempty"`
	// Name of the secret storing the TLS client cert's key data
	TLSClientCertKeySecret *apiv1.SecretKeySelector `json:"tlsClientCertKeySecret,omitempty"`
	// Github App Private Key PEM data
	GithubAppPrivateKeySecret *apiv1.SecretKeySelector `json:"githubAppPrivateKeySecret,omitempty"`
	// Github App ID of the app used to access the repo
	GithubAppId int64 `json:"githubAppID,omitempty"`
	// Github App Installation ID of the installed GitHub App
	GithubAppInstallationId int64 `json:"githubAppInstallationID,omitempty"`
	// Github App Enterprise base url if empty will default to https://api.github.com
	GithubAppEnterpriseBaseURL string `json:"githubAppEnterpriseBaseUrl,omitempty"`
	// EnableOCI specifies whether helm-oci support should be enabled for this repo
	EnableOCI bool `json:"enableOCI,omitempty"`
	// the type of the repositoryCredentials, "git" or "helm", assumed to be "git" if empty or absent
	Type string `json:"type,omitempty"`
	// GCPServiceAccountKey specifies the service account key in JSON format to be used for getting credentials to Google Cloud Source repos
	GCPServiceAccountKey *apiv1.SecretKeySelector `json:"gcpServiceAccountKey,omitempty"`
	// ForceHttpBasicAuth determines whether Argo CD should force use of basic auth for HTTP connected repositories
	ForceHttpBasicAuth bool `json:"forceHttpBasicAuth,omitempty"`
}

// DeepLink structure
type DeepLink struct {
	// URL that the deep link will redirect to
	URL string `json:"url"`
	// Title that will be displayed in the UI corresponding to that link
	Title string `json:"title"`
	// Description (optional) a description for what the deep link is about
	Description *string `json:"description,omitempty"`
	// IconClass (optional) a font-awesome icon class to be used when displaying the links in dropdown menus.
	IconClass *string `json:"icon.class,omitempty"`
	// Condition (optional) a conditional statement depending on which the deep link shall be rendered
	Condition *string `json:"if,omitempty"`
}

const (
	// settingServerSignatureKey designates the key for a server secret key inside a Kubernetes secret.
	settingServerSignatureKey = "server.secretkey"
	// gaTrackingID holds Google Analytics tracking id
	gaTrackingID = "ga.trackingid"
	// the URL for getting chat help, this will typically be your Slack channel for support
	helpChatURL = "help.chatUrl"
	// the text for getting chat help, defaults to "Chat now!"
	helpChatText = "help.chatText"
	// gaAnonymizeUsers specifies if user ids should be anonymized (hashed) before sending to Google Analytics. True unless value is set to 'false'
	gaAnonymizeUsers = "ga.anonymizeusers"
	// settingServerCertificate designates the key for the public cert used in TLS
	settingServerCertificate = "tls.crt"
	// settingServerPrivateKey designates the key for the private key used in TLS
	settingServerPrivateKey = "tls.key"
	// settingURLKey designates the key where Argo CD's external URL is set
	settingURLKey = "url"
	// settingAdditionalUrlsKey designates the key where Argo CD's additional external URLs are set
	settingAdditionalUrlsKey = "additionalUrls"
	// repositoriesKey designates the key where ArgoCDs repositories list is set
	repositoriesKey = "repositories"
	// repositoryCredentialsKey designates the key where ArgoCDs repositories credentials list is set
	repositoryCredentialsKey = "repository.credentials"
	// helmRepositoriesKey designates the key where list of helm repositories is set
	helmRepositoriesKey = "helm.repositories"
	// settingDexConfigKey designates the key for the dex config
	settingDexConfigKey = "dex.config"
	// settingsOIDCConfigKey designates the key for OIDC config
	settingsOIDCConfigKey = "oidc.config"
	// statusBadgeEnabledKey holds the key which enables of disables status badge feature
	statusBadgeEnabledKey = "statusbadge.enabled"
	// statusBadgeRootUrlKey holds the key for the root badge URL override
	statusBadgeRootUrlKey = "statusbadge.url"
	// settingsWebhookGitHubSecret is the key for the GitHub shared webhook secret
	settingsWebhookGitHubSecretKey = "webhook.github.secret"
	// settingsWebhookGitLabSecret is the key for the GitLab shared webhook secret
	settingsWebhookGitLabSecretKey = "webhook.gitlab.secret"
	// settingsWebhookBitbucketUUID is the key for Bitbucket webhook UUID
	settingsWebhookBitbucketUUIDKey = "webhook.bitbucket.uuid"
	// settingsWebhookBitbucketServerSecret is the key for BitbucketServer webhook secret
	settingsWebhookBitbucketServerSecretKey = "webhook.bitbucketserver.secret"
	// settingsWebhookGogsSecret is the key for Gogs webhook secret
	settingsWebhookGogsSecretKey = "webhook.gogs.secret"
	// settingsWebhookAzureDevOpsUsernameKey is the key for Azure DevOps webhook username
	settingsWebhookAzureDevOpsUsernameKey = "webhook.azuredevops.username"
	// settingsWebhookAzureDevOpsPasswordKey is the key for Azure DevOps webhook password
	settingsWebhookAzureDevOpsPasswordKey = "webhook.azuredevops.password"
	// settingsWebhookMaxPayloadSize is the key for the maximum payload size for webhooks in MB
	settingsWebhookMaxPayloadSizeMB = "webhook.maxPayloadSizeMB"
	// settingsApplicationInstanceLabelKey is the key to configure injected app instance label key
	settingsApplicationInstanceLabelKey = "application.instanceLabelKey"
	// settingsResourceTrackingMethodKey is the key to configure tracking method for application resources
	settingsResourceTrackingMethodKey = "application.resourceTrackingMethod"
	// resourcesCustomizationsKey is the key to the map of resource overrides
	resourceCustomizationsKey = "resource.customizations"
	// resourceExclusions is the key to the list of excluded resources
	resourceExclusionsKey = "resource.exclusions"
	// resourceInclusions is the key to the list of explicitly watched resources
	resourceInclusionsKey = "resource.inclusions"
	// resourceIgnoreResourceUpdatesEnabledKey is the key to a boolean determining whether the resourceIgnoreUpdates feature is enabled
	resourceIgnoreResourceUpdatesEnabledKey = "resource.ignoreResourceUpdatesEnabled"
	// resourceCustomLabelKey is the key to a custom label to show in node info, if present
	resourceCustomLabelsKey = "resource.customLabels"
	// resourceIncludeEventLabelKeys is the key to labels to be added onto Application k8s events if present on an Application or it's AppProject. Supports wildcard.
	resourceIncludeEventLabelKeys = "resource.includeEventLabelKeys"
	// resourceExcludeEventLabelKeys is the key to labels to be excluded from adding onto Application's k8s events. Supports wildcard.
	resourceExcludeEventLabelKeys = "resource.excludeEventLabelKeys"
	// kustomizeBuildOptionsKey is a string of kustomize build parameters
	kustomizeBuildOptionsKey = "kustomize.buildOptions"
	// kustomizeVersionKeyPrefix is a kustomize version key prefix
	kustomizeVersionKeyPrefix = "kustomize.version"
	// kustomizePathPrefixKey is a kustomize path for a specific version
	kustomizePathPrefixKey = "kustomize.path"
	// anonymousUserEnabledKey is the key which enables or disables anonymous user
	anonymousUserEnabledKey = "users.anonymous.enabled"
	// userSessionDurationKey is the key which specifies token expiration duration
	userSessionDurationKey = "users.session.duration"
	// diffOptions is the key where diff options are configured
	resourceCompareOptionsKey = "resource.compareoptions"
	// settingUiCssURLKey designates the key for user-defined CSS URL for UI customization
	settingUiCssURLKey = "ui.cssurl"
	// settingUiBannerContentKey designates the key for content of user-defined info banner for UI
	settingUiBannerContentKey = "ui.bannercontent"
	// settingUiBannerURLKey designates the key for the link for user-defined info banner for UI
	settingUiBannerURLKey = "ui.bannerurl"
	// settingUiBannerPermanentKey designates the key for whether the banner is permanent and not closeable
	settingUiBannerPermanentKey = "ui.bannerpermanent"
	// settingUiBannerPositionKey designates the key for the position of the banner
	settingUiBannerPositionKey = "ui.bannerposition"
	// settingsBinaryUrlsKey designates the key for the argocd binary URLs
	settingsBinaryUrlsKey = "help.download"
	// globalProjectsKey designates the key for global project settings
	globalProjectsKey = "globalProjects"
	// initialPasswordSecretName is the name of the secret that will hold the initial admin password
	initialPasswordSecretName = "argocd-initial-admin-secret"
	// initialPasswordSecretField is the name of the field in initialPasswordSecretName to store the password
	initialPasswordSecretField = "password"
	// initialPasswordLength defines the length of the generated initial password
	initialPasswordLength = 16
	// externalServerTLSSecretName defines the name of the external secret holding the server's TLS certificate
	externalServerTLSSecretName = "argocd-server-tls"
	// partOfArgoCDSelector holds label selector that should be applied to config maps and secrets used to manage Argo CD
	partOfArgoCDSelector = "app.kubernetes.io/part-of=argocd"
	// settingsPasswordPatternKey is the key to configure user password regular expression
	settingsPasswordPatternKey = "passwordPattern"
	// inClusterEnabledKey is the key to configure whether to allow in-cluster server address
	inClusterEnabledKey = "cluster.inClusterEnabled"
	// settingsServerRBACLogEnforceEnable is the key to configure whether logs RBAC enforcement is enabled
	settingsServerRBACLogEnforceEnableKey = "server.rbac.log.enforce.enable"
	// MaxPodLogsToRender the maximum number of pod logs to render
	settingsMaxPodLogsToRender = "server.maxPodLogsToRender"
	// helmValuesFileSchemesKey is the key to configure the list of supported helm values file schemas
	helmValuesFileSchemesKey = "helm.valuesFileSchemes"
	// execEnabledKey is the key to configure whether the UI exec feature is enabled
	execEnabledKey = "exec.enabled"
	// execShellsKey is the key to configure which shells are allowed for `exec` and in what order they are tried
	execShellsKey = "exec.shells"
	// oidcTLSInsecureSkipVerifyKey is the key to configure whether TLS cert verification is skipped for OIDC connections
	oidcTLSInsecureSkipVerifyKey = "oidc.tls.insecure.skip.verify"
	// ApplicationDeepLinks is the application deep link key
	ApplicationDeepLinks = "application.links"
	// ProjectDeepLinks is the project deep link key
	ProjectDeepLinks = "project.links"
	// ResourceDeepLinks is the resource deep link key
	ResourceDeepLinks = "resource.links"
	extensionConfig   = "extension.config"
	// RespectRBAC is the key to configure argocd to respect rbac while watching for resources
	RespectRBAC            = "resource.respectRBAC"
	RespectRBACValueStrict = "strict"
	RespectRBACValueNormal = "normal"
	// impersonationEnabledKey is the key to configure whether the application sync decoupling through impersonation feature is enabled
	impersonationEnabledKey = "application.sync.impersonation.enabled"
)

const (
	// default max webhook payload size is 1GB
	defaultMaxWebhookPayloadSize = int64(1) * 1024 * 1024 * 1024
)

var sourceTypeToEnableGenerationKey = map[v1alpha1.ApplicationSourceType]string{
	v1alpha1.ApplicationSourceTypeKustomize: "kustomize.enable",
	v1alpha1.ApplicationSourceTypeHelm:      "helm.enable",
	v1alpha1.ApplicationSourceTypeDirectory: "jsonnet.enable",
}

// SettingsManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type SettingsManager struct {
	ctx             context.Context
	clientset       kubernetes.Interface
	secrets         v1listers.SecretLister
	secretsInformer cache.SharedIndexInformer
	configmaps      v1listers.ConfigMapLister
	namespace       string
	// subscribers is a list of subscribers to settings updates
	subscribers []chan<- *ArgoCDSettings
	// mutex protects concurrency sensitive parts of settings manager: access to subscribers list and initialization flag
	mutex                 *sync.Mutex
	initContextCancel     func()
	reposCache            []Repository
	repoCredsCache        []RepositoryCredentials
	reposOrClusterChanged func()
}

type incompleteSettingsError struct {
	message string
}

type IgnoreStatus string

const (
	// IgnoreResourceStatusInCRD ignores status changes for all CRDs
	IgnoreResourceStatusInCRD IgnoreStatus = "crd"
	// IgnoreResourceStatusInAll ignores status changes for all resources
	IgnoreResourceStatusInAll IgnoreStatus = "all"
	// IgnoreResourceStatusInNone ignores status changes for no resources
	IgnoreResourceStatusInNone IgnoreStatus = "off"
)

type ArgoCDDiffOptions struct {
	IgnoreAggregatedRoles bool `json:"ignoreAggregatedRoles,omitempty"`

	// If set to true then differences caused by status are ignored.
	IgnoreResourceStatusField IgnoreStatus `json:"ignoreResourceStatusField,omitempty"`

	// If set to true then ignoreDifferences are applied to ignore application refresh on resource updates.
	IgnoreDifferencesOnResourceUpdates bool `json:"ignoreDifferencesOnResourceUpdates,omitempty"`
}

func (e *incompleteSettingsError) Error() string {
	return e.message
}

func (mgr *SettingsManager) onRepoOrClusterChanged() {
	if mgr.reposOrClusterChanged != nil {
		go mgr.reposOrClusterChanged()
	}
}

func (mgr *SettingsManager) RespectRBAC() (int, error) {
	cm, err := mgr.getConfigMap()
	if err != nil {
		return enginecache.RespectRbacDisabled, err
	}
	if cm.Data[RespectRBAC] != "" {
		switch cm.Data[RespectRBAC] {
		case RespectRBACValueNormal:
			return enginecache.RespectRbacNormal, nil
		case RespectRBACValueStrict:
			return enginecache.RespectRbacStrict, nil
		default:
			return enginecache.RespectRbacDisabled, fmt.Errorf("invalid value for %s: %s", RespectRBAC, cm.Data[RespectRBAC])
		}
	}
	return enginecache.RespectRbacDisabled, nil
}

func (mgr *SettingsManager) GetSecretsLister() (v1listers.SecretLister, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	return mgr.secrets, nil
}

func (mgr *SettingsManager) GetSecretsInformer() (cache.SharedIndexInformer, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, fmt.Errorf("error ensuring that the secrets manager is synced: %w", err)
	}
	return mgr.secretsInformer, nil
}

func (mgr *SettingsManager) updateSecret(callback func(*apiv1.Secret) error) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}
	argoCDSecret, err := mgr.secrets.Secrets(mgr.namespace).Get(common.ArgoCDSecretName)
	createSecret := false
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		argoCDSecret = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDSecretName,
			},
			Data: make(map[string][]byte),
		}
		createSecret = true
	}
	if argoCDSecret.Data == nil {
		argoCDSecret.Data = make(map[string][]byte)
	}

	updatedSecret := argoCDSecret.DeepCopy()
	err = callback(updatedSecret)
	if err != nil {
		return err
	}

	if !createSecret && reflect.DeepEqual(argoCDSecret.Data, updatedSecret.Data) {
		return nil
	}

	if createSecret {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Create(context.Background(), updatedSecret, metav1.CreateOptions{})
	} else {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(context.Background(), updatedSecret, metav1.UpdateOptions{})
	}
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

func (mgr *SettingsManager) updateConfigMap(callback func(*apiv1.ConfigMap) error) error {
	argoCDCM, err := mgr.getConfigMap()
	createCM := false
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		argoCDCM = &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDConfigMapName,
			},
		}
		createCM = true
	}
	if argoCDCM.Data == nil {
		argoCDCM.Data = make(map[string]string)
	}
	beforeUpdate := argoCDCM.DeepCopy()
	err = callback(argoCDCM)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(beforeUpdate.Data, argoCDCM.Data) {
		return nil
	}

	if createCM {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Create(context.Background(), argoCDCM, metav1.CreateOptions{})
	} else {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(context.Background(), argoCDCM, metav1.UpdateOptions{})
	}

	if err != nil {
		return err
	}

	mgr.invalidateCache()

	return mgr.ResyncInformers()
}

func (mgr *SettingsManager) getConfigMap() (*apiv1.ConfigMap, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, err
	}
	if argoCDCM.Data == nil {
		argoCDCM.Data = make(map[string]string)
	}
	return argoCDCM, err
}

// Returns the ConfigMap with the given name from the cluster.
// The ConfigMap must be labeled with "app.kubernetes.io/part-of: argocd" in
// order to be retrievable.
func (mgr *SettingsManager) GetConfigMapByName(configMapName string) (*apiv1.ConfigMap, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	configMap, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(configMapName)
	if err != nil {
		return nil, err
	}
	return configMap, err
}

func (mgr *SettingsManager) GetResourcesFilter() (*ResourcesFilter, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-cm: %w", err)
	}
	rf := &ResourcesFilter{}
	if value, ok := argoCDCM.Data[resourceInclusionsKey]; ok {
		includedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &includedResources)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling included resources %w", err)
		}
		rf.ResourceInclusions = includedResources
	}

	if value, ok := argoCDCM.Data[resourceExclusionsKey]; ok {
		excludedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &excludedResources)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling excluded resources %w", err)
		}
		rf.ResourceExclusions = excludedResources
	}
	return rf, nil
}

func (mgr *SettingsManager) GetAppInstanceLabelKey() (string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return "", err
	}
	label := argoCDCM.Data[settingsApplicationInstanceLabelKey]
	if label == "" {
		return common.LabelKeyAppInstance, nil
	}
	return label, nil
}

func (mgr *SettingsManager) GetTrackingMethod() (string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return "", err
	}
	return argoCDCM.Data[settingsResourceTrackingMethodKey], nil
}

func (mgr *SettingsManager) GetPasswordPattern() (string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return "", err
	}
	label := argoCDCM.Data[settingsPasswordPatternKey]
	if label == "" {
		return common.PasswordPatten, nil
	}
	return label, nil
}

func (mgr *SettingsManager) GetServerRBACLogEnforceEnable() (bool, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return false, err
	}

	if argoCDCM.Data[settingsServerRBACLogEnforceEnableKey] == "" {
		return false, nil
	}

	return strconv.ParseBool(argoCDCM.Data[settingsServerRBACLogEnforceEnableKey])
}

func (mgr *SettingsManager) GetMaxPodLogsToRender() (int64, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return 10, err
	}

	if argoCDCM.Data[settingsMaxPodLogsToRender] == "" {
		return 10, nil
	}

	return strconv.ParseInt(argoCDCM.Data[settingsMaxPodLogsToRender], 10, 64)
}

func (mgr *SettingsManager) GetDeepLinks(deeplinkType string) ([]DeepLink, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-cm: %w", err)
	}
	deepLinks := make([]DeepLink, 0)
	if value, ok := argoCDCM.Data[deeplinkType]; ok {
		err := yaml.Unmarshal([]byte(value), &deepLinks)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling deep links %w", err)
		}
	}
	return deepLinks, nil
}

func (mgr *SettingsManager) GetEnabledSourceTypes() (map[string]bool, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get argo-cd config map: %w", err)
	}
	res := map[string]bool{}
	for sourceType := range sourceTypeToEnableGenerationKey {
		res[string(sourceType)] = true
	}
	for sourceType, key := range sourceTypeToEnableGenerationKey {
		if val, ok := argoCDCM.Data[key]; ok && val != "" {
			res[string(sourceType)] = val == "true"
		}
	}
	// plugin based manifest generation cannot be disabled
	res[string(v1alpha1.ApplicationSourceTypePlugin)] = true
	return res, nil
}

func (mgr *SettingsManager) GetIgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	compareOptions, err := mgr.GetResourceCompareOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to get compare options: %w", err)
	}

	resourceOverrides, err := mgr.GetResourceOverrides()
	if err != nil {
		return nil, fmt.Errorf("failed to get resource overrides: %w", err)
	}

	for k, v := range resourceOverrides {
		resourceUpdates := v.IgnoreResourceUpdates
		if compareOptions.IgnoreDifferencesOnResourceUpdates {
			resourceUpdates.JQPathExpressions = append(resourceUpdates.JQPathExpressions, v.IgnoreDifferences.JQPathExpressions...)
			resourceUpdates.JSONPointers = append(resourceUpdates.JSONPointers, v.IgnoreDifferences.JSONPointers...)
			resourceUpdates.ManagedFieldsManagers = append(resourceUpdates.ManagedFieldsManagers, v.IgnoreDifferences.ManagedFieldsManagers...)
		}
		// Set the IgnoreDifferences because these are the overrides used by Normalizers
		v.IgnoreDifferences = resourceUpdates
		v.IgnoreResourceUpdates = v1alpha1.OverrideIgnoreDiff{}
		resourceOverrides[k] = v
	}

	if compareOptions.IgnoreDifferencesOnResourceUpdates {
		log.Info("Using diffing customizations to ignore resource updates")
	}

	addIgnoreDiffItemOverrideToGK(resourceOverrides, "*/*", "/metadata/resourceVersion")
	addIgnoreDiffItemOverrideToGK(resourceOverrides, "*/*", "/metadata/generation")
	addIgnoreDiffItemOverrideToGK(resourceOverrides, "*/*", "/metadata/managedFields")

	return resourceOverrides, nil
}

func (mgr *SettingsManager) GetIsIgnoreResourceUpdatesEnabled() (bool, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return false, fmt.Errorf("error retrieving config map: %w", err)
	}

	if argoCDCM.Data[resourceIgnoreResourceUpdatesEnabledKey] == "" {
		return false, nil
	}

	return strconv.ParseBool(argoCDCM.Data[resourceIgnoreResourceUpdatesEnabledKey])
}

// GetResourceOverrides loads Resource Overrides from argocd-cm ConfigMap
func (mgr *SettingsManager) GetResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving config map: %w", err)
	}
	resourceOverrides := map[string]v1alpha1.ResourceOverride{}
	if value, ok := argoCDCM.Data[resourceCustomizationsKey]; ok && value != "" {
		err := yaml.Unmarshal([]byte(value), &resourceOverrides)
		if err != nil {
			return nil, err
		}
	}

	err = mgr.appendResourceOverridesFromSplitKeys(argoCDCM.Data, resourceOverrides)
	if err != nil {
		return nil, err
	}

	var diffOptions ArgoCDDiffOptions
	if value, ok := argoCDCM.Data[resourceCompareOptionsKey]; ok {
		err := yaml.Unmarshal([]byte(value), &diffOptions)
		if err != nil {
			return nil, err
		}
	}

	crdGK := "apiextensions.k8s.io/CustomResourceDefinition"
	crdPrsvUnkn := "/spec/preserveUnknownFields"

	switch diffOptions.IgnoreResourceStatusField {
	case "", "crd":
		addStatusOverrideToGK(resourceOverrides, crdGK)
		addIgnoreDiffItemOverrideToGK(resourceOverrides, crdGK, crdPrsvUnkn)
	case "all":
		addStatusOverrideToGK(resourceOverrides, "*/*")
		log.Info("Ignore status for all objects")

	case "off", "false":
		log.Info("Not ignoring status for any object")

	default:
		addStatusOverrideToGK(resourceOverrides, crdGK)
		log.Warnf("Unrecognized value for ignoreResourceStatusField - %s, ignore status for CustomResourceDefinitions", diffOptions.IgnoreResourceStatusField)
	}

	return resourceOverrides, nil
}

func addStatusOverrideToGK(resourceOverrides map[string]v1alpha1.ResourceOverride, groupKind string) {
	if val, ok := resourceOverrides[groupKind]; ok {
		val.IgnoreDifferences.JSONPointers = append(val.IgnoreDifferences.JSONPointers, "/status")
		resourceOverrides[groupKind] = val
	} else {
		resourceOverrides[groupKind] = v1alpha1.ResourceOverride{
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/status"}},
		}
	}
}

func addIgnoreDiffItemOverrideToGK(resourceOverrides map[string]v1alpha1.ResourceOverride, groupKind, ignoreItem string) {
	if val, ok := resourceOverrides[groupKind]; ok {
		val.IgnoreDifferences.JSONPointers = append(val.IgnoreDifferences.JSONPointers, ignoreItem)
		resourceOverrides[groupKind] = val
	} else {
		resourceOverrides[groupKind] = v1alpha1.ResourceOverride{
			IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{ignoreItem}},
		}
	}
}

func (mgr *SettingsManager) appendResourceOverridesFromSplitKeys(cmData map[string]string, resourceOverrides map[string]v1alpha1.ResourceOverride) error {
	for k, v := range cmData {
		if !strings.HasPrefix(k, resourceCustomizationsKey) {
			continue
		}

		// config map key should be of format resource.customizations.<type>.<group-kind>
		parts := strings.SplitN(k, ".", 4)
		if len(parts) < 4 {
			continue
		}

		overrideKey, err := convertToOverrideKey(parts[3])
		if err != nil {
			return err
		}

		if overrideKey == "all" {
			overrideKey = "*/*"
		}

		overrideVal, ok := resourceOverrides[overrideKey]
		if !ok {
			overrideVal = v1alpha1.ResourceOverride{}
		}

		customizationType := parts[2]
		switch customizationType {
		case "health":
			overrideVal.HealthLua = v
		case "useOpenLibs":
			useOpenLibs, err := strconv.ParseBool(v)
			if err != nil {
				return err
			}
			overrideVal.UseOpenLibs = useOpenLibs
		case "actions":
			overrideVal.Actions = v
		case "ignoreDifferences":
			overrideIgnoreDiff := v1alpha1.OverrideIgnoreDiff{}
			err := yaml.Unmarshal([]byte(v), &overrideIgnoreDiff)
			if err != nil {
				return err
			}
			overrideVal.IgnoreDifferences = overrideIgnoreDiff
		case "ignoreResourceUpdates":
			overrideIgnoreUpdate := v1alpha1.OverrideIgnoreDiff{}
			err := yaml.Unmarshal([]byte(v), &overrideIgnoreUpdate)
			if err != nil {
				return err
			}
			overrideVal.IgnoreResourceUpdates = overrideIgnoreUpdate
		case "knownTypeFields":
			var knownTypeFields []v1alpha1.KnownTypeField
			err := yaml.Unmarshal([]byte(v), &knownTypeFields)
			if err != nil {
				return err
			}
			overrideVal.KnownTypeFields = knownTypeFields
		default:
			return fmt.Errorf("resource customization type %s not supported", customizationType)
		}
		resourceOverrides[overrideKey] = overrideVal
	}
	return nil
}

// Convert group-kind format to <group/kind>, allowed key format examples
// resource.customizations.health.cert-manager.io_Certificate
// resource.customizations.health.Certificate
func convertToOverrideKey(groupKind string) (string, error) {
	parts := strings.Split(groupKind, "_")
	if len(parts) == 2 {
		return fmt.Sprintf("%s/%s", parts[0], parts[1]), nil
	} else if len(parts) == 1 && groupKind != "" {
		return groupKind, nil
	}
	return "", fmt.Errorf("group kind should be in format `resource.customizations.<type>.<group_kind>` or resource.customizations.<type>.<kind>`, got group kind: '%s'", groupKind)
}

func GetDefaultDiffOptions() ArgoCDDiffOptions {
	return ArgoCDDiffOptions{IgnoreAggregatedRoles: false, IgnoreDifferencesOnResourceUpdates: false}
}

// GetResourceCompareOptions loads the resource compare options settings from the ConfigMap
func (mgr *SettingsManager) GetResourceCompareOptions() (ArgoCDDiffOptions, error) {
	// We have a sane set of default diff options
	diffOptions := GetDefaultDiffOptions()

	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return diffOptions, err
	}

	if value, ok := argoCDCM.Data[resourceCompareOptionsKey]; ok {
		err := yaml.Unmarshal([]byte(value), &diffOptions)
		if err != nil {
			return diffOptions, err
		}
	}

	return diffOptions, nil
}

// GetHelmSettings returns helm settings
func (mgr *SettingsManager) GetHelmSettings() (*v1alpha1.HelmOptions, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get argo-cd config map: %w", err)
	}
	helmOptions := &v1alpha1.HelmOptions{}
	if value, ok := argoCDCM.Data[helmValuesFileSchemesKey]; ok {
		for _, item := range strings.Split(value, ",") {
			if item := strings.TrimSpace(item); item != "" {
				helmOptions.ValuesFileSchemes = append(helmOptions.ValuesFileSchemes, item)
			}
		}
	} else {
		helmOptions.ValuesFileSchemes = []string{"https", "http"}
	}
	return helmOptions, nil
}

// GetKustomizeSettings loads the kustomize settings from argocd-cm ConfigMap
func (mgr *SettingsManager) GetKustomizeSettings() (*KustomizeSettings, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-cm: %w", err)
	}
	kustomizeVersionsMap := map[string]KustomizeVersion{}
	buildOptions := map[string]string{}
	settings := &KustomizeSettings{}

	// extract build options for the default version
	if options, ok := argoCDCM.Data[kustomizeBuildOptionsKey]; ok {
		settings.BuildOptions = options
	}

	// extract per-version binary paths and build options
	for k, v := range argoCDCM.Data {
		// extract version and path from kustomize.version.<version>
		if strings.HasPrefix(k, kustomizeVersionKeyPrefix) {
			err = addKustomizeVersion(kustomizeVersionKeyPrefix, k, v, kustomizeVersionsMap)
			if err != nil {
				return nil, fmt.Errorf("failed to add kustomize version from %q: %w", k, err)
			}
		}

		// extract version and path from kustomize.path.<version>
		if strings.HasPrefix(k, kustomizePathPrefixKey) {
			err = addKustomizeVersion(kustomizePathPrefixKey, k, v, kustomizeVersionsMap)
			if err != nil {
				return nil, fmt.Errorf("failed to add kustomize version from %q: %w", k, err)
			}
		}

		// extract version and build options from kustomize.buildOptions.<version>
		if strings.HasPrefix(k, kustomizeBuildOptionsKey) && k != kustomizeBuildOptionsKey {
			buildOptions[k[len(kustomizeBuildOptionsKey)+1:]] = v
		}
	}

	for _, v := range kustomizeVersionsMap {
		if _, ok := buildOptions[v.Name]; ok {
			v.BuildOptions = buildOptions[v.Name]
		}
		settings.Versions = append(settings.Versions, v)
	}
	return settings, nil
}

func addKustomizeVersion(prefix, name, path string, kvMap map[string]KustomizeVersion) error {
	version := name[len(prefix)+1:]
	if _, ok := kvMap[version]; ok {
		return fmt.Errorf("found duplicate kustomize version: %s", version)
	}
	kvMap[version] = KustomizeVersion{
		Name: version,
		Path: path,
	}
	return nil
}

// DEPRECATED. Helm repository credentials are now managed using RepoCredentials
func (mgr *SettingsManager) GetHelmRepositories() ([]HelmRepoCredentials, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving config map: %w", err)
	}
	helmRepositories := make([]HelmRepoCredentials, 0)
	helmRepositoriesStr := argoCDCM.Data[helmRepositoriesKey]
	if helmRepositoriesStr != "" {
		err := yaml.Unmarshal([]byte(helmRepositoriesStr), &helmRepositories)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling helm repositories: %w", err)
		}
	}
	return helmRepositories, nil
}

func (mgr *SettingsManager) GetRepositories() ([]Repository, error) {
	mgr.mutex.Lock()
	reposCache := mgr.reposCache
	mgr.mutex.Unlock()
	if reposCache != nil {
		return reposCache, nil
	}

	// Get the config map outside of the lock
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("failed to get argo-cd config map: %w", err)
	}

	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	repositories := make([]Repository, 0)
	repositoriesStr := argoCDCM.Data[repositoriesKey]
	if repositoriesStr != "" {
		err := yaml.Unmarshal([]byte(repositoriesStr), &repositories)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal repositories from config map key %q: %w", repositoriesKey, err)
		}
	}
	mgr.reposCache = repositories

	return mgr.reposCache, nil
}

func (mgr *SettingsManager) SaveRepositories(repos []Repository) error {
	return mgr.updateConfigMap(func(argoCDCM *apiv1.ConfigMap) error {
		if len(repos) > 0 {
			yamlStr, err := yaml.Marshal(repos)
			if err != nil {
				return err
			}
			argoCDCM.Data[repositoriesKey] = string(yamlStr)
		} else {
			delete(argoCDCM.Data, repositoriesKey)
		}
		return nil
	})
}

func (mgr *SettingsManager) SaveRepositoryCredentials(creds []RepositoryCredentials) error {
	return mgr.updateConfigMap(func(argoCDCM *apiv1.ConfigMap) error {
		if len(creds) > 0 {
			yamlStr, err := yaml.Marshal(creds)
			if err != nil {
				return err
			}
			argoCDCM.Data[repositoryCredentialsKey] = string(yamlStr)
		} else {
			delete(argoCDCM.Data, repositoryCredentialsKey)
		}
		return nil
	})
}

func (mgr *SettingsManager) GetRepositoryCredentials() ([]RepositoryCredentials, error) {
	mgr.mutex.Lock()
	repoCredsCache := mgr.repoCredsCache
	mgr.mutex.Unlock()
	if repoCredsCache != nil {
		return repoCredsCache, nil
	}

	// Get the config map outside of the lock
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving config map: %w", err)
	}

	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	creds := make([]RepositoryCredentials, 0)
	credsStr := argoCDCM.Data[repositoryCredentialsKey]
	if credsStr != "" {
		err := yaml.Unmarshal([]byte(credsStr), &creds)
		if err != nil {
			return nil, err
		}
	}
	mgr.repoCredsCache = creds

	return mgr.repoCredsCache, nil
}

func (mgr *SettingsManager) GetGoogleAnalytics() (*GoogleAnalytics, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving config map: %w", err)
	}
	return &GoogleAnalytics{
		TrackingID:     argoCDCM.Data[gaTrackingID],
		AnonymizeUsers: argoCDCM.Data[gaAnonymizeUsers] != "false",
	}, nil
}

func (mgr *SettingsManager) GetHelp() (*Help, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving config map: %w", err)
	}
	chatText, ok := argoCDCM.Data[helpChatText]
	if !ok {
		chatText = "Chat now!"
	}
	chatURL, ok := argoCDCM.Data[helpChatURL]
	if !ok {
		chatText = ""
	}
	return &Help{
		ChatURL:    chatURL,
		ChatText:   chatText,
		BinaryURLs: getDownloadBinaryUrlsFromConfigMap(argoCDCM),
	}, nil
}

// GetSettings retrieves settings from the ArgoCDConfigMap and secret.
func (mgr *SettingsManager) GetSettings() (*ArgoCDSettings, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-cm: %w", err)
	}
	argoCDSecret, err := mgr.secrets.Secrets(mgr.namespace).Get(common.ArgoCDSecretName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-secret: %w", err)
	}
	selector, err := labels.Parse(partOfArgoCDSelector)
	if err != nil {
		return nil, fmt.Errorf("error parsing Argo CD selector %w", err)
	}
	secrets, err := mgr.secrets.Secrets(mgr.namespace).List(selector)
	if err != nil {
		return nil, err
	}
	var settings ArgoCDSettings
	var errs []error
	updateSettingsFromConfigMap(&settings, argoCDCM)
	if err := mgr.updateSettingsFromSecret(&settings, argoCDSecret, secrets); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &settings, errs[0]
	}

	return &settings, nil
}

// Clears cached settings on configmap/secret change
func (mgr *SettingsManager) invalidateCache() {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

	mgr.reposCache = nil
	mgr.repoCredsCache = nil
}

func (mgr *SettingsManager) initialize(ctx context.Context) error {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmLabelSelector := fields.ParseSelectorOrDie(partOfArgoCDSelector)
		options.LabelSelector = cmLabelSelector.String()
	}

	eventHandler := cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			mgr.invalidateCache()
			mgr.onRepoOrClusterChanged()
		},
		AddFunc: func(obj interface{}) {
			mgr.onRepoOrClusterChanged()
		},
		DeleteFunc: func(obj interface{}) {
			mgr.onRepoOrClusterChanged()
		},
	}
	indexers := cache.Indexers{
		cache.NamespaceIndex:    cache.MetaNamespaceIndexFunc,
		ByClusterURLIndexer:     byClusterURLIndexerFunc,
		ByClusterNameIndexer:    byClusterNameIndexerFunc,
		ByProjectClusterIndexer: byProjectIndexerFunc(common.LabelValueSecretTypeCluster),
		ByProjectRepoIndexer:    byProjectIndexerFunc(common.LabelValueSecretTypeRepository),
	}
	cmInformer := v1.NewFilteredConfigMapInformer(mgr.clientset, mgr.namespace, 3*time.Minute, indexers, tweakConfigMap)
	secretsInformer := v1.NewSecretInformer(mgr.clientset, mgr.namespace, 3*time.Minute, indexers)
	_, err := cmInformer.AddEventHandler(eventHandler)
	if err != nil {
		log.Error(err)
	}

	_, err = secretsInformer.AddEventHandler(eventHandler)
	if err != nil {
		log.Error(err)
	}

	log.Info("Starting configmap/secret informers")
	go func() {
		cmInformer.Run(ctx.Done())
		log.Info("configmap informer cancelled")
	}()
	go func() {
		secretsInformer.Run(ctx.Done())
		log.Info("secrets informer cancelled")
	}()

	if !cache.WaitForCacheSync(ctx.Done(), cmInformer.HasSynced, secretsInformer.HasSynced) {
		return fmt.Errorf("Timed out waiting for settings cache to sync")
	}
	log.Info("Configmap/secret informer synced")

	tryNotify := func() {
		newSettings, err := mgr.GetSettings()
		if err != nil {
			log.Warnf("Unable to parse updated settings: %v", err)
		} else {
			mgr.notifySubscribers(newSettings)
		}
	}
	now := time.Now()
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if metaObj, ok := obj.(metav1.Object); ok {
				if metaObj.GetCreationTimestamp().After(now) {
					tryNotify()
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta, oldOk := oldObj.(metav1.Common)
			newMeta, newOk := newObj.(metav1.Common)
			if oldOk && newOk && oldMeta.GetResourceVersion() != newMeta.GetResourceVersion() {
				tryNotify()
			}
		},
	}
	_, err = secretsInformer.AddEventHandler(handler)
	if err != nil {
		log.Error(err)
	}
	_, err = cmInformer.AddEventHandler(handler)
	if err != nil {
		log.Error(err)
	}
	mgr.secrets = v1listers.NewSecretLister(secretsInformer.GetIndexer())
	mgr.secretsInformer = secretsInformer
	mgr.configmaps = v1listers.NewConfigMapLister(cmInformer.GetIndexer())
	return nil
}

func (mgr *SettingsManager) ensureSynced(forceResync bool) error {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	if !forceResync && mgr.secrets != nil && mgr.configmaps != nil {
		return nil
	}

	if mgr.initContextCancel != nil {
		mgr.initContextCancel()
	}
	ctx, cancel := context.WithCancel(mgr.ctx)
	mgr.initContextCancel = cancel
	return mgr.initialize(ctx)
}

func getDownloadBinaryUrlsFromConfigMap(argoCDCM *apiv1.ConfigMap) map[string]string {
	binaryUrls := map[string]string{}
	for _, archType := range []string{"darwin-amd64", "darwin-arm64", "windows-amd64", "linux-amd64", "linux-arm64", "linux-ppc64le", "linux-s390x"} {
		if val, ok := argoCDCM.Data[settingsBinaryUrlsKey+"."+archType]; ok {
			binaryUrls[archType] = val
		}
	}
	return binaryUrls
}

// updateSettingsFromConfigMap transfers settings from a Kubernetes configmap into an ArgoCDSettings struct.
func updateSettingsFromConfigMap(settings *ArgoCDSettings, argoCDCM *apiv1.ConfigMap) {
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.OIDCConfigRAW = argoCDCM.Data[settingsOIDCConfigKey]
	settings.KustomizeBuildOptions = argoCDCM.Data[kustomizeBuildOptionsKey]
	settings.StatusBadgeEnabled = argoCDCM.Data[statusBadgeEnabledKey] == "true"
	settings.StatusBadgeRootUrl = argoCDCM.Data[statusBadgeRootUrlKey]
	settings.AnonymousUserEnabled = argoCDCM.Data[anonymousUserEnabledKey] == "true"
	settings.UiCssURL = argoCDCM.Data[settingUiCssURLKey]
	settings.UiBannerContent = argoCDCM.Data[settingUiBannerContentKey]
	settings.UiBannerPermanent = argoCDCM.Data[settingUiBannerPermanentKey] == "true"
	settings.UiBannerPosition = argoCDCM.Data[settingUiBannerPositionKey]
	settings.ServerRBACLogEnforceEnable = argoCDCM.Data[settingsServerRBACLogEnforceEnableKey] == "true"
	settings.BinaryUrls = getDownloadBinaryUrlsFromConfigMap(argoCDCM)
	if err := validateExternalURL(argoCDCM.Data[settingURLKey]); err != nil {
		log.Warnf("Failed to validate URL in configmap: %v", err)
	}
	settings.URL = argoCDCM.Data[settingURLKey]
	if err := validateExternalURL(argoCDCM.Data[settingUiBannerURLKey]); err != nil {
		log.Warnf("Failed to validate UI banner URL in configmap: %v", err)
	}
	if argoCDCM.Data[settingAdditionalUrlsKey] != "" {
		if err := yaml.Unmarshal([]byte(argoCDCM.Data[settingAdditionalUrlsKey]), &settings.AdditionalURLs); err != nil {
			log.Warnf("Failed to decode all additional URLs in configmap: %v", err)
		}
	}
	for _, url := range settings.AdditionalURLs {
		if err := validateExternalURL(url); err != nil {
			log.Warnf("Failed to validate external URL in configmap: %v", err)
		}
	}
	settings.UiBannerURL = argoCDCM.Data[settingUiBannerURLKey]
	settings.UserSessionDuration = time.Hour * 24
	if userSessionDurationStr, ok := argoCDCM.Data[userSessionDurationKey]; ok {
		if val, err := timeutil.ParseDuration(userSessionDurationStr); err != nil {
			log.Warnf("Failed to parse '%s' key: %v", userSessionDurationKey, err)
		} else {
			settings.UserSessionDuration = *val
		}
	}
	settings.PasswordPattern = argoCDCM.Data[settingsPasswordPatternKey]
	if settings.PasswordPattern == "" {
		settings.PasswordPattern = common.PasswordPatten
	}
	if maxPodLogsToRenderStr, ok := argoCDCM.Data[settingsMaxPodLogsToRender]; ok {
		if val, err := strconv.ParseInt(maxPodLogsToRenderStr, 10, 64); err != nil {
			log.Warnf("Failed to parse '%s' key: %v", settingsMaxPodLogsToRender, err)
		} else {
			settings.MaxPodLogsToRender = val
		}
	}
	settings.InClusterEnabled = argoCDCM.Data[inClusterEnabledKey] != "false"
	settings.ExecEnabled = argoCDCM.Data[execEnabledKey] == "true"
	execShells := argoCDCM.Data[execShellsKey]
	if execShells != "" {
		settings.ExecShells = strings.Split(execShells, ",")
	} else {
		// Fall back to default. If you change this list, also change docs/operator-manual/argocd-cm.yaml.
		settings.ExecShells = []string{"bash", "sh", "powershell", "cmd"}
	}
	settings.TrackingMethod = argoCDCM.Data[settingsResourceTrackingMethodKey]
	settings.OIDCTLSInsecureSkipVerify = argoCDCM.Data[oidcTLSInsecureSkipVerifyKey] == "true"
	settings.ExtensionConfig = argoCDCM.Data[extensionConfig]
	settings.ImpersonationEnabled = argoCDCM.Data[impersonationEnabledKey] == "true"
}

// validateExternalURL ensures the external URL that is set on the configmap is valid
func validateExternalURL(u string) error {
	if u == "" {
		return nil
	}
	URL, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("Failed to parse URL: %w", err)
	}
	if URL.Scheme != "http" && URL.Scheme != "https" {
		return fmt.Errorf("URL must include http or https protocol")
	}
	return nil
}

// updateSettingsFromSecret transfers settings from a Kubernetes secret into an ArgoCDSettings struct.
func (mgr *SettingsManager) updateSettingsFromSecret(settings *ArgoCDSettings, argoCDSecret *apiv1.Secret, secrets []*apiv1.Secret) error {
	var errs []error
	secretKey, ok := argoCDSecret.Data[settingServerSignatureKey]
	if ok {
		settings.ServerSignature = secretKey
	} else {
		errs = append(errs, &incompleteSettingsError{message: "server.secretkey is missing"})
	}

	// The TLS certificate may be externally managed. We try to load it from an
	// external secret first. If the external secret doesn't exist, we either
	// load it from argocd-secret or generate (and persist) a self-signed one.
	cert, err := mgr.externalServerTLSCertificate()
	if err != nil {
		errs = append(errs, &incompleteSettingsError{message: fmt.Sprintf("could not read from secret %s/%s: %v", mgr.namespace, externalServerTLSSecretName, err)})
	} else {
		if cert != nil {
			settings.Certificate = cert
			settings.CertificateIsExternal = true
			log.Infof("Loading TLS configuration from secret %s/%s", mgr.namespace, externalServerTLSSecretName)
		} else {
			serverCert, certOk := argoCDSecret.Data[settingServerCertificate]
			serverKey, keyOk := argoCDSecret.Data[settingServerPrivateKey]
			if certOk && keyOk {
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				if err != nil {
					errs = append(errs, &incompleteSettingsError{message: fmt.Sprintf("invalid x509 key pair %s/%s in secret: %s", settingServerCertificate, settingServerPrivateKey, err)})
				} else {
					settings.Certificate = &cert
					settings.CertificateIsExternal = false
				}
			}
		}
	}
	secretValues := make(map[string]string, len(argoCDSecret.Data))
	for _, s := range secrets {
		for k, v := range s.Data {
			secretValues[fmt.Sprintf("%s:%s", s.Name, k)] = string(v)
		}
	}
	for k, v := range argoCDSecret.Data {
		secretValues[k] = string(v)
	}
	settings.Secrets = secretValues
	if len(errs) > 0 {
		return errs[0]
	}

	settings.WebhookGitHubSecret = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookGitHubSecretKey]), settings.Secrets)
	settings.WebhookGitLabSecret = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookGitLabSecretKey]), settings.Secrets)
	settings.WebhookBitbucketUUID = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookBitbucketUUIDKey]), settings.Secrets)
	settings.WebhookBitbucketServerSecret = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookBitbucketServerSecretKey]), settings.Secrets)
	settings.WebhookGogsSecret = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookGogsSecretKey]), settings.Secrets)
	settings.WebhookAzureDevOpsUsername = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookAzureDevOpsUsernameKey]), settings.Secrets)
	settings.WebhookAzureDevOpsPassword = ReplaceStringSecret(string(argoCDSecret.Data[settingsWebhookAzureDevOpsPasswordKey]), settings.Secrets)

	return nil
}

// externalServerTLSCertificate will try and load a TLS certificate from an
// external secret, instead of tls.crt and tls.key in argocd-secret. If both
// return values are nil, no external secret has been configured.
func (mgr *SettingsManager) externalServerTLSCertificate() (*tls.Certificate, error) {
	var cert tls.Certificate
	secret, err := mgr.secrets.Secrets(mgr.namespace).Get(externalServerTLSSecretName)
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, nil
		}
	}
	tlsCert, certOK := secret.Data[settingServerCertificate]
	tlsKey, keyOK := secret.Data[settingServerPrivateKey]
	if certOK && keyOK {
		cert, err = tls.X509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return nil, err
		}
	}
	return &cert, nil
}

// SaveSettings serializes ArgoCDSettings and upserts it into K8s secret/configmap
func (mgr *SettingsManager) SaveSettings(settings *ArgoCDSettings) error {
	err := mgr.updateConfigMap(func(argoCDCM *apiv1.ConfigMap) error {
		if settings.URL != "" {
			argoCDCM.Data[settingURLKey] = settings.URL
		} else {
			delete(argoCDCM.Data, settingURLKey)
		}
		if settings.DexConfig != "" {
			argoCDCM.Data[settingDexConfigKey] = settings.DexConfig
		} else {
			delete(argoCDCM.Data, settings.DexConfig)
		}
		if settings.OIDCConfigRAW != "" {
			argoCDCM.Data[settingsOIDCConfigKey] = settings.OIDCConfigRAW
		} else {
			delete(argoCDCM.Data, settingsOIDCConfigKey)
		}
		if settings.UiCssURL != "" {
			argoCDCM.Data[settingUiCssURLKey] = settings.UiCssURL
		}
		if settings.UiBannerContent != "" {
			argoCDCM.Data[settingUiBannerContentKey] = settings.UiBannerContent
		} else {
			delete(argoCDCM.Data, settingUiBannerContentKey)
		}
		if settings.UiBannerURL != "" {
			argoCDCM.Data[settingUiBannerURLKey] = settings.UiBannerURL
		} else {
			delete(argoCDCM.Data, settingUiBannerURLKey)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return mgr.updateSecret(func(argoCDSecret *apiv1.Secret) error {
		argoCDSecret.Data[settingServerSignatureKey] = settings.ServerSignature
		if settings.WebhookGitHubSecret != "" {
			argoCDSecret.Data[settingsWebhookGitHubSecretKey] = []byte(settings.WebhookGitHubSecret)
		}
		if settings.WebhookGitLabSecret != "" {
			argoCDSecret.Data[settingsWebhookGitLabSecretKey] = []byte(settings.WebhookGitLabSecret)
		}
		if settings.WebhookBitbucketUUID != "" {
			argoCDSecret.Data[settingsWebhookBitbucketUUIDKey] = []byte(settings.WebhookBitbucketUUID)
		}
		if settings.WebhookBitbucketServerSecret != "" {
			argoCDSecret.Data[settingsWebhookBitbucketServerSecretKey] = []byte(settings.WebhookBitbucketServerSecret)
		}
		if settings.WebhookGogsSecret != "" {
			argoCDSecret.Data[settingsWebhookGogsSecretKey] = []byte(settings.WebhookGogsSecret)
		}
		if settings.WebhookAzureDevOpsUsername != "" {
			argoCDSecret.Data[settingsWebhookAzureDevOpsUsernameKey] = []byte(settings.WebhookAzureDevOpsUsername)
		}
		if settings.WebhookAzureDevOpsPassword != "" {
			argoCDSecret.Data[settingsWebhookAzureDevOpsPasswordKey] = []byte(settings.WebhookAzureDevOpsPassword)
		}
		// we only write the certificate to the secret if it's not externally
		// managed.
		if settings.Certificate != nil && !settings.CertificateIsExternal {
			cert, key := tlsutil.EncodeX509KeyPair(*settings.Certificate)
			argoCDSecret.Data[settingServerCertificate] = cert
			argoCDSecret.Data[settingServerPrivateKey] = key
		} else {
			delete(argoCDSecret.Data, settingServerCertificate)
			delete(argoCDSecret.Data, settingServerPrivateKey)
		}
		return nil
	})
}

// Save the SSH known host data into the corresponding ConfigMap
func (mgr *SettingsManager) SaveSSHKnownHostsData(ctx context.Context, knownHostsList []string) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	certCM, err := mgr.GetConfigMapByName(common.ArgoCDKnownHostsConfigMapName)
	if err != nil {
		return err
	}

	if certCM.Data == nil {
		certCM.Data = make(map[string]string)
	}

	sshKnownHostsData := strings.Join(knownHostsList, "\n") + "\n"
	certCM.Data["ssh_known_hosts"] = sshKnownHostsData
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(ctx, certCM, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

func (mgr *SettingsManager) SaveTLSCertificateData(ctx context.Context, tlsCertificates map[string]string) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	certCM, err := mgr.GetConfigMapByName(common.ArgoCDTLSCertsConfigMapName)
	if err != nil {
		return err
	}

	certCM.Data = tlsCertificates
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(ctx, certCM, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

func (mgr *SettingsManager) SaveGPGPublicKeyData(ctx context.Context, gpgPublicKeys map[string]string) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	keysCM, err := mgr.GetConfigMapByName(common.ArgoCDGPGKeysConfigMapName)
	if err != nil {
		return err
	}

	keysCM.Data = gpgPublicKeys
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(ctx, keysCM, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

type SettingsManagerOpts func(mgs *SettingsManager)

func WithRepoOrClusterChangedHandler(handler func()) SettingsManagerOpts {
	return func(mgr *SettingsManager) {
		mgr.reposOrClusterChanged = handler
	}
}

// NewSettingsManager generates a new SettingsManager pointer and returns it
func NewSettingsManager(ctx context.Context, clientset kubernetes.Interface, namespace string, opts ...SettingsManagerOpts) *SettingsManager {
	mgr := &SettingsManager{
		ctx:       ctx,
		clientset: clientset,
		namespace: namespace,
		mutex:     &sync.Mutex{},
	}
	for i := range opts {
		opts[i](mgr)
	}

	return mgr
}

func (mgr *SettingsManager) ResyncInformers() error {
	return mgr.ensureSynced(true)
}

// IsSSOConfigured returns whether or not single-sign-on is configured
func (a *ArgoCDSettings) IsSSOConfigured() bool {
	if a.IsDexConfigured() {
		return true
	}
	if a.OIDCConfig() != nil {
		return true
	}
	return false
}

func (a *ArgoCDSettings) IsDexConfigured() bool {
	if a.URL == "" {
		return false
	}
	dexCfg, err := UnmarshalDexConfig(a.DexConfig)
	if err != nil {
		log.Warnf("invalid dex yaml config: %s", err.Error())
		return false
	}
	return len(dexCfg) > 0
}

// GetServerEncryptionKey generates a new server encryption key using the server signature as a passphrase
func (a *ArgoCDSettings) GetServerEncryptionKey() ([]byte, error) {
	return crypto.KeyFromPassphrase(string(a.ServerSignature))
}

func UnmarshalDexConfig(config string) (map[string]interface{}, error) {
	var dexCfg map[string]interface{}
	err := yaml.Unmarshal([]byte(config), &dexCfg)
	return dexCfg, err
}

func (a *ArgoCDSettings) oidcConfig() *oidcConfig {
	if a.OIDCConfigRAW == "" {
		return nil
	}
	configMap := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(a.OIDCConfigRAW), &configMap)
	if err != nil {
		log.Warnf("invalid oidc config: %v", err)
		return nil
	}

	configMap = ReplaceMapSecrets(configMap, a.Secrets)
	data, err := yaml.Marshal(configMap)
	if err != nil {
		log.Warnf("invalid oidc config: %v", err)
		return nil
	}

	config, err := unmarshalOIDCConfig(string(data))
	if err != nil {
		log.Warnf("invalid oidc config: %v", err)
		return nil
	}

	return &config
}

func (a *ArgoCDSettings) OIDCConfig() *OIDCConfig {
	config := a.oidcConfig()
	if config == nil {
		return nil
	}
	return config.toExported()
}

func unmarshalOIDCConfig(configStr string) (oidcConfig, error) {
	var config oidcConfig
	err := yaml.Unmarshal([]byte(configStr), &config)
	return config, err
}

func ValidateOIDCConfig(configStr string) error {
	_, err := unmarshalOIDCConfig(configStr)
	return err
}

// TLSConfig returns a tls.Config with the configured certificates
func (a *ArgoCDSettings) TLSConfig() *tls.Config {
	if a.Certificate == nil {
		return nil
	}
	certPool := x509.NewCertPool()
	pemCertBytes, _ := tlsutil.EncodeX509KeyPair(*a.Certificate)
	ok := certPool.AppendCertsFromPEM(pemCertBytes)
	if !ok {
		panic("bad certs")
	}
	return &tls.Config{
		RootCAs: certPool,
	}
}

func (a *ArgoCDSettings) IssuerURL() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.Issuer
	}
	if a.DexConfig != "" {
		return a.URL + common.DexAPIEndpoint
	}
	return ""
}

// UserInfoGroupsEnabled returns whether group claims should be fetch from UserInfo endpoint
func (a *ArgoCDSettings) UserInfoGroupsEnabled() bool {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.EnableUserInfoGroups
	}
	return false
}

// UserInfoPath returns the sub-path on which the IDP exposes the UserInfo endpoint
func (a *ArgoCDSettings) UserInfoPath() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.UserInfoPath
	}
	return ""
}

// UserInfoCacheExpiration returns the expiry time of the UserInfo cache
func (a *ArgoCDSettings) UserInfoCacheExpiration() time.Duration {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil && oidcConfig.UserInfoCacheExpiration != "" {
		userInfoCacheExpiration, err := time.ParseDuration(oidcConfig.UserInfoCacheExpiration)
		if err != nil {
			log.Warnf("Failed to parse 'oidc.config.userInfoCacheExpiration' key: %v", err)
		}
		return userInfoCacheExpiration
	}
	return 0
}

func (a *ArgoCDSettings) OAuth2ClientID() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.ClientID
	}
	if a.DexConfig != "" {
		return common.ArgoCDClientAppID
	}
	return ""
}

// OAuth2AllowedAudiences returns a list of audiences that are allowed for the OAuth2 client. If the user has not
// explicitly configured the list of audiences (or has configured an empty list), then the OAuth2 client ID is returned
// as the only allowed audience. When using the bundled Dex, that client ID is always "argo-cd".
func (a *ArgoCDSettings) OAuth2AllowedAudiences() []string {
	if config := a.oidcConfig(); config != nil {
		if len(config.AllowedAudiences) == 0 {
			allowedAudiences := []string{config.ClientID}
			if config.CLIClientID != "" {
				allowedAudiences = append(allowedAudiences, config.CLIClientID)
			}
			return allowedAudiences
		}
		return config.AllowedAudiences
	}
	if a.DexConfig != "" {
		return []string{common.ArgoCDClientAppID, common.ArgoCDCLIClientAppID}
	}
	return nil
}

func (a *ArgoCDSettings) SkipAudienceCheckWhenTokenHasNoAudience() bool {
	if config := a.oidcConfig(); config != nil {
		if config.SkipAudienceCheckWhenTokenHasNoAudience != nil {
			return *config.SkipAudienceCheckWhenTokenHasNoAudience
		}
		return false
	}
	// When using the bundled Dex, the audience check is required. Dex will always send JWTs with an audience.
	return false
}

func (a *ArgoCDSettings) OAuth2ClientSecret() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.ClientSecret
	}
	if a.DexConfig != "" {
		return a.DexOAuth2ClientSecret()
	}
	return ""
}

// OIDCTLSConfig returns the TLS config for the OIDC provider. If an external provider is configured, returns a TLS
// config using the root CAs (if any) specified in the OIDC config. If an external OIDC provider is not configured,
// returns the API server TLS config, because the API server proxies requests to Dex.
func (a *ArgoCDSettings) OIDCTLSConfig() *tls.Config {
	var tlsConfig *tls.Config

	oidcConfig := a.OIDCConfig()
	if oidcConfig != nil {
		tlsConfig = &tls.Config{}
		if oidcConfig.RootCA != "" {
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(oidcConfig.RootCA))
			if !ok {
				log.Warn("failed to append certificates from PEM: proceeding without custom rootCA")
			} else {
				tlsConfig.RootCAs = certPool
			}
		}
	} else {
		tlsConfig = a.TLSConfig()
	}
	if tlsConfig != nil && a.OIDCTLSInsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	return tlsConfig
}

func appendURLPath(inputURL string, inputPath string) (string, error) {
	u, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, inputPath)
	return u.String(), nil
}

func (a *ArgoCDSettings) RedirectURL() (string, error) {
	return appendURLPath(a.URL, common.CallbackEndpoint)
}

func (a *ArgoCDSettings) ArgoURLForRequest(r *http.Request) (string, error) {
	for _, candidateURL := range append([]string{a.URL}, a.AdditionalURLs...) {
		u, err := url.Parse(candidateURL)
		if err != nil {
			return "", err
		}
		if u.Host == r.Host && strings.HasPrefix(r.URL.RequestURI(), u.RequestURI()) {
			return candidateURL, nil
		}
	}
	return a.URL, nil
}

func (a *ArgoCDSettings) RedirectURLForRequest(r *http.Request) (string, error) {
	base, err := a.ArgoURLForRequest(r)
	if err != nil {
		return "", err
	}
	return appendURLPath(base, common.CallbackEndpoint)
}

func (a *ArgoCDSettings) RedirectAdditionalURLs() ([]string, error) {
	RedirectAdditionalURLs := []string{}
	for _, url := range a.AdditionalURLs {
		redirectURL, err := appendURLPath(url, common.CallbackEndpoint)
		if err != nil {
			return []string{}, err
		}
		RedirectAdditionalURLs = append(RedirectAdditionalURLs, redirectURL)
	}
	return RedirectAdditionalURLs, nil
}

func (a *ArgoCDSettings) DexRedirectURL() (string, error) {
	return appendURLPath(a.URL, common.DexCallbackEndpoint)
}

// DexOAuth2ClientSecret calculates an arbitrary, but predictable OAuth2 client secret string derived
// from the server secret. This is called by the dex startup wrapper (argocd-dex rundex), as well
// as the API server, such that they both independently come to the same conclusion of what the
// OAuth2 shared client secret should be.
func (a *ArgoCDSettings) DexOAuth2ClientSecret() string {
	h := sha256.New()
	_, err := h.Write(a.ServerSignature)
	if err != nil {
		panic(err)
	}
	sha := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(sha)[:40]
}

// Subscribe registers a channel in which to subscribe to settings updates
func (mgr *SettingsManager) Subscribe(subCh chan<- *ArgoCDSettings) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	mgr.subscribers = append(mgr.subscribers, subCh)
	log.Infof("%v subscribed to settings updates", subCh)
}

// Unsubscribe unregisters a channel from receiving of settings updates
func (mgr *SettingsManager) Unsubscribe(subCh chan<- *ArgoCDSettings) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	for i, ch := range mgr.subscribers {
		if ch == subCh {
			mgr.subscribers = append(mgr.subscribers[:i], mgr.subscribers[i+1:]...)
			log.Infof("%v unsubscribed from settings updates", subCh)
			return
		}
	}
}

func (mgr *SettingsManager) notifySubscribers(newSettings *ArgoCDSettings) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	if len(mgr.subscribers) > 0 {
		subscribers := make([]chan<- *ArgoCDSettings, len(mgr.subscribers))
		copy(subscribers, mgr.subscribers)
		// make sure subscribes are notified in a separate thread to avoid potential deadlock
		go func() {
			log.Infof("Notifying %d settings subscribers: %v", len(subscribers), subscribers)
			for _, sub := range subscribers {
				sub <- newSettings
			}
		}()
	}
}

func isIncompleteSettingsError(err error) bool {
	var incompleteSettingsErr *incompleteSettingsError
	return errors.As(err, &incompleteSettingsErr)
}

// InitializeSettings is used to initialize empty admin password, signature, certificate etc if missing
func (mgr *SettingsManager) InitializeSettings(insecureModeEnabled bool) (*ArgoCDSettings, error) {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

	cdSettings, err := mgr.GetSettings()
	if err != nil && !isIncompleteSettingsError(err) {
		return nil, err
	}
	if cdSettings == nil {
		cdSettings = &ArgoCDSettings{}
	}
	if cdSettings.ServerSignature == nil {
		// set JWT signature
		signature, err := util.MakeSignature(32)
		if err != nil {
			return nil, fmt.Errorf("error setting JWT signature: %w", err)
		}
		cdSettings.ServerSignature = signature
		log.Info("Initialized server signature")
	}
	err = mgr.UpdateAccount(common.ArgoCDAdminUsername, func(adminAccount *Account) error {
		if adminAccount.Enabled {
			now := time.Now().UTC()
			if adminAccount.PasswordHash == "" {
				randBytes := make([]byte, initialPasswordLength)
				for i := 0; i < initialPasswordLength; i++ {
					num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
					if err != nil {
						return err
					}
					randBytes[i] = letters[num.Int64()]
				}
				initialPassword := string(randBytes)

				hashedPassword, err := password.HashPassword(initialPassword)
				if err != nil {
					return err
				}
				ku := kube.NewKubeUtil(mgr.clientset, mgr.ctx)
				err = ku.CreateOrUpdateSecretField(mgr.namespace, initialPasswordSecretName, initialPasswordSecretField, initialPassword)
				if err != nil {
					return err
				}
				adminAccount.PasswordHash = hashedPassword
				adminAccount.PasswordMtime = &now
				log.Info("Initialized admin password")
			}
			if adminAccount.PasswordMtime == nil || adminAccount.PasswordMtime.IsZero() {
				adminAccount.PasswordMtime = &now
				log.Info("Initialized admin mtime")
			}
		} else {
			log.Info("admin disabled")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if cdSettings.Certificate == nil && !insecureModeEnabled {
		// generate TLS cert
		hosts := []string{
			"localhost",
			"argocd-server",
			fmt.Sprintf("argocd-server.%s", mgr.namespace),
			fmt.Sprintf("argocd-server.%s.svc", mgr.namespace),
			fmt.Sprintf("argocd-server.%s.svc.cluster.local", mgr.namespace),
		}
		certOpts := tlsutil.CertOptions{
			Hosts:        hosts,
			Organization: "Argo CD",
			IsCA:         false,
		}
		cert, err := tlsutil.GenerateX509KeyPair(certOpts)
		if err != nil {
			return nil, err
		}
		cdSettings.Certificate = cert
		log.Info("Initialized TLS certificate")
	}

	err = mgr.SaveSettings(cdSettings)
	if apierrors.IsConflict(err) {
		// assume settings are initialized by another instance of api server
		log.Warnf("conflict when initializing settings. assuming updated by another replica")
		return mgr.GetSettings()
	}
	return cdSettings, nil
}

// ReplaceMapSecrets takes a json object and recursively looks for any secret key references in the
// object and replaces the value with the secret value
func ReplaceMapSecrets(obj map[string]interface{}, secretValues map[string]string) map[string]interface{} {
	newObj := make(map[string]interface{})
	for k, v := range obj {
		switch val := v.(type) {
		case map[string]interface{}:
			newObj[k] = ReplaceMapSecrets(val, secretValues)
		case []interface{}:
			newObj[k] = replaceListSecrets(val, secretValues)
		case string:
			newObj[k] = ReplaceStringSecret(val, secretValues)
		default:
			newObj[k] = val
		}
	}
	return newObj
}

func replaceListSecrets(obj []interface{}, secretValues map[string]string) []interface{} {
	newObj := make([]interface{}, len(obj))
	for i, v := range obj {
		switch val := v.(type) {
		case map[string]interface{}:
			newObj[i] = ReplaceMapSecrets(val, secretValues)
		case []interface{}:
			newObj[i] = replaceListSecrets(val, secretValues)
		case string:
			newObj[i] = ReplaceStringSecret(val, secretValues)
		default:
			newObj[i] = val
		}
	}
	return newObj
}

// ReplaceStringSecret checks if given string is a secret key reference ( starts with $ ) and returns corresponding value from provided map
func ReplaceStringSecret(val string, secretValues map[string]string) string {
	if val == "" || !strings.HasPrefix(val, "$") {
		return val
	}
	secretKey := val[1:]
	secretVal, ok := secretValues[secretKey]
	if !ok {
		log.Warnf("config referenced '%s', but key does not exist in secret", val)
		return val
	}
	return strings.TrimSpace(secretVal)
}

// GetGlobalProjectsSettings loads the global project settings from argocd-cm ConfigMap
func (mgr *SettingsManager) GetGlobalProjectsSettings() ([]GlobalProjectSettings, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, fmt.Errorf("error retrieving argocd-cm: %w", err)
	}
	globalProjectSettings := make([]GlobalProjectSettings, 0)
	if value, ok := argoCDCM.Data[globalProjectsKey]; ok {
		if value != "" {
			err := yaml.Unmarshal([]byte(value), &globalProjectSettings)
			if err != nil {
				return nil, fmt.Errorf("error unmarshalling global project settings: %w", err)
			}
		}
	}
	return globalProjectSettings, nil
}

func (mgr *SettingsManager) GetNamespace() string {
	return mgr.namespace
}

func (mgr *SettingsManager) GetResourceCustomLabels() ([]string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return []string{}, fmt.Errorf("failed getting configmap: %w", err)
	}
	labels := argoCDCM.Data[resourceCustomLabelsKey]
	if labels != "" {
		return strings.Split(labels, ","), nil
	}
	return []string{}, nil
}

func (mgr *SettingsManager) GetIncludeEventLabelKeys() []string {
	labelKeys := []string{}
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		log.Error(fmt.Errorf("failed getting configmap: %w", err))
		return labelKeys
	}
	if value, ok := argoCDCM.Data[resourceIncludeEventLabelKeys]; ok {
		if value != "" {
			value = strings.ReplaceAll(value, " ", "")
			labelKeys = strings.Split(value, ",")
		}
	}
	return labelKeys
}

func (mgr *SettingsManager) GetExcludeEventLabelKeys() []string {
	labelKeys := []string{}
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		log.Error(fmt.Errorf("failed getting configmap: %w", err))
		return labelKeys
	}
	if value, ok := argoCDCM.Data[resourceExcludeEventLabelKeys]; ok {
		if value != "" {
			value = strings.ReplaceAll(value, " ", "")
			labelKeys = strings.Split(value, ",")
		}
	}
	return labelKeys
}

func (mgr *SettingsManager) GetMaxWebhookPayloadSize() int64 {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return defaultMaxWebhookPayloadSize
	}

	if argoCDCM.Data[settingsWebhookMaxPayloadSizeMB] == "" {
		return defaultMaxWebhookPayloadSize
	}

	maxPayloadSizeMB, err := strconv.ParseInt(argoCDCM.Data[settingsWebhookMaxPayloadSizeMB], 10, 64)
	if err != nil {
		log.Warnf("Failed to parse '%s' key: %v", settingsWebhookMaxPayloadSizeMB, err)
		return defaultMaxWebhookPayloadSize
	}

	return maxPayloadSizeMB * 1024 * 1024
}

// GetIsImpersonationEnabled returns true if application sync with impersonation feature is enabled in argocd-cm configmap
func (mgr *SettingsManager) IsImpersonationEnabled() bool {
	cm, err := mgr.getConfigMap()
	if err != nil {
		return false
	}
	return cm.Data[impersonationEnabledKey] == "true"
}
