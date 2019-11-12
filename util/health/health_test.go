package health

import (
	"io/ioutil"
	"testing"

	"github.com/argoproj/argo-cd/engine/util/lua"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/common"
	appv1 "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/engine/util/kube"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus appv1.HealthStatusCode) {
	health := getHealthStatus(yamlPath, t)
	assert.NotNil(t, health)
	assert.Equal(t, expectedStatus, health.Status)
}

func getHealthStatus(yamlPath string, t *testing.T) *appv1.HealthStatus {
	yamlBytes, err := ioutil.ReadFile(yamlPath)
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := GetResourceHealth(&obj, &lua.VM{})
	assert.Nil(t, err)
	return health
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
	assert.Nil(t, getHealthStatus("./testdata/knative-service.yaml", t))
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
	assertAppHealth(t, "./testdata/pod-imagepullbackoff.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-error.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-running-restart-always.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-running-restart-never.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-running-restart-onfailure.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/pod-failed.yaml", appv1.HealthStatusDegraded)
	assertAppHealth(t, "./testdata/pod-succeeded.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/pod-deletion.yaml", appv1.HealthStatusProgressing)
}

func TestApplication(t *testing.T) {
	assertAppHealth(t, "./testdata/application-healthy.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/application-degraded.yaml", appv1.HealthStatusDegraded)
}

func TestAppOfAppsHealth(t *testing.T) {
	newAppLiveObj := func(name string, status appv1.HealthStatusCode) (*unstructured.Unstructured, appv1.ResourceStatus) {
		app := appv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       "Application",
			},
			Status: appv1.ApplicationStatus{
				Health: appv1.HealthStatus{
					Status: status,
				},
			},
		}
		resStatus := appv1.ResourceStatus{
			Group:   "argoproj.io",
			Version: "v1alpha1",
			Kind:    "Application",
			Name:    name,
		}
		return kube.MustToUnstructured(&app), resStatus
	}

	missingApp, missingStatus := newAppLiveObj("foo", appv1.HealthStatusMissing)
	healthyApp, healthyStatus := newAppLiveObj("bar", appv1.HealthStatusHealthy)
	degradedApp, degradedStatus := newAppLiveObj("baz", appv1.HealthStatusDegraded)

	// verify missing child app does not affect app health
	{
		missingAndHealthyStatuses := []appv1.ResourceStatus{missingStatus, healthyStatus}
		missingAndHealthyLiveObjects := []*unstructured.Unstructured{missingApp, healthyApp}
		healthStatus, err := SetApplicationHealth(missingAndHealthyStatuses, missingAndHealthyLiveObjects, &lua.VM{}, noFilter)
		assert.NoError(t, err)
		assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)
	}

	// verify degraded does affect
	{
		degradedAndHealthyStatuses := []appv1.ResourceStatus{degradedStatus, healthyStatus}
		degradedAndHealthyLiveObjects := []*unstructured.Unstructured{degradedApp, healthyApp}
		healthStatus, err := SetApplicationHealth(degradedAndHealthyStatuses, degradedAndHealthyLiveObjects, &lua.VM{}, noFilter)
		assert.NoError(t, err)
		assert.Equal(t, appv1.HealthStatusDegraded, healthStatus.Status)
	}

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
	healthStatus, err := SetApplicationHealth(resources, liveObjs, &lua.VM{}, noFilter)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusDegraded, healthStatus.Status)

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	healthStatus, err = SetApplicationHealth(resources, liveObjs, &lua.VM{}, noFilter)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)
}

func TestAPIService(t *testing.T) {
	assertAppHealth(t, "./testdata/apiservice-v1-true.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1-false.yaml", appv1.HealthStatusProgressing)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-true.yaml", appv1.HealthStatusHealthy)
	assertAppHealth(t, "./testdata/apiservice-v1beta1-false.yaml", appv1.HealthStatusProgressing)
}

func TestGetStatusFromArgoWorkflow(t *testing.T) {
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

	status, message := GetStatusFromArgoWorkflow(&sampleWorkflow)
	assert.Equal(t, appv1.OperationRunning, status)
	assert.Equal(t, "This node is running", message)

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

	status, message = GetStatusFromArgoWorkflow(&sampleWorkflow)
	assert.Equal(t, appv1.OperationSucceeded, status)
	assert.Equal(t, "This node is has succeeded", message)

}

func noFilter(obj *unstructured.Unstructured) bool {
	return true
}
