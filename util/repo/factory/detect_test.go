package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

func TestDetect(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		r := &v1alpha1.Repository{Repo: "invalid"}
		err := DetectType(r, metrics.NopReporter)
		assert.Error(t, err)
		assert.Empty(t, r.Type)
	})
	t.Run("Explicit", func(t *testing.T) {
		r := &v1alpha1.Repository{Type: "my-type"}
		err := DetectType(r, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "my-type", r.Type)
	})
	t.Run("Helm", func(t *testing.T) {
		r := &v1alpha1.Repository{Repo: "https://kubernetes-charts.storage.googleapis.com", Name: "stable"}
		err := DetectType(r, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "helm", r.Type)
	})
	t.Run("Git", func(t *testing.T) {
		r := &v1alpha1.Repository{Repo: "https://github.com/argoproj/argocd-example-apps"}
		err := DetectType(r, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "git", r.Type)
	})
}
