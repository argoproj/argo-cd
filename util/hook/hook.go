package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// IsHook indicates if the object is either a Argo CD or Helm hook
func IsHook(obj *unstructured.Unstructured) bool {
	return IsArgoHook(obj) || IsHelmHook(obj)
}

// IsHelmHook indicates if the supplied object is a helm hook
func IsHelmHook(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	hooks, ok := annotations[common.AnnotationKeyHelmHook]
	if ok && hasHook(hooks, common.AnnotationValueHelmHookCRDInstall) {
		return false
	}
	return ok
}

func hasHook(hooks string, hook string) bool {
	for _, item := range strings.Split(hooks, ",") {
		if strings.TrimSpace(item) == hook {
			return true
		}
	}
	return false
}

// IsArgoHook indicates if the supplied object is an Argo CD application lifecycle hook
// (vs. a normal, synced application resource)
func IsArgoHook(obj *unstructured.Unstructured) bool {
	return len(GetHooks(obj)) > 0
}

func GetHooks(obj *unstructured.Unstructured) (hookTypes []HookType) {
	for _, hookType := range strings.Split(obj.GetAnnotations()[common.AnnotationKeyHook], ",") {
		hookType = strings.TrimSpace(hookType)
		switch HookType(hookType) {
		case HookTypePreSync, HookTypeSync, HookTypePostSync:
			hookTypes = append(hookTypes, HookType(hookType))
		}
	}
	return hookTypes
}

func HasHook(obj *unstructured.Unstructured, hookType HookType) bool {
	for _, candidate := range GetHooks(obj) {
		if hookType == candidate {
			return true
		}
	}
	return false
}
