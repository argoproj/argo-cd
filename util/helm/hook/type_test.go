package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test"
)

func TestTypes(t *testing.T) {
	assert.Nil(t, Types(NewPod()))
	assert.Equal(t, []Type{CRDInstall}, Types(Annotate(NewPod(), "helm.sh/hook", "crd-install")))
	assert.Equal(t, []Type{PreInstall}, Types(Annotate(NewPod(), "helm.sh/hook", "pre-install")))
	assert.Equal(t, []Type{PreUpgrade}, Types(Annotate(NewPod(), "helm.sh/hook", "pre-upgrade")))
	assert.Equal(t, []Type{PostUpgrade}, Types(Annotate(NewPod(), "helm.sh/hook", "post-upgrade")))
	assert.Equal(t, []Type{PostInstall}, Types(Annotate(NewPod(), "helm.sh/hook", "post-install")))
}

func TestType_HookType(t *testing.T) {
	assert.Equal(t, v1alpha1.HookTypePreSync, CRDInstall.HookType())
	assert.Equal(t, v1alpha1.HookTypePreSync, PreInstall.HookType())
	assert.Equal(t, v1alpha1.HookTypePreSync, PreUpgrade.HookType())
	assert.Equal(t, v1alpha1.HookTypePostSync, PostUpgrade.HookType())
	assert.Equal(t, v1alpha1.HookTypePostSync, PostInstall.HookType())
}
