package admin

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/security"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	secutil "github.com/argoproj/argo-cd/v3/util/security"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

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
			ClusterResourceWhitelist: []metav1.GroupKind{
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
status: {}
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
status: {}
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

func Test_importResources(t *testing.T) {
	type args struct {
		bak  string
		live string
	}

	tests := []struct {
		name                     string
		args                     args
		applicationNamespaces    []string
		applicationsetNamespaces []string
		prune                    bool
		skipResourcesWithLabel   string
	}{
		{
			name: "It should update the live object according to the backup object if the backup object doesn't have the skip label",
			args: args{
				bak: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: namespace
  labels:
    env: dev
  annotations:
    argocd.argoproj.io/instance: test-instance
  finalizers:
    - test.finalizer.io
data:
  foo: bar
`,
				live: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: namespace
  labels:
    env: prod
  annotations:
    argocd.argoproj.io/instance: old-instance
  finalizers: []
data:
  foo: old
`,
			},
			applicationNamespaces:    []string{"argocd", "dev"},
			applicationsetNamespaces: []string{"argocd", "prod"},
			skipResourcesWithLabel:   "env=dev",
		},
		{
			name: "It should update the data of the live object according to the backup object",
			args: args{
				bak: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: namespace
data:
  foo: bar
`,
				live: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-configmap
  namespace: namespace
data:
  foo: old				
`,
			},
			applicationNamespaces:    []string{},
			applicationsetNamespaces: []string{},
			prune:                    true,
		},
		{
			name: "Spec should be updated correctly in live object according to the backup object",
			args: args{
				bak: `apiVersion: v1
kind: Application
metadata:
  name: app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/example/updated.git
  destination:
    namespace: default
    server: https://kubernetes.default.svc
`,
				live: `apiVersion: v1
kind: Application
metadata:
  name: app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/example/old.git
  destination:
    namespace: default
    server: https://kubernetes.default.svc
`,
			},
			applicationNamespaces:    []string{"argocd", "dev"},
			applicationsetNamespaces: []string{"prod"},
		},
		{
			name: "It should update live object's spec according to the backup object",
			args: args{
				bak: `apiVersion: v1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - clusters: dev
  template:
    metadata:
      name: '{{appName}}'
    spec: {}
`,
				live: `apiVersion: v1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
spec:
  generators:
    - list:
        elements:
          - clusters: prod
  template:
    metadata:
      name: '{{appName}}'
    spec: {}
`,
			},
			applicationNamespaces:    []string{},
			applicationsetNamespaces: []string{"dev"},
		},
		{
			name: "It shouldn't update the live object if it's same as the backup object",
			args: args{
				bak: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm
  namespace: argo-cd
  labels:
    env: dev
data:
  foo: bar
`,
				live: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm
  namespace: argo-cd
  labels:
    env: dev
data:
  foo: bar
`,
			},
			applicationNamespaces:    []string{"argo-*"},
			applicationsetNamespaces: []string{"argo-*"},
		},
		{
			name: "Resources should be created when they're missing from live",
			args: args{
				bak: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm
  namespace: argocd
  labels:
    env: dev
data:
  foo: bar
`,
				live: `apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm
  namespaces: argocd
  labels:
    env: dev
`,
			},
			applicationNamespaces:    []string{"argocd", "dev"},
			applicationsetNamespaces: []string{"argocd", "prod"},
		},
		{
			name: "Live resources should be pruned if --prune flag is set",
			args: args{
				bak: `apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap-to-keep
  namespace: default
apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap-to-delete
  namespace: default
`,
				live: `apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap-to-keep
  namespace: default
`,
			},
			prune: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bakObj := decodeYAMLToUnstructured(t, tt.args.bak)
			liveObj := decodeYAMLToUnstructured(t, tt.args.live)
			pruneObjects := make(map[kube.ResourceKey]unstructured.Unstructured)

			configMap := &unstructured.Unstructured{}
			configMap.SetUnstructuredContent(map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "argocd-cmd-params-cm",
					"namespace": "default",
				},
				"data": map[string]interface{}{
					"application.namespaces":    "argocd,dev",
					"applicationset.namespaces": "argocd,stage",
				},
			})

			ctx := context.Background()
			gvr := schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "configmaps",
			}

			dynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), configMap)
			configMapResource := dynamicClient.Resource(gvr).Namespace("default")

			if len(tt.applicationNamespaces) == 0 || len(tt.applicationsetNamespaces) == 0 {
				defaultNs := getAdditionalNamespaces(ctx, configMapResource)

				if len(tt.applicationNamespaces) == 0 {
					tt.applicationNamespaces = defaultNs.applicationNamespaces
				}

				if len(tt.applicationsetNamespaces) == 0 {
					tt.applicationsetNamespaces = defaultNs.applicationsetNamespaces
				}
			}

			// check if the object is a configMap or not
			if isArgoCDConfigMap(bakObj.GetName()) {
				pruneObjects[kube.ResourceKey{Group: "", Kind: "ConfigMap", Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}] = *bakObj
			}
			// check if the object is a secret or not
			if isArgoCDSecret(*bakObj) {
				pruneObjects[kube.ResourceKey{Group: "", Kind: "Secret", Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}] = *bakObj
			}
			// check if the object is an application or not
			if bakObj.GetKind() == "Application" {
				if secutil.IsNamespaceEnabled(bakObj.GetNamespace(), "argocd", tt.applicationNamespaces) {
					pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "Application", Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}] = *bakObj
				}
			}
			// check if the object is a project or not
			if bakObj.GetKind() == "AppProject" {
				pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "AppProject", Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}] = *bakObj
			}
			// check if the object is an applicationSet or not
			if bakObj.GetKind() == "ApplicationSet" {
				if secutil.IsNamespaceEnabled(bakObj.GetNamespace(), "argocd", tt.applicationsetNamespaces) {
					pruneObjects[kube.ResourceKey{Group: "argoproj.io", Kind: "ApplicationSet", Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}] = *bakObj
				}
			}
			gvk := bakObj.GroupVersionKind()
			if bakObj.GetNamespace() == "" {
				bakObj.SetNamespace("argocd")
			}
			key := kube.ResourceKey{Group: gvk.Group, Kind: gvk.Kind, Name: bakObj.GetName(), Namespace: bakObj.GetNamespace()}
			delete(pruneObjects, key)

			var updatedLive *unstructured.Unstructured
			if slices.Contains(tt.applicationNamespaces, bakObj.GetNamespace()) || slices.Contains(tt.applicationsetNamespaces, bakObj.GetNamespace()) {
				if !isSkipLabelMatches(bakObj, tt.skipResourcesWithLabel) {
					if tt.prune {
						var dynClient dynamic.ResourceInterface
						switch key.Kind {
						case "Secret":
							dynClient = dynamicClient.Resource(secretResource).Namespace(liveObj.GetNamespace())
						case "AppProjectKind":
							dynClient = dynamicClient.Resource(appprojectsResource).Namespace(liveObj.GetNamespace())
						case "ApplicationSetKind":
							dynClient = dynamicClient.Resource(appplicationSetResource).Namespace(liveObj.GetNamespace())
						case "ApplicationKind":
							dynClient = dynamicClient.Resource(applicationsResource).Namespace(liveObj.GetNamespace())
						}

						err := dynClient.Delete(ctx, key.Name, metav1.DeleteOptions{})
						assert.NoError(t, err)
					} else {
						updatedLive = updateLive(bakObj, liveObj, false)

						assert.Equal(t, bakObj.GetLabels(), updatedLive.GetLabels())
						assert.Equal(t, bakObj.GetAnnotations(), updatedLive.GetAnnotations())
						assert.Equal(t, bakObj.GetFinalizers(), updatedLive.GetFinalizers())
						assert.Equal(t, bakObj.Object["data"], updatedLive.Object["data"])
					}
				}
			} else {
				assert.Nil(t, updatedLive)
			}
		})
	}
}

func decodeYAMLToUnstructured(t *testing.T, yamlStr string) *unstructured.Unstructured {
	t.Helper()

	var m map[string]any
	err := yaml.Unmarshal([]byte(yamlStr), &m)
	require.NoError(t, err)
	return &unstructured.Unstructured{Object: m}
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
