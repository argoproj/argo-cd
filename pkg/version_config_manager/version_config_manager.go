package version_config_manager

import (
	"errors"
	log "github.com/sirupsen/logrus"
)

const (
	CodefreshAPIProviderType = "CodereshAPI"
	ConfigMapProviderType    = "ConfigMap"
)

type VersionConfig struct {
	ProductLabel string `json:"productLabel"`
	JsonPath     string `json:"jsonPath"`
	ResourceName string `json:"resourceName"`
}

type ConfigProvider interface {
	GetConfig() (*VersionConfig, error)
}

type CodereshAPIConfigProvider struct {
	CodereshAPIEndpoint string
}

type ConfigMapProvider struct {
	ConfigMapPath string
}

func (codereshAPI *CodereshAPIConfigProvider) GetConfig() (*VersionConfig, error) {
	// Implement logic to fetch config from the CodereshAPI here.
	// For this example, we'll just return a mock config.
	return &VersionConfig{
		ProductLabel: "ProductLabelName=ProductName",
		JsonPath:     "{.appVersion}",
		ResourceName: "Chart.yaml",
	}, nil
}

func (cm *ConfigMapProvider) GetConfig() (*VersionConfig, error) {
	// Implement logic to fetch config from the config map here.
	// For this example, we'll just return a mock config.
	return &VersionConfig{
		ProductLabel: "ProductLabelName=ProductName",
		JsonPath:     "{.appVersion}",
		ResourceName: "Chart.yaml",
	}, nil
}

type VersionConfigManager struct {
	provider ConfigProvider
}

func NewVersionConfigManager(providerType string, source string) (*VersionConfigManager, error) {
	var provider ConfigProvider
	switch providerType {
	case CodefreshAPIProviderType:
		provider = &CodereshAPIConfigProvider{CodereshAPIEndpoint: source}
	case ConfigMapProviderType:
		provider = &ConfigMapProvider{ConfigMapPath: source}
	default:
		return nil, errors.New("Invalid provider type")
	}
	return &VersionConfigManager{provider: provider}, nil
}

func (v *VersionConfigManager) ObtainConfig() (*VersionConfig, error) {
	return v.provider.GetConfig()
}

func GetVersionConfig() *VersionConfig {
	versionConfigManager, err := NewVersionConfigManager("ConfigMap", "some-product-cm")
	if err != nil {
		log.Errorf("ERROR: Failed to create VersionConfigManager: %v", err)
		return nil
	}

	versionConfig, err := versionConfigManager.ObtainConfig()
	if err != nil {
		log.Printf("ERROR: Failed to obtain config: %v", err)
		return nil
	}

	return versionConfig
}
