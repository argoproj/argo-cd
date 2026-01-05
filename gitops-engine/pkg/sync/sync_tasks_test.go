package sync

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func Test_syncTasks_kindOrder(t *testing.T) {
	assert.Equal(t, -35, kindOrder["Namespace"])
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
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	},
	{
		phase: common.SyncPhaseSyncFail, targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "b",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
			},
		},
	},
	{
		phase:     common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase: common.SyncPhasePostSync, targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	},
}

var sortedTasks = syncTasks{
	{
		phase:     common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "b",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		phase:     common.SyncPhasePostSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase:     common.SyncPhaseSyncFail,
		targetObj: &unstructured.Unstructured{},
	},
}

var namedObjTasks = syncTasks{
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "a",
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"name": "b",
				},
			},
		},
	},
}

var unnamedTasks = syncTasks{
	{
		phase:     common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "-1",
					},
				},
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"GroupVersion": corev1.SchemeGroupVersion.String(),
			},
		},
	},
	{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	},
	{
		phase:     common.SyncPhasePostSync,
		targetObj: &unstructured.Unstructured{},
	},
	{
		phase:     common.SyncPhaseSyncFail,
		targetObj: &unstructured.Unstructured{},
	},
}

func Test_syncTasks_Filter(t *testing.T) {
	tasks := syncTasks{{phase: common.SyncPhaseSync}, {phase: common.SyncPhasePostSync}}

	assert.Equal(t, syncTasks{{phase: common.SyncPhaseSync}}, tasks.Filter(func(t *syncTask) bool {
		return t.phase == common.SyncPhaseSync
	}))
}

func TestSyncNamespaceAgainstCRD(t *testing.T) {
	crd := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Workflow",
			},
		},
	}
	namespace := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Namespace",
			},
		},
	}

	unsorted := syncTasks{crd, namespace}
	sort.Sort(unsorted)

	assert.Equal(t, syncTasks{namespace, crd}, unsorted)
}

func TestSyncTasksSort_NamespaceAndObjectInNamespace(t *testing.T) {
	hook1 := &syncTask{
		phase: common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Job",
				"metadata": map[string]any{
					"namespace": "myNamespace1",
					"name":      "mySyncHookJob1",
				},
			},
		},
	}
	hook2 := &syncTask{
		phase: common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Job",
				"metadata": map[string]any{
					"namespace": "myNamespace2",
					"name":      "mySyncHookJob2",
				},
			},
		},
	}
	namespace1 := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Namespace",
				"metadata": map[string]any{
					"name": "myNamespace1",
					"annotations": map[string]string{
						"argocd.argoproj.io/sync-wave": "1",
					},
				},
			},
		},
	}
	namespace2 := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Namespace",
				"metadata": map[string]any{
					"name": "myNamespace2",
					"annotations": map[string]string{
						"argocd.argoproj.io/sync-wave": "2",
					},
				},
			},
		},
	}

	unsorted := syncTasks{hook1, hook2, namespace1, namespace2}
	unsorted.Sort()

	assert.Equal(t, syncTasks{namespace1, hook1, namespace2, hook2}, unsorted)
	assert.Equal(t, 0, namespace1.wave())
	assert.Equal(t, common.SyncPhase(common.SyncPhasePreSync), namespace1.phase)
	assert.Equal(t, 0, namespace2.wave())
	assert.Equal(t, common.SyncPhase(common.SyncPhasePreSync), namespace2.phase)
}

func TestSyncTasksSort_CRDAndCR(t *testing.T) {
	cr := &syncTask{
		phase: common.SyncPhasePreSync,
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"kind":       "Workflow",
				"apiVersion": "argoproj.io/v1",
			},
		},
	}
	crd := &syncTask{
		targetObj: &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"spec": map[string]any{
					"group": "argoproj.io",
					"names": map[string]any{
						"kind": "Workflow",
					},
				},
			},
		},
	}

	unsorted := syncTasks{cr, crd}
	unsorted.Sort()

	assert.Equal(t, syncTasks{crd, cr}, unsorted)
}

func Test_syncTasks_multiStep(t *testing.T) {
	t.Run("Single", func(t *testing.T) {
		tasks := syncTasks{{liveObj: testingutils.Annotate(testingutils.NewPod(), common.AnnotationSyncWave, "-1"), phase: common.SyncPhaseSync}}
		assert.Equal(t, common.SyncPhaseSync, string(tasks.phase()))
		assert.Equal(t, -1, tasks.wave())
		assert.Equal(t, common.SyncPhaseSync, string(tasks.lastPhase()))
		assert.Equal(t, -1, tasks.lastWave())
		assert.False(t, tasks.multiStep())
	})
	t.Run("Double", func(t *testing.T) {
		tasks := syncTasks{
			{liveObj: testingutils.Annotate(testingutils.NewPod(), common.AnnotationSyncWave, "-1"), phase: common.SyncPhasePreSync},
			{liveObj: testingutils.Annotate(testingutils.NewPod(), common.AnnotationSyncWave, "1"), phase: common.SyncPhasePostSync},
		}
		assert.Equal(t, common.SyncPhasePreSync, string(tasks.phase()))
		assert.Equal(t, -1, tasks.wave())
		assert.Equal(t, common.SyncPhasePostSync, string(tasks.lastPhase()))
		assert.Equal(t, 1, tasks.lastWave())
		assert.True(t, tasks.multiStep())
	})
}
