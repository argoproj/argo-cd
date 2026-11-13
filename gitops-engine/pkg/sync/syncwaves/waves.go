package syncwaves

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/hook"
	helmhook "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/hook/helm"
)

func Wave(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWave]
	if ok {
		val, err := strconv.Atoi(text)
		if err == nil {
			return val
		}
	}
	// Helm hooks are ignored when Argo CD hooks are defined.
	if hook.HasArgoHookTypes(obj) {
		return 0
	}
	return helmhook.Weight(obj)
}
