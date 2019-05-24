package hook

import (
	"testing"

	"github.com/argoproj/argo-cd/test"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestNoHooks(t *testing.T) {
	obj := &unstructured.Unstructured{}
	assert.False(t, IsHook(obj))
	assert.Nil(t, Hooks(obj))
}

func TestOneHook(t *testing.T) {
	obj := example("Sync")
	assert.True(t, IsHook(obj))
	assert.Equal(t, []HookType{HookTypeSync}, Hooks(obj))
}

func TestTwoHooks(t *testing.T) {
	obj := example("PreSync,PostSync")
	assert.True(t, IsHook(obj))
	assert.Equal(t, []HookType{HookTypePreSync, HookTypePostSync}, Hooks(obj))
}

func TestSkipKook(t *testing.T) {
	obj := example("Skip")
	assert.False(t, IsHook(obj))
	assert.Nil(t, Hooks(obj))
}

func TestGarbageHook(t *testing.T) {
	obj := example("Garbage")
	assert.False(t, IsHook(obj))
	assert.Nil(t, Hooks(obj))
}

func example(hook string) *unstructured.Unstructured {
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": hook})
	return pod
}
