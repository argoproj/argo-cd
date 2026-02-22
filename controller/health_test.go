package controller

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
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
		Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
	}, {
		Group: "batch", Version: "v1", Kind: "Job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus)
	assert.Equal(t, health.HealthStatusHealthy, resourceStatuses[0].Health.Status)
	assert.Equal(t, health.HealthStatusDegraded, resourceStatuses[1].Health.Status)
	app.Status.Health.Status = healthStatus

	// now mark the job as a hook and retry. it should ignore the hook and consider the app healthy
	failedJob.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	healthStatus, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	app.Status.Health.Status = healthStatus

	// now we set the `argocd.argoproj.io/ignore-healthcheck: "true"` annotation on the job's target.
	// The app is considered healthy
	failedJob.SetAnnotations(nil)
	failedJobIgnoreHealthcheck := resourceFromFile("./testdata/job-failed-ignore-healthcheck.yaml")
	resources[1].Live = &failedJobIgnoreHealthcheck
	healthStatus, err = setApplicationHealth(resources, resourceStatuses, nil, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
}

func TestSetApplicationHealth_ResourceHealthNotPersisted(t *testing.T) {
	failedJob := resourceFromFile("./testdata/job-failed.yaml")

	resources := []managedResource{{
		Group: "batch", Version: "v1", Kind: "Job", Live: &failedJob,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, false)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusDegraded, healthStatus)

	assert.Nil(t, resourceStatuses[0].Health)
}

func TestSetApplicationHealth_NoResource(t *testing.T) {
	resources := []managedResource{}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
}

func TestSetApplicationHealth_OnlyHooks(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: string(synccommon.HookTypeSync)})

	resources := []managedResource{{
		Group: "", Version: "v1", Kind: "Pod", Target: &pod, Live: &pod,
	}}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
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

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
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

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
}

func TestSetApplicationHealth_MissingResource_WithChildApp(t *testing.T) {
	childApp := newAppLiveObj(health.HealthStatusUnknown)
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	resources := []managedResource{
		{Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Target: childApp, Live: childApp},
		{Group: "", Version: "v1", Kind: "Pod", Target: &pod},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusHealthy, healthStatus)
}

func TestSetApplicationHealth_AllMissingResources(t *testing.T) {
	pod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
	pod2 := pod.DeepCopy()
	pod2.SetName("pod2")

	resources := []managedResource{
		{Group: "", Version: "v1", Kind: "Pod", Target: &pod},
		{Group: "", Version: "v1", Kind: "Pod", Target: pod2},
	}
	resourceStatuses := initStatuses(resources)

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusMissing, healthStatus)
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

	healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
	require.NoError(t, err)
	assert.Equal(t, health.HealthStatusMissing, healthStatus)
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
			Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		t.Run(string(fmt.Sprintf("%s to %s", tc.oldStatus, tc.newStatus)), func(t *testing.T) {
			healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
			require.NoError(t, err)
			assert.Equal(t, tc.newStatus, healthStatus)
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
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusDegraded, healthStatus)
	})

	t.Run("ChildAppMissing", func(t *testing.T) {
		childApp := newAppLiveObj(health.HealthStatusMissing)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	})

	t.Run("ChildAppUnknown", func(t *testing.T) {
		childApp := newAppLiveObj(health.HealthStatusUnknown)
		resources := []managedResource{{
			Group: application.Group, Version: "v1alpha1", Kind: application.ApplicationKind, Live: childApp,
		}, {}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	})
}

