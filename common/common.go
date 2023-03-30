package common

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Default service addresses and URLS of Argo CD internal services
const (
	// DefaultRepoServerAddr is the gRPC address of the Argo CD repo server
	DefaultRepoServerAddr = "argocd-repo-server:8081"
	// DefaultDexServerAddr is the HTTP address of the Dex OIDC server, which we run a reverse proxy against
	DefaultDexServerAddr = "argocd-dex-server:5556"
	// DefaultRedisAddr is the default redis address
	DefaultRedisAddr = "argocd-redis:6379"
)

// Kubernetes ConfigMap and Secret resource names which hold Argo CD settings
const (
	ArgoCDConfigMapName              = "argocd-cm"
	ArgoCDSecretName                 = "argocd-secret"
	ArgoCDNotificationsConfigMapName = "argocd-notifications-cm"
	ArgoCDNotificationsSecretName    = "argocd-notifications-secret"
	ArgoCDRBACConfigMapName          = "argocd-rbac-cm"
	// ArgoCDKnownHostsConfigMapName contains SSH known hosts data for connecting repositories. Will get mounted as volume to pods
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"
	// ArgoCDTLSCertsConfigMapName contains TLS certificate data for connecting repositories. Will get mounted as volume to pods
	ArgoCDTLSCertsConfigMapName = "argocd-tls-certs-cm"
	ArgoCDGPGKeysConfigMapName  = "argocd-gpg-keys-cm"
)

// Some default configurables
const (
	DefaultSystemNamespace = "kube-system"
	DefaultRepoType        = "git"
)

// Default listener ports for ArgoCD components
const (
	DefaultPortAPIServer              = 8080
	DefaultPortRepoServer             = 8081
	DefaultPortArgoCDMetrics          = 8082
	DefaultPortArgoCDAPIServerMetrics = 8083
	DefaultPortRepoServerMetrics      = 8084
)

// DefaultAddressAPIServer for ArgoCD components
const (
	DefaultAddressAPIServer = "localhost"
)

// Default paths on the pod's file system
const (
	// DefaultPathTLSConfig is the default path where TLS certificates for repositories are located
	DefaultPathTLSConfig = "/app/config/tls"
	// DefaultPathSSHConfig is the default path where SSH known hosts are stored
	DefaultPathSSHConfig = "/app/config/ssh"
	// DefaultSSHKnownHostsName is the Default name for the SSH known hosts file
	DefaultSSHKnownHostsName = "ssh_known_hosts"
	// DefaultGnuPgHomePath is the Default path to GnuPG home directory
	DefaultGnuPgHomePath = "/app/config/gpg/keys"
	// DefaultAppConfigPath is the Default path to repo server TLS endpoint config
	DefaultAppConfigPath = "/app/config"
	// DefaultPluginSockFilePath is the Default path to cmp server plugin socket file
	DefaultPluginSockFilePath = "/home/argocd/cmp-server/plugins"
	// DefaultPluginConfigFilePath is the Default path to cmp server plugin configuration file
	DefaultPluginConfigFilePath = "/home/argocd/cmp-server/config"
	// PluginConfigFileName is the Plugin Config File is a ConfigManagementPlugin manifest located inside the plugin container
	PluginConfigFileName = "plugin.yaml"
)

// Argo CD application related constants
const (

	// ArgoCDAdminUsername is the username of the 'admin' user
	ArgoCDAdminUsername = "admin"
	// ArgoCDUserAgentName is the default user-agent name used by the gRPC API client library and grpc-gateway
	ArgoCDUserAgentName = "argocd-client"
	// ArgoCDSSAManager is the default argocd manager name used by server-side apply syncs
	ArgoCDSSAManager = "argocd-controller"
	// AuthCookieName is the HTTP cookie name where we store our auth token
	AuthCookieName = "argocd.token"
	// StateCookieName is the HTTP cookie name that holds temporary nonce tokens for CSRF protection
	StateCookieName = "argocd.oauthstate"
	// StateCookieMaxAge is the maximum age of the oauth state cookie
	StateCookieMaxAge = time.Minute * 5

	// ChangePasswordSSOTokenMaxAge is the max token age for password change operation
	ChangePasswordSSOTokenMaxAge = time.Minute * 5
	// GithubAppCredsExpirationDuration is the default time used to cache the GitHub app credentials
	GithubAppCredsExpirationDuration = time.Minute * 60

	// PasswordPatten is the default password patten
	PasswordPatten = `^.{8,32}$`
)

