package admin

import (
	"bytes"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/security"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
)

func newBackupObject(trackingValue string, trackingLabel bool, trackingAnnotation bool) *unstructured.Unstructured {
	cm := corev1.ConfigMap{
		Name:      "my-configmap",
		Namespace: "namespace",
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
		Name:      common.ArgoCDConfigMapName,
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
	}

	return kube.MustToUnstructured(&cm)
}

func newSecretsObject() *unstructured.Unstructured {
	secret := corev1.Secret{
		Name:      common.ArgoCDSecretName,
		Namespace: "default",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
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
		Name:      "default",
		Namespace: "argocd",
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
		Kind:      "Application",
		Name:      "test",
		Namespace: namespace,
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
		Kind:      "ApplicationSet",
		Name:      "test-appset",
		Namespace: namespace,
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
		stripStatus         bool
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
		{
			name:         "App Project status should be stripped from the exported manifest when strip-status is set",
			object:       newAppProject(),
			stripStatus:  true,
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
---
`,
		},
		{
			name:         "Application status should be stripped from the exported manifest when strip-status is set",
			object:       newApplication("argocd"),
			namespace:    "argocd",
			stripStatus:  true,
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
---
`,
		},
		{
			name:              "Application status should be stripped from the exported manifest when created in enabled namespace and strip-status is set",
			object:            newApplication("dev"),
			namespace:         "dev",
			enabledNamespaces: []string{"dev", "prod"},
			stripStatus:       true,
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
---
`,
		},
		{
			name:         "ApplicationSet status should be stripped from the exported manifest when strip-status is set",
			object:       newApplicationSet("argocd"),
			namespace:    "argocd",
			stripStatus:  true,
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
---
`,
		},
		{
			name:         "ConfigMap should be unaffected by strip-status since it has no status field",
			object:       newConfigmapObject(),
			stripStatus:  true,
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
			name:         "Secret should be unaffected by strip-status since it has no status field",
			object:       newSecretsObject(),
			stripStatus:  true,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			kind := tt.object.GetKind()
			if kind == "Application" || kind == "ApplicationSet" {
				if security.IsNamespaceEnabled(tt.namespace, "argocd", tt.enabledNamespaces) {
					export(&buf, *tt.object, ArgoCDNamespace, tt.stripStatus)
				}
			} else {
				export(&buf, *tt.object, ArgoCDNamespace, tt.stripStatus)
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

func TestCheckAppHasNoNeedToStopOperation(t *testing.T) {
	tests := []struct {
		name          string
		liveObj       *unstructured.Unstructured
		stopOperation bool
		expected      bool
	}{
		{
			name:          "stopOperation false returns true",
			liveObj:       newApplication("argocd"),
			stopOperation: false,
			expected:      true,
		},
		{
			name:          "stopOperation true with non-Application returns true",
			liveObj:       newConfigmapObject(),
			stopOperation: true,
			expected:      true,
		},
		{
			name:          "stopOperation true with Application having no operation returns true",
			liveObj:       newApplication("argocd"),
			stopOperation: true,
			expected:      true,
		},
		{
			name: "stopOperation true with Application having operation returns false",
			liveObj: func() *unstructured.Unstructured {
				app := newApplication("argocd")
				app.Object["operation"] = map[string]any{}
				return app
			}(),
			stopOperation: true,
			expected:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAppHasNoNeedToStopOperation(*tt.liveObj, tt.stopOperation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdateLive(t *testing.T) {
	tests := []struct {
		name          string
		backup        *unstructured.Unstructured
		live          *unstructured.Unstructured
		stopOperation bool
		validateFn    func(*testing.T, *unstructured.Unstructured)
	}{
		{
			name:   "ConfigMap data is updated from backup",
			backup: newBackupObject("backup", false, true),
			live:   newBackupObject("live", false, true),
			validateFn: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.NotNil(t, result)
				assert.Equal(t, "my-configmap", result.GetName())
			},
		},
		{
			name:   "Application spec is updated from backup",
			backup: newApplication("argocd"),
			live: func() *unstructured.Unstructured {
				app := newApplication("argocd")
				app.Object["spec"] = map[string]any{"project": "modified"}
				return app
			}(),
			validateFn: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.NotNil(t, result.Object["spec"])
				spec, ok := result.Object["spec"].(map[string]any)
				assert.True(t, ok)
				assert.Equal(t, "default", spec["project"])
			},
		},
		{
			name: "Application operation is cleared when stopOperation is true",
			backup: func() *unstructured.Unstructured {
				app := newApplication("argocd")
				return app
			}(),
			live: func() *unstructured.Unstructured {
				app := newApplication("argocd")
				app.Object["operation"] = map[string]any{"sync": map[string]any{}}
				return app
			}(),
			stopOperation: true,
			validateFn: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.Nil(t, result.Object["operation"])
			},
		},
		{
			name:   "ApplicationSet spec is updated from backup",
			backup: newApplicationSet("argocd"),
			live: func() *unstructured.Unstructured {
				appset := newApplicationSet("argocd")
				appset.Object["spec"] = map[string]any{"modified": true}
				return appset
			}(),
			validateFn: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				assert.NotNil(t, result.Object["spec"])
				spec, ok := result.Object["spec"].(map[string]any)
				assert.True(t, ok)
				assert.NotNil(t, spec["generators"])
			},
		},
		{
			name:   "Labels and annotations are updated from backup",
			backup: newBackupObject("backup-id", true, true),
			live:   newBackupObject("live-id", true, true),
			validateFn: func(t *testing.T, result *unstructured.Unstructured) {
				t.Helper()
				labels := result.GetLabels()
				assert.NotNil(t, labels)
				assert.Equal(t, "backup-id", labels[common.LabelKeyAppInstance])
				annotations := result.GetAnnotations()
				assert.NotNil(t, annotations)
				assert.Equal(t, "backup-id", annotations[common.AnnotationKeyAppInstance])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updateLive(tt.backup, tt.live, tt.stopOperation)
			tt.validateFn(t, result)
			assert.NotNil(t, result)
		})
	}
}
