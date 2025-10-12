package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func Test_ApplyTemplatePatch(t *testing.T) {
	testCases := []struct {
		name          string
		appTemplate   *appv1.Application
		templatePatch string
		expectedApp   *appv1.Application
	}{
		{
			name: "patch with JSON",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templatePatch: `{
				"metadata": {
					"annotations": {
						"annotation-some-key": "annotation-some-value"
					}
				},
				"spec": {
					"source": {
						"helm": {
							"valueFiles": [
								"values.test.yaml",
								"values.big.yaml"
							]
						}
					},
					"syncPolicy": {
						"automated": {
							"prune": true
						}
					}
				}
			}`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
					Annotations: map[string]string{
						"annotation-some-key": "annotation-some-value",
					},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
						Helm: &appv1.ApplicationSourceHelm{
							ValueFiles: []string{
								"values.test.yaml",
								"values.big.yaml",
							},
						},
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
					SyncPolicy: &appv1.SyncPolicy{
						Automated: &appv1.SyncPolicyAutomated{
							Prune: true,
						},
					},
				},
			},
		},
		{
			name: "patch with YAML",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templatePatch: `
metadata:
  annotations:
    annotation-some-key: annotation-some-value
spec:
  source:
    helm:
      valueFiles:
        - values.test.yaml
        - values.big.yaml
  syncPolicy:
    automated:
      prune: true`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
					Annotations: map[string]string{
						"annotation-some-key": "annotation-some-value",
					},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
						Helm: &appv1.ApplicationSourceHelm{
							ValueFiles: []string{
								"values.test.yaml",
								"values.big.yaml",
							},
						},
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
					SyncPolicy: &appv1.SyncPolicy{
						Automated: &appv1.SyncPolicyAutomated{
							Prune: true,
						},
					},
				},
			},
		},
		{
			name: "project field isn't overwritten",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster-guestbook",
					Namespace: "namespace",
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templatePatch: `
spec:
  project: my-project`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster-guestbook",
					Namespace: "namespace",
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			result, err := applyTemplatePatch(tcc.appTemplate, tcc.templatePatch)
			require.NoError(t, err)
			assert.Equal(t, *tcc.expectedApp, *result)
		})
	}
}

func Test_ApplyTemplateJSONPatch(t *testing.T) {
	testCases := []struct {
		name              string
		appTemplate       *appv1.Application
		templateJSONPatch string
		expectedApp       *appv1.Application
	}{
		{
			name: "json+path with JSON",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templateJSONPatch: `
[
	{
		"op": "add",
		"path": "/metadata/annotations",
		"value": {}
	},
	{
		"op": "add",
		"path": "/metadata/annotations/annotation-some-key",
		"value": "annotation-some-value"
	},
	{
		"op": "add",
		"path": "/spec/source/helm",
		"value": {}
	},
	{
		"op": "add",
		"path": "/spec/source/helm/valueFiles",
		"value": ["values.test.yaml", "values.big.yaml"]
	},
	{
		"op": "add",
		"path": "/spec/syncPolicy",
		"value": {}
	},
	{
		"op": "add",
		"path": "/spec/syncPolicy/automated",
		"value": {}
	},	
	{
		"op": "add",
		"path": "/spec/syncPolicy/automated/prune",
		"value": true
	}
]`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
					Annotations: map[string]string{
						"annotation-some-key": "annotation-some-value",
					},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
						Helm: &appv1.ApplicationSourceHelm{
							ValueFiles: []string{
								"values.test.yaml",
								"values.big.yaml",
							},
						},
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
					SyncPolicy: &appv1.SyncPolicy{
						Automated: &appv1.SyncPolicyAutomated{
							Prune: true,
						},
					},
				},
			},
		},
		{
			name: "json+path with YAML",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templateJSONPatch: `
- op: add
  path: /metadata/annotations
  value:
    annotation-some-key: annotation-some-value
- op: add
  path: /spec/source/helm
  value:
    valueFiles:
     - values.test.yaml
     - values.big.yaml
- op: add
  path: /spec/syncPolicy
  value:
    automated:
      prune: true
`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
					Annotations: map[string]string{
						"annotation-some-key": "annotation-some-value",
					},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
						Helm: &appv1.ApplicationSourceHelm{
							ValueFiles: []string{
								"values.test.yaml",
								"values.big.yaml",
							},
						},
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
					SyncPolicy: &appv1.SyncPolicy{
						Automated: &appv1.SyncPolicyAutomated{
							Prune: true,
						},
					},
				},
			},
		},
		{
			name: "json+patch sources",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Sources: appv1.ApplicationSources{
						appv1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						appv1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "blue-green",
							Helm: &appv1.ApplicationSourceHelm{
								Values: `
---
replicaCount: 3`,
							},
						},
					},
				},
			},
			templateJSONPatch: `
[
  {
    "op": "add",
    "path": "/spec/sources/1/helm/valuesObject",
    "value": {"replicaCount": 6}
  }
]`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Sources: appv1.ApplicationSources{
						appv1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "guestbook",
						},
						appv1.ApplicationSource{
							RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
							TargetRevision: "HEAD",
							Path:           "blue-green",
							Helm: &appv1.ApplicationSourceHelm{
								Values: `
---
replicaCount: 3`,
								ValuesObject: &runtime.RawExtension{
									Raw: []byte(`{"replicaCount":6}`),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "json+path no patch",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templateJSONPatch: `# No actual patch`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cluster-guestbook",
					Namespace:  "namespace",
					Finalizers: []string{appv1.ResourcesFinalizerName},
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
		},
		{
			name: "project field isn't overwritten",
			appTemplate: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster-guestbook",
					Namespace: "namespace",
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templateJSONPatch: `
[
	{
		"op": "replace",
		"path": "/spec/project",
		"value": "my-project"
	}
]`,
			expectedApp: &appv1.Application{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Application",
					APIVersion: "argoproj.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster-guestbook",
					Namespace: "namespace",
				},
				Spec: appv1.ApplicationSpec{
					Project: "default",
					Source: &appv1.ApplicationSource{
						RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
						TargetRevision: "HEAD",
						Path:           "guestbook",
					},
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tcc := tc
		t.Run(tcc.name, func(t *testing.T) {
			result, err := applyTemplateJSONPatch(tcc.appTemplate, tcc.templateJSONPatch)
			require.NoError(t, err)
			assert.Equal(t, *tcc.expectedApp, *result)
		})
	}
}

func TestError(t *testing.T) {
	app := &appv1.Application{}

	result, err := applyTemplatePatch(app, "hello world")
	require.Error(t, err)
	require.Nil(t, result)
}

// TestJsonPatchError verifies error handling for invalid JSON Patch input.
func TestJsonPatchError(t *testing.T) {
	app := &appv1.Application{}

	// Intentionally invalid JSON Patch format to test error handling
	result, err := applyTemplateJSONPatch(app, `["not": "valid", "patch": "/spec/something"]`)
	require.Error(t, err)
	require.Nil(t, result)
	require.Nil(t, result)

	// Intentionally invalid JSON Patch to test error handling.
	result, err = applyTemplateJSONPatch(app, `["op": "add", "path": "/spec/something/that/does/not/exist"]`)
	require.Error(t, err)
	require.Nil(t, result)
}