// Dex related constants
const (
	// DexAPIEndpoint is the endpoint where we serve the Dex API server
	DexAPIEndpoint = "/api/dex"
	// LoginEndpoint is Argo CD's shorthand login endpoint which redirects to dex's OAuth 2.0 provider's consent page
	LoginEndpoint = "/auth/login"
	// LogoutEndpoint is Argo CD's shorthand logout endpoint which invalidates OIDC session after logout
	LogoutEndpoint = "/auth/logout"
	// CallbackEndpoint is Argo CD's final callback endpoint we reach after OAuth 2.0 login flow has been completed
	CallbackEndpoint = "/auth/callback"
	// DexCallbackEndpoint is Argo CD's final callback endpoint when Dex is configured
	DexCallbackEndpoint = "/api/dex/callback"
	// ArgoCDClientAppName is name of the Oauth client app used when registering our web app to dex
	ArgoCDClientAppName = "Argo CD"
	// ArgoCDClientAppID is the Oauth client ID we will use when registering our app to dex
	ArgoCDClientAppID = "argo-cd"
	// ArgoCDCLIClientAppName is name of the Oauth client app used when registering our CLI to dex
	ArgoCDCLIClientAppName = "Argo CD CLI"
	// ArgoCDCLIClientAppID is the Oauth client ID we will use when registering our CLI to dex
	ArgoCDCLIClientAppID = "argo-cd-cli"
)

// Resource metadata labels and annotations (keys and values) used by Argo CD components
const (
	// LabelKeyAppInstance is the label key to use to uniquely identify the instance of an application
	// The Argo CD application name is used as the instance name
	LabelKeyAppInstance = "app.kubernetes.io/instance"
	// LabelKeyLegacyApplicationName is the legacy label (v0.10 and below) and is superseded by 'app.kubernetes.io/instance'
	LabelKeyLegacyApplicationName = "applications.argoproj.io/app-name"
	// LabelKeySecretType contains the type of argocd secret (currently: 'cluster', 'repository', 'repo-config' or 'repo-creds')
	LabelKeySecretType = "argocd.argoproj.io/secret-type"
	// LabelValueSecretTypeCluster indicates a secret type of cluster
	LabelValueSecretTypeCluster = "cluster"
	// LabelValueSecretTypeRepository indicates a secret type of repository
	LabelValueSecretTypeRepository = "repository"
	// LabelValueSecretTypeRepoCreds indicates a secret type of repository credentials
	LabelValueSecretTypeRepoCreds = "repo-creds"

	// AnnotationKeyAppInstance is the Argo CD application name is used as the instance name
	AnnotationKeyAppInstance = "argocd.argoproj.io/tracking-id"

	// AnnotationCompareOptions is a comma-separated list of options for comparison
	AnnotationCompareOptions = "argocd.argoproj.io/compare-options"

	// AnnotationKeyManagedBy is annotation name which indicates that k8s resource is managed by an application.
	AnnotationKeyManagedBy = "managed-by"
	// AnnotationValueManagedByArgoCD is a 'managed-by' annotation value for resources managed by Argo CD
	AnnotationValueManagedByArgoCD = "argocd.argoproj.io"

	// AnnotationKeyLinkPrefix tells the UI to add an external link icon to the application node
	// that links to the value given in the annotation.
	// The annotation key must be followed by a unique identifier. Ex: link.argocd.argoproj.io/dashboard
	// It's valid to have multiple annotations that match the prefix.
	// Values can simply be a url or they can have
	// an optional link title separated by a "|"
	// Ex: "http://grafana.example.com/d/yu5UH4MMz/deployments"
	// Ex: "Go to Dashboard|http://grafana.example.com/d/yu5UH4MMz/deployments"
	AnnotationKeyLinkPrefix = "link.argocd.argoproj.io/"

	// AnnotationKeyAppSkipReconcile tells the Application to skip the Application controller reconcile.
	// Skip reconcile when the value is "true" or any other string values that can be strconv.ParseBool() to be true.
	AnnotationKeyAppSkipReconcile = "argocd.argoproj.io/skip-reconcile"
)

