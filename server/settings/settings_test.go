package settings

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/settings"
	"sigs.k8s.io/yaml"
)

const testNamespace = "default"

func fixtures(data map[string]string) (*fake.Clientset, *settings.SettingsManager) {
	kubeClient := fake.NewClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeClient, testNamespace)
	return kubeClient, settingsManager
}

func TestSettingsServer(t *testing.T) {
	newServer := func(data map[string]string) *Server {
		_, settingsMgr := fixtures(data)
		return NewServer(settingsMgr, nil, nil, false, false, false)
	}

	t.Run("TestGetInstallationID", func(t *testing.T) {
		settingsServer := newServer(map[string]string{
			"installationID": "1234567890",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "1234567890", resp.InstallationID)
	})

	t.Run("TestGetInstallationIDNotSet", func(t *testing.T) {
		settingsServer := newServer(map[string]string{})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Empty(t, resp.InstallationID)
	})

	t.Run("TestGetTrackingMethod", func(t *testing.T) {
		settingsServer := newServer(map[string]string{
			"application.resourceTrackingMethod": "annotation+label",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "annotation+label", resp.TrackingMethod)
	})

	t.Run("TestGetAppLabelKey", func(t *testing.T) {
		settingsServer := newServer(map[string]string{
			"application.instanceLabelKey": "instance",
		})
		resp, err := settingsServer.Get(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, "instance", resp.AppLabelKey)
	})
}

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
