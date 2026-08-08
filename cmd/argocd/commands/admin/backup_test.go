package admin

import (
	"bytes"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/security"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

// Test_checkAppHasNoNeedToStopOperation verifies the guard used by `argocd admin import` to decide
// whether an Application has an in-flight operation that must be stopped before it is overwritten.
func Test_checkAppHasNoNeedToStopOperation(t *testing.T) {
	newApp := func(withOperation bool) unstructured.Unstructured {
		obj := unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "test-app"},
			},
		}
		if withOperation {
			obj.Object["operation"] = map[string]any{"sync": map[string]any{}}
		}
		return obj
	}

	configMap := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": "test-cm"},
		},
	}

	tests := []struct {
		name          string
		liveObj       unstructured.Unstructured
		stopOperation bool
		expected      bool
	}{
		{
			name:          "returns true when stopOperation is false even if the app has an operation",
			liveObj:       newApp(true),
			stopOperation: false,
			expected:      true,
		},
		{
			name:          "returns true for non-Application kinds when stopOperation is true",
			liveObj:       configMap,
			stopOperation: true,
			expected:      true,
		},
		{
			name:          "returns false when an Application has an in-flight operation and stopOperation is true",
			liveObj:       newApp(true),
			stopOperation: true,
			expected:      false,
		},
		{
			name:          "returns true when an Application has no operation and stopOperation is true",
			liveObj:       newApp(false),
			stopOperation: true,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAppHasNoNeedToStopOperation(tt.liveObj, tt.stopOperation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test_updateLive verifies that updateLive overlays the backup's metadata and type-specific payload
// onto a copy of the live resource, without mutating the original live object.
func Test_updateLive(t *testing.T) {
	t.Run("ConfigMap data/labels/annotations/finalizers come from the backup while other live fields are preserved", func(t *testing.T) {
		bak := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":        "my-cm",
					"labels":      map[string]any{"from": "backup"},
					"annotations": map[string]any{"from": "backup"},
					"finalizers":  []any{"backup-finalizer"},
				},
				"data": map[string]any{"key": "backup-value"},
			},
		}
		live := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name":            "my-cm",
					"namespace":       "argocd",
					"labels":          map[string]any{"from": "live"},
					"annotations":     map[string]any{"from": "live"},
					"resourceVersion": "12345",
				},
				"data": map[string]any{"key": "live-value"},
			},
		}

		result := updateLive(bak, live, false)

		// payload and mutable metadata are taken from the backup
		assert.Equal(t, map[string]any{"key": "backup-value"}, result.Object["data"])
		assert.Equal(t, map[string]string{"from": "backup"}, result.GetLabels())
		assert.Equal(t, map[string]string{"from": "backup"}, result.GetAnnotations())
		assert.Equal(t, []string{"backup-finalizer"}, result.GetFinalizers())
		// unrelated live metadata is left intact
		assert.Equal(t, "argocd", result.GetNamespace())
		assert.Equal(t, "12345", result.GetResourceVersion())
		// the original live object must not be mutated
		assert.Equal(t, map[string]any{"key": "live-value"}, live.Object["data"])
	})

	t.Run("AppProject spec is replaced with the backup spec", func(t *testing.T) {
		bak := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "AppProject",
				"metadata":   map[string]any{"name": "default"},
				"spec":       map[string]any{"sourceRepos": []any{"https://backup.example.com"}},
			},
		}
		live := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "AppProject",
				"metadata":   map[string]any{"name": "default"},
				"spec":       map[string]any{"sourceRepos": []any{"https://live.example.com"}},
			},
		}

		result := updateLive(bak, live, false)
		assert.Equal(t, map[string]any{"sourceRepos": []any{"https://backup.example.com"}}, result.Object["spec"])
	})

	t.Run("ApplicationSet spec is replaced with the backup spec", func(t *testing.T) {
		bak := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "ApplicationSet",
				"metadata":   map[string]any{"name": "test-appset"},
				"spec":       map[string]any{"goTemplate": true},
			},
		}
		live := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "ApplicationSet",
				"metadata":   map[string]any{"name": "test-appset"},
				"spec":       map[string]any{"goTemplate": false},
			},
		}

		result := updateLive(bak, live, false)
		assert.Equal(t, map[string]any{"goTemplate": true}, result.Object["spec"])
	})

	t.Run("Application operation is removed and spec/status come from the backup when stopOperation is true", func(t *testing.T) {
		bak := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"project": "backup-project"},
				"status":     map[string]any{"health": map[string]any{"status": "Healthy"}},
			},
		}
		live := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"project": "live-project"},
				"status":     map[string]any{"health": map[string]any{"status": "Degraded"}},
				"operation":  map[string]any{"sync": map[string]any{}},
			},
		}

		result := updateLive(bak, live, true)
		assert.Equal(t, map[string]any{"project": "backup-project"}, result.Object["spec"])
		assert.Equal(t, map[string]any{"health": map[string]any{"status": "Healthy"}}, result.Object["status"])
		assert.Nil(t, result.Object["operation"])
	})

	t.Run("Application operation is preserved when stopOperation is false", func(t *testing.T) {
		operation := map[string]any{"sync": map[string]any{"revision": "abc123"}}
		bak := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"project": "backup-project"},
			},
		}
		live := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "argoproj.io/v1alpha1",
				"kind":       "Application",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"project": "live-project"},
				"operation":  operation,
			},
		}

		result := updateLive(bak, live, false)
		assert.Equal(t, operation, result.Object["operation"])
	})
}
