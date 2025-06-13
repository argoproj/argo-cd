package settings

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	testutil "github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/test"
)

func fixtures(data map[string]string, opts ...func(secret *corev1.Secret)) (*fake.Clientset, *SettingsManager) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}
	secret := &corev1.Secret{
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
	kubeClient := fake.NewClientset(cm, secret)
	settingsManager := NewSettingsManager(context.Background(), kubeClient, "default")

	return kubeClient, settingsManager
}

func TestDocumentedArgoCDConfigMapIsValid(t *testing.T) {
	var argocdCM *corev1.ConfigMap
	settings := ArgoCDSettings{}
	data, err := os.ReadFile("../../docs/operator-manual/argocd-cm.yaml")
	require.NoError(t, err)
	err = yaml.Unmarshal(data, &argocdCM)
	require.NoError(t, err)
	updateSettingsFromConfigMap(&settings, argocdCM)
}

func TestGetConfigMapByName(t *testing.T) {
	t.Run("data is never nil", func(t *testing.T) {
		_, settingsManager := fixtures(nil)
		cm, err := settingsManager.GetConfigMapByName(common.ArgoCDConfigMapName)
		require.NoError(t, err)
		assert.NotNil(t, cm.Data)
	})
	t.Run("cannot update informer value", func(t *testing.T) {
		_, settingsManager := fixtures(nil)
		cm1, err := settingsManager.GetConfigMapByName(common.ArgoCDConfigMapName)
		require.NoError(t, err)
		cm1.Data["test"] = "invalid"
		cm2, err := settingsManager.GetConfigMapByName(common.ArgoCDConfigMapName)
		require.NoError(t, err)
		assert.NotContains(t, cm2.Data, "test")
	})
}

func TestGetSecretByName(t *testing.T) {
	t.Run("data is never nil", func(t *testing.T) {
		_, settingsManager := fixtures(nil, func(secret *corev1.Secret) { secret.Data = nil })
		secret, err := settingsManager.GetSecretByName(common.ArgoCDSecretName)
		require.NoError(t, err)
		assert.NotNil(t, secret.Data)
	})
	t.Run("cannot update informer value", func(t *testing.T) {
		_, settingsManager := fixtures(nil)
		s1, err := settingsManager.GetSecretByName(common.ArgoCDSecretName)
		require.NoError(t, err)
		s1.Data["test"] = []byte("invalid")
		s2, err := settingsManager.GetSecretByName(common.ArgoCDSecretName)
		require.NoError(t, err)
		assert.NotContains(t, s2.Data, "test")
	})
}

func TestGetExtensionConfigs(t *testing.T) {
	type cases struct {
		name        string
		input       map[string]string
		expected    map[string]string
		expectedLen int
	}

	testCases := []cases{
		{
			name:        "will return main config successfully",
			expectedLen: 1,
			input: map[string]string{
				extensionConfig: "test",
			},
			expected: map[string]string{
				"": "test",
			},
		},
		{
			name:        "will return main and additional config successfully",
			expectedLen: 2,
			input: map[string]string{
				extensionConfig:                       "main config",
				extensionConfig + ".anotherExtension": "another config",
			},
			expected: map[string]string{
				"":                 "main config",
				"anotherExtension": "another config",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// When
			output := getExtensionConfigs(tc.input)

			// Then
			assert.Len(t, output, tc.expectedLen)
			assert.Equal(t, tc.expected, output)
		})
	}
}

func TestGetResourceFilter(t *testing.T) {
	data := map[string]string{
		"resource.exclusions": "\n  - apiGroups: [\"group1\"]\n    kinds: [\"kind1\"]\n    clusters: [\"cluster1\"]\n",
		"resource.inclusions": "\n  - apiGroups: [\"group2\"]\n    kinds: [\"kind2\"]\n    clusters: [\"cluster2\"]\n",
	}
	_, settingsManager := fixtures(data)
	filter, err := settingsManager.GetResourcesFilter()
	require.NoError(t, err)
	assert.Equal(t, &ResourcesFilter{
		ResourceExclusions: []FilteredResource{{APIGroups: []string{"group1"}, Kinds: []string{"kind1"}, Clusters: []string{"cluster1"}}},
		ResourceInclusions: []FilteredResource{{APIGroups: []string{"group2"}, Kinds: []string{"kind2"}, Clusters: []string{"cluster2"}}},
	}, filter)
}

func TestInClusterServerAddressEnabled(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"cluster.inClusterEnabled": "true",
	})
	argoCDCM, err := settingsManager.getConfigMap()
	require.NoError(t, err)
	assert.Equal(t, "true", argoCDCM.Data[inClusterEnabledKey])

	_, settingsManager = fixtures(map[string]string{
		"cluster.inClusterEnabled": "false",
	})
	argoCDCM, err = settingsManager.getConfigMap()
	require.NoError(t, err)
	assert.NotEqual(t, "true", argoCDCM.Data[inClusterEnabledKey])
}

func TestInClusterServerAddressEnabledByDefault(t *testing.T) {
	kubeClient := fake.NewClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.ArgoCDConfigMapName,
				Namespace: "default",
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{},
		},
		&corev1.Secret{
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
	settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
	settings, err := settingsManager.GetSettings()
	require.NoError(t, err)
	assert.True(t, settings.InClusterEnabled)
}

