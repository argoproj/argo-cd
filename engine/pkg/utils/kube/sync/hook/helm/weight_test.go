package helm

import (
	"testing"

	. "github.com/argoproj/argo-cd/engine/pkg/utils/testing"

	"github.com/stretchr/testify/assert"
)

func TestWeight(t *testing.T) {
	assert.Equal(t, Weight(NewPod()), 0)
	assert.Equal(t, Weight(Annotate(NewPod(), "helm.sh/hook-weight", "1")), 1)
}
