package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func IsHook(obj *unstructured.Unstructured) bool {
	return len(Hooks(obj)) > 0
}

func Hooks(obj *unstructured.Unstructured) (hookTypes []HookType) {
	for _, hookType := range strings.Split(obj.GetAnnotations()[common.AnnotationKeyHook], ",") {
		hookType = strings.TrimSpace(hookType)
		switch HookType(hookType) {
		case HookTypePreSync, HookTypeSync, HookTypePostSync:
			hookTypes = append(hookTypes, HookType(hookType))
		}
	}
	return hookTypes
}
