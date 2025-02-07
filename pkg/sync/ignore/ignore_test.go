package ignore

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/stretchr/testify/assert"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func newHook(obj *unstructured.Unstructured, hookType common.HookType) *unstructured.Unstructured {
	return testingutils.Annotate(obj, "argocd.argoproj.io/hook", string(hookType))
}

func TestIgnore(t *testing.T) {
	assert.False(t, Ignore(testingutils.NewPod()))
	assert.False(t, Ignore(newHook(testingutils.NewPod(), "Sync")))
	assert.True(t, Ignore(newHook(testingutils.NewPod(), "garbage")))
	assert.False(t, Ignore(testingutils.HelmHook(testingutils.NewPod(), "pre-install")))
	assert.True(t, Ignore(testingutils.HelmHook(testingutils.NewPod(), "garbage")))
}
