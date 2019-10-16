package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test"
)

func TestTypes(t *testing.T) {
	assert.Nil(t, Types(NewPod()))
	assert.Equal(t, []Type{PreInstall}, Types(Annotate(NewPod(), "helm.sh/hook", "pre-install")))
	assert.Equal(t, []Type{PreUpgrade}, Types(Annotate(NewPod(), "helm.sh/hook", "pre-upgrade")))
	assert.Equal(t, []Type{PostUpgrade}, Types(Annotate(NewPod(), "helm.sh/hook", "post-upgrade")))
	assert.Equal(t, []Type{PostInstall}, Types(Annotate(NewPod(), "helm.sh/hook", "post-install")))
	// helm calls "crd-install" a hook, but it really can't be treated as such
	assert.Empty(t, Types(Annotate(NewPod(), "helm.sh/hook", "crd-install")))
	// we do not consider these supported hooks
	assert.Nil(t, Types(Annotate(NewPod(), "helm.sh/hook", "pre-rollback")))
	assert.Nil(t, Types(Annotate(NewPod(), "helm.sh/hook", "post-rollback")))
	assert.Nil(t, Types(Annotate(NewPod(), "helm.sh/hook", "test-success")))
	assert.Nil(t, Types(Annotate(NewPod(), "helm.sh/hook", "test-failure")))
}

func TestType_HookType(t *testing.T) {
	assert.Equal(t, v1alpha1.HookTypePreSync, PreInstall.HookType())
	assert.Equal(t, v1alpha1.HookTypePreSync, PreUpgrade.HookType())
	assert.Equal(t, v1alpha1.HookTypePostSync, PostUpgrade.HookType())
	assert.Equal(t, v1alpha1.HookTypePostSync, PostInstall.HookType())
}
