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
			name: "json patch with YAML",
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
					Destination: appv1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "guestbook",
					},
				},
			},
			templatePatch: `
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
    "path": "/spec/sources/1/helm/valuesObject",
    "value": {"image":{"tag":"v6"}}
  }
]
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
									Raw: []byte(`{"image":{"tag":"v6"}}`),
								},
							},
						},
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

func TestError(t *testing.T) {
	app := &appv1.Application{}

	result, err := applyTemplatePatch(app, "hello world")
	require.Error(t, err)
	require.Nil(t, result)
}
