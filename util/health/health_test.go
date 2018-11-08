package health

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus appv1.HealthStatusCode) {
	yamlBytes, err := ioutil.ReadFile(yamlPath)
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := GetAppHealth(kube.KubectlCmd{}, &obj)
	assert.Nil(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, expectedStatus, health.Status)
}

func TestDeploymentHealth(t *testing.T) {
	assertAppHealth(t, "../kube/testdata/nginx.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/deployment-progressing.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/deployment-degraded.yaml", appv1.HealthStatusDegraded)
}

func TestStatefulSetHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/statefulset.yaml", appv1.HealthStatusHealthy)
}

func TestPVCHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/pvc-bound.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pvc-pending.yaml", appv1.HealthStatusProgressing)
}

func TestServiceHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/svc-clusterip.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer-unassigned.yaml", appv1.HealthStatusProgressing)
}

func TestIngressHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/ingress.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/ingress-unassigned.yaml", appv1.HealthStatusProgressing)
}

func TestCRD(t *testing.T) {
	// This ensures we do not try to compare only based on "Kind"
	assertAppHealth(t, "./testdata/knative-service.yaml", appv1.HealthStatusHealthy)
}
