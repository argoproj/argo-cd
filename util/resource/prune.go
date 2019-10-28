package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
)

// Whether you should not prune the object.
// It only makes sense for this to be a live object.
func NoPrune(obj *unstructured.Unstructured) bool {
	return obj != nil && HasAnnotationOption(obj, common.AnnotationSyncOptions, "Prune=false")
}
