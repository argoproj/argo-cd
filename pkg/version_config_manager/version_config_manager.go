package version_config_manager

import (
	"github.com/argoproj/argo-cd/v2/pkg/codefresh"
	"github.com/argoproj/argo-cd/v2/reposerver/cache"
	log "github.com/sirupsen/logrus"
)

type VersionConfig struct {
	JsonPath     string `json:"jsonPath"`
	ResourceName string `json:"resourceName"`
}

func (v *VersionConfigManager) GetVersionConfig(app *codefresh.ApplicationIdentity) (*VersionConfig, error) {
	var appConfig *codefresh.ApplicationConfiguration

	// Get from cache
	appConfig, err := v.cache.GetCfAppConfig(app.Cluster, app.Namespace, app.Name)
	if err == nil {
		log.Infof("CfAppConfig cache hit: '%s'", cache.CfAppConfigCacheKey(app.Cluster, app.Namespace, app.Name))
		log.Infof("CfAppConfig. Use config from cache. File: %s, jsonPath: %s", appConfig.VersionSource.File, appConfig.VersionSource.JsonPath)
		return &VersionConfig{
			JsonPath:     appConfig.VersionSource.JsonPath,
			ResourceName: appConfig.VersionSource.File,
		}, nil
	}

	if err != nil {
		log.Errorf("CfAppConfig cache get error for '%s': %v", cache.CfAppConfigCacheKey(app.Cluster, app.Namespace, app.Name), err)
	}

	// Get from Codefresh API
	appConfig, err = v.requests.GetApplicationConfiguration(app)
	if err != nil {
		log.Infof("Failed to get application config from API: %v", err)
		return nil, err
	}
	log.Infof("CfAppConfig. Use config from API. File: %s, jsonPath: %s", appConfig.VersionSource.File, appConfig.VersionSource.JsonPath)

	if appConfig != nil {
		// Set to cache
		err = v.cache.SetCfAppConfig(app.Cluster, app.Namespace, app.Name, appConfig)
		if err == nil {
			log.Infof("CfAppConfig saved to cache hit: '%s'", cache.CfAppConfigCacheKey(app.Cluster, app.Namespace, app.Name))
		} else {
			log.Errorf("CfAppConfig cache set error for '%s': %v", cache.CfAppConfigCacheKey(app.Cluster, app.Namespace, app.Name), err)
		}

		return &VersionConfig{
			JsonPath:     appConfig.VersionSource.JsonPath,
			ResourceName: appConfig.VersionSource.File,
		}, nil
	}

	// Default value
	log.Infof("Used default CfAppConfig for: '%s'", cache.CfAppConfigCacheKey(app.Cluster, app.Namespace, app.Name))
	return &VersionConfig{
		JsonPath:     "{.appVersion}",
		ResourceName: "Chart.yaml",
	}, nil
}

type VersionConfigManager struct {
	requests codefresh.CodefreshGraphQLInterface
	cache    *cache.Cache
}

func NewVersionConfigManager(requests codefresh.CodefreshGraphQLInterface, cache *cache.Cache) *VersionConfigManager {
	return &VersionConfigManager{
		requests,
		cache,
	}
}
