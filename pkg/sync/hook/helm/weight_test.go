package helm

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/utils/testing"

	"github.com/stretchr/testify/assert"
)

func TestWeight(t *testing.T) {
	assert.Equal(t, 0, Weight(NewPod()))
	assert.Equal(t, 1, Weight(Annotate(NewPod(), "helm.sh/hook-weight", "1")))
}
