package health

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func getJobHealth(obj *unstructured.Unstructured) (*HealthStatus, error) {
	gvk := obj.GroupVersionKind()
	switch gvk {
	case batchv1.SchemeGroupVersion.WithKind(kube.JobKind):
		var job batchv1.Job
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &job)
		if err != nil {
			return nil, fmt.Errorf("failed to convert unstructured Job to typed: %v", err)
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
			complete = true
			message = condition.Message
			if condition.Status == corev1.ConditionTrue {
				isSuspended = true
			}
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
	} else if isSuspended {
		return &HealthStatus{
			Status:  HealthStatusSuspended,
			Message: failMsg,
		}, nil
	} else {
		return &HealthStatus{
			Status:  HealthStatusHealthy,
			Message: message,
		}, nil
	}
}
