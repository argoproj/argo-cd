package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestGetFilteredDexConfig(t *testing.T) {
	tests := []struct {
		name               string
		dexConfig          string
		dexAuthConnectorID string
		expectedConnectors []string // expected connector IDs in output YAML
	}{
		{
			name:               "No connector ID configured, returns fallback",
			dexConfig:          "connectors: [{id: github},{id: gitlab}]",
			dexAuthConnectorID: "",
			expectedConnectors: []string{"github", "gitlab"},
		},
		{
			name:               "Connector ID matches one connector",
			dexConfig:          "connectors: [{id: github},{id: gitlab}]",
			dexAuthConnectorID: "gitlab",
			expectedConnectors: []string{"gitlab"},
		},
		{
			name:               "Connector ID does not match any connector, returns fallback",
			dexConfig:          "connectors: [{id: github}]",
			dexAuthConnectorID: "gitlab",
			expectedConnectors: []string{"github"},
		},
		{
			name:               "Empty DexConfig, returns fallback",
			dexConfig:          "",
			dexAuthConnectorID: "github",
			expectedConnectors: []string{},
		},
		{
			name:               "Invalid YAML, returns fallback",
			dexConfig:          "invalid: [",
			dexAuthConnectorID: "github",
			expectedConnectors: []string{},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			settings := &settings.ArgoCDSettings{
				DexConfig:          tc.dexConfig,
				DexAuthConnectorID: tc.dexAuthConnectorID,
				URL:                "http://localhost", // to pass IsDexConfigured
			}
			result := GetFilteredDexConfig(settings)
			// If expectedConnectors is empty, expect fallback (original DexConfig)
			if len(tc.expectedConnectors) == 0 {
				assert.Equal(t, []byte(tc.dexConfig), result)
				return
			}
			// Otherwise, unmarshal and check connectors
			type PartialDexConfig struct {
				Connectors []struct {
					ID string `yaml:"id"`
				} `yaml:"connectors"`
			}
			var filtered PartialDexConfig
			err := yaml.Unmarshal(result, &filtered)
			require.NoError(t, err)
			var ids []string
			for _, c := range filtered.Connectors {
				ids = append(ids, c.ID)
			}
			assert.ElementsMatch(t, tc.expectedConnectors, ids)
		})
	}
}
