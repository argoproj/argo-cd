package controller

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/pkg/utils/health"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/lua"
)

// getOperationPhase returns a hook status from an _live_ unstructured object
func (sc *syncContext) getOperationPhase(hook *unstructured.Unstructured) (v1alpha1.OperationPhase, string, error) {
	phase := v1alpha1.OperationSucceeded
	message := fmt.Sprintf("%s created", hook.GetName())

	resHealth, err := health.GetResourceHealth(hook, lua.ResourceHealthOverrides(sc.resourceOverrides))
	if err != nil {
		return "", "", err
	}
	if resHealth != nil {
		switch resHealth.Status {
		case health.HealthStatusUnknown, health.HealthStatusDegraded:
			phase = v1alpha1.OperationFailed
			message = resHealth.Message
		case health.HealthStatusProgressing, health.HealthStatusSuspended:
			phase = v1alpha1.OperationRunning
			message = resHealth.Message
		case health.HealthStatusHealthy:
			phase = v1alpha1.OperationSucceeded
			message = resHealth.Message
		}
	}
	return phase, message, nil
}
