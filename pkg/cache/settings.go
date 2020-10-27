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

type noopSettings struct {
}

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
		cache.resyncTimeout = timeout
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
