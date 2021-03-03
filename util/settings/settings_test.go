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

func fixtures(data map[string]string, opts ...func(secret *v1.Secret)) (*fake.Clientset, *SettingsManager) {
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{},
	}
	for i := range opts {
		opts[i](secret)
	}
	kubeClient := fake.NewSimpleClientset(cm, secret)
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
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repositories"], "- url: http://foo\n")

	repos, err := settingsManager.GetRepositories()
	assert.NoError(t, err)
	assert.ElementsMatch(t, repos, []Repository{{URL: "http://foo"}})
}

func TestSaveRepositoriesNoConfigMap(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

	err := settingsManager.SaveRepositories([]Repository{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, cm.Data["repositories"], "- url: http://foo\n")
}

func TestSaveRepositoryCredentials(t *testing.T) {
	kubeClient, settingsManager := fixtures(nil)
	err := settingsManager.SaveRepositoryCredentials([]RepositoryCredentials{{URL: "http://foo"}})
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps("default").Get(context.Background(), common.ArgoCDConfigMapName, metav1.GetOptions{})
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
	ignoreStatus := v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
		JSONPointers: []string{"/status"},
	}}
	crdGK := "apiextensions.k8s.io/CustomResourceDefinition"

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
		IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/webhooks/0/clientConfig/caBundle"}},
	}, webHookOverrides)

	// by default, crd status should be ignored
	crdOverrides := overrides[crdGK]
	assert.NotNil(t, crdOverrides)
	assert.Equal(t, ignoreStatus, crdOverrides)

	// with value all, status of all objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: all`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	globalOverrides := overrides["*/*"]
	assert.NotNil(t, globalOverrides)
	assert.Equal(t, ignoreStatus, globalOverrides)

	// with value crd, status of crd objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: crd`,

		"resource.customizations": `
    apiextensions.k8s.io/CustomResourceDefinition:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	crdOverrides = overrides[crdGK]
	assert.NotNil(t, crdOverrides)
	assert.Equal(t, v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{JSONPointers: []string{"/webhooks/0/clientConfig/caBundle", "/status"}}}, crdOverrides)

	// with incorrect value, status of crd objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: foobar`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	defaultOverrides := overrides[crdGK]
	assert.NotNil(t, defaultOverrides)
	assert.Equal(t, ignoreStatus, defaultOverrides)
	assert.Equal(t, ignoreStatus, defaultOverrides)

	// with value off, status of no objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: off`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(overrides))

}

func TestSettingsManager_GetResourceOverrides_with_empty_string(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		resourceCustomizationsKey: "",
	})
	overrides, err := settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	assert.Len(t, overrides, 1)
}

func TestGetResourceCompareOptions(t *testing.T) {
	// ignoreAggregatedRules is true
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreAggregatedRoles: true",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		assert.NoError(t, err)
		assert.True(t, compareOptions.IgnoreAggregatedRoles)
	}

	// ignoreAggregatedRules is false
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreAggregatedRoles: false",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		assert.NoError(t, err)
		assert.False(t, compareOptions.IgnoreAggregatedRoles)
	}

	// The empty resource.compareoptions should result in default being returned
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		defaultOptions := GetDefaultDiffOptions()
		assert.NoError(t, err)
		assert.Equal(t, defaultOptions.IgnoreAggregatedRoles, compareOptions.IgnoreAggregatedRoles)
	}

	// resource.compareoptions not defined - should result in default being returned
	{
		_, settingsManager := fixtures(map[string]string{})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		defaultOptions := GetDefaultDiffOptions()
		assert.NoError(t, err)
		assert.Equal(t, defaultOptions.IgnoreAggregatedRoles, compareOptions.IgnoreAggregatedRoles)
	}
}

func TestSettingsManager_GetKustomizeBuildOptions(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{})

		settings, err := settingsManager.GetKustomizeSettings()

		assert.NoError(t, err)
		assert.Empty(t, settings.BuildOptions)
		assert.Empty(t, settings.Versions)
	})
	t.Run("Set", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"kustomize.buildOptions":   "foo",
			"kustomize.version.v3.2.1": "somePath",
		})

		options, err := settingsManager.GetKustomizeSettings()

		assert.NoError(t, err)
		assert.Equal(t, "foo", options.BuildOptions)
		assert.Equal(t, []KustomizeVersion{{Name: "v3.2.1", Path: "somePath"}}, options.Versions)
	})
}

func TestKustomizeSettings_GetOptions(t *testing.T) {
	settings := KustomizeSettings{Versions: []KustomizeVersion{
		{Name: "v1", Path: "path_v1"},
		{Name: "v2", Path: "path_v2"},
		{Name: "v3", Path: "path_v3"},
	}}

	t.Run("VersionDoesNotExist", func(t *testing.T) {
		_, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v4"}})
		assert.Error(t, err)
	})

	t.Run("VersionExists", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v2"}})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "path_v2", ver.BinaryPath)
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

func Test_validateExternalURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		errMsg string
	}{
		{name: "Valid URL", url: "https://my.domain.com"},
		{name: "No URL - Valid", url: ""},
		{name: "Invalid URL", url: "my.domain.com", errMsg: "URL must include http or https protocol"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExternalURL(tt.url)
			if tt.errMsg != "" {
				assert.EqualError(t, err, tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetOIDCSecretTrim(t *testing.T) {
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
				"oidc.config": "\n  name: Okta\n  clientSecret: test-secret\r\n \n  clientID: aaaabbbbccccddddeee\n",
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
	assert.Equal(t, "test-secret", oidcConfig.ClientSecret)
}
