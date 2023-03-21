package lua

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// This struct represents a wrapper, that is returned from Lua custom action script, around the unstructured k8s resource + a k8s verb
// that will need to be performed on this returned resource.
// Currently only "create" and "patch" operations are supported for custom actions
// This replaces the traditional architecture of "Lua action returns the source resource for ArgoCD to patch".
// This enables ArgoCD to create NEW resources upon custom action.
// Note that the Lua code in the custom action is coupled to this type, since Lua json output is then unmarshalled to this struct.
// TODO: maybe K8SOperation needs to be an enum of supported k8s verbs, with a custom json marshaller/unmarshaller
type ImpactedResource struct {
	UnstructuredObj *unstructured.Unstructured `json:"resource"`
	K8SOperation    string                     `json:"operation"`
}
