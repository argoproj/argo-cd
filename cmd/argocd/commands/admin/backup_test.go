package admin

import (
	"bytes"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/security"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
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

func Test_checkAppHasNoNeedToStopOperation(t *testing.T) {
	tests := []struct {
		name          string
		liveObj       unstructured.Unstructured
		stopOperation bool
		expected      bool
	}{
		{
			name: "Application with operation and stopOperation=true returns false",
			liveObj: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "Application",
					"operation": map[string]any{
						"sync": map[string]any{},
					},
				},
			},
			stopOperation: true,
			expected:      false,
		},
		{
			name: "Application with operation and stopOperation=false returns true",
			liveObj: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "Application",
					"operation": map[string]any{
						"sync": map[string]any{},
					},
				},
			},
			stopOperation: false,
			expected:      true,
		},
		{
			name: "Application without operation and stopOperation=true returns true",
			liveObj: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "Application",
				},
			},
			stopOperation: true,
			expected:      true,
		},
		{
			name: "Non-Application resource with stopOperation=true returns true",
			liveObj: unstructured.Unstructured{
				Object: map[string]any{
					"kind": "ConfigMap",
				},
			},
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

func Test_updateLive(t *testing.T) {
	tests := []struct {
		name          string
		bak           *unstructured.Unstructured
		live          *unstructured.Unstructured
		stopOperation bool
		validate      func(t *testing.T, result *unstructured.Unstructured)
	}{
		{
			name: "Secret updates data field",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Secret",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name":        "test-secret",
						"annotations": map[string]any{"backup-annotation": "value"},
						"labels":      map[string]any{"backup-label": "value"},
					},
					"data": map[string]any{
						"password": "bmV3cGFzc3dvcmQ=",
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Secret",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name":            "test-secret",
						"resourceVersion": "12345",
					},
					"data": map[string]any{
						"password": "b2xkcGFzc3dvcmQ=",
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.Equal(t, "bmV3cGFzc3dvcmQ=", result.Object["data"].(map[string]any)["password"])
				assert.Equal(t, "12345", result.GetResourceVersion())
			},
		},
		{
			name: "ConfigMap updates data field",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ConfigMap",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name": "test-cm",
					},
					"data": map[string]any{
						"config.yaml": "new: value",
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ConfigMap",
					"apiVersion": "v1",
					"metadata": map[string]any{
						"name":            "test-cm",
						"resourceVersion": "67890",
					},
					"data": map[string]any{
						"config.yaml": "old: value",
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.Equal(t, "new: value", result.Object["data"].(map[string]any)["config.yaml"])
			},
		},
		{
			name: "AppProject updates spec field",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "AppProject",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name": "test-project",
					},
					"spec": map[string]any{
						"description": "new description",
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "AppProject",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name":            "test-project",
						"resourceVersion": "11111",
					},
					"spec": map[string]any{
						"description": "old description",
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				spec := result.Object["spec"].(map[string]any)
				assert.Equal(t, "new description", spec["description"])
			},
		},
		{
			name: "Application updates spec and status",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name": "test-app",
					},
					"spec": map[string]any{
						"project": "default",
					},
					"status": map[string]any{
						"sync": map[string]any{"status": "Synced"},
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name":            "test-app",
						"resourceVersion": "22222",
					},
					"spec": map[string]any{
						"project": "other",
					},
					"status": map[string]any{
						"sync": map[string]any{"status": "OutOfSync"},
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				spec := result.Object["spec"].(map[string]any)
				assert.Equal(t, "default", spec["project"])
				status := result.Object["status"].(map[string]any)
				sync := status["sync"].(map[string]any)
				assert.Equal(t, "Synced", sync["status"])
			},
		},
		{
			name: "Application with stopOperation=true clears operation",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name": "test-app",
					},
					"spec": map[string]any{
						"project": "default",
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name":            "test-app",
						"resourceVersion": "33333",
					},
					"spec": map[string]any{
						"project": "default",
					},
					"operation": map[string]any{
						"sync": map[string]any{
							"revision": "abc123",
						},
					},
				},
			},
			stopOperation: true,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.Nil(t, result.Object["operation"])
			},
		},
		{
			name: "Application with stopOperation=false preserves operation",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name": "test-app",
					},
					"spec": map[string]any{
						"project": "default",
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "Application",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name":            "test-app",
						"resourceVersion": "44444",
					},
					"spec": map[string]any{
						"project": "default",
					},
					"operation": map[string]any{
						"sync": map[string]any{
							"revision": "abc123",
						},
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.NotNil(t, result.Object["operation"])
			},
		},
		{
			name: "ApplicationSet updates spec field",
			bak: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ApplicationSet",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name": "test-appset",
					},
					"spec": map[string]any{
						"generators": []any{},
					},
				},
			},
			live: &unstructured.Unstructured{
				Object: map[string]any{
					"kind":       "ApplicationSet",
					"apiVersion": "argoproj.io/v1alpha1",
					"metadata": map[string]any{
						"name":            "test-appset",
						"resourceVersion": "55555",
					},
					"spec": map[string]any{
						"generators": []any{
							map[string]any{"list": map[string]any{}},
						},
					},
				},
			},
			stopOperation: false,
			validate: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				spec := result.Object["spec"].(map[string]any)
				generators := spec["generators"].([]any)
				assert.Empty(t, generators)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateLive(tt.bak, tt.live, tt.stopOperation)
			tt.validate(t, result)
		})
	}
}
