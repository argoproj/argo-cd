package settings

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v2/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestInClusterServerAddressEnabled(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"cluster.inClusterEnabled": "true",
	})
	argoCDCM, err := settingsManager.getConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, true, argoCDCM.Data[inClusterEnabledKey] == "true")

	_, settingsManager = fixtures(map[string]string{
		"cluster.inClusterEnabled": "false",
	})
	argoCDCM, err = settingsManager.getConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, false, argoCDCM.Data[inClusterEnabledKey] == "true")
}

func TestInClusterServerAddressEnabledByDefault(t *testing.T) {
	kubeClient := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{},
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
	assert.Equal(t, true, settings.InClusterEnabled)
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
	ignoreCRDFields := v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
		JSONPointers: []string{"/status", "/spec/preserveUnknownFields"},
	}}
	crdGK := "apiextensions.k8s.io/CustomResourceDefinition"

	_, settingsManager := fixtures(map[string]string{
		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[0].clientConfig.caBundle`,
	})
	overrides, err := settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	webHookOverrides := overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"]
	assert.NotNil(t, webHookOverrides)

	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
			JSONPointers:      []string{"/webhooks/0/clientConfig/caBundle"},
			JQPathExpressions: []string{".webhooks[0].clientConfig.caBundle"},
		},
	}, webHookOverrides)

	// by default, crd status should be ignored
	crdOverrides := overrides[crdGK]
	assert.NotNil(t, crdOverrides)
	assert.Equal(t, ignoreCRDFields, crdOverrides)

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
        - /webhooks/0/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[0].clientConfig.caBundle`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	assert.NoError(t, err)

	crdOverrides = overrides[crdGK]
	assert.NotNil(t, crdOverrides)
	assert.Equal(t, v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
		JSONPointers:      []string{"/webhooks/0/clientConfig/caBundle", "/status", "/spec/preserveUnknownFields"},
		JQPathExpressions: []string{".webhooks[0].clientConfig.caBundle"},
	}}, crdOverrides)

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

