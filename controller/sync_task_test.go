package controller

import (
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSortSyncTask(t *testing.T) {
	task2 := syncTaskWithSyncWave("2", false)
	task1 := syncTaskWithSyncWave("1", false)
	tests := []struct {
		name  string
		tasks syncTasks
		want  syncTasks
	}{
		{"TestNoop", []syncTask{}, []syncTask{}},
		{"TestOneTask", []syncTask{task1}, []syncTask{task1}},
		{"TestUnsorted", unsortedManifest, sortedManifest},
		{"TestWave", []syncTask{task1, task2}, []syncTask{task1, task2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(tt.tasks)
			assert.Equal(t, tt.want, tt.tasks)
		})
	}
}

func syncTaskWithSyncWave(syncWave string, successful bool) syncTask {
	return syncTask{
		targetObj:  objWithSyncWave(syncWave),
		successful: successful,
	}
}

func objWithSyncWave(syncWave string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": metaDataWithSyncWaveAnnotations(syncWave),
		},
	}
}

func metaDataWithSyncWaveAnnotations(syncWave string) map[string]interface{} {
	return map[string]interface{}{
		"annotations": map[string]interface{}{
			"argocd.argoproj.io/sync-wave": syncWave,
		},
	}
}

var unsortedManifest syncTasks = []syncTask{
	syncTaskWithSyncWave("1", false),
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	},
}

var sortedManifest syncTasks = []syncTask{
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	},
	syncTaskWithSyncWave("1", false),
}

func Test_syncTask_getWave(t *testing.T) {

	const noTasksWave = math.MaxInt32
	tests := []struct {
		name      string
		syncTasks syncTasks
		want      int
	}{
		{"TestEmpty", syncTasks{}, noTasksWave},
		{"TestOneTask", syncTasks{syncTask{successful: false, targetObj: &unstructured.Unstructured{}}}, 0},
		{"TestOneTaskWithWave", syncTasks{syncTaskWithSyncWave("1", false)}, 1},
		{"TestTwoTasksWithWave", syncTasks{syncTaskWithSyncWave("1", false), syncTaskWithSyncWave("2", false)}, 1},
		{"TestTwoTasksWithWaveOneGood", syncTasks{syncTaskWithSyncWave("1", true), syncTaskWithSyncWave("2", false)}, 2},
		{"TestTwoTasksWithWaveAllGood", syncTasks{syncTaskWithSyncWave("1", true), syncTaskWithSyncWave("2", true)}, noTasksWave},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.syncTasks.getNextWave())
		})
	}
}

func Test_syncTask_isHook(t *testing.T) {
	tests := []struct {
		name    string
		liveObj *unstructured.Unstructured
		want    bool
	}{
		{"TestNonHook", &unstructured.Unstructured{}, false},
		{"TestHook", &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/hook": "foo",
					},
				},
			},
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := syncTask{
				liveObj: tt.liveObj,
			}
			if got := task.isHook(); got != tt.want {
				t.Errorf("syncTask.isHook() = %v, want %v", got, tt.want)
			}
		})
	}
}
