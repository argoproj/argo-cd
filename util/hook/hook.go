package hook

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	resHookTypes := strings.Split(annotations[common.AnnotationKeyHook], ",")
	for _, hookType := range resHookTypes {
		hookType = strings.TrimSpace(hookType)
		switch argoappv1.HookType(hookType) {
		case argoappv1.HookTypePreSync, argoappv1.HookTypeSync, argoappv1.HookTypePostSync:
			return true
		}
	}
	return false
}
