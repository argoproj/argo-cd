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

func fixtures(data map[string]string) (*fake.Clientset, *SettingsManager) {
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

	return kubeClient, settingsManager
}

func TestGetRepositories(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"repositories": "\n  - url: http://foo\n",
	})
	filter, err := settingsManager.GetRepositories()
	assert.NoError(t, err)
	assert.Equal(t, []Repository{{URL: "http://foo"}}, filter)
}

func TestSaveRepositories(t *testing.T) {
	kubeClient, settingsManager := fixtures(nil)
	err := settingsManager.SaveRepositories([]Repository{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repositories"], "- url: http://foo\n")

	repos, err := settingsManager.GetRepositories()
	assert.NoError(t, err)
	assert.ElementsMatch(t, repos, []Repository{{URL: "http://foo"}})
}

func TestSaveRepositoresNoConfigMap(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

	err := settingsManager.SaveRepositories([]Repository{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repositories"], "- url: http://foo\n")
}

func TestSaveRepositoryCredentials(t *testing.T) {
	kubeClient, settingsManager := fixtures(nil)
	err := settingsManager.SaveRepositoryCredentials([]RepositoryCredentials{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repository.credentials"], "- url: http://foo\n")

	creds, err := settingsManager.GetRepositoryCredentials()
	assert.NoError(t, err)
	assert.ElementsMatch(t, creds, []RepositoryCredentials{{URL: "http://foo"}})
}

func TestGetRepositoryCredentials(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"repository.credentials": "\n  - url: http://foo\n",
	})
	filter, err := settingsManager.GetRepositoryCredentials()
	assert.NoError(t, err)
	assert.Equal(t, []RepositoryCredentials{{URL: "http://foo"}}, filter)
}

func TestGetResourceFilter(t *testing.T) {
	data := map[string]string{
		"resource.exclusions": "\n  - apiGroups: [\"group1\"]\n    kinds: [\"kind1\"]\n    clusters: [\"cluster1\"]\n",
		"resource.inclusions": "\n  - apiGroups: [\"group2\"]\n    kinds: [\"kind2\"]\n    clusters: [\"cluster2\"]\n",
	}
	_, settingsManager := fixtures(data)
	filter, err := settingsManager.GetResourcesFilter()
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
	_, settingsManager := fixtures(data)
	plugins, err := settingsManager.GetConfigManagementPlugins()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []v1alpha1.ConfigManagementPlugin{{
		Name:     "kasane",
		Init:     &v1alpha1.Command{Command: []string{"kasane", "update"}},
		Generate: v1alpha1.Command{Command: []string{"kasane", "show"}},
	}}, plugins)
}

func TestGetAppInstanceLabelKey(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"application.instanceLabelKey": "testLabel",
	})
	label, err := settingsManager.GetAppInstanceLabelKey()
	assert.NoError(t, err)
	assert.Equal(t, "testLabel", label)
}

func TestGetResourceOverrides(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle`,
	})
	overrides, err := settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	webHookOverrides := overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"]
	assert.NotNil(t, webHookOverrides)

	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: "jsonPointers:\n- /webhooks/0/clientConfig/caBundle",
	}, webHookOverrides)
}

func TestSettingsManager_GetKustomizeBuildOptions(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{})

		options, err := settingsManager.GetKustomizeBuildOptions()

		assert.NoError(t, err)
		assert.Empty(t, options)
	})
	t.Run("Set", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{"kustomize.buildOptions": "foo"})

		options, err := settingsManager.GetKustomizeBuildOptions()

		assert.NoError(t, err)
		assert.Equal(t, "foo", options)
	})
}

func TestGetGoogleAnalytics(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"ga.trackingid": "123",
	})
	ga, err := settingsManager.GetGoogleAnalytics()
	assert.NoError(t, err)
	assert.Equal(t, "123", ga.TrackingID)
	assert.Equal(t, true, ga.AnonymizeUsers)
}

func TestSettingsManager_GetHelp(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		_, settingsManager := fixtures(nil)
		h, err := settingsManager.GetHelp()
		assert.NoError(t, err)
		assert.Empty(t, h.ChatURL)
		assert.Equal(t, "Chat now!", h.ChatText)

	})
	t.Run("Set", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"help.chatUrl":  "foo",
			"help.chatText": "bar",
		})
		h, err := settingsManager.GetHelp()
		assert.NoError(t, err)
		assert.Equal(t, "foo", h.ChatURL)
		assert.Equal(t, "bar", h.ChatText)
	})
}

func TestGetOIDCConfig(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"oidc.config": "\n  requestedIDTokenClaims: {\"groups\": {\"essential\": true}}\n",
			},
		},
		&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDSecretName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string][]byte{
				"admin.password":   nil,
				"server.secretkey": nil,
			},
		},
	)
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
	settings, err := settingsManager.GetSettings()
	assert.NoError(t, err)

	oidcConfig := settings.OIDCConfig()
	assert.NotNil(t, oidcConfig)

	claim := oidcConfig.RequestedIDTokenClaims["groups"]
	assert.NotNil(t, claim)
	assert.Equal(t, true, claim.Essential)
}

func TestRedirectURL(t *testing.T) {
	cases := map[string][]string{
		"https://localhost:4000":         {"https://localhost:4000/auth/callback", "https://localhost:4000/api/dex/callback"},
		"https://localhost:4000/":        {"https://localhost:4000/auth/callback", "https://localhost:4000/api/dex/callback"},
		"https://localhost:4000/argocd":  {"https://localhost:4000/argocd/auth/callback", "https://localhost:4000/argocd/api/dex/callback"},
		"https://localhost:4000/argocd/": {"https://localhost:4000/argocd/auth/callback", "https://localhost:4000/argocd/api/dex/callback"},
	}
	for given, expected := range cases {
		settings := ArgoCDSettings{URL: given}
		redirectURL, err := settings.RedirectURL()
		assert.NoError(t, err)
		assert.Equal(t, expected[0], redirectURL)
		dexRedirectURL, err := settings.DexRedirectURL()
		assert.NoError(t, err)
		assert.Equal(t, expected[1], dexRedirectURL)
	}
}
