package ignore

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/hook"
)

// should we Ignore this resource?
func Ignore(obj *unstructured.Unstructured) bool {
	return hook.IsHook(obj) && len(hook.Types(obj)) == 0
}
