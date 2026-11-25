package health

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

func getJobHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case batchv1.SchemeGroupVersion.WithKind(kube.JobKind):
		var job batchv1.Job
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &job)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Job to typed: %w", err)
		}
		return getBatchv1JobHealth(&job)
	default:
		return nil, fmt.Errorf("unsupported Job GVK: %s", gvk)
	}
}

func getBatchv1JobHealth(job *batchv1.Job) (*HealthStatus, error) {
	failed := false
	var failMsg string
	complete := false
	var message string
	isSuspended := false
	for _, condition := range job.Status.Conditions {
		switch condition.Type {
		case batchv1.JobFailed:
			failed = true
			complete = true
			failMsg = condition.Message
		case batchv1.JobComplete:
			complete = true
			message = condition.Message
		case batchv1.JobSuspended:
			// A resumed Job keeps this condition with status False while it runs.
			if condition.Status == corev1.ConditionTrue {
				complete = true
				message = condition.Message
				isSuspended = true
			}
		}
	}
	switch {
	case !complete:
		return &HealthStatus{
			Status:  HealthStatusProgressing,
			Message: message,
		}, nil
	case failed:
		return &HealthStatus{
			Status:  HealthStatusDegraded,
			Message: failMsg,
		}, nil
	case isSuspended:
		return &HealthStatus{
			Status:  HealthStatusSuspended,
			Message: message,
		}, nil
	default:
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: message,
		}, nil
	}
}