// Environment variables for tuning and debugging Argo CD
const (
	// EnvVarSSODebug is an environment variable to enable additional OAuth debugging in the API server
	EnvVarSSODebug = "ARGOCD_SSO_DEBUG"
	// EnvVarRBACDebug is an environment variable to enable additional RBAC debugging in the API server
	EnvVarRBACDebug = "ARGOCD_RBAC_DEBUG"
	// EnvVarSSHDataPath overrides the location where SSH known hosts for repo access data is stored
	EnvVarSSHDataPath = "ARGOCD_SSH_DATA_PATH"
	// EnvVarTLSDataPath overrides the location where TLS certificate for repo access data is stored
	EnvVarTLSDataPath = "ARGOCD_TLS_DATA_PATH"
	// EnvGitAttemptsCount specifies number of git remote operations attempts count
	EnvGitAttemptsCount = "ARGOCD_GIT_ATTEMPTS_COUNT"
	// EnvGitRetryMaxDuration specifices max duration of git remote operation retry
	EnvGitRetryMaxDuration = "ARGOCD_GIT_RETRY_MAX_DURATION"
	// EnvGitRetryDuration specifies duration of git remote operation retry
	EnvGitRetryDuration = "ARGOCD_GIT_RETRY_DURATION"
	// EnvGitRetryFactor specifies fator of git remote operation retry
	EnvGitRetryFactor = "ARGOCD_GIT_RETRY_FACTOR"
	// EnvGitSubmoduleEnabled overrides git submodule support, true by default
	EnvGitSubmoduleEnabled = "ARGOCD_GIT_MODULES_ENABLED"
	// EnvGnuPGHome is the path to ArgoCD's GnuPG keyring for signature verification
	EnvGnuPGHome = "ARGOCD_GNUPGHOME"
	// EnvWatchAPIBufferSize is the buffer size used to transfer K8S watch events to watch API consumer
	EnvWatchAPIBufferSize = "ARGOCD_WATCH_API_BUFFER_SIZE"
	// EnvPauseGenerationAfterFailedAttempts will pause manifest generation after the specified number of failed generation attempts
	EnvPauseGenerationAfterFailedAttempts = "ARGOCD_PAUSE_GEN_AFTER_FAILED_ATTEMPTS"
	// EnvPauseGenerationMinutes pauses manifest generation for the specified number of minutes, after sufficient manifest generation failures
	EnvPauseGenerationMinutes = "ARGOCD_PAUSE_GEN_MINUTES"
	// EnvPauseGenerationRequests pauses manifest generation for the specified number of requests, after sufficient manifest generation failures
	EnvPauseGenerationRequests = "ARGOCD_PAUSE_GEN_REQUESTS"
	// EnvControllerReplicas is the number of controller replicas
	EnvControllerReplicas = "ARGOCD_CONTROLLER_REPLICAS"
	// EnvControllerShard is the shard number that should be handled by controller
	EnvControllerShard = "ARGOCD_CONTROLLER_SHARD"
	// EnvEnableGRPCTimeHistogramEnv enables gRPC metrics collection
	EnvEnableGRPCTimeHistogramEnv = "ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM"
	// EnvGithubAppCredsExpirationDuration controls the caching of Github app credentials. This value is in minutes (default: 60)
	EnvGithubAppCredsExpirationDuration = "ARGOCD_GITHUB_APP_CREDS_EXPIRATION_DURATION"
	// EnvHelmIndexCacheDuration controls how the helm repository index file is cached for (default: 0)
	EnvHelmIndexCacheDuration = "ARGOCD_HELM_INDEX_CACHE_DURATION"
	// EnvAppConfigPath allows to override the configuration path for repo server
	EnvAppConfigPath = "ARGOCD_APP_CONF_PATH"
	// EnvLogFormat log format that is defined by `--logformat` option
	EnvLogFormat = "ARGOCD_LOG_FORMAT"
	// EnvLogLevel log level that is defined by `--loglevel` option
	EnvLogLevel = "ARGOCD_LOG_LEVEL"
	// EnvMaxCookieNumber max number of chunks a cookie can be broken into
	EnvMaxCookieNumber = "ARGOCD_MAX_COOKIE_NUMBER"
	// EnvPluginSockFilePath allows to override the pluginSockFilePath for repo server and cmp server
	EnvPluginSockFilePath = "ARGOCD_PLUGINSOCKFILEPATH"
	// EnvCMPChunkSize defines the chunk size in bytes used when sending files to the cmp server
	EnvCMPChunkSize = "ARGOCD_CMP_CHUNK_SIZE"
	// EnvCMPWorkDir defines the full path of the work directory used by the CMP server
	EnvCMPWorkDir = "ARGOCD_CMP_WORKDIR"
)

// Config Management Plugin related constants
const (
	// DefaultCMPChunkSize defines chunk size in bytes used when sending files to the cmp server
	DefaultCMPChunkSize = 1024

	// DefaultCMPWorkDirName defines the work directory name used by the cmp-server
	DefaultCMPWorkDirName = "_cmp_server"

	ConfigMapPluginDeprecationWarning = "argocd-cm plugins are deprecated, and support will be removed in v2.7. Upgrade your plugin to be installed via sidecar. https://argo-cd.readthedocs.io/en/stable/user-guide/config-management-plugins/"
)

