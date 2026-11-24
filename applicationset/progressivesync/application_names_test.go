package progressivesync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestApplicationNames(t *testing.T) {
	t.Parallel()

	t.Run("empty slice", func(t *testing.T) {
		assert.Equal(t, []string{}, applicationNames(nil))
	})

	t.Run("returns names only without full object fields", func(t *testing.T) {
		apps := []argov1alpha1.Application{
			{ObjectMeta: metav1.ObjectMeta{Name: "app-one", Namespace: "argocd"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "app-two", Namespace: "argocd"}},
		}
		assert.Equal(t, []string{"app-one", "app-two"}, applicationNames(apps))
	})
}
