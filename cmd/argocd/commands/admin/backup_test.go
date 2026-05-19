package admin

import (
	"bytes"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/security"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"

	"github.com/argoproj/argo-cd/v3/common"
)

func newBackupObject(trackingValue string, trackingLabel bool, trackingAnnotation bool) *unstructured.Unstructured {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-configmap",
			Namespace: "namespace",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}
	if trackingLabel {
		cm.SetLabels(map[string]string{
			common.LabelKeyAppInstance: trackingValue,
		})
	}
	if trackingAnnotation {
		cm.SetAnnotations(map[string]string{
			common.AnnotationKeyAppInstance: trackingValue,
		})
	}
	return kube.MustToUnstructured(&cm)
}

func newConfigmapObject() *unstructured.Unstructured {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}

	return kube.MustToUnstructured(&cm)
}

func newSecretsObject() *unstructured.Unstructured {
	secret := corev1.Secret{
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
	}

	return kube.MustToUnstructured(&secret)
}

func newAppProject() *unstructured.Unstructured {
	appProject := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "argocd",
		},
		Spec: v1alpha1.AppProjectSpec{
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Namespace: "*",
					Server:    "*",
				},
			},
			ClusterResourceWhitelist: []v1alpha1.ClusterResourceRestrictionItem{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			SourceRepos: []string{"*"},
		},
	}

	return kube.MustToUnstructured(&appProject)
}

func newApplication(namespace string) *unstructured.Unstructured {
	app := v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			Kind: "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			Source:  &v1alpha1.ApplicationSource{},
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    v1alpha1.KubernetesInternalAPIServerAddr,
				Namespace: "default",
			},
		},
	}

	return kube.MustToUnstructured(&app)
}

func newApplicationSet(namespace string) *unstructured.Unstructured {
	appSet := v1alpha1.ApplicationSet{
		TypeMeta: metav1.TypeMeta{
			Kind: "ApplicationSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{
				{
					Git: &v1alpha1.GitGenerator{
						RepoURL: "https://github.com/org/repo",
					},
				},
			},
		},
	}

	return kube.MustToUnstructured(&appSet)
}

