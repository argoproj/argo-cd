package helm

import (
	"testing"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"

	"github.com/stretchr/testify/assert"
)

func TestWeight(t *testing.T) {
	assert.Equal(t, 0, Weight(testingutils.NewPod()))
	assert.Equal(t, 1, Weight(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-weight", "1")))
}
