package health

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus HealthStatusCode) {
	health := getHealthStatus(yamlPath, t)
	assert.NotNil(t, health)
	assert.Equal(t, expectedStatus, health.Status)
}

func getHealthStatus(yamlPath string, t *testing.T) *HealthStatus {
	yamlBytes, err := ioutil.ReadFile(yamlPath)
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := GetResourceHealth(&obj, nil)
	assert.Nil(t, err)
	return health
}

func TestDeploymentHealth(t *testing.T) {
	assertAppHealth(t, "../kube/testdata/nginx.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/deployment-progressing.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/deployment-suspended.yaml", HealthStatusSuspended)
	assertAppHealth(t, "./testdata/deployment-degraded.yaml", HealthStatusDegraded)
}

func TestStatefulSetHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/statefulset.yaml", HealthStatusHealthy)
}

func TestPVCHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/pvc-bound.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pvc-pending.yaml", HealthStatusProgressing)
}

func TestServiceHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/svc-clusterip.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/svc-loadbalancer-unassigned.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/svc-loadbalancer-nonemptylist.yaml", HealthStatusHealthy)
}

func TestIngressHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/ingress.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/ingress-unassigned.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/ingress-nonemptylist.yaml", HealthStatusHealthy)
}

func TestCRD(t *testing.T) {
	assert.Nil(t, getHealthStatus("./testdata/knative-service.yaml", t))
}

func TestJob(t *testing.T) {
	assertAppHealth(t, "./testdata/job-running.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/job-failed.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/job-succeeded.yaml", HealthStatusHealthy)
}

func TestPod(t *testing.T) {
	assertAppHealth(t, "./testdata/pod-pending.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-not-ready.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-crashloop.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-imagepullbackoff.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-error.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-running-restart-always.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-running-restart-never.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-restart-onfailure.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-failed.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-succeeded.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-deletion.yaml", HealthStatusProgressing)
}

func TestApplication(t *testing.T) {
	assertAppHealth(t, "./testdata/application-healthy.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/application-degraded.yaml", HealthStatusDegraded)
}

func TestAPIService(t *testing.T) {
	assertAppHealth(t, "./testdata/apiservice-v1-true.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1-false.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-true.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-false.yaml", HealthStatusProgressing)
}

func TestGetArgoWorkflowHealth(t *testing.T) {
	sampleWorkflow := unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"entrypoint":    "sampleEntryPoint",
			"extraneousKey": "we are agnostic to extraneous keys",
		},
		"status": map[string]interface{}{
			"phase":   "Running",
			"message": "This node is running",
		},
	},
	}

	health, err := getArgoWorkflowHealth(&sampleWorkflow)
	assert.NoError(t, err)
	assert.Equal(t, HealthStatusProgressing, health.Status)
	assert.Equal(t, "This node is running", health.Message)

	sampleWorkflow = unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"entrypoint":    "sampleEntryPoint",
			"extraneousKey": "we are agnostic to extraneous keys",
		},
		"status": map[string]interface{}{
			"phase":   "Succeeded",
			"message": "This node is has succeeded",
		},
	},
	}

	health, err = getArgoWorkflowHealth(&sampleWorkflow)
	assert.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Equal(t, "This node is has succeeded", health.Message)
}
