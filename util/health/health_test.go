package health

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
)

func assertAppHealth(t *testing.T, yamlPath string, expectedStatus appv1.HealthStatusCode) {
	yamlBytes, err := ioutil.ReadFile(yamlPath)
	if assert.NoError(t, err, yamlPath) {
		var obj unstructured.Unstructured
		err = yaml.Unmarshal(yamlBytes, &obj)
		if assert.NoError(t, err, yamlPath) {
			health, err := GetResourceHealth(&obj, nil)
			if assert.NoError(t, err, yamlPath) {
				if assert.NotNil(t, health, yamlPath) {
					assert.Equal(t, expectedStatus, health.Status, yamlPath)
				}
			}
		}
	}
}

func TestCRD(t *testing.T) {
	assertAppHealth(t, "./testdata/knative-service.yaml", appv1.HealthStatusUnknown)
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
	unknownApp, unknownStatus := newAppLiveObj("fooz", appv1.HealthStatusUnknown)
	healthyApp, healthyStatus := newAppLiveObj("bar", appv1.HealthStatusHealthy)
	degradedApp, degradedStatus := newAppLiveObj("baz", appv1.HealthStatusDegraded)

	// verify missing child app does not affect app health
	{
		missingAndHealthyStatuses := []appv1.ResourceStatus{missingStatus, healthyStatus}
		missingAndHealthyLiveObjects := []*unstructured.Unstructured{missingApp, healthyApp}
		healthStatus, err := SetApplicationHealth(missingAndHealthyStatuses, missingAndHealthyLiveObjects, nil, noFilter)
		assert.NoError(t, err)
		assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)
	}

	// verify unknown child app does not affect app health
	{
		unknownAndHealthyStatuses := []appv1.ResourceStatus{unknownStatus, healthyStatus}
		unknownAndHealthyLiveObjects := []*unstructured.Unstructured{unknownApp, healthyApp}
		healthStatus, err := SetApplicationHealth(unknownAndHealthyStatuses, unknownAndHealthyLiveObjects, nil, noFilter)
		assert.NoError(t, err)
		assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)
	}

	// verify degraded does affect
	{
		degradedAndHealthyStatuses := []appv1.ResourceStatus{degradedStatus, healthyStatus}
		degradedAndHealthyLiveObjects := []*unstructured.Unstructured{degradedApp, healthyApp}
		healthStatus, err := SetApplicationHealth(degradedAndHealthyStatuses, degradedAndHealthyLiveObjects, nil, noFilter)
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
	healthStatus, err := SetApplicationHealth(resources, liveObjs, nil, noFilter)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusDegraded, healthStatus.Status)

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	healthStatus, err = SetApplicationHealth(resources, liveObjs, nil, noFilter)
	assert.NoError(t, err)
	assert.Equal(t, appv1.HealthStatusHealthy, healthStatus.Status)
}
func noFilter(obj *unstructured.Unstructured) bool {
	return true
}
