package settings

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/common"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpdateSettingsFromConfigMap(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantErr bool
		get     func(settings ArgoCDSettings) interface{}
		want    interface{}
	}{
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
			name:  "TestHelmRepositories",
			key:   "helm.repositories",
			value: "\n  - url: http://foo\n",
			get: func(settings ArgoCDSettings) interface{} {
				return settings.HelmRepositories
			},
			want: []HelmRepoCredentials{{URL: "http://foo"}},
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

func TestGetResourceFilter(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
		},
		Data: map[string]string{
			"resource.exclusions": "\n  - apiGroups: [\"group1\"]\n    kinds: [\"kind1\"]\n    clusters: [\"cluster1\"]\n",
			"resource.inclusions": "\n  - apiGroups: [\"group2\"]\n    kinds: [\"kind2\"]\n    clusters: [\"cluster2\"]\n",
		},
	})
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
	filter, err := settingsManager.GetResourcesFilter()
	assert.NoError(t, err)
	assert.Equal(t, &ResourcesFilter{
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"group1"}, Kinds: []string{"kind1"}, Clusters: []string{"cluster1"}}},
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"group2"}, Kinds: []string{"kind2"}, Clusters: []string{"cluster2"}}},
	}, filter)

}
