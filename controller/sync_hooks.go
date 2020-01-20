package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/health"
)

// getOperationPhase returns a hook status from an _live_ unstructured object
func (sc *syncContext) getOperationPhase(hook *unstructured.Unstructured) (v1alpha1.OperationPhase, string, error) {
	phase := v1alpha1.OperationSucceeded
	message := fmt.Sprintf("%s created", hook.GetName())

	resHealth, err := health.GetResourceHealth(hook, sc.resourceOverrides)
	if err != nil {
		return "", "", err
	}
	if resHealth != nil {
		switch resHealth.Status {
		case v1alpha1.HealthStatusUnknown, v1alpha1.HealthStatusDegraded:
			phase = v1alpha1.OperationFailed
			message = resHealth.Message
		case v1alpha1.HealthStatusProgressing, v1alpha1.HealthStatusSuspended:
			phase = v1alpha1.OperationRunning
			message = resHealth.Message
		case v1alpha1.HealthStatusHealthy:
			phase = v1alpha1.OperationSucceeded
			message = resHealth.Message
		}
	}
	return phase, message, nil
}
