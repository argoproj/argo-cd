package cache

import (
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

// NewNoopSettings returns cache settings that has not health customizations and don't filter any resources
func NewNoopSettings() *noopSettings {
	return &noopSettings{}
}

type noopSettings struct{}

func (f *noopSettings) GetResourceHealth(_ *unstructured.Unstructured) (*health.HealthStatus, error) {
	return nil, nil
}

func (f *noopSettings) IsExcludedResource(_, _, _ string) bool {
	return false
}

// Settings caching customizations
type Settings struct {
	// ResourceHealthOverride contains health assessment overrides
	ResourceHealthOverride health.HealthOverride
	// ResourcesFilter holds filter that excludes resources
	ResourcesFilter kube.ResourceFilter
}

type UpdateSettingsFunc func(cache *clusterCache)

// SetKubectl allows to override kubectl wrapper implementation
func SetKubectl(kubectl kube.Kubectl) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.kubectl = kubectl
	}
}

// SetPopulateResourceInfoHandler updates handler that populates resource info
func SetPopulateResourceInfoHandler(handler OnPopulateResourceInfoHandler) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.populateResourceInfoHandler = handler
	}
}

// SetSettings updates caching settings
func SetSettings(settings Settings) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.settings = Settings{settings.ResourceHealthOverride, settings.ResourcesFilter}
	}
}

// SetNamespaces updates list of monitored namespaces
func SetNamespaces(namespaces []string) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.namespaces = namespaces
	}
}

// SetClusterResources specifies if cluster level resource included or not.
// Flag is used only if cluster is changed to namespaced mode using SetNamespaces setting
func SetClusterResources(val bool) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.clusterResources = val
	}
}

// SetConfig updates cluster rest config
func SetConfig(config *rest.Config) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.config = config
	}
}

// SetListPageSize sets the page size for list pager.
func SetListPageSize(listPageSize int64) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.listPageSize = listPageSize
	}
}

// SetListPageBufferSize sets the number of pages to prefetch for list pager.
func SetListPageBufferSize(listPageBufferSize int32) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.listPageBufferSize = listPageBufferSize
	}
}

// SetListSemaphore sets the semaphore for list operations.
// Taking an object rather than a number allows to share a semaphore among multiple caches if necessary.
func SetListSemaphore(listSemaphore WeightedSemaphore) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.listSemaphore = listSemaphore
	}
}

// SetResyncTimeout updates cluster re-sync timeout
func SetResyncTimeout(timeout time.Duration) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.syncStatus.lock.Lock()
		defer cache.syncStatus.lock.Unlock()

		cache.syncStatus.resyncTimeout = timeout
	}
}

// SetWatchResyncTimeout updates cluster re-sync timeout
func SetWatchResyncTimeout(timeout time.Duration) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.watchResyncTimeout = timeout
	}
}

// SetClusterSyncRetryTimeout updates cluster sync retry timeout when sync error happens
func SetClusterSyncRetryTimeout(timeout time.Duration) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.clusterSyncRetryTimeout = timeout
	}
}

// SetLogr sets the logger to use.
func SetLogr(log logr.Logger) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.log = log
		if kcmd, ok := cache.kubectl.(*kube.KubectlCmd); ok {
			kcmd.Log = log
		}
	}
}

// SetTracer sets the tracer to use.
func SetTracer(tracer tracing.Tracer) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		if kcmd, ok := cache.kubectl.(*kube.KubectlCmd); ok {
			kcmd.Tracer = tracer
		}
	}
}

// SetRetryOptions sets cluster list retry options
func SetRetryOptions(maxRetries int32, useBackoff bool, retryFunc ListRetryFunc) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		// Max retries must be at least one
		if maxRetries < 1 {
			maxRetries = 1
		}
		cache.listRetryLimit = maxRetries
		cache.listRetryUseBackoff = useBackoff
		cache.listRetryFunc = retryFunc
	}
}

// SetRespectRBAC allows to set whether to respect the controller rbac in list/watches
func SetRespectRBAC(respectRBAC int) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		// if invalid value is provided disable respect rbac
		if respectRBAC < RespectRbacDisabled || respectRBAC > RespectRbacStrict {
			cache.respectRBAC = RespectRbacDisabled
		} else {
			cache.respectRBAC = respectRBAC
		}
	}
}

// SetBatchEventsProcessing allows to set whether to process events in batch
func SetBatchEventsProcessing(batchProcessing bool) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.batchEventsProcessing = batchProcessing
	}
}

// SetEventProcessingInterval allows to set the interval for processing events
func SetEventProcessingInterval(interval time.Duration) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		cache.eventProcessingInterval = interval
	}
}
