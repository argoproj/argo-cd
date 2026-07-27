package controller

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/lua"
)

var (
	app = &appv1.Application{
		Status: appv1.ApplicationStatus{
			Health: appv1.AppHealthStatus{
				LastTransitionTime: &metav1.Time{Time: time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC)},
			},
		},
	}
	testTimestamp = metav1.Time{Time: time.Date(2020, time.January, 1, 12, 0, 0, 0, time.UTC)}
)

func initStatuses(resources []managedResource) []appv1.ResourceStatus {
	statuses := make([]appv1.ResourceStatus, len(resources))
	for i := range resources {
		statuses[i] = appv1.ResourceStatus{Group: resources[i].Group, Kind: resources[i].Kind, Version: resources[i].Version}
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
		Group: "", Version: "v1", Kind: "Pod", Namespace: "default", Name: "running-pod", Live: &runningPod,
	}, {
		Group: "batch", Version: "v1", Kind: "Job", Namespace: "default", Name: "failed-job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus)
	assert.Equal(t, health.HealthStatusHealthy, resourceStatuses[0].Health.Status)
	assert.Equal(t, health.HealthStatusDegraded, resourceStatuses[1].Health.Status)
	// The cause of the Degraded app health is the failed Job, not the healthy Pod.
	assert.Equal(t, "Caused by batch/Job:default/failed-job", healthCauses)
	app.Status.Health.Status = healthStatus

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	healthStatus, healthCauses, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	// A Healthy app has no contributing causes.
	assert.Empty(t, healthCauses)
	app.Status.Health.Status = healthStatus

	// now we set the `argocd.argoproj.io/ignore-healthcheck: "true"` annotation on the job's target.
	// The app is considered healthy
	failedJob.SetAnnotations(nil)
	failedJobIgnoreHealthcheck := resourceFromFile("./testdata/job-failed-ignore-healthcheck.yaml")
	resources[1].Live = &failedJobIgnoreHealthcheck
	healthStatus, healthCauses, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_ResourceHealthNotPersisted(t *testing.T) {
	failedJob := resourceFromFile("./testdata/job-failed.yaml")

	resources := []managedResource{{
		Group: "batch", Version: "v1", Kind: "Job", Namespace: "default", Name: "failed-job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, false)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus)

	assert.Nil(t, resourceStatuses[0].Health)
	// Causes must still be reported even though per-resource health is not persisted.
	assert.Equal(t, "Caused by batch/Job:default/failed-job", healthCauses)
}

func TestSetApplicationHealth_NoResource(t *testing.T) {
	resources := []managedResource{}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_OnlyHooks(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: string(synccommon.HookTypeSync)})

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "Pod", Target: &pod, Live: &pod,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	// Hooks are skipped, so the Healthy app has no causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_MissingResource(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod2 := pod.DeepCopy()
	pod2.SetName("pod2")

	resources := []managedResource{
		{Group: "", Version: "v1", Kind: "Pod", Target: &pod},
		{Group: "", Version: "v1", Kind: "Pod", Target: pod2, Live: pod2},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	// The missing target-only resource does not degrade the app, so there are no causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_MissingResource_WithIgnoreHealthcheck(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod2 := pod.DeepCopy()
	pod2.SetName("pod2")
	pod2.SetAnnotations(map[string]string{common.AnnotationIgnoreHealthCheck: "true"})

	resources := []managedResource{
		{Group: "", Version: "v1", Kind: "Pod", Target: &pod},
		{Group: "", Version: "v1", Kind: "Pod", Target: pod2, Live: pod2},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	// The ignored resource is not aggregated, so the Healthy app has no causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_MissingResource_WithChildApp(t *testing.T) {
	childApp := newAppLiveObj(health.HealthStatusUnknown)
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	resources := []managedResource{
		{Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Target: childApp, Live: childApp},
		{Group: "", Version: "v1", Kind: "Pod", Target: &pod},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	// An Unknown child app does not affect the parent, so the Healthy app has no causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_AllMissingResources(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod2 := pod.DeepCopy()
	pod2.SetName("pod2")

	resources := []managedResource{
		{Group: "", Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod", Target: &pod},
		{Group: "", Version: "v1", Kind: "Pod", Namespace: "default", Name: "pod2", Target: pod2},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusMissing, healthStatus)
	// The Missing app health from the all-missing fallback does not attribute individual causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_AllMissingResources_WithHooks(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod2 := pod.DeepCopy()
	pod2.SetName("pod2")
	pod2.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: string(synccommon.HookTypeSync)})

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "Pod", Target: &pod,
	}, {
		Group: "", Version: "v1", Kind: "Pod", Target: pod2, Live: pod2,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusMissing, healthStatus)
	// The all-missing fallback does not attribute individual causes.
	assert.Empty(t, healthCauses)
}

func TestSetApplicationHealth_MultipleCauses(t *testing.T) {
	failedJob := resourceFromFile("./testdata/job-failed.yaml")
	failedJob2 := failedJob.DeepCopy()
	failedJob2.SetName("failed-job-2")
	runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")

	resources := []managedResource{
		{Group: "", Version: "v1", Kind: "Pod", Namespace: "default", Name: "running-pod", Live: &runningPod},
		{Group: "batch", Version: "v1", Kind: "Job", Namespace: "default", Name: "failed-job", Live: &failedJob},
		{Group: "batch", Version: "v1", Kind: "Job", Namespace: "default", Name: "failed-job-2", Live: failedJob2},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus)
	// Both failed Jobs are causes; the healthy Pod is not.
	assert.Equal(t, "Caused by batch/Job:default/failed-job, batch/Job:default/failed-job-2", healthCauses)
}

func TestFormatHealthCauses(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.Empty(t, formatHealthCauses(nil))
	})

	t.Run("single namespaced resource", func(t *testing.T) {
		out := formatHealthCauses([]managedResource{
			{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "frontend"},
		})
		assert.Equal(t, "Caused by apps/Deployment:default/frontend", out)
	})

	t.Run("cluster-scoped resource omits namespace and empty group", func(t *testing.T) {
		out := formatHealthCauses([]managedResource{
			{Kind: "PersistentVolume", Name: "pv-1"},
		})
		assert.Equal(t, "Caused by PersistentVolume:pv-1", out)
	})

	t.Run("truncates after max with count", func(t *testing.T) {
		var causes []managedResource
		for i := range 8 {
			causes = append(causes, managedResource{Kind: "Pod", Namespace: "default", Name: fmt.Sprintf("pod-%d", i)})
		}
		out := formatHealthCauses(causes)
		assert.Contains(t, out, "and 5 more")
		assert.Equal(t, maxHealthCausesShown, strings.Count(out, "Pod:default/"))
	})
}

func TestSetApplicationHealth_HealthImproves(t *testing.T) {
	testCases := []struct {
		oldStatus health.HealthStatusCode
		newStatus health.HealthStatusCode
	}{
		{health.HealthStatusUnknown, health.HealthStatusDegraded},
		{health.HealthStatusDegraded, health.HealthStatusProgressing},
		{health.HealthStatusMissing, health.HealthStatusProgressing},
		{health.HealthStatusProgressing, health.HealthStatusSuspended},
		{health.HealthStatusSuspended, health.HealthStatusHealthy},
	}

	for _, tc := range testCases {
		overrides := lua.ResourceHealthOverrides{
			lua.GetConfigMapKey(schema.FromAPIVersionAndKind("v1", "Pod")): appv1.ResourceOverride{
				HealthLua: fmt.Sprintf("hs = {}\nhs.status = %q\nhs.message = \"\"return hs", tc.newStatus),
			},
		}

		runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
		resources := []managedResource{{
			Group: "", Version: "v1", Kind: "Pod", Namespace: "default", Name: "running-pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		t.Run(string(fmt.Sprintf("%s to %s", tc.oldStatus, tc.newStatus)), func(t *testing.T) {
			healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
			require.NoError(t, err)
			assert.Equal(t, tc.newStatus, healthStatus)
			// A non-Healthy app attributes the offending Pod as its cause; a Healthy app has none.
			if tc.newStatus == health.HealthStatusHealthy {
				assert.Empty(t, healthCauses)
			} else {
				assert.Equal(t, "Caused by Pod:default/running-pod", healthCauses)
			}
		})
	}
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
			Health: appv1.AppHealthStatus{
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
		childApp := newAppLiveObj(health.HealthStatusDegraded)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Namespace: "default", Name: "foo", Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusDegraded, healthStatus)
		// The Degraded child app is the cause of the parent's Degraded health.
		assert.Equal(t, "Caused by argoproj.io/Application:default/foo", healthCauses)
	})

	t.Run("ChildAppMissing", func(t *testing.T) {
		childApp := newAppLiveObj(health.HealthStatusMissing)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Namespace: "default", Name: "foo", Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
		// A Missing child app does not affect the parent, so there are no causes.
		assert.Empty(t, healthCauses)
	})

	t.Run("ChildAppUnknown", func(t *testing.T) {
		childApp := newAppLiveObj(health.HealthStatusUnknown)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Namespace: "default", Name: "foo", Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, healthCauses, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
		// An Unknown child app does not affect the parent, so there are no causes.
		assert.Empty(t, healthCauses)
	})
}
