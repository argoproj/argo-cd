package helm

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/resource"
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

var hookTypes = map[Type]v1alpha1.HookType{
	PreInstall:  v1alpha1.HookTypePreSync,
	PreUpgrade:  v1alpha1.HookTypePreSync,
	PostUpgrade: v1alpha1.HookTypePostSync,
	PostInstall: v1alpha1.HookTypePostSync,
}

func (t Type) HookType() v1alpha1.HookType {
	return hookTypes[t]
}

func Types(obj *unstructured.Unstructured) []Type {
	var types []Type
	for _, text := range resource.GetAnnotationCSVs(obj, "helm.sh/hook") {
		t, ok := NewType(text)
		if ok {
			types = append(types, t)
		}
	}
	return types
}
