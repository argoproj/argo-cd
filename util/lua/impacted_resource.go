package lua

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// This struct represents a wrapper, that is returned from Lua custom action script, around the unstructured k8s resource + a k8s operation
// that will need to be performed on this returned resource.
// Currently only "create" and "patch" operations are supported for custom actions.
// This replaces the traditional architecture of "Lua action returns the source resource for ArgoCD to patch".
// This enables ArgoCD to create NEW resources upon custom action.
// Note that the Lua code in the custom action is coupled to this type, since Lua json output is then unmarshalled to this struct.
// Avoided using iota, since need the mapping of the string value the end users will write in Lua code ("create" and "patch").
// TODO: maybe there is a nicer general way to marshal and unmarshal, instead of explicit iteration over the enum values.
type K8SOperation string

const (
	CreateOperation K8SOperation = "create"
	PatchOperation  K8SOperation = "patch"
)

type ImpactedResource struct {
	UnstructuredObj *unstructured.Unstructured `json:"resource"`
	K8SOperation    K8SOperation               `json:"operation"`
}

func (op *K8SOperation) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"create"`:
		*op = CreateOperation
	case `"patch"`:
		*op = PatchOperation
	default:
		return fmt.Errorf("unsupported operation: %s", data)
	}
	return nil
}

func (op K8SOperation) MarshalJSON() ([]byte, error) {
	switch op {
	case CreateOperation:
		return []byte(`"create"`), nil
	case PatchOperation:
		return []byte(`"patch"`), nil
	default:
		return nil, fmt.Errorf("unsupported operation: %s", op)
	}
}
