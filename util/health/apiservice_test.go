package health

import (
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestAPIService(t *testing.T) {
	assertAppHealth(t, "./testdata/apiservice-v1-true.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1-false.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-true.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-false.yaml", appv1.HealthStatusProgressing)
}
