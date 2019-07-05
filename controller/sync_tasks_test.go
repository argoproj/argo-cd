package controller

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func Test_syncTasks_kindOrder(t *testing.T) {
	assert.Equal(t, -27, kindOrder["Namespace"])
	assert.Equal(t, -1, kindOrder["APIService"])
	assert.Equal(t, 0, kindOrder["MyCRD"])
}

func TestSortSyncTask(t *testing.T) {
	sort.Sort(unsortedTasks)
	assert.Equal(t, sortedTasks, unsortedTasks)
}

func TestAnySyncTasks(t *testing.T) {
	res := unsortedTasks.Any(func(task *syncTask) bool {
		return task.name() == "a"
	})
	assert.True(t, res)

	res = unsortedTasks.Any(func(task *syncTask) bool {
		return task.name() == "does-not-exist"
	})
	assert.False(t, res)

}

func TestAllSyncTasks(t *testing.T) {
	res := unsortedTasks.All(func(task *syncTask) bool {
		return task.name() != ""
	})
	assert.False(t, res)

	res = unsortedTasks.All(func(task *syncTask) bool {
		return task.name() == "a"
	})
	assert.False(t, res)
}

func TestSplitSyncTasks(t *testing.T) {
	named, unnamed := sortedTasks.Split(func(task *syncTask) bool {
		return task.name() != ""
	})
	assert.Equal(t, named, namedObjTasks)
	assert.Equal(t, unnamed, unnamedTasks)
}

var unsortedTasks = syncTasks{
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
		phase: SyncPhaseSyncFail, targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
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
		phase:     SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase: SyncPhasePostSync, targetObj: &unstructured.Unstructured{},
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

var sortedTasks = syncTasks{
	{
		phase:     SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
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
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		phase:     SyncPhasePostSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase:     SyncPhaseSyncFail,
		targetObj: &unstructured.Unstructured{},
	},
}

var namedObjTasks = syncTasks{
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	},
}

var unnamedTasks = syncTasks{
	{
		phase:     SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
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
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		phase:     SyncPhasePostSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase:     SyncPhaseSyncFail,
		targetObj: &unstructured.Unstructured{},
	},
}

func Test_syncTasks_Filter(t *testing.T) {
	tasks := syncTasks{{phase: SyncPhaseSync}, {phase: SyncPhasePostSync}}

	assert.Equal(t, syncTasks{{phase: SyncPhaseSync}}, tasks.Filter(func(t *syncTask) bool {
		return t.phase == SyncPhaseSync
	}))
}

func TestSyncNamespaceAgainstCRD(t *testing.T) {
	crd := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "Workflow",
			},
		}}
	namespace := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "Namespace",
			},
		},
	}

	unsorted := syncTasks{crd, namespace}
	sort.Sort(unsorted)

	assert.Equal(t, syncTasks{namespace, crd}, unsorted)
}