// Test_exportResources tests for the resources exported when using the `argocd admin export` command
func Test_exportResources(t *testing.T) {
	tests := []struct {
		name                string
		object              *unstructured.Unstructured
		namespace           string
		enabledNamespaces   []string
		expectedFileContent string
		expectExport        bool
	}{
		{
			name:         "ConfigMap should be in the exported manifest",
			object:       newConfigmapObject(),
			expectExport: true,
			expectedFileContent: `apiVersion: ""
kind: ""
metadata:
  labels:
    app.kubernetes.io/part-of: argocd
  name: argocd-cm
---
`,
		},
		{
			name:         "Secret should be in the exported manifest",
			object:       newSecretsObject(),
			expectExport: true,
			expectedFileContent: `apiVersion: ""
data:
  admin.password: null
  server.secretkey: null
kind: ""
metadata:
  labels:
    app.kubernetes.io/part-of: argocd
  name: argocd-secret
  namespace: default
---
`,
		},
		{
			name:         "App Project should be in the exported manifest",
			object:       newAppProject(),
			expectExport: true,
			expectedFileContent: `apiVersion: ""
kind: ""
metadata:
  name: default
spec:
  clusterResourceWhitelist:
  - group: '*'
    kind: '*'
  destinations:
  - namespace: '*'
    server: '*'
  sourceRepos:
  - '*'
status: {}
---
`,
		},
		{
			name:         "Application should be in the exported manifest when created in the default 'argocd' namespace",
			object:       newApplication("argocd"),
			namespace:    "argocd",
			expectExport: true,
			expectedFileContent: `apiVersion: ""
kind: Application
metadata:
  name: test
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc
  project: default
  source:
    repoURL: ""
status:
  health: {}
  sourceHydrator: {}
  summary: {}
  sync:
    comparedTo:
      destination: {}
      source:
        repoURL: ""
    status: ""
---
`,
		},
		{
			name:              "Application should be in the exported manifest when created in the enabled namespaces",
			object:            newApplication("dev"),
			namespace:         "dev",
			enabledNamespaces: []string{"dev", "prod"},
			expectExport:      true,
			expectedFileContent: `apiVersion: ""
kind: Application
metadata:
  name: test
  namespace: dev
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc
  project: default
  source:
    repoURL: ""
status:
  health: {}
  sourceHydrator: {}
  summary: {}
  sync:
    comparedTo:
      destination: {}
      source:
        repoURL: ""
    status: ""
---
`,
		},
		{
			name:                "Application should not be in the exported manifest when it's neither created in the default argod namespace nor in enabled namespace",
			object:              newApplication("staging"),
			namespace:           "staging",
			enabledNamespaces:   []string{"dev", "prod"},
			expectExport:        false,
			expectedFileContent: ``,
		},
		{
			name:         "ApplicationSet should be in the exported manifest when created in the default 'argocd' namespace",
			object:       newApplicationSet("argocd"),
			namespace:    "argocd",
			expectExport: true,
			expectedFileContent: `apiVersion: ""
kind: ApplicationSet
metadata:
  name: test-appset
spec:
  generators:
  - git:
      repoURL: https://github.com/org/repo
      revision: ""
      template:
        metadata: {}
        spec:
          destination: {}
          project: ""
  template:
    metadata: {}
    spec:
      destination: {}
      project: ""
status:
  health: {}
---
`,
		},
		{
			name:              "ApplicationSet should be in the exported manifest when created in the enabled namespaces",
			object:            newApplicationSet("dev"),
			namespace:         "dev",
			enabledNamespaces: []string{"dev", "prod"},
			expectExport:      true,
			expectedFileContent: `apiVersion: ""
kind: ApplicationSet
metadata:
  name: test-appset
  namespace: dev
spec:
  generators:
  - git:
      repoURL: https://github.com/org/repo
      revision: ""
      template:
        metadata: {}
        spec:
          destination: {}
          project: ""
  template:
    metadata: {}
    spec:
      destination: {}
      project: ""
status:
  health: {}
---
`,
		},
		{
			name:                "ApplicationSet should not be in the exported manifest when neither created in the default 'argocd' namespace nor in enabled namespaces",
			object:              newApplicationSet("staging"),
			namespace:           "staging",
			enabledNamespaces:   []string{"dev", "prod"},
			expectExport:        false,
			expectedFileContent: ``,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			kind := tt.object.GetKind()
			if kind == "Application" || kind == "ApplicationSet" {
				if security.IsNamespaceEnabled(tt.namespace, "argocd", tt.enabledNamespaces) {
					export(&buf, *tt.object, ArgoCDNamespace)
				}
			} else {
				export(&buf, *tt.object, ArgoCDNamespace)
			}

			content := buf.String()
			if tt.expectExport {
				assert.Equal(t, tt.expectedFileContent, content)
			} else {
				assert.Empty(t, content)
			}
		})
	}
}

func Test_updateTracking(t *testing.T) {
	type args struct {
		bak  *unstructured.Unstructured
		live *unstructured.Unstructured
	}
	tests := []struct {
		name     string
		args     args
		expected *unstructured.Unstructured
	}{
		{
			name: "update annotation when present in live",
			args: args{
				bak:  newBackupObject("bak", false, true),
				live: newBackupObject("live", false, true),
			},
			expected: newBackupObject("live", false, true),
		},
		{
			name: "update default label when present in live",
			args: args{
				bak:  newBackupObject("bak", true, true),
				live: newBackupObject("live", true, true),
			},
			expected: newBackupObject("live", true, true),
		},
		{
			name: "do not update if live object does not have tracking",
			args: args{
				bak:  newBackupObject("bak", true, true),
				live: newBackupObject("live", false, false),
			},
			expected: newBackupObject("bak", true, true),
		},
		{
			name: "do not update if bak object does not have tracking",
			args: args{
				bak:  newBackupObject("bak", false, false),
				live: newBackupObject("live", true, true),
			},
			expected: newBackupObject("bak", false, false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateTracking(tt.args.bak, tt.args.live)
			assert.Equal(t, tt.expected, tt.args.bak)
		})
	}
}

