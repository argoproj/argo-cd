package health

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func getJobHealth(obj *unstructured.Unstructured) (*appv1.HealthStatus, error) {
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
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusProgressing,
			Message: message,
		}, nil
	} else if failed {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusDegraded,
			Message: failMsg,
		}, nil
	} else {
		return &appv1.HealthStatus{
			Status:  appv1.HealthStatusHealthy,
			Message: message,
		}, nil
	}
}
