package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestGenericHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/status-pending.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/status-running.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/status-successful.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/status-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/status-error.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/status-inconclusive.yaml", appv1.HealthStatusSuspended)
}
