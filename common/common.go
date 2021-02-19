package common

import (
	"os"
	"strconv"
	"time"
)

// Default service addresses and URLS of Argo CD internal services
const (
	// DefaultRepoServerAddr is the gRPC address of the Argo CD repo server
	DefaultRepoServerAddr = "argocd-repo-server:8081"
	// DefaultDexServerAddr is the HTTP address of the Dex OIDC server, which we run a reverse proxy against
	DefaultDexServerAddr = "http://argocd-dex-server:5556"
	// DefaultRedisAddr is the default redis address
	DefaultRedisAddr = "argocd-redis:6379"
)

// Kubernetes ConfigMap and Secret resource names which hold Argo CD settings
const (
	ArgoCDConfigMapName     = "argocd-cm"
	ArgoCDSecretName        = "argocd-secret"
	ArgoCDRBACConfigMapName = "argocd-rbac-cm"
	// Contains SSH known hosts data for connecting repositories. Will get mounted as volume to pods
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"
	// Contains TLS certificate data for connecting repositories. Will get mounted as volume to pods
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

// Default paths on the pod's file system
const (
	// The default path where TLS certificates for repositories are located
	DefaultPathTLSConfig = "/app/config/tls"
	// The default path where SSH known hosts are stored
	DefaultPathSSHConfig = "/app/config/ssh"
	// Default name for the SSH known hosts file
	DefaultSSHKnownHostsName = "ssh_known_hosts"
	// Default path to GnuPG home directory
	DefaultGnuPgHomePath = "/app/config/gpg/keys"
)

const (
	DefaultSyncRetryDuration    = 5 * time.Second
	DefaultSyncRetryMaxDuration = 3 * time.Minute
	DefaultSyncRetryFactor      = int64(2)
)

// Argo CD application related constants
const (
	// KubernetesInternalAPIServerAddr is address of the k8s API server when accessing internal to the cluster
	KubernetesInternalAPIServerAddr = "https://kubernetes.default.svc"
	// DefaultAppProjectName contains name of 'default' app project, which is available in every Argo CD installation
	DefaultAppProjectName = "default"
	// ArgoCDAdminUsername is the username of the 'admin' user
	ArgoCDAdminUsername = "admin"
	// ArgoCDUserAgentName is the default user-agent name used by the gRPC API client library and grpc-gateway
	ArgoCDUserAgentName = "argocd-client"
	// AuthCookieName is the HTTP cookie name where we store our auth token
	AuthCookieName = "argocd.token"
	// RevisionHistoryLimit is the max number of successful sync to keep in history
	RevisionHistoryLimit = 10
	// ChangePasswordSSOTokenMaxAge is the max token age for password change operation
	ChangePasswordSSOTokenMaxAge = time.Minute * 5
	// GithubAppCredsExpirationDuration is the default time used to cache the GitHub app credentials
	GithubAppCredsExpirationDuration = time.Minute * 60
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
	// LegacyLabelApplicationName is the legacy label (v0.10 and below) and is superceded by 'app.kubernetes.io/instance'
	LabelKeyLegacyApplicationName = "applications.argoproj.io/app-name"
	// LabelKeySecretType contains the type of argocd secret (currently: 'cluster')
	LabelKeySecretType = "argocd.argoproj.io/secret-type"
	// LabelValueSecretTypeCluster indicates a secret type of cluster
	LabelValueSecretTypeCluster = "cluster"

	// AnnotationCompareOptions is a comma-separated list of options for comparison
	AnnotationCompareOptions = "argocd.argoproj.io/compare-options"

	// AnnotationKeyRefresh is the annotation key which indicates that app needs to be refreshed. Removed by application controller after app is refreshed.
	// Might take values 'normal'/'hard'. Value 'hard' means manifest cache and target cluster state cache should be invalidated before refresh.
	AnnotationKeyRefresh = "argocd.argoproj.io/refresh"
	// AnnotationKeyManagedBy is annotation name which indicates that k8s resource is managed by an application.
	AnnotationKeyManagedBy = "managed-by"
	// AnnotationValueManagedByArgoCD is a 'managed-by' annotation value for resources managed by Argo CD
	AnnotationValueManagedByArgoCD = "argocd.argoproj.io"
	// ResourcesFinalizerName the finalizer value which we inject to finalize deletion of an application
	ResourcesFinalizerName = "resources-finalizer.argocd.argoproj.io"

	// AnnotationKeyManifestGeneratePaths is an annotation that contains a list of semicolon-separated paths in the
	// manifests repository that affects the manifest generation. Paths might be either relative or absolute. The
	// absolute path means an absolute path within the repository and the relative path is relative to the application
	// source path within the repository.
	AnnotationKeyManifestGeneratePaths = "argocd.argoproj.io/manifest-generate-paths"

	// AnnotationKeyLinkPrefix tells the UI to add an external link icon to the application node
	// that links to the value given in the annotation.
	// The annotation key must be followed by a unique identifier. Ex: link.argocd.argoproj.io/dashboard
	// It's valid to have multiple annotations that match the prefix.
	// Values can simply be a url or they can have
	// an optional link title separated by a "|"
	// Ex: "http://grafana.example.com/d/yu5UH4MMz/deployments"
	// Ex: "Go to Dashboard|http://grafana.example.com/d/yu5UH4MMz/deployments"
	AnnotationKeyLinkPrefix = "link.argocd.argoproj.io/"
)

// Environment variables for tuning and debugging Argo CD
const (
	// EnvVarSSODebug is an environment variable to enable additional OAuth debugging in the API server
	EnvVarSSODebug = "ARGOCD_SSO_DEBUG"
	// EnvVarRBACDebug is an environment variable to enable additional RBAC debugging in the API server
	EnvVarRBACDebug = "ARGOCD_RBAC_DEBUG"
	// EnvVarFakeInClusterConfig is an environment variable to fake an in-cluster RESTConfig using
	// the current kubectl context (for development purposes)
	EnvVarFakeInClusterConfig = "ARGOCD_FAKE_IN_CLUSTER"
	// Overrides the location where SSH known hosts for repo access data is stored
	EnvVarSSHDataPath = "ARGOCD_SSH_DATA_PATH"
	// Overrides the location where TLS certificate for repo access data is stored
	EnvVarTLSDataPath = "ARGOCD_TLS_DATA_PATH"
	// Specifies number of git remote operations attempts count
	EnvGitAttemptsCount = "ARGOCD_GIT_ATTEMPTS_COUNT"
	// Overrides git submodule support, true by default
	EnvGitSubmoduleEnabled = "ARGOCD_GIT_MODULES_ENABLED"
	// EnvK8sClientQPS is the QPS value used for the kubernetes client (default: 50)
	EnvK8sClientQPS = "ARGOCD_K8S_CLIENT_QPS"
	// EnvK8sClientBurst is the burst value used for the kubernetes client (default: twice the client QPS)
	EnvK8sClientBurst = "ARGOCD_K8S_CLIENT_BURST"
	// EnvClusterCacheResyncDuration is the env variable that holds cluster cache re-sync duration
	EnvClusterCacheResyncDuration = "ARGOCD_CLUSTER_CACHE_RESYNC_DURATION"
	// EnvK8sClientMaxIdleConnections is the number of max idle connections in K8s REST client HTTP transport (default: 500)
	EnvK8sClientMaxIdleConnections = "ARGOCD_K8S_CLIENT_MAX_IDLE_CONNECTIONS"
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

// GetGnuPGHomePath retrieves the path to use for GnuPG home directory, which is either taken from GNUPGHOME environment or a default value
func GetGnuPGHomePath() string {
	if gnuPgHome := os.Getenv(EnvGnuPGHome); gnuPgHome == "" {
		return DefaultGnuPgHomePath
	} else {
		return gnuPgHome
	}
}

var (
	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	K8sClientConfigQPS float32 = 50
	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst int = 100
	// K8sMaxIdleConnections controls the number of max idle connections in K8s REST client HTTP transport
	K8sMaxIdleConnections = 500
	// K8sMaxIdleConnections controls the duration of cluster cache refresh
	K8SClusterResyncDuration = 12 * time.Hour
)

func init() {
	if envQPS := os.Getenv(EnvK8sClientQPS); envQPS != "" {
		if qps, err := strconv.ParseFloat(envQPS, 32); err != nil {
			K8sClientConfigQPS = float32(qps)
		}
	}
	if envBurst := os.Getenv(EnvK8sClientBurst); envBurst != "" {
		if burst, err := strconv.Atoi(envBurst); err != nil {
			K8sClientConfigBurst = burst
		}
	} else {
		K8sClientConfigBurst = 2 * int(K8sClientConfigQPS)
	}

	if envMaxConn := os.Getenv(EnvK8sClientMaxIdleConnections); envMaxConn != "" {
		if maxConn, err := strconv.Atoi(envMaxConn); err != nil {
			K8sMaxIdleConnections = maxConn
		}
	}
	if clusterResyncDurationStr := os.Getenv(EnvClusterCacheResyncDuration); clusterResyncDurationStr != "" {
		if duration, err := time.ParseDuration(clusterResyncDurationStr); err == nil {
			K8SClusterResyncDuration = duration
		}
	}
}
