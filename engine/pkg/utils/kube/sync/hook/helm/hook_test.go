package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/engine/pkg/utils/testing"
)

func TestIsHook(t *testing.T) {
	assert.False(t, IsHook(NewPod()))
	assert.True(t, IsHook(Annotate(NewPod(), "helm.sh/hook", "anything")))
	// helm calls "crd-install" a hook, but it really can't be treated as such
	assert.False(t, IsHook(Annotate(NewCRD(), "helm.sh/hook", "crd-install")))
}
