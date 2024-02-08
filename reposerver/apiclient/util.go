package apiclient

import (
	"github.com/argoproj/argo-cd/v2/pkg/version_config_manager"
)

func (m *ManifestResponse) GetCompiledManifests() []string {
	manifests := make([]string, len(m.Manifests))
	for i, m := range m.Manifests {
		manifests[i] = m.CompiledManifest
	}
	return manifests
}

func GetVersionConfig() *VersionConfig {
	versionConfig := version_config_manager.GetVersionConfig()

	if versionConfig == nil {
		return nil
	}

	return &VersionConfig{
		ProductLabel: versionConfig.ProductLabel,
		ResourceName: versionConfig.ResourceName,
		JsonPath:     versionConfig.JsonPath,
	}
}
