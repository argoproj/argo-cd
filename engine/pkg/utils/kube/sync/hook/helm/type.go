package helm

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	resourceutil "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/resource"
)

type Type string

const (
	PreInstall  Type = "pre-install"
	PreUpgrade  Type = "pre-upgrade"
	PostUpgrade Type = "post-upgrade"
	PostInstall Type = "post-install"
)

func NewType(t string) (Type, bool) {
	return Type(t),
		t == string(PreInstall) ||
			t == string(PreUpgrade) ||
			t == string(PostUpgrade) ||
			t == string(PostInstall)
}

var hookTypes = map[Type]common.HookType{
	PreInstall:  common.HookTypePreSync,
	PreUpgrade:  common.HookTypePreSync,
	PostUpgrade: common.HookTypePostSync,
	PostInstall: common.HookTypePostSync,
}

func (t Type) HookType() common.HookType {
	return hookTypes[t]
}

func Types(obj *unstructured.Unstructured) []Type {
	var types []Type
	for _, text := range resourceutil.GetAnnotationCSVs(obj, "helm.sh/hook") {
		t, ok := NewType(text)
		if ok {
			types = append(types, t)
		}
	}
	return types
}
