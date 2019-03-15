package health

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus appv1.HealthStatusCode) {
	yamlBytes, err := ioutil.ReadFile(yamlPath)
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := GetResourceHealth(&obj, nil)
	assert.Nil(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, expectedStatus, health.Status)
}

func TestDeploymentHealth(t *testing.T) {
	assertAppHealth(t, "../kube/testdata/nginx.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/deployment-progressing.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/deployment-suspended.yaml", appv1.HealthStatusSuspended)
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
	assertAppHealth(t, "./testdata/svc-loadbalancer-nonemptylist.yaml", appv1.HealthStatusHealthy)
}

func TestIngressHealth(t *testing.T) {
	assertAppHealth(t, "./testdata/ingress.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/ingress-unassigned.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/ingress-nonemptylist.yaml", appv1.HealthStatusHealthy)
}

func TestCRD(t *testing.T) {
	// This ensures we do not try to compare only based on "Kind"
	assertAppHealth(t, "./testdata/knative-service.yaml", appv1.HealthStatusHealthy)
}

func TestJob(t *testing.T) {
	assertAppHealth(t, "./testdata/job-running.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/job-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/job-succeeded.yaml", appv1.HealthStatusHealthy)
}

func TestPod(t *testing.T) {
	assertAppHealth(t, "./testdata/pod-pending.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-not-ready.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-crashloop.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-error.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-running-restart-always.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-running-restart-never.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-restart-onfailure.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-succeeded.yaml", appv1.HealthStatusHealthy)
}

func TestSetApplicationHealth(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("./testdata/job-failed.yaml")
	assert.Nil(t, err)
	var failedJob unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &failedJob)
	assert.Nil(t, err)

	yamlBytes, err = ioutil.ReadFile("./testdata/pod-running-restart-always.yaml")
	assert.Nil(t, err)
	var runningPod unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &runningPod)
	assert.Nil(t, err)

	resources := []appv1.ResourceStatus{
		{
			Group:   "",
			Version: "v1",
			Kind:    "Pod",
			Name:    runningPod.GetName(),
		},
		{
			Group:   "batch",
			Version: "v1",
			Kind:    "Job",
			Name:    failedJob.GetName(),
		},
	}
	liveObjs := []*unstructured.Unstructured{
		&runningPod,
		&failedJob,
	}
	healthStatus, err := SetApplicationHealth(resources, liveObjs, nil)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusDegraded, healthStatus.Status)

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	healthStatus, err = SetApplicationHealth(resources, liveObjs, nil)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)

}
