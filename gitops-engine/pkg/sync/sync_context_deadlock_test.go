package sync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/klog/v2/textlogger"

	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestGetSyncTasks_TerminatingHookNoFinalizer(t *testing.T) {
	// Setup a hook that is terminating (has DeletionTimestamp)
	terminatingHook := testingutils.NewPod()
	terminatingHook.SetName("terminating-hook")
	terminatingHook.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	
	now := metav1.NewTime(time.Now())
	terminatingHook.SetDeletionTimestamp(&now)

	// Mock discovery client
	fakeDisco := &fakediscovery.FakeDiscovery{Fake: &fake.NewSimpleClientset().Fake}
	fakeDisco.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Namespaced: true,
					Kind:       "Pod",
					Group:      "",
					Version:    "v1",
					Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		},
	}

	syncCtx := &syncContext{
		hooks:     []*unstructured.Unstructured{terminatingHook},
		resources: make(map[kube.ResourceKey]reconciledResource),
		log:       textlogger.NewLogger(textlogger.NewConfig()),
		disco:     fakeDisco,
		permissionValidator: func(_ *unstructured.Unstructured, _ *metav1.APIResource) error {
			return nil
		},
		syncRes:   make(map[string]common.ResourceSyncResult),
		startedAt: time.Now(),
	}
	
	// Mock liveObj to return the terminating hook
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{terminatingHook},
		Target: []*unstructured.Unstructured{nil},
	})

	tasks, successful := syncCtx.getSyncTasks()
	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	
	// The target object in the task should NOT have the hook finalizer
	task := tasks[0]
	assert.False(t, hook.HasHookFinalizer(task.targetObj), "Terminating hook should NOT have the hook finalizer added")
}

func TestGetSyncTasks_ActiveHookHasFinalizer(t *testing.T) {
	// Setup a hook that is active (no DeletionTimestamp)
	activeHook := testingutils.NewPod()
	activeHook.SetName("active-hook")
	activeHook.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})

	// Mock discovery client
	fakeDisco := &fakediscovery.FakeDiscovery{Fake: &fake.NewSimpleClientset().Fake}
	fakeDisco.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "pods",
					Namespaced: true,
					Kind:       "Pod",
					Group:      "",
					Version:    "v1",
					Verbs:      []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		},
	}

	syncCtx := &syncContext{
		hooks:     []*unstructured.Unstructured{activeHook},
		resources: make(map[kube.ResourceKey]reconciledResource),
		log:       textlogger.NewLogger(textlogger.NewConfig()),
		disco:     fakeDisco,
		permissionValidator: func(_ *unstructured.Unstructured, _ *metav1.APIResource) error {
			return nil
		},
		syncRes:   make(map[string]common.ResourceSyncResult),
		startedAt: time.Now(),
	}
	
	// Mock liveObj to return the active hook (or nil, either way it should get the finalizer)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{activeHook},
		Target: []*unstructured.Unstructured{nil},
	})

	tasks, successful := syncCtx.getSyncTasks()
	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	
	// The target object in the task SHOULD have the hook finalizer
	task := tasks[0]
	assert.True(t, hook.HasHookFinalizer(task.targetObj), "Active hook SHOULD have the hook finalizer added")
}
