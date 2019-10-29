package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestIngressHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/ingress.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/ingress-unassigned.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/ingress-nonemptylist.yaml", appv1.HealthStatusHealthy)
}
