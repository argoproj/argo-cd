package ignore

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/engine/pkg/utils/testing"
)

func newHook(obj *unstructured.Unstructured, hookType common.HookType) *unstructured.Unstructured {
	return Annotate(obj, "argocd.argoproj.io/hook", string(hookType))
}

func TestIgnore(t *testing.T) {
	assert.False(t, Ignore(NewPod()))
	assert.False(t, Ignore(newHook(NewPod(), "Sync")))
	assert.True(t, Ignore(newHook(NewPod(), "garbage")))
	assert.False(t, Ignore(HelmHook(NewPod(), "pre-install")))
	assert.True(t, Ignore(HelmHook(NewPod(), "garbage")))
}
