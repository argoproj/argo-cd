package settings

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestArgoCDSettings_IsExcludedResource(t *testing.T) {
	settings := &ArgoCDSettings{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", ""))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", ""))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", ""))
}

func TestUpdateSettingsFromConfigMapResourceExclusions(t *testing.T) {

	settings := ArgoCDSettings{}
	configMap := v1.ConfigMap{}
	err := updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Nil(t, settings.ResourceExclusions)

	configMap.Data = map[string]string{
		"resource.exclusions": "\n  - apiGroups: []\n    kinds: []\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Equal(t, []FilteredResource{{APIGroups: []string{}, Kinds: []string{}, Clusters: []string{}}}, settings.ResourceExclusions)
}

func TestUpdateSettingsFromConfigMapResourceInclusionsAddInclusion(t *testing.T) {

	settings := ArgoCDSettings{}
	configMap := v1.ConfigMap{}
	err := updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Nil(t, settings.ResourceExclusions)

	configMap.Data = map[string]string{
		"resource.inclusions": "\n  - apiGroups: []\n    kinds: [managed_only]\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Equal(t, []FilteredResource{{APIGroups: []string{}, Kinds: []string{"managed_only"}, Clusters: []string{}}}, settings.ResourceInclusions)
}
