package shared

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetApplication(t *testing.T) {
	t.Run("valid application", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name":      "guestbook",
					"namespace": "argocd",
				},
				"spec": map[string]any{
					"destination": map[string]any{
						"namespace": "guestbook",
					},
				},
			},
		}

		app, err := GetApplication(obj)

		require.NoError(t, err)
		require.Equal(t, "guestbook", app.Name)
		require.Equal(t, "argocd", app.Namespace)
		require.Equal(t, "guestbook", app.Spec.Destination.Namespace)
	})

	t.Run("invalid application", func(t *testing.T) {
		obj := &unstructured.Unstructured{
			Object: map[string]any{
				"spec": map[string]any{
					"destination": "invalid",
				},
			},
		}

		app, err := GetApplication(obj)

		require.Error(t, err)
		require.Nil(t, app)
	})
}
