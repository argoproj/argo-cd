package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestDeploymentHealth(t *testing.T) {
	assertAppHealth(t, "../kube/testdata/nginx.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/deployment-progressing.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/deployment-suspended.yaml", appv1.HealthStatusSuspended)
	assertAppHealth(t, "./testdata/deployment-degraded.yaml", appv1.HealthStatusDegraded)
}
