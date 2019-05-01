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
	tests := []struct {
		name      string
		syncTasks syncTasks
		want      syncTasks
	}{
		{"TestNoop", []syncTask{}, []syncTask{}},
		{"TestUnsorted", unsortedManifest, sortedManifest},
		{"TestWave", []syncTask{syncTaskWithSyncWave("2", false), syncTaskWithSyncWave("1", false)}, []syncTask{syncTaskWithSyncWave("1", false), syncTaskWithSyncWave("2", false)}},
		{"TestModified", []syncTask{syncTaskWithSyncWave("2", false), syncTaskWithSyncWave("1", false)}, []syncTask{syncTaskWithSyncWave("1", false), syncTaskWithSyncWave("2", false)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(tt.syncTasks)
			assert.Equal(t, tt.want, tt.syncTasks)
		})
	}
}

func syncTaskWithSyncWave(syncWave string, modified bool) syncTask {
	return syncTask{
		targetObj: objWithSyncWave(syncWave),
		modified:  modified,
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

	tests := []struct {
		name      string
		syncTasks syncTasks
		want      int
	}{
		{"TestEmpty", syncTasks{}, math.MinInt32},
		{"TestOneTask", syncTasks{syncTask{}}, 0},
		{"TestOneTaskWithWave", syncTasks{syncTaskWithSyncWave("1", true)}, 1},
		{"TestTwoTasksWithWave", syncTasks{syncTaskWithSyncWave("1", true), syncTaskWithSyncWave("2", true)}, 1},
		{"TestTwoTasksWithWaveOneUnmodified", syncTasks{syncTaskWithSyncWave("1", false), syncTaskWithSyncWave("2", true)}, 2},
		{"TestTwoTasksWithWaveAllUnmodified", syncTasks{syncTaskWithSyncWave("1", false), syncTaskWithSyncWave("2", false)}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.syncTasks.getNextWave())
		})
	}
}
