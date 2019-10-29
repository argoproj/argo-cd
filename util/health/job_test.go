package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestJob(t *testing.T) {
	assertAppHealth(t, "./testdata/job-running.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/job-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/job-succeeded.yaml", appv1.HealthStatusHealthy)
}
