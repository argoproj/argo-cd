package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestTypes(t *testing.T) {
	assert.Nil(t, Types(testingutils.NewPod()))
	assert.Equal(t, []Type{PreInstall}, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "pre-install")))
	assert.Equal(t, []Type{PreUpgrade}, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "pre-upgrade")))
	assert.Equal(t, []Type{PostUpgrade}, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "post-upgrade")))
	assert.Equal(t, []Type{PostInstall}, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "post-install")))
	// helm calls "crd-install" a hook, but it really can't be treated as such
	assert.Empty(t, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "crd-install")))
	// we do not consider these supported hooks
	assert.Nil(t, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "pre-rollback")))
	assert.Nil(t, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "post-rollback")))
	assert.Nil(t, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "test-success")))
	assert.Nil(t, Types(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", "test-failure")))
}

func TestType_HookType(t *testing.T) {
	assert.Equal(t, common.HookTypePreSync, PreInstall.HookType())
	assert.Equal(t, common.HookTypePreSync, PreUpgrade.HookType())
	assert.Equal(t, common.HookTypePostSync, PostUpgrade.HookType())
	assert.Equal(t, common.HookTypePostSync, PostInstall.HookType())
}
