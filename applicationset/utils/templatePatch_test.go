package utils

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validate(t *testing.T, patch string) {
	app := &appv1.Application{
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
	}

	result, err := ApplyPatchTemplate(app, patch)
	require.NoError(t, err)
	require.Contains(t, result.ObjectMeta.Annotations, "annotation-some-key")
	assert.Equal(t, result.ObjectMeta.Annotations["annotation-some-key"], "annotation-some-value")

	assert.Equal(t, result.Spec.SyncPolicy.Automated.Prune, true)
	require.Contains(t, result.Spec.Source.Helm.ValueFiles, "values.test.yaml")
	require.Contains(t, result.Spec.Source.Helm.ValueFiles, "values.big.yaml")
}

func TestWithJson(t *testing.T) {

	validate(t, `{
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
	}`)

}

func TestWithYaml(t *testing.T) {

	validate(t, `
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
      prune: true`)
}

func TestError(t *testing.T) {
	app := &appv1.Application{}

	result, err := ApplyPatchTemplate(app, "hello world")
	require.Error(t, err)
	require.Nil(t, result)
}
