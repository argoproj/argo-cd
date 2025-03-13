package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestNoHooks(t *testing.T) {
	obj := &unstructured.Unstructured{}
	assert.False(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.Nil(t, Types(obj))
}

func TestOneHook(t *testing.T) {
	hookTypesString := []string{"PreSync", "Sync", "PostSync", "SyncFail"}
	hookTypes := []common.HookType{common.HookTypePreSync, common.HookTypeSync, common.HookTypePostSync, common.HookTypeSyncFail}
	for i, hook := range hookTypesString {
		obj := example(hook)
		assert.True(t, IsHook(obj))
		assert.False(t, Skip(obj))
		assert.Equal(t, []common.HookType{hookTypes[i]}, Types(obj))
	}
}

// peculiar case of something marked with "Skip" cannot, by definition, be a hook
// IMHO this is bad design  as it conflates a flag on something that can never be a hook, with something that is
// always a hook, creating a nasty exception we always need to check for, and a bunch of horrible edge cases
func TestSkipHook(t *testing.T) {
	obj := example("Skip")
	assert.False(t, IsHook(obj))
	assert.True(t, Skip(obj))
	assert.Equal(t, []common.HookType{common.HookTypeSkip}, Types(obj))
}

// we treat garbage as the user intended you to be a hook, but spelled it wrong, so you are a hook, but we don't
// know what phase you're a part of
func TestGarbageHook(t *testing.T) {
	obj := example("Garbage")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.Nil(t, Types(obj))
}

func TestTwoHooks(t *testing.T) {
	obj := example("PreSync,PostSync")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.ElementsMatch(t, []common.HookType{common.HookTypePreSync, common.HookTypePostSync}, Types(obj))
}

func TestDupHookTypes(t *testing.T) {
	assert.Equal(t, []common.HookType{common.HookTypeSync}, Types(example("Sync,Sync")))
}

// horrible edge case
func TestSkipAndHook(t *testing.T) {
	obj := example("Skip,PreSync,PostSync")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.ElementsMatch(t, []common.HookType{common.HookTypeSkip, common.HookTypePreSync, common.HookTypePostSync}, Types(obj))
}

func TestGarbageAndHook(t *testing.T) {
	obj := example("Sync,Garbage")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.Equal(t, []common.HookType{common.HookTypeSync}, Types(obj))
}

func TestHelmHook(t *testing.T) {
	obj := testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "pre-install")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.Equal(t, []common.HookType{common.HookTypePreSync}, Types(obj))
}

func TestGarbageHelmHook(t *testing.T) {
	obj := testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "garbage")
	assert.True(t, IsHook(obj))
	assert.False(t, Skip(obj))
	assert.Nil(t, Types(obj))
}

// we should ignore Helm hooks if we have an Argo CD hook
func TestBothHooks(t *testing.T) {
	obj := testingutils.Annotate(example("Sync"), "helm.sh/hook", "pre-install")
	assert.Equal(t, []common.HookType{common.HookTypeSync}, Types(obj))
}

func example(hook string) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", hook)
}
