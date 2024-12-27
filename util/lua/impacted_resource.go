package lua

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// K8SOperation represents the operation type for k8s resources.
type K8SOperation string

// Currently, only "create" and "patch" operations are supported.
const (
	CreateOperation K8SOperation = "create"
	PatchOperation  K8SOperation = "patch"
)

// operationMapping maps K8SOperation values to their string representations.
var operationMapping = map[K8SOperation]string{
	CreateOperation: "create",
	PatchOperation:  "patch",
}

// reverseOperationMapping is the inverse of operationMapping.
var reverseOperationMapping = func() map[string]K8SOperation {
	m := make(map[string]K8SOperation)
	for k, v := range operationMapping {
		m[v] = k
	}
	return m
}()

// Represents a wrapper for the unstructured Kubernetes resource and the operation (e.g., "create" or "patch") to be performed.
type ImpactedResource struct {
	UnstructuredObj *unstructured.Unstructured `json:"resource"`
	K8SOperation    K8SOperation               `json:"operation"`
}

// UnmarshalJSON for K8SOperation using reverseOperationMapping.
func (op *K8SOperation) UnmarshalJSON(data []byte) error {
	var opStr string
	if err := json.Unmarshal(data, &opStr); err != nil {
		return err
	}

	if operation, exists := reverseOperationMapping[opStr]; exists {
		*op = operation
		return nil
	}
	return fmt.Errorf("unsupported operation: %s", opStr)
}

// MarshalJSON for K8SOperation using operationMapping.
func (op K8SOperation) MarshalJSON() ([]byte, error) {
	if opStr, exists := operationMapping[op]; exists {
		return json.Marshal(opStr)
	}
	return nil, fmt.Errorf("unsupported operation: %s", op)
}
