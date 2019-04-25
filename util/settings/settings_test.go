package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestArgoCDSettings_IsExcludedResource(t *testing.T) {
	settings := &ArgoCDSettings{}
	assert.True(t, settings.IsExcludedResource("events.k8s.io", "", ""))
	assert.True(t, settings.IsExcludedResource("metrics.k8s.io", "", ""))
	assert.False(t, settings.IsExcludedResource("rubbish.io", "", ""))
}

func Test_updateSettingsFromConfigMap(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		get     func(settings ArgoCDSettings) interface{}
		want    interface{}
	}{
		{
			name:  "TestResourceExclusions",
			key:   "resource.exclusions",
			value: "\n  - apiGroups: []\n    kinds: []\n    clusters: []\n",
			get: func(settings ArgoCDSettings) interface{} {
				return settings.ResourceExclusions
			},
			want: []ExcludedResource{{APIGroups: []string{}, Kinds: []string{}, Clusters: []string{}}},
		},
		{
			name:  "TestRepositories",
			key:   "repositories",
			value: "\n  - url: http://foo\n",
			get: func(settings ArgoCDSettings) interface{} {
				return settings.Repositories
			},
			want: []RepoCredentials{{URL: "http://foo"}},
		},
		{
			name:  "TestRepositoryCredentials",
			key:   "repository.credentials",
			value: "\n  - url: http://foo\n",
			get: func(settings ArgoCDSettings) interface{} {
				return settings.RepositoryCredentials
			},
			want: []RepoCredentials{{URL: "http://foo"}},
		},
	}
	for _, tt := range tests {
		settings := ArgoCDSettings{}
		configMap := v1.ConfigMap{
			Data: map[string]string{
				tt.key: tt.value,
			},
		}
		t.Run(tt.name, func(t *testing.T) {
			if err := updateSettingsFromConfigMap(&settings, &configMap); (err != nil) != tt.wantErr {
				t.Errorf("updateSettingsFromConfigMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, tt.get(settings))
		})
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

func TestResourceInclusions(t *testing.T) {

	settings := ArgoCDSettings{}
	configMap := v1.ConfigMap{}
	err := updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Nil(t, settings.ResourceExclusions)

	configMap.Data = map[string]string{
		"resource.inclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: []\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Equal(t, []FilteredResource{{APIGroups: []string{"whitelisted-resource"}, Kinds: []string{}, Clusters: []string{}}}, settings.ResourceInclusions)

	assert.True(t, settings.IsExcludedResource("non-whitelisted-resource", "", ""))
	assert.False(t, settings.IsExcludedResource("whitelisted-resource", "", ""))
}

func TestResourceInclusionsExclusionNonMutex(t *testing.T) {

	settings := ArgoCDSettings{}
	configMap := v1.ConfigMap{}
	err := updateSettingsFromConfigMap(&settings, &configMap)

	assert.NoError(t, err)
	assert.Nil(t, settings.ResourceExclusions)

	configMap.Data = map[string]string{
		"resource.inclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: []\n    clusters: []\n",
		"resource.exclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: [\"blacklisted-kind\"]\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)
	assert.NoError(t, err)

	assert.True(t, settings.IsExcludedResource("whitelisted-resource", "blacklisted-kind", ""))
	assert.False(t, settings.IsExcludedResource("whitelisted-resource", "", ""))
	assert.False(t, settings.IsExcludedResource("whitelisted-resource", "non-blacklisted-kind", ""))

	configMap.Data = map[string]string{
		"resource.inclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: [\"whitelisted-kind\"]\n    clusters: []\n",
		"resource.exclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: []\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)
	assert.NoError(t, err)

	assert.True(t, settings.IsExcludedResource("whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, settings.IsExcludedResource("whitelisted-resource", "", ""))
	assert.True(t, settings.IsExcludedResource("whitelisted-resource", "non-whitelisted-kind", ""))

	configMap.Data = map[string]string{
		"resource.inclusions": "\n  - apiGroups: [\"foo-bar\"]\n    kinds: [\"whitelisted-kind\"]\n    clusters: []\n",
		"resource.exclusions": "\n  - apiGroups: [\"whitelisted-resource\"]\n    kinds: []\n    clusters: []\n",
	}

	err = updateSettingsFromConfigMap(&settings, &configMap)
	assert.NoError(t, err)

	assert.True(t, settings.IsExcludedResource("not-whitelisted-resource", "whitelisted-kind", ""))
	assert.True(t, settings.IsExcludedResource("not-whitelisted-resource", "", ""))
}
