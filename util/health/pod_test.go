package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestPod(t *testing.T) {
	assertAppHealth(t, "./testdata/pod-pending.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-not-ready.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-crashloop.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-imagepullbackoff.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-error.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-running-restart-always.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-running-restart-never.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-restart-onfailure.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-succeeded.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-deletion.yaml", appv1.HealthStatusProgressing)
}