func TestParseHealthAggregateOverrides(t *testing.T) {
	t.Run("SingleMapping", func(t *testing.T) {
		result, err := parseHealthAggregateOverrides("Suspended=Healthy")
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, result["Suspended"])
		assert.Len(t, result, 1)
	})

	t.Run("MultipleMappings", func(t *testing.T) {
		result, err := parseHealthAggregateOverrides("Suspended=Healthy,Progressing=Degraded")
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, result["Suspended"])
		assert.Equal(t, health.HealthStatusDegraded, result["Progressing"])
		assert.Len(t, result, 2)
	})

	t.Run("WithWhitespace", func(t *testing.T) {
		result, err := parseHealthAggregateOverrides(" Suspended = Healthy , Progressing = Degraded ")
		require.NoError(t, err)
		assert.Equal(t, health.HealthStatusHealthy, result["Suspended"])
		assert.Equal(t, health.HealthStatusDegraded, result["Progressing"])
		assert.Len(t, result, 2)
	})

	t.Run("EmptyString", func(t *testing.T) {
		result, err := parseHealthAggregateOverrides("")
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("InvalidFormat_NoEquals", func(t *testing.T) {
		_, err := parseHealthAggregateOverrides("Suspended")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mapping format")
	})

	t.Run("InvalidFormat_MultipleEquals", func(t *testing.T) {
		_, err := parseHealthAggregateOverrides("Suspended=Healthy=Extra")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid mapping format")
	})

	t.Run("InvalidFormat_EmptySource", func(t *testing.T) {
		_, err := parseHealthAggregateOverrides("=Healthy")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source and target cannot be empty")
	})

	t.Run("InvalidFormat_EmptyTarget", func(t *testing.T) {
		_, err := parseHealthAggregateOverrides("Suspended=")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source and target cannot be empty")
	})
}

func TestSetApplicationHealth_WithAggregateAsAnnotation(t *testing.T) {
	t.Run("AnnotationOverridesStatus", func(t *testing.T) {
		suspendedJob := resourceFromFile("./testdata/job-suspended.yaml")
		// Add annotation to override Suspended -> Progressing
		annotations := suspendedJob.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["argocd.argoproj.io/health-aggregate-overrides"] = "Suspended=Progressing"
		suspendedJob.SetAnnotations(annotations)

		resources := []managedResource{{
			Group: "batch", Version: "v1", Kind: "Job", Live: &suspendedJob,
		}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
		require.NoError(t, err)
		// Job is Suspended but annotation maps it to Progressing
		assert.Equal(t, health.HealthStatusProgressing, healthStatus)
		assert.Equal(t, health.HealthStatusSuspended, resourceStatuses[0].Health.Status) // Resource status unchanged
	})

	t.Run("AnnotationWithNoMatchingStatus", func(t *testing.T) {
		runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
		// Add annotation that doesn't match the pod's status
		annotations := runningPod.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["argocd.argoproj.io/health-aggregate-overrides"] = "Suspended=Progressing"
		runningPod.SetAnnotations(annotations)

		resources := []managedResource{{
			Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
		require.NoError(t, err)
		// Pod is Healthy, annotation doesn't match, so use original status
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
	})

	t.Run("InvalidAnnotationReturnsError", func(t *testing.T) {
		runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
		// Add invalid annotation
		annotations := runningPod.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["argocd.argoproj.io/health-aggregate-overrides"] = "InvalidFormat"
		runningPod.SetAnnotations(annotations)

		resources := []managedResource{{
			Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		_, err := setApplicationHealth(resources, resourceStatuses, lua.ResourceHealthOverrides{}, app, true)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse health aggregate overrides annotation")
		assert.Contains(t, err.Error(), "invalid mapping format")
	})
}

func TestSetApplicationHealth_WithLuaAggregateAs(t *testing.T) {
	t.Run("LuaAggregateAsUsedWhenNoAnnotation", func(t *testing.T) {
		overrides := lua.ResourceHealthOverrides{
			lua.GetConfigMapKey(schema.FromAPIVersionAndKind("v1", "Pod")): appv1.ResourceOverride{
				HealthLua: `
					hs = {}
					hs.status = "Suspended"
					hs.message = "Pod is suspended"
					hs.aggregateAs = "Healthy"
					return hs
				`,
			},
		}

		runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
		resources := []managedResource{{
			Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		// Lua returns Suspended but aggregateAs is Healthy
		assert.Equal(t, health.HealthStatusHealthy, healthStatus)
		assert.Equal(t, health.HealthStatusSuspended, resourceStatuses[0].Health.Status)
	})

	t.Run("AnnotationOverridesLuaAggregateAs", func(t *testing.T) {
		overrides := lua.ResourceHealthOverrides{
			lua.GetConfigMapKey(schema.FromAPIVersionAndKind("v1", "Pod")): appv1.ResourceOverride{
				HealthLua: `
					hs = {}
					hs.status = "Suspended"
					hs.message = "Pod is suspended"
					hs.aggregateAs = "Healthy"
					return hs
				`,
			},
		}

		runningPod := resourceFromFile("./testdata/pod-running-restart-always.yaml")
		// Add annotation that overrides status
		annotations := runningPod.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations["argocd.argoproj.io/health-aggregate-overrides"] = "Suspended=Degraded"
		runningPod.SetAnnotations(annotations)

		resources := []managedResource{{
			Group: "", Version: "v1", Kind: "Pod", Live: &runningPod,
		}}
		resourceStatuses := initStatuses(resources)

		healthStatus, err := setApplicationHealth(resources, resourceStatuses, overrides, app, true)
		require.NoError(t, err)
		// Annotation maps Suspended -> Degraded, ignoring Lua's aggregateAs
		assert.Equal(t, health.HealthStatusDegraded, healthStatus)
		assert.Equal(t, health.HealthStatusSuspended, resourceStatuses[0].Health.Status)
	})
}
