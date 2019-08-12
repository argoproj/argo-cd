package syncwaves

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/test"
)

func TestWave(t *testing.T) {
	assert.Equal(t, 0, Wave(NewPod()))
	assert.Equal(t, 1, Wave(Annotate(NewPod(), "argocd.argoproj.io/sync-wave", "1")))
	assert.Equal(t, 1, Wave(Annotate(NewPod(), "helm.sh/hook-weight", "1")))
}
