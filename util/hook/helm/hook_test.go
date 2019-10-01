package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/test"
)

func TestIsHook(t *testing.T) {
	assert.False(t, IsHook(NewPod()))
	assert.True(t, IsHook(Annotate(NewPod(), "helm.sh/hook", "anything")))
}
