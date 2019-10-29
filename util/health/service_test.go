package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestServiceHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/svc-clusterip.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer-unassigned.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/svc-loadbalancer-nonemptylist.yaml", appv1.HealthStatusHealthy)
}
