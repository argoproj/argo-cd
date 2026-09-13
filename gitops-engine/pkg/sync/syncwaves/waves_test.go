package syncwaves

import (
	"testing"

	"github.com/stretchr/testify/assert"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

func TestWave(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, Wave(testingutils.NewPod()))
	assert.Equal(t, 1, Wave(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "1")))
	assert.Equal(t, 1, Wave(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-weight", "1")))
}
