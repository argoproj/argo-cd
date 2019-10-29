package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestPVCHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/pvc-bound.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pvc-pending.yaml", appv1.HealthStatusProgressing)
}
