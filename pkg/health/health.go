package health

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Represents resource health status
type HealthStatusCode string

const (
	// Indicates that health assessment failed and actual health status is unknown
	HealthStatusUnknown HealthStatusCode = "Unknown"
	// Progressing health status means that resource is not healthy but still have a chance to reach healthy state
	HealthStatusProgressing HealthStatusCode = "Progressing"
	// Resource is 100% healthy
	HealthStatusHealthy HealthStatusCode = "Healthy"
	// Assigned to resources that are suspended or paused. The typical example is a
	// [suspended](https://kubernetes.io/docs/tasks/job/automated-tasks-with-cron-jobs/#suspend) CronJob.
	HealthStatusSuspended HealthStatusCode = "Suspended"
	// Degrade status is used if resource status indicates failure or resource could not reach healthy state
	// within some timeout.
	HealthStatusDegraded HealthStatusCode = "Degraded"
	// Indicates that resource is missing in the cluster.
	HealthStatusMissing HealthStatusCode = "Missing"
)

// Implements custom health assessment that overrides built-in assessment
type HealthOverride interface {
	GetResourceHealth(obj *unstructured.Unstructured) (*HealthStatus, error)
}

// Holds health assessment results
type HealthStatus struct {
	Status  HealthStatusCode `json:"status,omitempty"`
	Message string           `json:"message,omitempty"`
}

// healthOrder is a list of health codes in order of most healthy to least healthy
var healthOrder = []HealthStatusCode{
	HealthStatusHealthy,
	HealthStatusSuspended,
	HealthStatusProgressing,
	HealthStatusDegraded,
	HealthStatusMissing,
	HealthStatusUnknown,
}

// IsWorse returns whether or not the new health status code is a worser condition than the current
func IsWorse(current, new HealthStatusCode) bool {
	currentIndex := 0
	newIndex := 0
	for i, code := range healthOrder {
		if current == code {
			currentIndex = i
		}
		if new == code {
			newIndex = i
		}
	}
	return newIndex > currentIndex
}

// GetResourceHealth returns the health of a k8s resource
func GetResourceHealth(obj *unstructured.Unstructured, healthOverride HealthOverride) (health *HealthStatus, err error) {
	if obj.GetDeletionTimestamp() != nil {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Pending deletion",
		}, nil
	}

	if healthOverride != nil {
		health, err := healthOverride.GetResourceHealth(obj)
		if err != nil {
			health = &HealthStatus{
				Status:  HealthStatusUnknown,
				Message: err.Error(),
			}
			return health, err
		}
		if health != nil {
			return health, nil
		}
	}

	gvk := obj.GroupVersionKind()
	switch gvk.Group {
	case "apps":
		switch gvk.Kind {
		case kube.DeploymentKind:
			health, err = getDeploymentHealth(obj)
		case kube.StatefulSetKind:
			health, err = getStatefulSetHealth(obj)
		case kube.ReplicaSetKind:
			health, err = getReplicaSetHealth(obj)
		case kube.DaemonSetKind:
			health, err = getDaemonSetHealth(obj)
		}
	case "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind:
			health, err = getDeploymentHealth(obj)
		case kube.IngressKind:
			health, err = getIngressHealth(obj)
		case kube.ReplicaSetKind:
			health, err = getReplicaSetHealth(obj)
		case kube.DaemonSetKind:
			health, err = getDaemonSetHealth(obj)
		}
	case "argoproj.io":
		switch gvk.Kind {
		case "Workflow":
			health, err = getArgoWorkflowHealth(obj)
		}
	case "apiregistration.k8s.io":
		switch gvk.Kind {
		case kube.APIServiceKind:
			health, err = getAPIServiceHealth(obj)
		}
	case "networking.k8s.io":
		switch gvk.Kind {
		case kube.IngressKind:
			health, err = getIngressHealth(obj)
		}
	case "":
		switch gvk.Kind {
		case kube.ServiceKind:
			health, err = getServiceHealth(obj)
		case kube.PersistentVolumeClaimKind:
			health, err = getPVCHealth(obj)
		case kube.PodKind:
			health, err = getPodHealth(obj)
		}
	case "batch":
		switch gvk.Kind {
		case kube.JobKind:
			health, err = getJobHealth(obj)
		}
	case "autoscaling":
		switch gvk.Kind {
		case kube.HorizontalPodAutoscalerKind:
			health, err = getHPAHealth(obj)
		}
	}
	if err != nil {
		health = &HealthStatus{
			Status:  HealthStatusUnknown,
			Message: err.Error(),
		}
	}
	return health, err
}
