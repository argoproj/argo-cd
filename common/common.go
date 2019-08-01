package common

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
)

// Default system namespace
const (
	DefaultSystemNamespace = "kube-system"
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
	// The default base path where application config is located
	DefaultPathAppConfig = "/app/config"
	// The default path where TLS certificates for repositories are located
	DefaultPathTLSConfig = "/app/config/tls"
	// The default path where SSH known hosts are stored
	DefaultPathSSHConfig = "/app/config/ssh"
	// Default name for the SSH known hosts file
	DefaultSSHKnownHostsName = "ssh_known_hosts"
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
	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	K8sClientConfigQPS = 25
	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst = 50
)

// Dex related constants
const (
	// DexAPIEndpoint is the endpoint where we serve the Dex API server
	DexAPIEndpoint = "/api/dex"
	// LoginEndpoint is Argo CD's shorthand login endpoint which redirects to dex's OAuth 2.0 provider's consent page
	LoginEndpoint = "/auth/login"
	// CallbackEndpoint is Argo CD's final callback endpoint we reach after OAuth 2.0 login flow has been completed
	CallbackEndpoint = "/auth/callback"
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
	// AnnotationSyncOptions is a comma-separated list of options for syncing
	AnnotationSyncOptions = "argocd.argoproj.io/sync-options"
	// AnnotationSyncWave indicates which wave of the sync the resource or hook should be in
	AnnotationSyncWave = "argocd.argoproj.io/sync-wave"
	// AnnotationKeyHook contains the hook type of a resource
	AnnotationKeyHook = "argocd.argoproj.io/hook"
	// AnnotationKeyHookDeletePolicy is the policy of deleting a hook
	AnnotationKeyHookDeletePolicy = "argocd.argoproj.io/hook-delete-policy"
	// AnnotationKeyRefresh is the annotation key which indicates that app needs to be refreshed. Removed by application controller after app is refreshed.
	// Might take values 'normal'/'hard'. Value 'hard' means manifest cache and target cluster state cache should be invalidated before refresh.
	AnnotationKeyRefresh = "argocd.argoproj.io/refresh"
	// AnnotationKeyManagedBy is annotation name which indicates that k8s resource is managed by an application.
	AnnotationKeyManagedBy = "managed-by"
	// AnnotationValueManagedByArgoCD is a 'managed-by' annotation value for resources managed by Argo CD
	AnnotationValueManagedByArgoCD = "argocd.argoproj.io"
	// AnnotationKeyHelmHook is the helm hook annotation
	AnnotationKeyHelmHook = "helm.sh/hook"
	// AnnotationValueHelmHookCRDInstall is a value of crd helm hook
	AnnotationValueHelmHookCRDInstall = "crd-install"
	// ResourcesFinalizerName the finalizer value which we inject to finalize deletion of an application
	ResourcesFinalizerName = "resources-finalizer.argocd.argoproj.io"
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
)

const (
	// MinClientVersion is the minimum client version that can interface with this API server.
	// When introducing breaking changes to the API or datastructures, this number should be bumped.
	// The value here may be lower than the current value in VERSION
	MinClientVersion = "1.0.0"
	// CacheVersion is a objects version cached using util/cache/cache.go.
	// Number should be bumped in case of backward incompatible change to make sure cache is invalidated after upgrade.
	CacheVersion = "1.0.0"
)