func TestGetAppInstanceLabelKey(t *testing.T) {
	t.Run("should get custom instanceLabelKey", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"application.instanceLabelKey": "testLabel",
		})
		label, err := settingsManager.GetAppInstanceLabelKey()
		require.NoError(t, err)
		assert.Equal(t, "testLabel", label)
	})

	t.Run("should get default instanceLabelKey if custom not defined", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{})
		label, err := settingsManager.GetAppInstanceLabelKey()
		require.NoError(t, err)
		assert.Equal(t, common.LabelKeyAppInstance, label)
	})
}

func TestGetTrackingMethod(t *testing.T) {
	t.Run("should get custom trackingMethod", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"application.resourceTrackingMethod": string(v1alpha1.TrackingMethodLabel),
		})
		label, err := settingsManager.GetTrackingMethod()
		require.NoError(t, err)
		assert.Equal(t, string(v1alpha1.TrackingMethodLabel), label)
	})

	t.Run("should get default trackingMethod if custom not defined", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{})
		label, err := settingsManager.GetTrackingMethod()
		require.NoError(t, err)
		assert.Equal(t, string(v1alpha1.TrackingMethodAnnotation), label)
	})
}

func TestApplicationFineGrainedRBACInheritanceDisabledDefault(t *testing.T) {
	_, settingsManager := fixtures(nil)
	flag, err := settingsManager.ApplicationFineGrainedRBACInheritanceDisabled()
	require.NoError(t, err)
	assert.True(t, flag)
}

func TestApplicationFineGrainedRBACInheritanceDisabled(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"server.rbac.disableApplicationFineGrainedRBACInheritance": "false",
	})
	flag, err := settingsManager.ApplicationFineGrainedRBACInheritanceDisabled()
	require.NoError(t, err)
	assert.False(t, flag)
}

func TestGetIsIgnoreResourceUpdatesEnabled(t *testing.T) {
	_, settingsManager := fixtures(nil)
	ignoreResourceUpdatesEnabled, err := settingsManager.GetIsIgnoreResourceUpdatesEnabled()
	require.NoError(t, err)
	assert.True(t, ignoreResourceUpdatesEnabled)

	_, settingsManager = fixtures(map[string]string{
		"resource.ignoreResourceUpdatesEnabled": "true",
	})
	ignoreResourceUpdatesEnabled, err = settingsManager.GetIsIgnoreResourceUpdatesEnabled()
	require.NoError(t, err)
	assert.True(t, ignoreResourceUpdatesEnabled)
}

func TestGetIsIgnoreResourceUpdatesEnabledFalse(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"resource.ignoreResourceUpdatesEnabled": "false",
	})
	ignoreResourceUpdatesEnabled, err := settingsManager.GetIsIgnoreResourceUpdatesEnabled()
	require.NoError(t, err)
	assert.False(t, ignoreResourceUpdatesEnabled)
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
        - /webhooks/0/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[0].clientConfig.caBundle
      ignoreResourceUpdates: |
        jsonPointers:
        - /webhooks/1/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[1].clientConfig.caBundle`,
	})
	overrides, err := settingsManager.GetResourceOverrides()
	require.NoError(t, err)

	webHookOverrides := overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"]
	assert.NotNil(t, webHookOverrides)

	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
			JSONPointers:      []string{"/webhooks/0/clientConfig/caBundle"},
			JQPathExpressions: []string{".webhooks[0].clientConfig.caBundle"},
		},
		IgnoreResourceUpdates: v1alpha1.OverrideIgnoreDiff{
			JSONPointers:      []string{"/webhooks/1/clientConfig/caBundle"},
			JQPathExpressions: []string{".webhooks[1].clientConfig.caBundle"},
		},
	}, webHookOverrides)

	// by default, all status should be ignored
	globalOverrides := overrides["*/*"]
	assert.NotNil(t, globalOverrides)
	assert.Equal(t, ignoreStatus, globalOverrides)

	// with value all, status of all objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: all`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	require.NoError(t, err)

	globalOverrides = overrides["*/*"]
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
	require.NoError(t, err)

	crdOverrides := overrides[crdGK]
	assert.NotNil(t, crdOverrides)
	assert.Equal(t, v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
		JSONPointers:      []string{"/webhooks/0/clientConfig/caBundle", "/status"},
		JQPathExpressions: []string{".webhooks[0].clientConfig.caBundle"},
	}}, crdOverrides)

	// with incorrect value, status of all objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: foobar`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	require.NoError(t, err)

	globalOverrides = overrides["*/*"]
	assert.NotNil(t, globalOverrides)
	assert.Equal(t, ignoreStatus, globalOverrides)

	// with value non-string off, status of no objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: off`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	require.NoError(t, err)
	assert.Empty(t, overrides)

	// with value non-string false, status of no objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: false`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	require.NoError(t, err)
	assert.Empty(t, overrides)

	// with value none, status of no objects should be ignored
	_, settingsManager = fixtures(map[string]string{
		"resource.compareoptions": `
    ignoreResourceStatusField: none`,
	})
	overrides, err = settingsManager.GetResourceOverrides()
	require.NoError(t, err)
	assert.Empty(t, overrides)
}

func TestGetResourceOverridesHealthWithWildcard(t *testing.T) {
	data := map[string]string{
		"resource.customizations": `
    "*.aws.crossplane.io/*":
      health.lua: |
        foo`,
	}

	t.Run("TestResourceHealthOverrideWithWildcard", func(t *testing.T) {
		_, settingsManager := fixtures(data)

		overrides, err := settingsManager.GetResourceOverrides()
		require.NoError(t, err)
		assert.Len(t, overrides, 2)
		assert.Equal(t, "foo", overrides["*.aws.crossplane.io/*"].HealthLua)
	})
}

