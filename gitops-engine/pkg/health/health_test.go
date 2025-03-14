/*
Package provides functionality that allows assessing the health state of a Kubernetes resource.
*/

package health

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus HealthStatusCode) {
	t.Helper()
	health := getHealthStatus(t, yamlPath)
	assert.NotNil(t, health)
	assert.Equal(t, expectedStatus, health.Status)
}

func getHealthStatus(t *testing.T, yamlPath string) *HealthStatus {
	t.Helper()
	yamlBytes, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	health, err := GetResourceHealth(&obj, nil)
	require.NoError(t, err)
	return health
}

func TestDeploymentHealth(t *testing.T) {
	assertAppHealth(t, "../utils/kube/testdata/nginx.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/deployment-progressing.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/deployment-suspended.yaml", HealthStatusSuspended)
	assertAppHealth(t, "./testdata/deployment-degraded.yaml", HealthStatusDegraded)
}

func TestStatefulSetHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/statefulset.yaml", HealthStatusHealthy)
}

func TestStatefulSetOnDeleteHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/statefulset-ondelete.yaml", HealthStatusHealthy)
}

func TestDaemonSetOnDeleteHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/daemonset-ondelete.yaml", HealthStatusHealthy)
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
	assert.Nil(t, getHealthStatus(t, "./testdata/knative-service.yaml"))
}

func TestJob(t *testing.T) {
	assertAppHealth(t, "./testdata/job-running.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/job-failed.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/job-succeeded.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/job-suspended.yaml", HealthStatusSuspended)
}

func TestHPA(t *testing.T) {
	assertAppHealth(t, "./testdata/hpa-v2-healthy.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v2-degraded.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/hpa-v2-progressing.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/hpa-v2beta2-healthy.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v2beta1-healthy-disabled.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v2beta1-healthy.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v1-degraded.yaml", HealthStatusDegraded)
	assertAppHealth(t, "./testdata/hpa-v1-healthy.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v1-healthy-toofew.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/hpa-v1-progressing.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/hpa-v1-progressing-with-no-annotations.yaml", HealthStatusProgressing)
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
	assert.Nil(t, getHealthStatus(t, "./testdata/application-healthy.yaml"))
	assert.Nil(t, getHealthStatus(t, "./testdata/application-degraded.yaml"))
}

func TestAPIService(t *testing.T) {
	assertAppHealth(t, "./testdata/apiservice-v1-true.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1-false.yaml", HealthStatusProgressing)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-true.yaml", HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-false.yaml", HealthStatusProgressing)
}

func TestGetArgoWorkflowHealth(t *testing.T) {
	sampleWorkflow := unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"entrypoint":    "sampleEntryPoint",
				"extraneousKey": "we are agnostic to extraneous keys",
			},
			"status": map[string]any{
				"phase":   "Running",
				"message": "This node is running",
			},
		},
	}

	health, err := getArgoWorkflowHealth(&sampleWorkflow)
	require.NoError(t, err)
	assert.Equal(t, HealthStatusProgressing, health.Status)
	assert.Equal(t, "This node is running", health.Message)

	sampleWorkflow = unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"entrypoint":    "sampleEntryPoint",
				"extraneousKey": "we are agnostic to extraneous keys",
			},
			"status": map[string]any{
				"phase":   "Succeeded",
				"message": "This node is has succeeded",
			},
		},
	}

	health, err = getArgoWorkflowHealth(&sampleWorkflow)
	require.NoError(t, err)
	assert.Equal(t, HealthStatusHealthy, health.Status)
	assert.Equal(t, "This node is has succeeded", health.Message)

	sampleWorkflow = unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"entrypoint":    "sampleEntryPoint",
				"extraneousKey": "we are agnostic to extraneous keys",
			},
		},
	}

	health, err = getArgoWorkflowHealth(&sampleWorkflow)
	require.NoError(t, err)
	assert.Equal(t, HealthStatusProgressing, health.Status)
	assert.Equal(t, "", health.Message)
}
