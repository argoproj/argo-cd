package health

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
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
	HealthStatusMissing,
	HealthStatusDegraded,
	HealthStatusUnknown,
}

// IsWorse returns whether or not the new health status code is a worse condition than the current
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
	if obj.GetDeletionTimestamp() != nil && !hook.HasHookFinalizer(obj) {
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
			return health, fmt.Errorf("failed to get resource health for %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
		if health != nil {
			return health, nil
		}
	}

	if healthCheck := GetHealthCheckFunc(obj.GroupVersionKind()); healthCheck != nil {
		if health, err = healthCheck(obj); err != nil {
			health = &HealthStatus{
				Status:  HealthStatusUnknown,
				Message: err.Error(),
			}
		}
	}
	return health, err
}

var builtinHealthChecks = map[schema.GroupKind]func(obj *unstructured.Unstructured) (*HealthStatus, error){
	{Group: "apps", Kind: kube.DeploymentKind}:                     getDeploymentHealth,
	{Group: "apps", Kind: kube.StatefulSetKind}:                    getStatefulSetHealth,
	{Group: "apps", Kind: kube.ReplicaSetKind}:                     getReplicaSetHealth,
	{Group: "apps", Kind: kube.DaemonSetKind}:                      getDaemonSetHealth,
	{Group: "extensions", Kind: kube.IngressKind}:                  getIngressHealth,
	{Group: "argoproj.io", Kind: "Workflow"}:                       getArgoWorkflowHealth,
	{Group: "apiregistration.k8s.io", Kind: kube.APIServiceKind}:   getAPIServiceHealth,
	{Group: "networking.k8s.io", Kind: kube.IngressKind}:           getIngressHealth,
	{Group: "", Kind: kube.ServiceKind}:                            getServiceHealth,
	{Group: "", Kind: kube.PersistentVolumeClaimKind}:              getPVCHealth,
	{Group: "", Kind: kube.PodKind}:                                getPodHealth,
	{Group: "batch", Kind: kube.JobKind}:                           getJobHealth,
	{Group: "autoscaling", Kind: kube.HorizontalPodAutoscalerKind}: getHPAHealth,
}

// GetBuiltinHealthCheckGVKs returns a copy of the GroupVersionKind list for built-in Go health checks
func GetBuiltinHealthCheckGVKs() []schema.GroupVersionKind {
	gvks := make([]schema.GroupVersionKind, 0, len(builtinHealthChecks))
	for gk := range builtinHealthChecks {
		gvks = append(gvks, schema.GroupVersionKind{Group: gk.Group, Kind: gk.Kind})
	}
	return gvks
}

// GetHealthCheckFunc returns built-in health check function or nil if health check is not supported
func GetHealthCheckFunc(gvk schema.GroupVersionKind) func(obj *unstructured.Unstructured) (*HealthStatus, error) {
	return builtinHealthChecks[gvk.GroupKind()]
}
