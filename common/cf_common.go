package common

// Default service addresses and URLS of Argo CD internal services
const (
	// DefaultApplicationServerAddr is the HTTP address of the Argo CD server
	DefaultApplicationServerAddr = "argo-cd-server:80"
	// DefaultRedisHaProxyAddr is the default HTTP address of the sources server
	DefaultSourcesServerAddr = "sources-server:8090"
)

// Default listener ports for ArgoCD components
const (
	DefaultPortEventReporterServerMetrics = 8087
	DefaultPortEventReporterServer        = 8088

	DefaultPortACRServer = 8090
)

// DefaultAddressAPIServer for ArgoCD components
const (
	DefaultAddressEventReporterServer        = "0.0.0.0"
	DefaultAddressACRController              = "0.0.0.0"
	DefaultAddressEventReporterServerMetrics = "0.0.0.0"
)

// Environment variables for tuning and debugging Argo CD
const (
	// EnvApplicationEventCacheDuration controls the expiration of application events cache
	EnvApplicationEventCacheDuration = "ARGOCD_APP_EVENTS_CACHE_DURATION"
	// EnvResourceEventCacheDuration controls the expiration of resource events cache
	EnvResourceEventCacheDuration = "ARGOCD_RESOURCE_EVENTS_CACHE_DURATION"
	// EnvEventReporterShardingAlgorithm is the distribution sharding algorithm to be used: legacy
	EnvEventReporterShardingAlgorithm = "EVENT_REPORTER_SHARDING_ALGORITHM"
	// EnvEventReporterReplicas is the number of EventReporter replicas
	EnvEventReporterReplicas = "EVENT_REPORTER_REPLICAS"
	// EnvEventReporterShard is the shard number that should be handled by reporter
	EnvEventReporterShard = "EVENT_REPORTER_SHARD"
)

// CF Event reporter constants
const (
	EventReporterLegacyShardingAlgorithm  = "legacy"
	DefaultEventReporterShardingAlgorithm = EventReporterLegacyShardingAlgorithm
)