const (
	// MinClientVersion is the minimum client version that can interface with this API server.
	// When introducing breaking changes to the API or datastructures, this number should be bumped.
	// The value here may be lower than the current value in VERSION
	MinClientVersion = "1.4.0"
	// CacheVersion is a objects version cached using util/cache/cache.go.
	// Number should be bumped in case of backward incompatible change to make sure cache is invalidated after upgrade.
	CacheVersion = "1.8.3"
)

// Constants used by util/clusterauth package
const (
	ClusterAuthRequestTimeout = 10 * time.Second
	BearerTokenTimeout        = 30 * time.Second
)

const (
	DefaultGitRetryMaxDuration time.Duration = time.Second * 5        // 5s
	DefaultGitRetryDuration    time.Duration = time.Millisecond * 250 // 0.25s
	DefaultGitRetryFactor                    = int64(2)
)

// GetGnuPGHomePath retrieves the path to use for GnuPG home directory, which is either taken from GNUPGHOME environment or a default value
func GetGnuPGHomePath() string {
	if gnuPgHome := os.Getenv(EnvGnuPGHome); gnuPgHome == "" {
		return DefaultGnuPgHomePath
	} else {
		return gnuPgHome
	}
}

// GetPluginSockFilePath retrieves the path of plugin sock file, which is either taken from PluginSockFilePath environment or a default value
func GetPluginSockFilePath() string {
	if pluginSockFilePath := os.Getenv(EnvPluginSockFilePath); pluginSockFilePath == "" {
		return DefaultPluginSockFilePath
	} else {
		return pluginSockFilePath
	}
}

// GetCMPChunkSize will return the env var EnvCMPChunkSize value if defined or DefaultCMPChunkSize otherwise.
// If EnvCMPChunkSize is defined but not a valid int, DefaultCMPChunkSize will be returned
func GetCMPChunkSize() int {
	if chunkSizeStr := os.Getenv(EnvCMPChunkSize); chunkSizeStr != "" {
		chunkSize, err := strconv.Atoi(chunkSizeStr)
		if err != nil {
			logrus.Warnf("invalid env var value for %s: not a valid int: %s. Default value will be used.", EnvCMPChunkSize, err)
			return DefaultCMPChunkSize
		}
		return chunkSize
	}
	return DefaultCMPChunkSize
}

// GetCMPWorkDir will return the full path of the work directory used by the CMP server.
// This directory and all it's contents will be deleted durring CMP bootstrap.
func GetCMPWorkDir() string {
	if workDir := os.Getenv(EnvCMPWorkDir); workDir != "" {
		return filepath.Join(workDir, DefaultCMPWorkDirName)
	}
	return filepath.Join(os.TempDir(), DefaultCMPWorkDirName)
}

const (
	// AnnotationApplicationSetRefresh is an annotation that is added when an ApplicationSet is requested to be refreshed by a webhook. The ApplicationSet controller will remove this annotation at the end of reconciliation.
	AnnotationApplicationSetRefresh = "argocd.argoproj.io/application-set-refresh"
)

// gRPC settings
const (
	GRPCKeepAliveEnforcementMinimum = 10 * time.Second
	// GRPCKeepAliveTime is 2x enforcement minimum to ensure network jitter does not introduce ENHANCE_YOUR_CALM errors
	GRPCKeepAliveTime = 2 * GRPCKeepAliveEnforcementMinimum
)

// Security severity logging
const (
	SecurityField     = "security"
	SecurityCWEField  = "CWE"
	SecurityEmergency = 5 // Indicates unmistakably malicious events that should NEVER occur accidentally and indicates an active attack (i.e. brute forcing, DoS)
	SecurityCritical  = 4 // Indicates any malicious or exploitable event that had a side effect (i.e. secrets being left behind on the filesystem)
	SecurityHigh      = 3 // Indicates likely malicious events but one that had no side effects or was blocked (i.e. out of bounds symlinks in repos)
	SecurityMedium    = 2 // Could indicate malicious events, but has a high likelihood of being user/system error (i.e. access denied)
	SecurityLow       = 1 // Unexceptional entries (i.e. successful access logs)
)

// TokenVerificationError is a generic error message for a failure to verify a JWT
const TokenVerificationError = "failed to verify the token"

var TokenVerificationErr = errors.New(TokenVerificationError)

var PermissionDeniedAPIError = status.Error(codes.PermissionDenied, "permission denied")
