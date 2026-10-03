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

func TestWaveIgnoresHelmWeightWhenArgoHookDefined(t *testing.T) {
	t.Parallel()
	// Per documentation, defining any Argo CD hook causes all Helm hooks to be ignored,
	// so helm.sh/hook-weight must not apply.
	obj := testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "pre-install"), "argocd.argoproj.io/hook", "Sync")
	obj = testingutils.Annotate(obj, "helm.sh/hook-weight", "-20")
	assert.Equal(t, 0, Wave(obj))
	// Helm weight still applies when no Argo CD hook is defined.
	assert.Equal(t, -20, Wave(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-weight", "-20")))
}