func TestGetResourceOverrides_with_splitted_keys(t *testing.T) {
	data := map[string]string{
		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - foo
    certmanager.k8s.io/Certificate:
      health.lua.useOpenLibs: true
      health.lua: |
        foo
    cert-manager.io/Certificate:
      health.lua: |
        foo
    apps/Deployment:
      actions: |
        foo`,
	}

	t.Run("MergedKey", func(t *testing.T) {
		crdGK := "apiextensions.k8s.io/CustomResourceDefinition"
		_, settingsManager := fixtures(data)

		overrides, err := settingsManager.GetResourceOverrides()
		assert.NoError(t, err)
		assert.Equal(t, 5, len(overrides))
		assert.Equal(t, 2, len(overrides[crdGK].IgnoreDifferences.JSONPointers))
		assert.Equal(t, 1, len(overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, "foo", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers[0])
		assert.Equal(t, "foo\n", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.Equal(t, true, overrides["certmanager.k8s.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "foo\n", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.Equal(t, false, overrides["cert-manager.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "foo", overrides["apps/Deployment"].Actions)
	})

	t.Run("SplitKeys", func(t *testing.T) {
		newData := map[string]string{
			"resource.customizations.health.admissionregistration.k8s.io_MutatingWebhookConfiguration": "bar",
			"resource.customizations.ignoreDifferences.admissionregistration.k8s.io_MutatingWebhookConfiguration": `jsonPointers:
        - bar`,
			"resource.customizations.knownTypeFields.admissionregistration.k8s.io_MutatingWebhookConfiguration": `
- field: foo
  type: bar`,
			"resource.customizations.health.certmanager.k8s.io_Certificate":      "bar",
			"resource.customizations.health.cert-manager.io_Certificate":         "bar",
			"resource.customizations.useOpenLibs.certmanager.k8s.io_Certificate": "false",
			"resource.customizations.useOpenLibs.cert-manager.io_Certificate":    "true",
			"resource.customizations.actions.apps_Deployment":                    "bar",
			"resource.customizations.actions.Deployment":                         "bar",
			"resource.customizations.health.iam-manager.k8s.io_Iamrole":          "bar",
			"resource.customizations.health.Iamrole":                             "bar",
			"resource.customizations.ignoreDifferences.iam-manager.k8s.io_Iamrole": `jsonPointers:
        - bar`,
			"resource.customizations.ignoreDifferences.apps_Deployment": `jqPathExpressions:
        - bar`,
			"resource.customizations.ignoreDifferences.all": `managedFieldsManagers: 
        - kube-controller-manager
        - argo-rollouts`,
		}
		crdGK := "apiextensions.k8s.io/CustomResourceDefinition"

		_, settingsManager := fixtures(mergemaps(data, newData))

		overrides, err := settingsManager.GetResourceOverrides()
		assert.NoError(t, err)
		assert.Equal(t, 9, len(overrides))
		assert.Equal(t, 2, len(overrides[crdGK].IgnoreDifferences.JSONPointers))
		assert.Equal(t, "/status", overrides[crdGK].IgnoreDifferences.JSONPointers[0])
		assert.Equal(t, "/spec/preserveUnknownFields", overrides[crdGK].IgnoreDifferences.JSONPointers[1])
		assert.Equal(t, 1, len(overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers[0])
		assert.Equal(t, 1, len(overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].KnownTypeFields))
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].KnownTypeFields[0].Type)
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].HealthLua)
		assert.Equal(t, "bar", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.Equal(t, false, overrides["certmanager.k8s.io/Certificate"].UseOpenLibs)
		assert.Equal(t, true, overrides["cert-manager.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "bar", overrides["apps/Deployment"].Actions)
		assert.Equal(t, "bar", overrides["Deployment"].Actions)
		assert.Equal(t, "bar", overrides["iam-manager.k8s.io/Iamrole"].HealthLua)
		assert.Equal(t, "bar", overrides["Iamrole"].HealthLua)
		assert.Equal(t, 1, len(overrides["iam-manager.k8s.io/Iamrole"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, 1, len(overrides["apps/Deployment"].IgnoreDifferences.JQPathExpressions))
		assert.Equal(t, "bar", overrides["apps/Deployment"].IgnoreDifferences.JQPathExpressions[0])
		assert.Equal(t, 2, len(overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers))
		assert.Equal(t, "kube-controller-manager", overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers[0])
		assert.Equal(t, "argo-rollouts", overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers[1])
	})

	t.Run("SplitKeysCompareOptionsAll", func(t *testing.T) {
		newData := map[string]string{
			"resource.customizations.health.cert-manager.io_Certificate": "bar",
			"resource.customizations.actions.apps_Deployment":            "bar",
			"resource.compareoptions":                                    `ignoreResourceStatusField: all`,
		}
		_, settingsManager := fixtures(mergemaps(data, newData))

		overrides, err := settingsManager.GetResourceOverrides()
		assert.NoError(t, err)
		assert.Equal(t, 5, len(overrides))
		assert.Equal(t, 1, len(overrides["*/*"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, 1, len(overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, "foo\n", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["apps/Deployment"].Actions)
	})

	t.Run("SplitKeysCompareOptionsOff", func(t *testing.T) {
		newData := map[string]string{
			"resource.customizations.health.cert-manager.io_Certificate": "bar",
			"resource.customizations.actions.apps_Deployment":            "bar",
			"resource.compareoptions":                                    `ignoreResourceStatusField: off`,
		}
		_, settingsManager := fixtures(mergemaps(data, newData))

		overrides, err := settingsManager.GetResourceOverrides()
		assert.NoError(t, err)
		assert.Equal(t, 4, len(overrides))
		assert.Equal(t, 1, len(overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers))
		assert.Equal(t, "foo\n", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["apps/Deployment"].Actions)
	})
}

func mergemaps(mapA map[string]string, mapB map[string]string) map[string]string {
	for k, v := range mapA {
		mapB[k] = v
	}
	return mapB
}

func TestConvertToOverrideKey(t *testing.T) {
	key, err := convertToOverrideKey("cert-manager.io_Certificate")
	assert.NoError(t, err)
	assert.Equal(t, "cert-manager.io/Certificate", key)

	key, err = convertToOverrideKey("Certificate")
	assert.NoError(t, err)
	assert.Equal(t, "Certificate", key)

	_, err = convertToOverrideKey("")
	assert.NotNil(t, err)

	_, err = convertToOverrideKey("_")
	assert.NoError(t, err)
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

	t.Run("Kustomize settings per-version", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"kustomize.buildOptions":        "--global true",
			"kustomize.version.v3.2.1":      "/path_3.2.1",
			"kustomize.buildOptions.v3.2.3": "--options v3.2.3",
			"kustomize.path.v3.2.3":         "/path_3.2.3",
			"kustomize.path.v3.2.4":         "/path_3.2.4",
			"kustomize.buildOptions.v3.2.4": "--options v3.2.4",
			"kustomize.buildOptions.v3.2.5": "--options v3.2.5",
		})

		got, err := settingsManager.GetKustomizeSettings()

		assert.NoError(t, err)
		assert.Equal(t, "--global true", got.BuildOptions)
		want := &KustomizeSettings{
			BuildOptions: "--global true",
			Versions: []KustomizeVersion{
				{Name: "v3.2.1", Path: "/path_3.2.1"},
				{Name: "v3.2.3", Path: "/path_3.2.3", BuildOptions: "--options v3.2.3"},
				{Name: "v3.2.4", Path: "/path_3.2.4", BuildOptions: "--options v3.2.4"},
			},
		}
		sortVersionsByName := func(versions []KustomizeVersion) {
			sort.Slice(versions, func(i, j int) bool {
				return versions[i].Name > versions[j].Name
			})
		}
		sortVersionsByName(want.Versions)
		sortVersionsByName(got.Versions)
		assert.EqualValues(t, want, got)
	})

	t.Run("Kustomize settings per-version with duplicate versions", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"kustomize.buildOptions":        "--global true",
			"kustomize.version.v3.2.1":      "/path_3.2.1",
			"kustomize.buildOptions.v3.2.1": "--options v3.2.3",
			"kustomize.path.v3.2.2":         "/other_path_3.2.2",
			"kustomize.path.v3.2.1":         "/other_path_3.2.1",
		})

		got, err := settingsManager.GetKustomizeSettings()
		assert.EqualError(t, err, "found duplicate kustomize version: v3.2.1")
		assert.Empty(t, got)
	})

	t.Run("Config map with no Kustomize settings", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"other.options": "--global true",
		})

		got, err := settingsManager.GetKustomizeSettings()
		assert.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestKustomizeSettings_GetOptions(t *testing.T) {
	settings := KustomizeSettings{
		BuildOptions: "--opt1 val1",
		Versions: []KustomizeVersion{
			{Name: "v1", Path: "path_v1"},
			{Name: "v2", Path: "path_v2"},
			{Name: "v3", Path: "path_v3", BuildOptions: "--opt2 val2"},
		},
	}

	t.Run("VersionDoesNotExist", func(t *testing.T) {
		_, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v4"}})
		assert.Error(t, err)
	})

	t.Run("DefaultBuildOptions", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "", ver.BinaryPath)
		assert.Equal(t, "--opt1 val1", ver.BuildOptions)
	})

	t.Run("VersionExists", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v2"}})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "path_v2", ver.BinaryPath)
		assert.Equal(t, "", ver.BuildOptions)
	})

	t.Run("VersionExistsWithBuildOption", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v3"}})
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "path_v3", ver.BinaryPath)
		assert.Equal(t, "--opt2 val2", ver.BuildOptions)
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
	t.Run("GetBinaryUrls", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"help.download.darwin-amd64": "amd64-path",
			"help.download.unsupported":  "nowhere",
		})
		h, err := settingsManager.GetHelp()
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{"darwin-amd64": "amd64-path"}, h.BinaryURLs)
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

func getCNFromCertificate(cert *tls.Certificate) string {
	c, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return ""
	}
	return c.Subject.CommonName
}

func Test_GetTLSConfiguration(t *testing.T) {
	t.Run("Valid external TLS secret with success", func(t *testing.T) {
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
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      externalServerTLSSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"tls.crt": []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-test-server.crt")),
					"tls.key": []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-test-server.key")),
				},
			},
		)
		settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		assert.NoError(t, err)
		assert.True(t, settings.CertificateIsExternal)
		assert.NotNil(t, settings.Certificate)
		assert.Contains(t, getCNFromCertificate(settings.Certificate), "localhost")
	})

	t.Run("Valid external TLS secret overrides argocd-secret", func(t *testing.T) {
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
					"tls.crt":          []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-e2e-server.crt")),
					"tls.key":          []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-e2e-server.key")),
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      externalServerTLSSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"tls.crt": []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-test-server.crt")),
					"tls.key": []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-test-server.key")),
				},
			},
		)
		settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		assert.NoError(t, err)
		assert.True(t, settings.CertificateIsExternal)
		assert.NotNil(t, settings.Certificate)
		assert.Contains(t, getCNFromCertificate(settings.Certificate), "localhost")
	})
	t.Run("Invalid external TLS secret", func(t *testing.T) {
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
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      externalServerTLSSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"tls.crt": []byte(""),
					"tls.key": []byte(""),
				},
			},
		)
		settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "could not read from secret")
		assert.NotNil(t, settings)
	})
	t.Run("No external TLS secret", func(t *testing.T) {
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
					"tls.crt":          []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-e2e-server.crt")),
					"tls.key":          []byte(testutil.MustLoadFileToString("../../test/fixture/certs/argocd-e2e-server.key")),
				},
			},
		)
		settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		assert.NoError(t, err)
		assert.False(t, settings.CertificateIsExternal)
		assert.NotNil(t, settings.Certificate)
		assert.Contains(t, getCNFromCertificate(settings.Certificate), "Argo CD E2E")
	})
}

func TestDownloadArgoCDBinaryUrls(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"help.download.darwin-amd64": "some-url",
	})
	argoCDCM, err := settingsManager.getConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, "some-url", argoCDCM.Data["help.download.darwin-amd64"])

	_, settingsManager = fixtures(map[string]string{
		"help.download.unsupported": "some-url",
	})
	argoCDCM, err = settingsManager.getConfigMap()
	assert.NoError(t, err)
	assert.Equal(t, "some-url", argoCDCM.Data["help.download.unsupported"])
}

func TestSecretKeyRef(t *testing.T) {
	data := map[string]string{
		"oidc.config": `name: Okta
issuer: https://dev-123456.oktapreview.com
clientID: aaaabbbbccccddddeee
clientSecret: $acme:clientSecret
# Optional set of OIDC scopes to request. If omitted, defaults to: ["openid", "profile", "email", "groups"]
requestedScopes: ["openid", "profile", "email"]
# Optional set of OIDC claims to request on the ID token.
requestedIDTokenClaims: {"groups": {"essential": true}}`,
	}
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
	argocdSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"admin.password":   nil,
			"server.secretkey": nil,
		},
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acme",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"clientSecret": []byte("deadbeef"),
		},
	}
	kubeClient := fake.NewSimpleClientset(cm, secret, argocdSecret)
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

	settings, err := settingsManager.GetSettings()
	assert.NoError(t, err)

	oidcConfig := settings.OIDCConfig()
	assert.Equal(t, oidcConfig.ClientSecret, "deadbeef")
}

func TestGetEnableManifestGeneration(t *testing.T) {
	testCases := []struct {
		name    string
		enabled bool
		data    map[string]string
		source  string
	}{{
		name:    "default",
		enabled: true,
		data:    map[string]string{},
		source:  string(v1alpha1.ApplicationSourceTypeKustomize),
	}, {
		name:    "disabled",
		enabled: false,
		data:    map[string]string{"kustomize.enable": `false`},
		source:  string(v1alpha1.ApplicationSourceTypeKustomize),
	}, {
		name:    "enabled",
		enabled: true,
		data:    map[string]string{"kustomize.enable": `true`},
		source:  string(v1alpha1.ApplicationSourceTypeKustomize),
	}}
	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: tc.data,
			}
			argocdSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"admin.password":   nil,
					"server.secretkey": nil,
				},
			}

			kubeClient := fake.NewSimpleClientset(cm, argocdSecret)
			settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

			enableManifestGeneration, err := settingsManager.GetEnabledSourceTypes()
			require.NoError(t, err)

			assert.Equal(t, enableManifestGeneration[tc.source], tc.enabled)
		})
	}
}

func TestGetHelmSettings(t *testing.T) {
	testCases := []struct {
		name     string
		data     map[string]string
		expected []string
	}{{
		name:     "Default",
		data:     map[string]string{},
		expected: []string{"http", "https"},
	}, {
		name: "Configured Not Empty",
		data: map[string]string{
			"helm.valuesFileSchemes": "s3, git",
		},
		expected: []string{"s3", "git"},
	}, {
		name: "Configured Empty",
		data: map[string]string{
			"helm.valuesFileSchemes": "",
		},
		expected: nil,
	}}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: tc.data,
			}
			argocdSecret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"admin.password":   nil,
					"server.secretkey": nil,
				},
			}
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "acme",
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string][]byte{
					"clientSecret": []byte("deadbeef"),
				},
			}
			kubeClient := fake.NewSimpleClientset(cm, secret, argocdSecret)
			settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

			helmSettings, err := settingsManager.GetHelmSettings()
			assert.NoError(t, err)

			assert.ElementsMatch(t, tc.expected, helmSettings.ValuesFileSchemes)
		})
	}
}