func TestIsSkipLabelMatches(t *testing.T) {
	tests := []struct {
		name       string
		obj        *unstructured.Unstructured
		skipLabels string
		expected   bool
	}{
		{
			name: "Label matches",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label": "value",
						},
					},
				},
			},
			skipLabels: "test-label=value",
			expected:   true,
		},
		{
			name: "Label does not match",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"different-label": "value",
						},
					},
				},
			},
			skipLabels: "test-label=value",
			expected:   false,
		},
		{
			name: "Empty skip labels",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label": "value",
						},
					},
				},
			},
			skipLabels: "",
			expected:   false,
		},
		{
			name: "No labels value",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label":    "value",
							"another-label": "value2",
						},
					},
				},
			},
			skipLabels: "test-label",
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSkipLabelMatches(tt.obj, tt.skipLabels)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunExport(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")

	rbacCM := newConfigmapObject()
	rbacCM.SetName(common.ArgoCDRBACConfigMapName)
	rbacCM.SetNamespace("argocd")
	rbacCM.SetAPIVersion("v1")
	rbacCM.SetKind("ConfigMap")

	knownHostsCM := newConfigmapObject()
	knownHostsCM.SetName(common.ArgoCDKnownHostsConfigMapName)
	knownHostsCM.SetNamespace("argocd")
	knownHostsCM.SetAPIVersion("v1")
	knownHostsCM.SetKind("ConfigMap")

	tlsCM := newConfigmapObject()
	tlsCM.SetName(common.ArgoCDTLSCertsConfigMapName)
	tlsCM.SetNamespace("argocd")
	tlsCM.SetAPIVersion("v1")
	tlsCM.SetKind("ConfigMap")

	secret := newSecretsObject()
	secret.SetNamespace("argocd")
	secret.SetAPIVersion("v1")
	secret.SetKind("Secret")

	proj := newAppProject()
	proj.SetNamespace("argocd")
	proj.SetAPIVersion("argoproj.io/v1alpha1")
	proj.SetKind("AppProject")

	app := newApplication("argocd")
	app.SetNamespace("argocd")
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")

	appSet := newApplicationSet("argocd")
	appSet.SetNamespace("argocd")
	appSet.SetAPIVersion("argoproj.io/v1alpha1")
	appSet.SetKind("ApplicationSet")

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM, cm, rbacCM, knownHostsCM, tlsCM, secret, proj, app, appSet)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	var buf bytes.Buffer
	err := runExport(t.Context(), acdClients, fakeDynClient, "argocd", &buf, nil, nil)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "argocd-cm")
	assert.Contains(t, output, "argocd-rbac-cm")
	assert.Contains(t, output, "argocd-ssh-known-hosts-cm")
	assert.Contains(t, output, "argocd-tls-certs-cm")
	assert.Contains(t, output, "argocd-secret")
	assert.Contains(t, output, "default")
	assert.Contains(t, output, "test")
	assert.Contains(t, output, "test-appset")
}

func TestNewExportCommand(t *testing.T) {
	cmd := NewExportCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "export", cmd.Name())
}

func TestNewImportCommand(t *testing.T) {
	cmd := NewImportCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "import", cmd.Name())
}

