package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestIsHook(t *testing.T) {
	assert.False(t, IsHook(testingutils.NewPod()))
	assert.True(t, IsHook(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "anything")))
	// helm calls "crd-install" a hook, but it really can't be treated as such
	assert.False(t, IsHook(testingutils.Annotate(testingutils.NewCRD(), "helm.sh/hook", "crd-install")))
}
