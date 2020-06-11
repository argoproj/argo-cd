package cache

import (
	"reflect"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
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
		if !reflect.DeepEqual(cache.settings, settings) {
			log.WithField("server", cache.config.Host).Infof("Changing cluster cache settings to: %v", settings)
			cache.settings = Settings{settings.ResourceHealthOverride, settings.ResourcesFilter}
		}
	}
}

// SetNamespaces updates list of monitored namespaces
func SetNamespaces(namespaces []string) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		if !reflect.DeepEqual(cache.namespaces, namespaces) {
			log.WithField("server", cache.config.Host).Infof("Changing cluster namespaces to: %v", namespaces)
			cache.namespaces = namespaces
		}
	}
}

// SetConfig updates cluster rest config
func SetConfig(config *rest.Config) UpdateSettingsFunc {
	return func(cache *clusterCache) {
		if !reflect.DeepEqual(cache.config, config) {
			log.WithField("server", cache.config.Host).Infof("Changing cluster config to: %v", config)
			cache.config = config
		}
	}
}
