package health

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type nodePhase string

// Workflow and node statuses
// See: https://github.com/argoproj/argo-workflows/blob/master/pkg/apis/workflow/v1alpha1/workflow_phase.go
const (
	nodePending   nodePhase = "Pending"
	nodeRunning   nodePhase = "Running"
	nodeSucceeded nodePhase = "Succeeded"
	nodeFailed    nodePhase = "Failed"
	nodeError     nodePhase = "Error"
)

func getArgoWorkflowHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "phase")
	message, _, _ := unstructured.NestedString(obj.Object, "status", "message")

	switch nodePhase(phase) {
	case "", nodePending, nodeRunning:
		return &HealthStatus{Status: HealthStatusProgressing, Message: message}, nil
	case nodeSucceeded:
		return &HealthStatus{Status: HealthStatusHealthy, Message: message}, nil
	case nodeFailed, nodeError:
		return &HealthStatus{Status: HealthStatusDegraded, Message: message}, nil
	}
	return &HealthStatus{Status: HealthStatusUnknown, Message: message}, nil
}
