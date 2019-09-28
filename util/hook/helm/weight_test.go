package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/test"
)

func TestWeight(t *testing.T) {
	assert.Equal(t, Weight(NewPod()), 0)
	assert.Equal(t, Weight(Annotate(NewPod(), "helm.sh/hook-weight", "1")), 1)
}
