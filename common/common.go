package common

const (
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
	// ResourcesFinalizerName the finalizer value which we inject to finalize deletion of an application
	ResourcesFinalizerName = "resources-finalizer.argocd.argoproj.io"
	// RevisionHistoryLimit is the max number of successful sync to keep in history
	RevisionHistoryLimit = 10
	// KubernetesInternalAPIServerAddr is address of the k8s API server when accessing internal to the cluster
	KubernetesInternalAPIServerAddr = "https://kubernetes.default.svc"
	// DefaultAppProjectName contains name of 'default' app project, which is available in every Argo CD installation
	DefaultAppProjectName = "default"
	// EnvVarFakeInClusterConfig is an environment variable to fake an in-cluster RESTConfig using
	// the current kubectl context (for development purposes)
	EnvVarFakeInClusterConfig = "ARGOCD_FAKE_IN_CLUSTER"
	// K8sClientConfigQPS controls the QPS to be used in K8s REST client configs
	K8sClientConfigQPS = 25
	// K8sClientConfigBurst controls the burst to be used in K8s REST client configs
	K8sClientConfigBurst = 50
	// LabelKeyAppInstance is the label key to use to uniquely identify the instance of an application
	// The Argo CD application name is used as the instance name
	LabelKeyAppInstance = "app.kubernetes.io/instance"
	// LegacyLabelApplicationName is the legacy label (v0.10 and below) and is superceded by 'app.kubernetes.io/instance'
	LabelKeyLegacyApplicationName = "applications.argoproj.io/app-name"
	// Overrides the location where TLS certificate for repo access data is stored
	EnvVarTLSDataPath = "ARGOCD_TLS_DATA_PATH"
	// The default path where TLS certificates for repositories are located
	DefaultPathTLSConfig = "/app/config/tls"
	// The default path where SSH known hosts are stored
	DefaultPathSSHConfig = "/app/config/ssh"
	// Default name for the SSH known hosts file
	DefaultSSHKnownHostsName = "ssh_known_hosts"
	// Overrides the location where SSH known hosts for repo access data is stored
	EnvVarSSHDataPath = "ARGOCD_SSH_DATA_PATH"
	// Specifies number of git remote operations attempts count
	EnvGitAttemptsCount = "ARGOCD_GIT_ATTEMPTS_COUNT"
)
