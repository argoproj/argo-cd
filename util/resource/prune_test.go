package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/test"
)

func TestNoPrune(t *testing.T) {
	assert.False(t, NoPrune(nil))
	assert.False(t, NoPrune(NewPod()))
	assert.True(t, NoPrune(Annotate(NewPod(), "argocd.argoproj.io/sync-options", "Prune=false")))
}