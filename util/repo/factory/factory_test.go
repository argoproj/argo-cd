package factory

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

func TestDetect(t *testing.T) {
	t.Run("Explicit", func(t *testing.T) {
		repoType, err := Detect(&v1alpha1.Repository{Type: "my-type"}, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "my-type", repoType)
	})
	t.Run("Helm", func(t *testing.T) {
		repoType, err := Detect(&v1alpha1.Repository{Repo: "https://kubernetes-charts.storage.googleapis.com", Name: "stable"}, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "helm", repoType)
	})
	t.Run("Git", func(t *testing.T) {
		repoType, err := Detect(&v1alpha1.Repository{Repo: "https://github.com/argoproj/argocd-example-apps"}, metrics.NopReporter)
		assert.NoError(t, err)
		assert.Equal(t, "git", repoType)
	})
}
