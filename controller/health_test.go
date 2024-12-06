package controller

import (
	"os"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/lua"
)

var (
	app           = &appv1.Application{}
	testTimestamp = metav1.NewTime(time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC))
)

func initStatuses(resources []managedResource) []appv1.ResourceStatus {
	statuses := make([]appv1.ResourceStatus, len(resources))
	for i := range resources {
		statuses[i] = appv1.ResourceStatus{
			Group:   resources[i].Group,
			Kind:    resources[i].Kind,
			Version: resources[i].Version,
			Health:  &appv1.HealthStatus{LastTransitionTime: testTimestamp},
		}
	}
	return statuses
}

func resourceFromFile(filePath string) unstructured.Unstructured {
	yamlBytes, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	var res unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &res)
	if err != nil {
		panic(err)
	}
	return res
}

func TestSetApplicationHealth(t *testing.T) {
	failedJob := resourceFromFile("./testdata/job-failed.yaml")
	runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
	}, {
		Group: "batch", Version: "v1", Kind: "Job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)
	// Populate health status
	resourceStatuses[0].Health.Status = health.HealthStatusHealthy
	app.Status.Health.LastTransitionTime = testTimestamp

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	firstHealthStatusTransitionTime := healthStatus.LastTransitionTime
	assert.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus.Status)
	assert.NotEqual(t, testTimestamp, firstHealthStatusTransitionTime)

	assert.Equal(t, health.HealthStatusHealthy, resourceStatuses[0].Health.Status)
	assert.Equal(t, testTimestamp, resourceStatuses[0].Health.LastTransitionTime)
	assert.Equal(t, health.HealthStatusDegraded, resourceStatuses[1].Health.Status)
	assert.Equal(t, firstHealthStatusTransitionTime, resourceStatuses[1].Health.LastTransitionTime)
	// Mark both health statuses as degraded, as app is degraded.
	resourceStatuses[0].Health.Status = health.HealthStatusDegraded
	resourceStatuses[1].Health.Status = health.HealthStatusDegraded

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	healthStatus, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus.Status)
	assert.Equal(t, testTimestamp, healthStatus.LastTransitionTime)

	// now we set the `argocd.argoproj.io/ignore-healthcheck: "true"` annotation on the job's target.
	// The app is considered healthy
	failedJob.SetAnnotations(nil)
	failedJobIgnoreHealthcheck := resourceFromFile("./testdata/job-failed-ignore-healthcheck.yaml")
	resources[1].Target = &failedJobIgnoreHealthcheck
	healthStatus, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus.Status)
}

func TestSetApplicationHealth_ResourceHealthNotPersisted(t *testing.T) {
	failedJob := resourceFromFile("./testdata/job-failed.yaml")

	resources := []managedResource{{
		Group: "batch", Version: "v1", Kind: "Job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, false)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus.Status)

	assert.Nil(t, resourceStatuses[0].Health)
	assert.False(t, healthStatus.LastTransitionTime.IsZero())
}

func TestSetApplicationHealth_MissingResource(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "Pod", Target: &pod,
	}, {}}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusMissing, healthStatus.Status)
	assert.False(t, healthStatus.LastTransitionTime.IsZero())
}

func TestSetApplicationHealth_HealthImproves(t *testing.T) {
	overrides := lua.ResourceHealthOverrides{
		lua.GetConfigMapKey(appv1.ApplicationSchemaGroupVersionKind): appv1.ResourceOverride{
			HealthLua: `
hs = {}
hs.status = "Progressing"
hs.message = ""
return hs`,
		},
	}

	degradedApp := newAppLiveObj(health.HealthStatusDegraded)
	timestamp := metav1.Now()
	resources := []managedResource{{
		Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: degradedApp,
	}, {}}
	resourceStatuses := initStatuses(resources)
	resourceStatuses[0].Health.Status = health.HealthStatusDegraded
	resourceStatuses[0].Health.LastTransitionTime = timestamp

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
	assert.NoError(t, err)
	assert.Equal(t, health.HealthStatusProgressing, healthStatus.Status)
	assert.NotEqual(t, healthStatus.LastTransitionTime, timestamp)
}

func TestSetApplicationHealth_MissingResourceNoBuiltHealthCheck(t *testing.T) {
	cm := resourceFromFile("./testdata/configmap.yaml")

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "ConfigMap", Target: &cm,
	}}
	resourceStatuses := initStatuses(resources)

	t.Run("NoOverride", func(t *testing.T) {
		healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus.Status)
		assert.Equal(t, health.HealthStatusMissing, resourceStatuses[0].Health.Status)
	})

	t.Run("HasOverride", func(t *testing.T) {
		healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{
			lua.GetConfigMapKey(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}): appv1.ResourceOverride{
				HealthLua: "some health check",
			},
		}, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusMissing, healthStatus.Status)
		assert.False(t, healthStatus.LastTransitionTime.IsZero())
	})
}

func newAppLiveObj(status health.HealthStatusCode) *unstructured.Unstructured {
	app := appv1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       application.ApplicationKind,
		},
		Status: appv1.ApplicationStatus{
			Health: appv1.HealthStatus{
				Status: status,
			},
		},
	}

	return kube.MustToUnstructured(&app)
}

func TestChildAppHealth(t *testing.T) {
	overrides := lua.ResourceHealthOverrides{
		lua.GetConfigMapKey(appv1.ApplicationSchemaGroupVersionKind): appv1.ResourceOverride{
			HealthLua: `
hs = {}
hs.status = "Progressing"
hs.message = ""
if obj.status ~= nil then
  if obj.status.health ~= nil then
	hs.status = obj.status.health.status
	if obj.status.health.message ~= nil then
	  hs.message = obj.status.health.message
	end
  end
end
return hs`,
		},
	}

	t.Run("ChildAppDegraded", func(t *testing.T) {
		degradedApp := newAppLiveObj(health.HealthStatusDegraded)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: degradedApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusDegraded, healthStatus.Status)
	})

	t.Run("ChildAppMissing", func(t *testing.T) {
		degradedApp := newAppLiveObj(health.HealthStatusMissing)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: degradedApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus.Status)
	})
}
