package syncwaves

import (
	"strconv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	helmhook "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/hook/helm"
)

func Wave(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWave]
	if ok {
		val, err := strconv.Atoi(text)
		if err == nil {
			return val
		}
	}
	return helmhook.Weight(obj)
}
