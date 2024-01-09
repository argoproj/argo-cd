package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
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
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
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
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
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
					Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
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

func TestError(t *testing.T) {
	app := &appv1.Application{}

	result, err := applyTemplatePatch(app, "hello world")
	require.Error(t, err)
	require.Nil(t, result)
}
