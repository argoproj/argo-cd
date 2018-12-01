package health

import (
	"fmt"

	"k8s.io/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/apis/apps"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	hookutil "github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
)

// SetApplicationHealth updates the health statuses of all resources performed in the comparison
func SetApplicationHealth(kubectl kube.Kubectl, comparisonResult *appv1.ComparisonResult, liveObjs []*unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var savedErr error
	appHealth := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if comparisonResult.Status == appv1.ComparisonStatusUnknown {
		appHealth.Status = appv1.HealthStatusUnknown
	}
	for i, liveObj := range liveObjs {
		var resHealth *appv1.HealthStatus
		var err error
		if liveObj == nil {
			resHealth = &appv1.HealthStatus{Status: appv1.HealthStatusMissing}
		} else {
			resHealth, err = GetResourceHealth(kubectl, liveObj)
			if err != nil && savedErr == nil {
				savedErr = err
			}
		}
		comparisonResult.Resources[i].Health = *resHealth
		// Don't allow resource hooks to affect health status
		isHook := liveObj != nil && hookutil.IsHook(liveObj)
		if !isHook && IsWorse(appHealth.Status, resHealth.Status) {
			appHealth.Status = resHealth.Status
		}
	}
	return &appHealth, savedErr
}

// GetResourceHealth returns the health of a k8s resource
func GetResourceHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var err error
	var health *appv1.HealthStatus

	gvk := obj.GroupVersionKind()
	switch gvk.Group {
	case "apps", "extensions":
		switch gvk.Kind {
		case kube.DeploymentKind:
			health, err = getDeploymentHealth(kubectl, obj)
		case kube.IngressKind:
			health, err = getIngressHealth(kubectl, obj)
		case kube.StatefulSetKind:
			health, err = getStatefulSetHealth(kubectl, obj)
		case kube.ReplicaSetKind:
			health, err = getReplicaSetHealth(kubectl, obj)
		case kube.DaemonSetKind:
			health, err = getDaemonSetHealth(kubectl, obj)
		}
	case "":
		switch gvk.Kind {
		case kube.ServiceKind:
			health, err = getServiceHealth(kubectl, obj)
		case kube.PersistentVolumeClaimKind:
			health, err = getPVCHealth(kubectl, obj)
		case kube.PodKind:
			health, err = getPodHealth(kubectl, obj)
		}
	case "batch":
		switch gvk.Kind {
		case kube.JobKind:
			health, err = getJobHealth(kubectl, obj)
		}
	}
	if err != nil {
		health = &appv1.HealthStatus{
			Status:        appv1.HealthStatusUnknown,
			StatusDetails: err.Error(),
		}
	} else if health == nil {
		health = &appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	}
	return health, err
}

// healthOrder is a list of health codes in order of most healthy to least healthy
var healthOrder = []appv1.HealthStatusCode{
	appv1.HealthStatusHealthy,
	appv1.HealthStatusProgressing,
	appv1.HealthStatusDegraded,
	appv1.HealthStatusMissing,
	appv1.HealthStatusUnknown,
}

// IsWorse returns whether or not the new health status code is a worser condition than the current
func IsWorse(current, new appv1.HealthStatusCode) bool {
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

func getPVCHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var pvc coreV1.PersistentVolumeClaim
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pvc)
	if err != nil {
		return nil, err
	}

	switch pvc.Status.Phase {
	case coreV1.ClaimLost:
		return &appv1.HealthStatus{Status: appv1.HealthStatusDegraded}, nil
	case coreV1.ClaimPending:
		return &appv1.HealthStatus{Status: appv1.HealthStatusProgressing}, nil
	case coreV1.ClaimBound:
		return &appv1.HealthStatus{Status: appv1.HealthStatusHealthy}, nil
	default:
		return &appv1.HealthStatus{Status: appv1.HealthStatusUnknown}, nil
	}
}

func getIngressHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kubectl.ConvertToVersion(obj, "extensions", "v1beta1")
	if err != nil {
		return nil, err
	}
	var ingress extv1beta1.Ingress
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ingress)
	if err != nil {
		return nil, err
	}

	health := appv1.HealthStatus{Status: appv1.HealthStatusProgressing}
	for _, ingress := range ingress.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" || ingress.IP != "" {
			health.Status = appv1.HealthStatusHealthy
			break
		}
	}
	return &health, nil
}

func getServiceHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var service coreV1.Service
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &service)
	if err != nil {
		return nil, err
	}
	health := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		health.Status = appv1.HealthStatusProgressing
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" || ingress.IP != "" {
				health.Status = appv1.HealthStatusHealthy
				break
			}
		}
	}
	return &health, nil
}

func getDeploymentHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kubectl.ConvertToVersion(obj, "apps", "v1")
	if err != nil {
		return nil, err
	}
	var deployment appsv1.Deployment
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &deployment)
	if err != nil {
		return nil, err
	}

	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L80
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := getDeploymentCondition(deployment.Status, v1.DeploymentProgressing)
		if cond != nil && cond.Reason == "ProgressDeadlineExceeded" {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusDegraded,
				StatusDetails: fmt.Sprintf("Deployment %q exceeded its progress deadline", obj.GetName()),
			}, nil
		} else if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...", deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas),
			}, nil
		} else if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...", deployment.Status.Replicas-deployment.Status.UpdatedReplicas),
			}, nil
		} else if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for rollout to finish: observed deployment generation less then desired generation",
		}, nil
	}

	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}

func getDaemonSetHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kubectl.ConvertToVersion(obj, "apps", "v1")
	if err != nil {
		return nil, err
	}
	var daemon appsv1.DaemonSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &daemon)

	if err != nil {
		return nil, err
	}

	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L110
	if daemon.Generation <= daemon.Status.ObservedGeneration {
		if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...", daemon.Name, daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled),
			}, nil
		}
		if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...", daemon.Name, daemon.Status.NumberAvailable, daemon.Status.DesiredNumberScheduled),
			}, nil
		}

	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for rollout to finish: observed daemon set generation less then desired generation",
		}, nil
	}
	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
	}, nil
}

func getStatefulSetHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kubectl.ConvertToVersion(obj, "apps", "v1")
	if err != nil {
		return nil, err
	}
	var sts appsv1.StatefulSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &sts)
	if err != nil {
		return nil, err
	}

	// Borrowed at kubernetes/kubectl/rollout_status.go https://github.com/kubernetes/kubernetes/blob/5232ad4a00ec93942d0b2c6359ee6cd1201b46bc/pkg/kubectl/rollout_status.go#L131
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for statefulset spec update to be observed...",
		}, nil
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: fmt.Sprintf("Waiting for %d pods to be ready...", *sts.Spec.Replicas-sts.Status.ReadyReplicas),
		}, nil
	}
	if sts.Spec.UpdateStrategy.Type == apps.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return &appv1.HealthStatus{
					Status: appv1.HealthStatusProgressing,
					StatusDetails: fmt.Sprintf("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...",
						sts.Status.UpdatedReplicas, (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition)),
				}, nil
			}
		}
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusHealthy,
			StatusDetails: fmt.Sprintf("partitioned roll out complete: %d new pods have been updated...", sts.Status.UpdatedReplicas),
		}, nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: fmt.Sprintf("waiting for statefulset rolling update to complete %d pods at revision %s...", sts.Status.UpdatedReplicas, sts.Status.UpdateRevision),
		}, nil
	}
	return &appv1.HealthStatus{
		Status:        appv1.HealthStatusHealthy,
		StatusDetails: fmt.Sprintf("statefulset rolling update complete %d pods at revision %s...", sts.Status.CurrentReplicas, sts.Status.CurrentRevision),
	}, nil
}

func getReplicaSetHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	obj, err := kubectl.ConvertToVersion(obj, "apps", "v1")
	if err != nil {
		return nil, err
	}
	var replicaSet appsv1.ReplicaSet
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &replicaSet)

	if err != nil {
		return nil, err
	}

	if replicaSet.Generation <= replicaSet.Status.ObservedGeneration {
		cond := getReplicaSetCondition(replicaSet.Status, v1.ReplicaSetReplicaFailure)
		if cond != nil && cond.Status == coreV1.ConditionTrue {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusDegraded,
				StatusDetails: cond.Message,
			}, nil
		} else if replicaSet.Spec.Replicas != nil && replicaSet.Status.AvailableReplicas < *replicaSet.Spec.Replicas {
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas are available...", replicaSet.Status.AvailableReplicas, *replicaSet.Spec.Replicas),
			}, nil
		}
	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: "Waiting for rollout to finish: observed replica set generation less then desired generation",
		}, nil
	}

	return &appv1.HealthStatus{
		Status: appv1.HealthStatusHealthy,
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

func getJobHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var job batchv1.Job
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &job)
	if err != nil {
		return nil, err
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
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: message,
		}, nil
	} else if failed {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusDegraded,
			StatusDetails: failMsg,
		}, nil
	} else {
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusHealthy,
			StatusDetails: message,
		}, nil
	}
}

func getPodHealth(kubectl kube.Kubectl, obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
	var pod coreV1.Pod
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &pod)
	if err != nil {
		return nil, err
	}
	switch pod.Status.Phase {
	case coreV1.PodPending:
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusProgressing,
			StatusDetails: pod.Status.Message,
		}, nil
	case coreV1.PodSucceeded:
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusHealthy,
			StatusDetails: pod.Status.Message,
		}, nil
	case coreV1.PodFailed:
		return &appv1.HealthStatus{
			Status:        appv1.HealthStatusDegraded,
			StatusDetails: pod.Status.Message,
		}, nil
	case coreV1.PodRunning:
		switch pod.Spec.RestartPolicy {
		case coreV1.RestartPolicyAlways:
			// if pod is ready, it is automatically healthy
			if podutil.IsPodReady(&pod) {
				return &appv1.HealthStatus{
					Status:        appv1.HealthStatusHealthy,
					StatusDetails: pod.Status.Message,
				}, nil
			}
			// if it's not ready, check to see if any container terminated, if so, it's degraded
			for _, ctrStatus := range pod.Status.ContainerStatuses {
				if ctrStatus.LastTerminationState.Terminated != nil {
					return &appv1.HealthStatus{
						Status:        appv1.HealthStatusDegraded,
						StatusDetails: pod.Status.Message,
					}, nil
				}
			}
			// otherwise we are progressing towards a ready state
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: pod.Status.Message,
			}, nil
		case coreV1.RestartPolicyOnFailure, coreV1.RestartPolicyNever:
			// pods set with a restart policy of OnFailure or Never, have a finite life.
			// These pods are typically resource hooks. Thus, we consider these as Progressing
			// instead of healthy.
			return &appv1.HealthStatus{
				Status:        appv1.HealthStatusProgressing,
				StatusDetails: pod.Status.Message,
			}, nil
		}
	}
	return &appv1.HealthStatus{
		Status:        appv1.HealthStatusUnknown,
		StatusDetails: pod.Status.Message,
	}, nil
}