func TestRunImport_CreatesNewObjects(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	// Build backup YAML
	var buf bytes.Buffer
	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	export(&buf, *cm, "argocd")

	secret := newSecretsObject()
	secret.SetNamespace("argocd")
	secret.SetAPIVersion("v1")
	secret.SetKind("Secret")
	export(&buf, *secret, "argocd")

	proj := newAppProject()
	proj.SetNamespace("argocd")
	proj.SetAPIVersion("argoproj.io/v1alpha1")
	proj.SetKind("AppProject")
	export(&buf, *proj, "argocd")

	app := newApplication("argocd")
	app.SetNamespace("argocd")
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")
	export(&buf, *app, "argocd")

	appSet := newApplicationSet("argocd")
	appSet.SetNamespace("argocd")
	appSet.SetAPIVersion("argoproj.io/v1alpha1")
	appSet.SetKind("ApplicationSet")
	export(&buf, *appSet, "argocd")

	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), false, false, false, false, false, false, false, "", nil, nil)
	require.NoError(t, err)

	// Verify objects were created
	_, err = fakeDynClient.Resource(configMapResource).Namespace("argocd").Get(t.Context(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	require.NoError(t, err)
	_, err = fakeDynClient.Resource(secretResource).Namespace("argocd").Get(t.Context(), common.ArgoCDSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	_, err = fakeDynClient.Resource(appprojectsResource).Namespace("argocd").Get(t.Context(), "default", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = fakeDynClient.Resource(applicationsResource).Namespace("argocd").Get(t.Context(), "test", metav1.GetOptions{})
	require.NoError(t, err)
	_, err = fakeDynClient.Resource(appplicationSetResource).Namespace("argocd").Get(t.Context(), "test-appset", metav1.GetOptions{})
	require.NoError(t, err)
}

func TestRunImport_UpdatesExisting(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	existingCM := newConfigmapObject()
	existingCM.SetName(common.ArgoCDConfigMapName)
	existingCM.SetNamespace("argocd")
	existingCM.SetAPIVersion("v1")
	existingCM.SetKind("ConfigMap")
	existingCM.Object["data"] = map[string]any{"old": "value"}

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM, existingCM)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	// Build backup YAML with new data
	var buf bytes.Buffer
	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.Object["data"] = map[string]any{"new": "value"}
	export(&buf, *cm, "argocd")

	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), false, false, false, false, false, false, false, "", nil, nil)
	require.NoError(t, err)

	updatedCM, err := fakeDynClient.Resource(configMapResource).Namespace("argocd").Get(t.Context(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	require.NoError(t, err)
	data, _, _ := unstructured.NestedMap(updatedCM.Object, "data")
	assert.Equal(t, map[string]any{"new": "value"}, data)
}

func TestRunImport_Prune(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	existingSecret := newSecretsObject()
	existingSecret.SetNamespace("argocd")
	existingSecret.SetAPIVersion("v1")
	existingSecret.SetKind("Secret")

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM, existingSecret)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	// Build backup YAML with only a ConfigMap (no secret)
	var buf bytes.Buffer
	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	export(&buf, *cm, "argocd")

	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), true, false, false, false, false, false, false, "", nil, nil)
	require.NoError(t, err)

	// Verify secret was pruned
	_, err = fakeDynClient.Resource(secretResource).Namespace("argocd").Get(t.Context(), common.ArgoCDSecretName, metav1.GetOptions{})
	require.Error(t, err)
}

func TestRunImport_DryRun(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	var buf bytes.Buffer
	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	export(&buf, *cm, "argocd")

	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), false, true, false, false, false, false, false, "", nil, nil)
	require.NoError(t, err)

	// Verify object was NOT created because dryRun=true
	_, err = fakeDynClient.Resource(configMapResource).Namespace("argocd").Get(t.Context(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	require.Error(t, err)
}

func TestRunImport_SkipLabel(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	var buf bytes.Buffer
	cm := newConfigmapObject()
	cm.SetName(common.ArgoCDConfigMapName)
	cm.SetNamespace("argocd")
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	cm.SetLabels(map[string]string{"skip-label": "true"})
	export(&buf, *cm, "argocd")

	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), false, false, false, false, false, false, false, "skip-label=true", nil, nil)
	require.NoError(t, err)

	// Verify object was NOT created because it has the skip label
	_, err = fakeDynClient.Resource(configMapResource).Namespace("argocd").Get(t.Context(), common.ArgoCDConfigMapName, metav1.GetOptions{})
	require.Error(t, err)
}

func TestRunImport_NamespaceFilter(t *testing.T) {
	scheme := k8sruntime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	cmdParamsCM := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name":      common.ArgoCDCmdParamsConfigMapName,
				"namespace": "argocd",
			},
		},
	}

	fakeDynClient := dynfake.NewSimpleDynamicClient(scheme, cmdParamsCM)

	acdClients := &argoCDClientsets{
		configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
		secrets:         fakeDynClient.Resource(secretResource).Namespace("argocd"),
		projects:        fakeDynClient.Resource(appprojectsResource).Namespace("argocd"),
		applications:    fakeDynClient.Resource(applicationsResource).Namespace("argocd"),
		applicationSets: fakeDynClient.Resource(appplicationSetResource).Namespace("argocd"),
	}

	var buf bytes.Buffer
	app := newApplication("unauthorized-ns")
	app.SetNamespace("unauthorized-ns")
	app.SetAPIVersion("argoproj.io/v1alpha1")
	app.SetKind("Application")
	export(&buf, *app, "argocd")

	// Only allow "argocd" namespace
	err := runImport(t.Context(), acdClients, fakeDynClient, "argocd", buf.Bytes(), false, false, false, false, false, false, false, "", []string{"argocd"}, nil)
	require.NoError(t, err)

	// Verify app was NOT created because namespace is not enabled
	_, err = fakeDynClient.Resource(applicationsResource).Namespace("unauthorized-ns").Get(t.Context(), "test", metav1.GetOptions{})
	require.Error(t, err)
}
