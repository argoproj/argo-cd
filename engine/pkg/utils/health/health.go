package health

import (
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"

	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
)

type HealthStatusCode string

const (
	HealthStatusUnknown     HealthStatusCode = "Unknown"
	HealthStatusProgressing HealthStatusCode = "Progressing"
	HealthStatusHealthy     HealthStatusCode = "Healthy"
	HealthStatusSuspended   HealthStatusCode = "Suspended"
	HealthStatusDegraded    HealthStatusCode = "Degraded"
	HealthStatusMissing     HealthStatusCode = "Missing"
)

type HealthOverride interface {
	GetResourceHealth(obj *unstructured.Unstructured) (*HealthStatus, error)
}

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
	case "apps", "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind:
			health, err = getDeploymentHealth(obj)
		case kube.IngressKind:
			health, err = getIngressHealth(obj)
		case kube.StatefulSetKind:
			health, err = getStatefulSetHealth(obj)
		case kube.ReplicaSetKind:
			health, err = getReplicaSetHealth(obj)
		case kube.DaemonSetKind:
			health, err = getDaemonSetHealth(obj)
		}
	case "argoproj.io":
		switch gvk.Kind {
		case "Application":
			health, err = getApplicationHealth(obj)
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
	}
	if err != nil {
		health = &HealthStatus{
			Status:  HealthStatusUnknown,
			Message: err.Error(),
		}
	}
	return health, err
}

func getPVCHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	pvc := &coreV1.PersistentVolumeClaim{}
	err := scheme.Scheme.Convert(obj, pvc, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, pvc, err)
	}
	switch pvc.Status.Phase {
	case coreV1.ClaimLost:
		return &HealthStatus{Status: HealthStatusDegraded}, nil
	case coreV1.ClaimPending:
		return &HealthStatus{Status: HealthStatusProgressing}, nil
	case coreV1.ClaimBound:
		return &HealthStatus{Status: HealthStatusHealthy}, nil
	default:
		return &HealthStatus{Status: HealthStatusUnknown}, nil
	}
}

func getIngressHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	ingress := &extv1beta1.Ingress{}
	err := scheme.Scheme.Convert(obj, ingress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, ingress, err)
	}
	health := HealthStatus{}
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		health.Status = HealthStatusHealthy
	} else {
		health.Status = HealthStatusProgressing
	}
	return &health, nil
}

func getServiceHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	service := &coreV1.Service{}
	err := scheme.Scheme.Convert(obj, service, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, service, err)
	}
	health := HealthStatus{Status: HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			health.Status = HealthStatusHealthy
		} else {
			health.Status = HealthStatusProgressing
		}
	}
	return &health, nil
}

func getDeploymentHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	deployment := &appsv1.Deployment{}
	err := scheme.Scheme.Convert(obj, deployment, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, deployment, err)
	}
	if deployment.Spec.Paused {
		return &HealthStatus{
			Status:  HealthStatusSuspended,
			Message: "Deployment is paused",
		}, nil
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L80
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, v1.DeploymentProgressing)
		if cond != nil && cond.Reason == "ProgressDeadlineExceeded" {
			return &HealthStatus{
				Status:  HealthStatusDegraded,
				Message: fmt.Sprintf("Deployment %q exceeded its progress deadline", obj.GetName()),
			}, nil
		} else if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
			}, nil
		} else if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...", deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
			}, nil
		} else if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed deployment generation less then desired generation",
		}, nil
	}

	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func init() {
	_ = apiregistrationv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	_ = apiregistrationv1beta1.SchemeBuilder.AddToScheme(scheme.Scheme)
}

func getApplicationHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	status, _, _ := unstructured.NestedString(obj.Object, "status", "health", "status")
	message, _, _ := unstructured.NestedString(obj.Object, "status", "health", "message")
	return &HealthStatus{Status: HealthStatusCode(status), Message: message}, nil
}

func getDaemonSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	daemon := &appsv1.DaemonSet{}
	err := scheme.Scheme.Convert(obj, daemon, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, daemon, err)
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L110
	if daemon.Generation <= daemon.Status.ObservedGeneration {
		if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...", daemon.Name, daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
		if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...", daemon.Name, daemon.Status.NumberAvailable, daemon.Status.DesiredNumberScheduled),
			}, nil
		}

	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed daemon set generation less then desired generation",
		}, nil
	}
	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func getStatefulSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	sts := &appsv1.StatefulSet{}
	err := scheme.Scheme.Convert(obj, sts, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, sts, err)
	}
	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L131
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for statefulset spec update to be observed...",
		}, nil
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: fmt.Sprintf("Waiting for %d pods to be ready...", *sts.Spec.Replicas-sts.Status.ReadyReplicas),
		}, nil
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &HealthStatus{
					Status: HealthStatusProgressing,
					Message: fmt.Sprintf("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...",
						sts.Status.UpdatedReplicas, (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition)),
				}, nil
			}
		}
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: fmt.Sprintf("partitioned roll out complete: %d new pods have been updated...", sts.Status.UpdatedReplicas),
		}, nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: fmt.Sprintf("waiting for statefulset rolling update to complete %d pods at revision %s...", sts.Status.UpdatedReplicas, sts.Status.UpdateRevision),
		}, nil
	}
	return &HealthStatus{
		Status:  HealthStatusHealthy,
		Message: fmt.Sprintf("statefulset rolling update complete %d pods at revision %s...", sts.Status.CurrentReplicas, sts.Status.CurrentRevision),
	}, nil
}

func getReplicaSetHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	replicaSet := &appsv1.ReplicaSet{}
	err := scheme.Scheme.Convert(obj, replicaSet, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, replicaSet, err)
	}
	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getReplicaSetCondition(replicaSet.Status, v1.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == coreV1.ConditionTrue {
			return &HealthStatus{
				Status:  HealthStatusDegraded,
				Message: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &HealthStatus{
		Status: HealthStatusHealthy,
	}, nil
}

func getDeploymentCondition(status v1.DeploymentStatus, condType v1.DeploymentConditionType) *v1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func getReplicaSetCondition(status v1.ReplicaSetStatus, condType v1.ReplicaSetConditionType) *v1.ReplicaSetCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

func getJobHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	job := &batchv1.Job{}
	err := scheme.Scheme.Convert(obj, job, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, job, err)
	}
	failed := false
	var failMsg string
	complete := false
	var message string
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobFailed:
			failed = true
			complete = true
			failMsg = condition.Message
		case batchv1.JobComplete:
			complete = true
			message = condition.Message
		}
	}
	if !complete {
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: message,
		}, nil
	} else if failed {
		return &HealthStatus{
			Status:  HealthStatusDegraded,
			Message: failMsg,
		}, nil
	} else {
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: message,
		}, nil
	}
}

func getPodHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	pod := &coreV1.Pod{}
	err := scheme.Scheme.Convert(obj, pod, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, pod, err)
	}

	// This logic cannot be applied when the pod.Spec.RestartPolicy is: coreV1.RestartPolicyOnFailure,
	// coreV1.RestartPolicyNever, otherwise it breaks the resource hook logic.
	// The issue is, if we mark a pod with ImagePullBackOff as Degraded, and the pod is used as a resource hook,
	// then we will prematurely fail the PreSync/PostSync hook. Meanwhile, when that error condition is resolved
	// (e.g. the image is available), the resource hook pod will unexpectedly be executed even though the sync has
	// completed.
	if pod.Spec.RestartPolicy == coreV1.RestartPolicyAlways {
		var status HealthStatusCode
		var messages []string

		for _, containerStatus := range pod.Status.ContainerStatuses {
			waiting := containerStatus.State.Waiting
			// Article listing common container errors: https://medium.com/kokster/debugging-crashloopbackoffs-with-init-containers-26f79e9fb5bf
			if waiting != nil && (strings.HasPrefix(waiting.Reason, "Err") || strings.HasSuffix(waiting.Reason, "Error") || strings.HasSuffix(waiting.Reason, "BackOff")) {
				status = HealthStatusDegraded
				messages = append(messages, waiting.Message)
			}
		}

		if status != "" {
			return &HealthStatus{
				Status:  status,
				Message: strings.Join(messages, ", "),
			}, nil
		}
	}

	getFailMessage := func(ctr *coreV1.ContainerStatus) string {
		if ctr.State.Terminated != nil {
			if ctr.State.Terminated.Message != "" {
				return ctr.State.Terminated.Message
			}
			if ctr.State.Terminated.Reason == "OOMKilled" {
				return ctr.State.Terminated.Reason
			}
			if ctr.State.Terminated.ExitCode != 0 {
				return fmt.Sprintf("container %q failed with exit code %d", ctr.Name, ctr.State.Terminated.ExitCode)
			}
		}
		return ""
	}

	switch pod.Status.Phase {
	case coreV1.PodPending:
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: pod.Status.Message,
		}, nil
	case coreV1.PodSucceeded:
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: pod.Status.Message,
		}, nil
	case coreV1.PodFailed:
		if pod.Status.Message != "" {
			// Pod has a nice error message. Use that.
			return &HealthStatus{Status: HealthStatusDegraded, Message: pod.Status.Message}, nil
		}
		for _, ctr := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if msg := getFailMessage(&ctr); msg != "" {
				return &HealthStatus{Status: HealthStatusDegraded, Message: msg}, nil
			}
		}

		return &HealthStatus{Status: HealthStatusDegraded, Message: ""}, nil
	case coreV1.PodRunning:
		switch pod.Spec.RestartPolicy {
		case coreV1.RestartPolicyAlways:
			// if pod is ready, it is automatically healthy
			if podutil.IsPodReady(pod) {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: pod.Status.Message,
				}, nil
			}
			// if it's not ready, check to see if any container terminated, if so, it's degraded
			for _, ctrStatus := range pod.Status.ContainerStatuses {
				if ctrStatus.LastTerminationState.Terminated != nil {
					return &HealthStatus{
						Status:  HealthStatusDegraded,
						Message: pod.Status.Message,
					}, nil
				}
			}
			// otherwise we are progressing towards a ready state
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		case coreV1.RestartPolicyOnFailure, coreV1.RestartPolicyNever:
			// pods set with a restart policy of OnFailure or Never, have a finite life.
			// These pods are typically resource hooks. Thus, we consider these as Progressing
			// instead of healthy.
			return &HealthStatus{
				Status:  HealthStatusProgressing,
				Message: pod.Status.Message,
			}, nil
		}
	}
	return &HealthStatus{
		Status:  HealthStatusUnknown,
		Message: pod.Status.Message,
	}, nil
}

func getAPIServiceHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	apiservice := &apiregistrationv1.APIService{}
	err := scheme.Scheme.Convert(obj, apiservice, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to convert %T to %T: %v", obj, apiservice, err)
	}

	for _, c := range apiservice.Status.Conditions {
		switch c.Type {
		case apiregistrationv1.Available:
			if c.Status == apiregistrationv1.ConditionTrue {
				return &HealthStatus{
					Status:  HealthStatusHealthy,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			} else {
				return &HealthStatus{
					Status:  HealthStatusProgressing,
					Message: fmt.Sprintf("%s: %s", c.Reason, c.Message),
				}, nil
			}
		}
	}
	return &HealthStatus{
		Status:  HealthStatusProgressing,
		Message: "Waiting to be processed",
	}, nil
}

type NodePhase string

// Workflow and node statuses
const (
	NodePending   NodePhase = "Pending"
	NodeRunning   NodePhase = "Running"
	NodeSucceeded NodePhase = "Succeeded"
	NodeSkipped   NodePhase = "Skipped"
	NodeFailed    NodePhase = "Failed"
	NodeError     NodePhase = "Error"
)

// An agnostic workflow object only considers Status.Phase and Status.Message. It is agnostic to the API version or any
// other fields.
type ArgoWorkflow struct {
	Status struct {
		Phase   NodePhase
		Message string
	}
}

func getArgoWorkflowHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	var wf ArgoWorkflow
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &wf)
	if err != nil {
		return nil, err
	}
	switch wf.Status.Phase {
	case NodePending, NodeRunning:
		return &HealthStatus{Status: HealthStatusProgressing, Message: wf.Status.Message}, nil
	case NodeSucceeded:
		return &HealthStatus{Status: HealthStatusHealthy, Message: wf.Status.Message}, nil
	case NodeFailed, NodeError:
		return &HealthStatus{Status: HealthStatusDegraded, Message: wf.Status.Message}, nil
	}
	return &HealthStatus{Status: HealthStatusHealthy, Message: wf.Status.Message}, nil
}
