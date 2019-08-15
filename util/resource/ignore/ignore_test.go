package ignore

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/test"
)

func TestIgnore(t *testing.T) {
	assert.False(t, Ignore(NewPod()))
	assert.False(t, Ignore(Hook(NewPod(), "Sync")))
	assert.True(t, Ignore(Hook(NewPod(), "garbage")))
	assert.False(t, Ignore(HelmHook(NewPod(), "pre-install")))
	assert.True(t, Ignore(HelmHook(NewPod(), "garbage")))
}
