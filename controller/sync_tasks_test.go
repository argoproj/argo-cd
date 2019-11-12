package controller

import (
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/engine/pkg"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/engine/common"
	. "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test"
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
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase: SyncPhaseSyncFail, TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhasePreSync,
		TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase: SyncPhasePostSync, TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	}},
}

var sortedTasks = syncTasks{
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhasePreSync,
		TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhasePostSync,
		TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhaseSyncFail,
		TargetObj: &unstructured.Unstructured{},
	}},
}

var namedObjTasks = syncTasks{
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "a",
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "b",
				},
			},
		},
	}},
}

var unnamedTasks = syncTasks{
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhasePreSync,
		TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhasePostSync,
		TargetObj: &unstructured.Unstructured{},
	}},
	{SyncTaskInfo: pkg.SyncTaskInfo{
		Phase:     SyncPhaseSyncFail,
		TargetObj: &unstructured.Unstructured{},
	}},
}

func Test_syncTasks_Filter(t *testing.T) {
	tasks := syncTasks{{SyncTaskInfo: pkg.SyncTaskInfo{Phase: SyncPhaseSync}}, {SyncTaskInfo: pkg.SyncTaskInfo{Phase: SyncPhasePostSync}}}

	assert.Equal(t, syncTasks{{SyncTaskInfo: pkg.SyncTaskInfo{Phase: SyncPhaseSync}}}, tasks.Filter(func(t *syncTask) bool {
		return t.Phase == SyncPhaseSync
	}))
}

func TestSyncNamespaceAgainstCRD(t *testing.T) {
	crd := &syncTask{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "Workflow",
			},
		}}}
	namespace := &syncTask{SyncTaskInfo: pkg.SyncTaskInfo{
		TargetObj: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind": "Namespace",
			},
		}},
	}

	unsorted := syncTasks{crd, namespace}
	sort.Sort(unsorted)

	assert.Equal(t, syncTasks{namespace, crd}, unsorted)
}

func Test_syncTasks_multiStep(t *testing.T) {
	t.Run("Single", func(t *testing.T) {
		tasks := syncTasks{{SyncTaskInfo: pkg.SyncTaskInfo{LiveObj: Annotate(NewPod(), common.AnnotationSyncWave, "-1"), Phase: SyncPhaseSync}}}
		assert.Equal(t, SyncPhaseSync, tasks.phase())
		assert.Equal(t, -1, tasks.wave())
		assert.Equal(t, SyncPhaseSync, tasks.lastPhase())
		assert.Equal(t, -1, tasks.lastWave())
		assert.False(t, tasks.multiStep())
	})
	t.Run("Double", func(t *testing.T) {
		tasks := syncTasks{
			{SyncTaskInfo: pkg.SyncTaskInfo{LiveObj: Annotate(NewPod(), common.AnnotationSyncWave, "-1"), Phase: SyncPhasePreSync}},
			{SyncTaskInfo: pkg.SyncTaskInfo{LiveObj: Annotate(NewPod(), common.AnnotationSyncWave, "1"), Phase: SyncPhasePostSync}},
		}
		assert.Equal(t, SyncPhasePreSync, tasks.phase())
		assert.Equal(t, -1, tasks.wave())
		assert.Equal(t, SyncPhasePostSync, tasks.lastPhase())
		assert.Equal(t, 1, tasks.lastWave())
		assert.True(t, tasks.multiStep())
	})
}
