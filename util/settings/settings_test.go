package settings

import (
	"testing"

	"k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestArgoCDSettings_IsExcludedResource(t *testing.T) {
	settings := &ArgoCDSettings{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", ""))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", ""))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", ""))
}

func TestUpdateSettingsFromConfigMapExcludedResources(t *testing.T) {

	settings := ArgoCDSettings{}
	configMap := v1.ConfigMap{}
	err := updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Nil(t, settings.ExcludedResources)

	configMap.Data = map[string]string{
		"excludedResources": "\n  - apiGroups: []\n    kinds: []\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Equal(t, []ExcludedResource{{APIGroups: []string{}, Kinds: []string{}, Clusters: []string{}}}, settings.ExcludedResources)
}