func TestSettingsManager_GetResourceOverrides_with_empty_string(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		resourceCustomizationsKey: "",
	})
	overrides, err := settingsManager.GetResourceOverrides()
	require.NoError(t, err)

	assert.Len(t, overrides, 1)
}

func TestGetResourceOverrides_with_splitted_keys(t *testing.T) {
	data := map[string]string{
		"resource.compareoptions": `ignoreResourceStatusField: none`,

		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - foo
      ignoreResourceUpdates: |
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
		_, settingsManager := fixtures(data)

		overrides, err := settingsManager.GetResourceOverrides()
		require.NoError(t, err)
		assert.Len(t, overrides, 4)
		assert.Len(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers, 1)
		assert.Equal(t, "foo", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers[0])
		assert.Len(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreResourceUpdates.JSONPointers, 1)
		assert.Equal(t, "foo", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreResourceUpdates.JSONPointers[0])
		assert.Equal(t, "foo\n", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.True(t, overrides["certmanager.k8s.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "foo\n", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.False(t, overrides["cert-manager.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "foo", overrides["apps/Deployment"].Actions)
	})

	t.Run("SplitKeys", func(t *testing.T) {
		newData := map[string]string{
			"resource.compareoptions": `ignoreResourceStatusField: none`,

			"resource.customizations.health.admissionregistration.k8s.io_MutatingWebhookConfiguration": "bar",
			"resource.customizations.ignoreDifferences.admissionregistration.k8s.io_MutatingWebhookConfiguration": `jsonPointers:
        - bar`,
			"resource.customizations.ignoreResourceUpdates.admissionregistration.k8s.io_MutatingWebhookConfiguration": `jsonPointers:
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
			"resource.customizations.ignoreResourceUpdates.iam-manager.k8s.io_Iamrole": `jsonPointers:
        - bar`,
			"resource.customizations.ignoreResourceUpdates.apps_Deployment": `jqPathExpressions:
        - bar`,
		}

		_, settingsManager := fixtures(mergemaps(data, newData))

		overrides, err := settingsManager.GetResourceOverrides()
		require.NoError(t, err)
		assert.Len(t, overrides, 8)
		assert.Len(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers, 1)
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreDifferences.JSONPointers[0])
		assert.Len(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreResourceUpdates.JSONPointers, 1)
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].IgnoreResourceUpdates.JSONPointers[0])
		assert.Len(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].KnownTypeFields, 1)
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].KnownTypeFields[0].Type)
		assert.Equal(t, "bar", overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"].HealthLua)
		assert.Equal(t, "bar", overrides["certmanager.k8s.io/Certificate"].HealthLua)
		assert.Equal(t, "bar", overrides["cert-manager.io/Certificate"].HealthLua)
		assert.False(t, overrides["certmanager.k8s.io/Certificate"].UseOpenLibs)
		assert.True(t, overrides["cert-manager.io/Certificate"].UseOpenLibs)
		assert.Equal(t, "bar", overrides["apps/Deployment"].Actions)
		assert.Equal(t, "bar", overrides["Deployment"].Actions)
		assert.Equal(t, "bar", overrides["iam-manager.k8s.io/Iamrole"].HealthLua)
		assert.Equal(t, "bar", overrides["Iamrole"].HealthLua)
		assert.Len(t, overrides["iam-manager.k8s.io/Iamrole"].IgnoreDifferences.JSONPointers, 1)
		assert.Len(t, overrides["apps/Deployment"].IgnoreDifferences.JQPathExpressions, 1)
		assert.Equal(t, "bar", overrides["apps/Deployment"].IgnoreDifferences.JQPathExpressions[0])
		assert.Len(t, overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers, 2)
		assert.Equal(t, "kube-controller-manager", overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers[0])
		assert.Equal(t, "argo-rollouts", overrides["*/*"].IgnoreDifferences.ManagedFieldsManagers[1])
		assert.Len(t, overrides["iam-manager.k8s.io/Iamrole"].IgnoreResourceUpdates.JSONPointers, 1)
		assert.Len(t, overrides["apps/Deployment"].IgnoreResourceUpdates.JQPathExpressions, 1)
		assert.Equal(t, "bar", overrides["apps/Deployment"].IgnoreResourceUpdates.JQPathExpressions[0])
	})
}

func mergemaps(mapA map[string]string, mapB map[string]string) map[string]string {
	for k, v := range mapA {
		mapB[k] = v
	}
	return mapB
}

func TestGetIgnoreResourceUpdatesOverrides(t *testing.T) {
	allDefault := v1alpha1.ResourceOverride{IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
		JSONPointers: []string{"/metadata/resourceVersion", "/metadata/generation", "/metadata/managedFields"},
	}}
	allGK := "*/*"

	testCustomizations := map[string]string{
		"resource.compareoptions": `
            ignoreResourceStatusField: none
            ignoreDifferencesOnResourceUpdates: true`,

		"resource.customizations": `
    admissionregistration.k8s.io/MutatingWebhookConfiguration:
      ignoreDifferences: |
        jsonPointers:
        - /webhooks/0/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[0].clientConfig.caBundle
      ignoreResourceUpdates: |
        jsonPointers:
        - /webhooks/1/clientConfig/caBundle
        jqPathExpressions:
        - .webhooks[1].clientConfig.caBundle`,
	}

	_, settingsManager := fixtures(testCustomizations)
	overrides, err := settingsManager.GetIgnoreResourceUpdatesOverrides()
	require.NoError(t, err)

	// default overrides should always be present
	allOverrides := overrides[allGK]
	assert.NotNil(t, allOverrides)
	assert.Equal(t, allDefault, allOverrides)

	// with ignoreDifferencesOnResourceUpdates, ignoreDifferences should be added
	assert.NotNil(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"])
	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
			JSONPointers:      []string{"/webhooks/1/clientConfig/caBundle", "/webhooks/0/clientConfig/caBundle"},
			JQPathExpressions: []string{".webhooks[1].clientConfig.caBundle", ".webhooks[0].clientConfig.caBundle"},
		},
		IgnoreResourceUpdates: v1alpha1.OverrideIgnoreDiff{},
	}, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"])

	// without ignoreDifferencesOnResourceUpdates, only ignoreResourceUpdates should be added
	_, settingsManager = fixtures(mergemaps(map[string]string{
		"resource.compareoptions": `
            ignoreResourceStatusField: none
            ignoreDifferencesOnResourceUpdates: false`,
	}, testCustomizations))
	overrides, err = settingsManager.GetIgnoreResourceUpdatesOverrides()
	require.NoError(t, err)
	assert.NotNil(t, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"])
	assert.Equal(t, v1alpha1.ResourceOverride{
		IgnoreDifferences: v1alpha1.OverrideIgnoreDiff{
			JSONPointers:      []string{"/webhooks/1/clientConfig/caBundle"},
			JQPathExpressions: []string{".webhooks[1].clientConfig.caBundle"},
		},
		IgnoreResourceUpdates: v1alpha1.OverrideIgnoreDiff{},
	}, overrides["admissionregistration.k8s.io/MutatingWebhookConfiguration"])
}

func TestConvertToOverrideKey(t *testing.T) {
	key, err := convertToOverrideKey("cert-manager.io_Certificate")
	require.NoError(t, err)
	assert.Equal(t, "cert-manager.io/Certificate", key)

	key, err = convertToOverrideKey("Certificate")
	require.NoError(t, err)
	assert.Equal(t, "Certificate", key)

	_, err = convertToOverrideKey("")
	require.Error(t, err)

	_, err = convertToOverrideKey("_")
	require.NoError(t, err)
}

func TestGetResourceCompareOptions(t *testing.T) {
	// ignoreAggregatedRules is true
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreAggregatedRoles: true",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		require.NoError(t, err)
		assert.True(t, compareOptions.IgnoreAggregatedRoles)
	}

	// ignoreAggregatedRules is false
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreAggregatedRoles: false",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		require.NoError(t, err)
		assert.False(t, compareOptions.IgnoreAggregatedRoles)
	}

	// ignoreDifferencesOnResourceUpdates is true
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreDifferencesOnResourceUpdates: true",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		require.NoError(t, err)
		assert.True(t, compareOptions.IgnoreDifferencesOnResourceUpdates)
	}

	// ignoreDifferencesOnResourceUpdates is false
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "ignoreDifferencesOnResourceUpdates: false",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		require.NoError(t, err)
		assert.False(t, compareOptions.IgnoreDifferencesOnResourceUpdates)
	}

	// The empty resource.compareoptions should result in default being returned
	{
		_, settingsManager := fixtures(map[string]string{
			"resource.compareoptions": "",
		})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		defaultOptions := GetDefaultDiffOptions()
		require.NoError(t, err)
		assert.Equal(t, defaultOptions.IgnoreAggregatedRoles, compareOptions.IgnoreAggregatedRoles)
		assert.Equal(t, defaultOptions.IgnoreDifferencesOnResourceUpdates, compareOptions.IgnoreDifferencesOnResourceUpdates)
	}

	// resource.compareoptions not defined - should result in default being returned
	{
		_, settingsManager := fixtures(map[string]string{})
		compareOptions, err := settingsManager.GetResourceCompareOptions()
		defaultOptions := GetDefaultDiffOptions()
		require.NoError(t, err)
		assert.Equal(t, defaultOptions.IgnoreAggregatedRoles, compareOptions.IgnoreAggregatedRoles)
		assert.Equal(t, defaultOptions.IgnoreDifferencesOnResourceUpdates, compareOptions.IgnoreDifferencesOnResourceUpdates)
	}
}

func TestSettingsManager_GetKustomizeBuildOptions(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{})

		settings, err := settingsManager.GetKustomizeSettings()

		require.NoError(t, err)
		assert.Empty(t, settings.BuildOptions)
		assert.Empty(t, settings.Versions)
	})
	t.Run("Set", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"kustomize.buildOptions":   "foo",
			"kustomize.version.v3.2.1": "somePath",
		})

		options, err := settingsManager.GetKustomizeSettings()

		require.NoError(t, err)
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

		require.NoError(t, err)
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
		assert.Equal(t, want, got)
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
		require.ErrorContains(t, err, "found duplicate kustomize version: v3.2.1")
		assert.Empty(t, got)
	})

	t.Run("Config map with no Kustomize settings", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"other.options": "--global true",
		})

		got, err := settingsManager.GetKustomizeSettings()
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}

func TestSettingsManager_GetEventLabelKeys(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		expectedKeys []string
	}{
		{
			name:         "Comma separated data",
			data:         "app,env, tier,    example.com/team-*, *",
			expectedKeys: []string{"app", "env", "tier", "example.com/team-*", "*"},
		},
		{
			name:         "Empty data",
			expectedKeys: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, settingsManager := fixtures(map[string]string{})
			if tt.data != "" {
				_, settingsManager = fixtures(map[string]string{
					resourceIncludeEventLabelKeys: tt.data,
					resourceExcludeEventLabelKeys: tt.data,
				})
			}

			inKeys := settingsManager.GetIncludeEventLabelKeys()
			assert.Len(t, inKeys, len(tt.expectedKeys))

			exKeys := settingsManager.GetExcludeEventLabelKeys()
			assert.Len(t, exKeys, len(tt.expectedKeys))

			for i := range tt.expectedKeys {
				assert.Equal(t, tt.expectedKeys[i], inKeys[i])
				assert.Equal(t, tt.expectedKeys[i], exKeys[i])
			}
		})
	}
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
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v4"},
		})
		require.Error(t, err)
	})

	t.Run("DefaultBuildOptions", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{})
		require.NoError(t, err)
		assert.Empty(t, ver.BinaryPath)
		assert.Equal(t, "--opt1 val1", ver.BuildOptions)
	})

	t.Run("VersionExists", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v2"},
		})
		require.NoError(t, err)
		assert.Equal(t, "path_v2", ver.BinaryPath)
		assert.Empty(t, ver.BuildOptions)
	})

	t.Run("VersionExistsWithBuildOption", func(t *testing.T) {
		ver, err := settings.GetOptions(v1alpha1.ApplicationSource{
			Kustomize: &v1alpha1.ApplicationSourceKustomize{Version: "v3"},
		})
		require.NoError(t, err)
		assert.Equal(t, "path_v3", ver.BinaryPath)
		assert.Equal(t, "--opt2 val2", ver.BuildOptions)
	})
}

func TestGetGoogleAnalytics(t *testing.T) {
	_, settingsManager := fixtures(map[string]string{
		"ga.trackingid": "123",
	})
	ga, err := settingsManager.GetGoogleAnalytics()
	require.NoError(t, err)
	assert.Equal(t, "123", ga.TrackingID)
	assert.True(t, ga.AnonymizeUsers)
}

func TestSettingsManager_GetHelp(t *testing.T) {
	t.Run("Default", func(t *testing.T) {
		_, settingsManager := fixtures(nil)
		h, err := settingsManager.GetHelp()
		require.NoError(t, err)
		assert.Empty(t, h.ChatURL)
		assert.Empty(t, h.ChatText)
	})
	t.Run("Set", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"help.chatUrl":  "foo",
			"help.chatText": "bar",
		})
		h, err := settingsManager.GetHelp()
		require.NoError(t, err)
		assert.Equal(t, "foo", h.ChatURL)
		assert.Equal(t, "bar", h.ChatText)
	})
	t.Run("SetOnlyChatUrl", func(t *testing.T) {
		_, settingManager := fixtures(map[string]string{
			"help.chatUrl": "foo",
		})
		h, err := settingManager.GetHelp()
		require.NoError(t, err)
		assert.Equal(t, "foo", h.ChatURL)
		assert.Equal(t, "Chat now!", h.ChatText)
	})
	t.Run("SetOnlyChatText", func(t *testing.T) {
		_, settingManager := fixtures(map[string]string{
			"help.chatText": "bar",
		})
		h, err := settingManager.GetHelp()
		require.NoError(t, err)
		assert.Empty(t, h.ChatURL)
		assert.Empty(t, h.ChatText)
	})
	t.Run("GetBinaryUrls", func(t *testing.T) {
		_, settingsManager := fixtures(map[string]string{
			"help.download.darwin-amd64": "amd64-path",
			"help.download.linux-s390x":  "s390x-path",
			"help.download.unsupported":  "nowhere",
		})
		h, err := settingsManager.GetHelp()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"darwin-amd64": "amd64-path", "linux-s390x": "s390x-path"}, h.BinaryURLs)
	})
}

func TestSettingsManager_GetSettings(t *testing.T) {
	t.Run("UserSessionDurationNotProvided", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: nil,
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string][]byte{
					"server.secretkey": nil,
				},
			},
		)
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		s, err := settingsManager.GetSettings()
		require.NoError(t, err)
		assert.Equal(t, time.Hour*24, s.UserSessionDuration)
	})
	t.Run("UserSessionDurationInvalidFormat", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string]string{
					"users.session.duration": "10hh",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string][]byte{
					"server.secretkey": nil,
				},
			},
		)
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		s, err := settingsManager.GetSettings()
		require.NoError(t, err)
		assert.Equal(t, time.Hour*24, s.UserSessionDuration)
	})
	t.Run("UserSessionDurationProvided", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string]string{
					"users.session.duration": "10h",
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string][]byte{
					"server.secretkey": nil,
				},
			},
		)
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		s, err := settingsManager.GetSettings()
		require.NoError(t, err)
		assert.Equal(t, time.Hour*10, s.UserSessionDuration)
	})
}

func TestGetOIDCConfig(t *testing.T) {
	kubeClient := fake.NewClientset(
		&corev1.ConfigMap{
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
		&corev1.Secret{
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
	settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
	settings, err := settingsManager.GetSettings()
	require.NoError(t, err)

	oidcConfig := settings.OIDCConfig()
	assert.NotNil(t, oidcConfig)

	claim := oidcConfig.RequestedIDTokenClaims["groups"]
	assert.NotNil(t, claim)
	assert.True(t, claim.Essential)
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
		require.NoError(t, err)
		assert.Equal(t, expected[0], redirectURL)
		dexRedirectURL, err := settings.DexRedirectURL()
		require.NoError(t, err)
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
				require.NoError(t, err)
			}
		})
	}
}

func TestGetOIDCSecretTrim(t *testing.T) {
	kubeClient := fake.NewClientset(
		&corev1.ConfigMap{
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
		&corev1.Secret{
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
	settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
	settings, err := settingsManager.GetSettings()
	require.NoError(t, err)

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
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
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
			&corev1.Secret{
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
			&corev1.Secret{
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
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		require.NoError(t, err)
		assert.True(t, settings.CertificateIsExternal)
		assert.NotNil(t, settings.Certificate)
		assert.Contains(t, getCNFromCertificate(settings.Certificate), "localhost")
	})

	t.Run("Valid external TLS secret overrides argocd-secret", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
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
			&corev1.Secret{
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
			&corev1.Secret{
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
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		require.NoError(t, err)
		assert.True(t, settings.CertificateIsExternal)
		assert.NotNil(t, settings.Certificate)
		assert.Contains(t, getCNFromCertificate(settings.Certificate), "localhost")
	})
	t.Run("Invalid external TLS secret", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
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
			&corev1.Secret{
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
			&corev1.Secret{
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
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		require.ErrorContains(t, err, "could not read from secret")
		assert.NotNil(t, settings)
	})
	t.Run("No external TLS secret", func(t *testing.T) {
		kubeClient := fake.NewClientset(
			&corev1.ConfigMap{
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
			&corev1.Secret{
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
		settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
		settings, err := settingsManager.GetSettings()
		require.NoError(t, err)
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
	require.NoError(t, err)
	assert.Equal(t, "some-url", argoCDCM.Data["help.download.darwin-amd64"])

	_, settingsManager = fixtures(map[string]string{
		"help.download.linux-s390x": "some-url",
	})
	argoCDCM, err = settingsManager.getConfigMap()
	require.NoError(t, err)
	assert.Equal(t, "some-url", argoCDCM.Data["help.download.linux-s390x"])

	_, settingsManager = fixtures(map[string]string{
		"help.download.unsupported": "some-url",
	})
	argoCDCM, err = settingsManager.getConfigMap()
	require.NoError(t, err)
	assert.Equal(t, "some-url", argoCDCM.Data["help.download.unsupported"])
}

func TestSecretKeyRef(t *testing.T) {
	data := map[string]string{
		"oidc.config": `name: Okta
issuer: $ext:issuerSecret
clientID: aaaabbbbccccddddeee
clientSecret: $ext:clientSecret
# Optional set of OIDC scopes to request. If omitted, defaults to: ["openid", "profile", "email", "groups"]
requestedScopes: ["openid", "profile", "email"]
# Optional set of OIDC claims to request on the ID token.
requestedIDTokenClaims: {"groups": {"essential": true}}`,
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data,
	}
	argocdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"admin.password":        nil,
			"server.secretkey":      nil,
			"webhook.github.secret": []byte("$ext:webhook.github.secret"),
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ext",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string][]byte{
			"issuerSecret":          []byte("https://dev-123456.oktapreview.com"),
			"clientSecret":          []byte("deadbeef"),
			"webhook.github.secret": []byte("mywebhooksecret"),
		},
	}
	kubeClient := fake.NewClientset(cm, secret, argocdSecret)
	settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")

	settings, err := settingsManager.GetSettings()
	require.NoError(t, err)
	assert.Equal(t, "mywebhooksecret", settings.WebhookGitHubSecret)

	oidcConfig := settings.OIDCConfig()
	assert.Equal(t, "https://dev-123456.oktapreview.com", oidcConfig.Issuer)
	assert.Equal(t, "deadbeef", oidcConfig.ClientSecret)
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
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: tc.data,
			}
			argocdSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"admin.password":   nil,
					"server.secretkey": nil,
				},
			}

			kubeClient := fake.NewClientset(cm, argocdSecret)
			settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")

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
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDConfigMapName,
					Namespace: "default",
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: tc.data,
			}
			argocdSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDSecretName,
					Namespace: "default",
				},
				Data: map[string][]byte{
					"admin.password":   nil,
					"server.secretkey": nil,
				},
			}
			secret := &corev1.Secret{
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
			kubeClient := fake.NewClientset(cm, secret, argocdSecret)
			settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")

			helmSettings, err := settingsManager.GetHelmSettings()
			require.NoError(t, err)

			assert.ElementsMatch(t, tc.expected, helmSettings.ValuesFileSchemes)
		})
	}
}

func TestArgoCDSettings_OIDCTLSConfig_OIDCTLSInsecureSkipVerify(t *testing.T) {
	certParsed, err := tls.X509KeyPair(test.Cert, test.PrivateKey)
	require.NoError(t, err)

	testCases := []struct {
		name               string
		settings           *ArgoCDSettings
		expectNilTLSConfig bool
	}{
		{
			name: "OIDC configured, no root CA",
			settings: &ArgoCDSettings{OIDCConfigRAW: `name: Test
issuer: aaa
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`},
		},
		{
			name: "OIDC configured, valid root CA",
			settings: &ArgoCDSettings{OIDCConfigRAW: fmt.Sprintf(`
name: Test
issuer: aaa
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
rootCA: |
  %s
`, strings.ReplaceAll(string(test.Cert), "\n", "\n  "))},
		},
		{
			name: "OIDC configured, invalid root CA",
			settings: &ArgoCDSettings{OIDCConfigRAW: `name: Test
issuer: aaa
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
rootCA: "invalid"`},
		},
		{
			name:               "OIDC not configured, no cert configured",
			settings:           &ArgoCDSettings{},
			expectNilTLSConfig: true,
		},
		{
			name:     "OIDC not configured, cert configured",
			settings: &ArgoCDSettings{Certificate: &certParsed},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			if testCase.expectNilTLSConfig {
				assert.Nil(t, testCase.settings.OIDCTLSConfig())
			} else {
				assert.False(t, testCase.settings.OIDCTLSConfig().InsecureSkipVerify)

				testCase.settings.OIDCTLSInsecureSkipVerify = true

				assert.True(t, testCase.settings.OIDCTLSConfig().InsecureSkipVerify)
			}
		})
	}
}

func Test_OAuth2AllowedAudiences(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		settings *ArgoCDSettings
		expected []string
	}{
		{
			name:     "Empty",
			settings: &ArgoCDSettings{},
			expected: []string{},
		},
		{
			name: "OIDC configured, no audiences specified, clientID used",
			settings: &ArgoCDSettings{OIDCConfigRAW: `name: Test
issuer: aaa
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]`},
			expected: []string{"xxx"},
		},
		{
			name: "OIDC configured, no audiences specified, clientID and cliClientID used",
			settings: &ArgoCDSettings{OIDCConfigRAW: `name: Test
issuer: aaa
clientID: xxx
cliClientID: cli-xxx
clientSecret: yyy
requestedScopes: ["oidc"]`},
			expected: []string{"xxx", "cli-xxx"},
		},
		{
			name: "OIDC configured, audiences specified",
			settings: &ArgoCDSettings{OIDCConfigRAW: `name: Test
issuer: aaa
clientID: xxx
clientSecret: yyy
requestedScopes: ["oidc"]
allowedAudiences: ["aud1", "aud2"]`},
			expected: []string{"aud1", "aud2"},
		},
		{
			name: "Dex configured",
			settings: &ArgoCDSettings{DexConfig: `connectors:
  - type: github
    id: github
    name: GitHub
    config:
      clientID: aabbccddeeff00112233
      clientSecret: $dex.github.clientSecret
      orgs:
      - name: your-github-org
`},
			expected: []string{common.ArgoCDClientAppID, common.ArgoCDCLIClientAppID},
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			t.Parallel()
			assert.ElementsMatch(t, tcc.expected, tcc.settings.OAuth2AllowedAudiences())
		})
	}
}

func TestReplaceStringSecret(t *testing.T) {
	secretValues := map[string]string{"my-secret-key": "my-secret-value"}
	result := ReplaceStringSecret("$my-secret-key", secretValues)
	assert.Equal(t, "my-secret-value", result)

	result = ReplaceStringSecret("$invalid-secret-key", secretValues)
	assert.Equal(t, "$invalid-secret-key", result)

	result = ReplaceStringSecret("", secretValues)
	assert.Empty(t, result)

	result = ReplaceStringSecret("my-value", secretValues)
	assert.Equal(t, "my-value", result)
}

func TestRedirectURLForRequest(t *testing.T) {
	generateRequest := func(url string) *http.Request {
		r, err := http.NewRequest(http.MethodPost, url, nil)
		require.NoError(t, err)
		return r
	}

	testCases := []struct {
		Name        string
		Settings    *ArgoCDSettings
		Request     *http.Request
		ExpectedURL string
		ExpectError bool
	}{
		{
			Name: "Single URL",
			Settings: &ArgoCDSettings{
				URL: "https://example.org",
			},
			Request:     generateRequest("https://example.org/login"),
			ExpectedURL: "https://example.org/auth/callback",
			ExpectError: false,
		},
		{
			Name: "Request does not match configured URL.",
			Settings: &ArgoCDSettings{
				URL: "https://otherhost.org",
			},
			Request:     generateRequest("https://example.org/login"),
			ExpectedURL: "https://otherhost.org/auth/callback",
			ExpectError: false,
		},
		{
			Name: "Cannot parse URL.",
			Settings: &ArgoCDSettings{
				URL: ":httpsotherhostorg",
			},
			Request:     generateRequest("https://example.org/login"),
			ExpectedURL: "",
			ExpectError: true,
		},
		{
			Name: "Match extended URL in settings.URL.",
			Settings: &ArgoCDSettings{
				URL:            "https://otherhost.org",
				AdditionalURLs: []string{"https://anotherhost.org"},
			},
			Request:     generateRequest("https://anotherhost.org/login"),
			ExpectedURL: "https://anotherhost.org/auth/callback",
			ExpectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := tc.Settings.RedirectURLForRequest(tc.Request)
			assert.Equal(t, tc.ExpectedURL, result)
			if tc.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedirectAdditionalURLs(t *testing.T) {
	testCases := []struct {
		Name           string
		Settings       *ArgoCDSettings
		ExpectedResult []string
		ExpectedError  bool
	}{
		{
			Name: "Good case with two AdditionalURLs",
			Settings: &ArgoCDSettings{
				URL:            "https://example.org",
				AdditionalURLs: []string{"https://anotherhost.org", "https://yetanother.org"},
			},
			ExpectedResult: []string{
				"https://anotherhost.org/auth/callback",
				"https://yetanother.org/auth/callback",
			},
			ExpectedError: false,
		},
		{
			Name: "Bad URL causes error",
			Settings: &ArgoCDSettings{
				URL:            "https://example.org",
				AdditionalURLs: []string{":httpsotherhostorg"},
			},
			ExpectedResult: []string{},
			ExpectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result, err := tc.Settings.RedirectAdditionalURLs()
			if tc.ExpectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.ExpectedResult, result)
		})
	}
}

func TestUseAzureWorkloadIdentity(t *testing.T) {
	testCases := []struct {
		Name           string
		Settings       *ArgoCDSettings
		ExpectedResult bool
	}{
		{
			Name: "UseAzureWorkloadIdentity defined and set to true",
			Settings: &ArgoCDSettings{
				OIDCConfigRAW: "{ \"azure\": {\"useWorkloadIdentity\": true }}",
			},
			ExpectedResult: true,
		},
		{
			Name: "UseAzureWorkloadIdentity defined and set to false",
			Settings: &ArgoCDSettings{
				OIDCConfigRAW: "{ \"azure\": {\"useWorkloadIdentity\": false }}",
			},
			ExpectedResult: false,
		},
		{
			Name: "UseAzureWorkloadIdentity not defined, with azure key present",
			Settings: &ArgoCDSettings{
				OIDCConfigRAW: "{ \"azure\": {}}",
			},
			ExpectedResult: false,
		},
		{
			Name: "UseAzureWorkloadIdentity not defined",
			Settings: &ArgoCDSettings{
				OIDCConfigRAW: "{}",
			},
			ExpectedResult: false,
		},
		{
			Name:           "OIDC config isnot defined",
			Settings:       &ArgoCDSettings{},
			ExpectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			result := tc.Settings.UseAzureWorkloadIdentity()
			require.Equal(t, tc.ExpectedResult, result)
		})
	}
}

func TestIsImpersonationEnabled(t *testing.T) {
	// When there is no argocd-cm itself,
	// Then IsImpersonationEnabled() must return false (default value) and an error with appropriate error message.
	kubeClient := fake.NewClientset()
	settingsManager := NewSettingsManager(t.Context(), kubeClient, "default")
	featureFlag, err := settingsManager.IsImpersonationEnabled()
	require.False(t, featureFlag,
		"with no argocd-cm config map, IsImpersonationEnabled() must return return false (default value)")
	require.ErrorContains(t, err, "configmap \"argocd-cm\" not found",
		"with no argocd-cm config map, IsImpersonationEnabled() must return an error")

	// When there is no impersonation feature flag present in the argocd-cm,
	// Then IsImpersonationEnabled() must return false (default value) and nil error.
	_, settingsManager = fixtures(map[string]string{})
	featureFlag, err = settingsManager.IsImpersonationEnabled()
	require.False(t, featureFlag,
		"with empty argocd-cm config map, IsImpersonationEnabled() must return false (default value)")
	require.NoError(t, err,
		"with empty argocd-cm config map, IsImpersonationEnabled() must not return any error")

	// When user disables the feature explicitly,
	// Then IsImpersonationEnabled() must return false and nil error.
	_, settingsManager = fixtures(map[string]string{
		"application.sync.impersonation.enabled": "false",
	})
	featureFlag, err = settingsManager.IsImpersonationEnabled()
	require.False(t, featureFlag,
		"when user enables the flag in argocd-cm config map, IsImpersonationEnabled() must return user set value")
	require.NoError(t, err,
		"when user enables the flag in argocd-cm config map, IsImpersonationEnabled() must not return any error")

	// When user enables the feature explicitly,
	// Then IsImpersonationEnabled() must return true and nil error.
	_, settingsManager = fixtures(map[string]string{
		"application.sync.impersonation.enabled": "true",
	})
	featureFlag, err = settingsManager.IsImpersonationEnabled()
	require.True(t, featureFlag,
		"when user enables the flag in argocd-cm config map, IsImpersonationEnabled() must return user set value")
	require.NoError(t, err,
		"when user enables the flag in argocd-cm config map, IsImpersonationEnabled() must not return any error")
}

func TestSettingsManager_GetHideSecretAnnotations(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output map[string]bool
	}{
		{
			name:   "Empty input",
			input:  "",
			output: map[string]bool{},
		},
		{
			name:   "Comma separated data",
			input:  "example.com/token-secret.value,token,key",
			output: map[string]bool{"example.com/token-secret.value": true, "token": true, "key": true},
		},
		{
			name:   "Comma separated data with space",
			input:  "example.com/token-secret.value, token,    key",
			output: map[string]bool{"example.com/token-secret.value": true, "token": true, "key": true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, settingsManager := fixtures(map[string]string{
				resourceSensitiveAnnotationsKey: tt.input,
			})
			keys := settingsManager.GetSensitiveAnnotations()
			assert.Len(t, keys, len(tt.output))
			assert.Equal(t, tt.output, keys)
		})
	}
}

func TestSettingsManager_GetAllowedNodeLabels(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []string
	}{
		{
			name:   "Empty input",
			input:  "",
			output: []string{},
		},
		{
			name:   "Comma separated data",
			input:  "example.com/label,label1,label2",
			output: []string{"example.com/label", "label1", "label2"},
		},
		{
			name:   "Comma separated data with space",
			input:  "example.com/label, label1,    label2",
			output: []string{"example.com/label", "label1", "label2"},
		},
		{
			name:   "Comma separated data with invalid label",
			input:  "example.com/label,_invalid,label1,label2",
			output: []string{"example.com/label", "label1", "label2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, settingsManager := fixtures(map[string]string{
				allowedNodeLabelsKey: tt.input,
			})
			keys := settingsManager.GetAllowedNodeLabels()
			assert.Len(t, keys, len(tt.output))
			assert.Equal(t, tt.output, keys)
		})
	}
}
