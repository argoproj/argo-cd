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
}
