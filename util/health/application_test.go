package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestApplication(t *testing.T) {
	assertAppHealth(t, "./testdata/application-healthy.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/application-degraded.yaml", appv1.HealthStatusDegraded)
}
