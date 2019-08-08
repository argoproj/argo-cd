package settings

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetRepositories(t *testing.T) {
	data := map[string]string{
		"repositories": "\n  - url: http://foo\n",
	}
	settingsManager := newSettingsManager(data)
	filter, err := settingsManager.GetRepositories()
	assert.NoError(t, err)
	assert.Equal(t, []RepoCredentials{{URL: "http://foo"}}, filter)
}

func newSettingsManager(data map[string]string) *SettingsManager {
	kubeClient := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	})
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
	return settingsManager
}

func TestSaveRepositories(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	})
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
	err := settingsManager.SaveRepositories([]RepoCredentials{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repositories"], "- url: http://foo\n")
}

func TestGetRepositoryCredentials(t *testing.T) {
	data := map[string]string{
		"repository.credentials": "\n  - url: http://foo\n",
	}
	filter, err := newSettingsManager(data).GetRepositoryCredentials()
	assert.NoError(t, err)
	assert.Equal(t, []RepoCredentials{{URL: "http://foo"}}, filter)
}

func TestGetResourceFilter(t *testing.T) {
	data := map[string]string{
		"resource.exclusions": "\n  - apiGroups: [\"group1\"]\n    kinds: [\"kind1\"]\n    clusters: [\"cluster1\"]\n",
		"resource.inclusions": "\n  - apiGroups: [\"group2\"]\n    kinds: [\"kind2\"]\n    clusters: [\"cluster2\"]\n",
	}
	filter, err := newSettingsManager(data).GetResourcesFilter()
	assert.NoError(t, err)
	assert.Equal(t, &ResourcesFilter{
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"group1"}, Kinds: []string{"kind1"}, Clusters: []string{"cluster1"}}},
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"group2"}, Kinds: []string{"kind2"}, Clusters: []string{"cluster2"}}},
	}, filter)
}

func TestGetConfigManagementPlugins(t *testing.T) {
	data := map[string]string{
		"configManagementPlugins": `
      - name: kasane
        init:
          command: [kasane, update]
        generate:
          command: [kasane, show]`,
	}
	plugins, err := newSettingsManager(data).GetConfigManagementPlugins()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []v1alpha1.ConfigManagementPlugin{{
		Name:     "kasane",
		Init:     &v1alpha1.Command{Command: []string{"kasane", "update"}},
		Generate: v1alpha1.Command{Command: []string{"kasane", "show"}},
	}}, plugins)
}

func TestGetAppInstanceLabelKey(t *testing.T) {
	data := map[string]string{
		"application.instanceLabelKey": "testLabel",
	}
	label, err := newSettingsManager(data).GetAppInstanceLabelKey()
	assert.NoError(t, err)
	assert.Equal(t, "testLabel", label)
}

func TestGetResourceOverrides(t *testing.T) {
	data := map[string]string{
		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle`,
	}
	overrides, err := newSettingsManager(data).GetResourceOverrides()
	assert.NoError(t, err)

	webHookOverrides := overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"]
	assert.NotNil(t, webHookOverrides)

	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: "jsonPointers:\n- /webhooks/0/clientConfig/caBundle",
	}, webHookOverrides)
}

func TestGetHelmRepositories(t *testing.T) {
	data := map[string]string{
		"helm.repositories": "\n  - url: http://foo\n",
	}
	helmRepositories, err := newSettingsManager(data).GetHelmRepositories()
	assert.NoError(t, err)

	assert.ElementsMatch(t, helmRepositories, []HelmRepoCredentials{{URL: "http://foo"}})
}

func TestGetGoogleAnalytics(t *testing.T) {
	data := map[string]string{
		"ga.trackingid": "123",
	}
	ga, err := newSettingsManager(data).GetGoogleAnalytics()
	assert.NoError(t, err)
	assert.Equal(t, "123", ga.TrackingID)
	assert.Equal(t, true, ga.AnonymizeUsers)
}

func TestSettingsManager_GetHelp(t *testing.T) {
	data := map[string]string{
		"help.chatUrl": "foo",
	}
	h, err := newSettingsManager(data).GetHelp()
	assert.NoError(t, err)
	assert.Equal(t, "foo", h.ChatURL)
}
