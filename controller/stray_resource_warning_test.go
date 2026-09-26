package controller

import (
	"fmt"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestLiveObjHasPruneFalse(t *testing.T) {
	t.Parallel()

	t.Run("no annotations", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]any{
				"name":      "data-my-sts-0",
				"namespace": "default",
			},
		}}
		assert.False(t, liveObjHasPruneFalse(obj))
	})

	t.Run("prune false on live object", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]any{
				"name":      "data-my-sts-0",
				"namespace": "default",
				"annotations": map[string]any{
					synccommon.AnnotationSyncOptions: "Prune=false,Delete=false",
				},
			},
		}}
		assert.True(t, liveObjHasPruneFalse(obj))
	})

	t.Run("delete false only is not prune protection", func(t *testing.T) {
		t.Parallel()
		// Delete=false only gates cascade delete of the whole Application
		// (shouldBeDeleted in appcontroller.go); gitops-engine's own prune
		// path never looks at it, so this must NOT be treated as protection
		// here, a resource with only Delete=false is still pruned normally.
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "PersistentVolumeClaim",
			"metadata": map[string]any{
				"name":      "data-my-sts-0",
				"namespace": "default",
				"annotations": map[string]any{
					synccommon.AnnotationSyncOptions: "Delete=false",
				},
			},
		}}
		assert.False(t, liveObjHasPruneFalse(obj))
	})
}

func TestIsPruningDisabledForObj(t *testing.T) {
	t.Parallel()

	// Delegates to gitops-engine's own sync.IsPruningDisabled; this test only
	// verifies the wiring at the controller boundary, not the annotation logic
	// itself (covered by gitops-engine's own tests).
	pruneFalse := synccommon.SyncValueFalse
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					synccommon.AnnotationSyncOptions: "Prune=false",
				},
			},
		},
	}
	assert.True(t, sync.IsPruningDisabled(obj, nil))
	assert.True(t, sync.IsPruningDisabled(&unstructured.Unstructured{}, &pruneFalse))
	assert.False(t, sync.IsPruningDisabled(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "x", "namespace": "ns"},
	}}, nil))
}

func TestObjRequiresPruneConfirmation(t *testing.T) {
	t.Parallel()

	// Same note as TestIsPruningDisabledForObj above: wiring check only.
	pruneConfirm := synccommon.SyncValueConfirm
	pruneFalse := synccommon.SyncValueFalse

	t.Run("object annotation confirm", func(t *testing.T) {
		t.Parallel()
		obj := &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{
				"annotations": map[string]any{
					synccommon.AnnotationSyncOptions: "Prune=confirm",
				},
			},
		}}
		assert.True(t, sync.ObjRequiresPruneConfirmation(obj, nil))
	})

	t.Run("default confirm", func(t *testing.T) {
		t.Parallel()
		assert.True(t, sync.ObjRequiresPruneConfirmation(&unstructured.Unstructured{}, &pruneConfirm))
	})

	t.Run("default false", func(t *testing.T) {
		t.Parallel()
		assert.False(t, sync.ObjRequiresPruneConfirmation(&unstructured.Unstructured{}, &pruneFalse))
	})
}

// strayResourceWarningTestManager builds a real *appStateManager (via the
// same fake-controller wiring the rest of this package's tests use) so
// warnUnprotectedStrayResources can be exercised end to end, including the
// real audit logger writing real Events against a fake Kubernetes client.
func strayResourceWarningTestManager(t *testing.T) (*appStateManager, *ApplicationController, *v1alpha1.Application) {
	t.Helper()
	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	sm, ok := ctrl.appStateManager.(*appStateManager)
	require.True(t, ok, "appStateManager must be the concrete *appStateManager for this test")
	return sm, ctrl, app
}

func strayLiveObj(name string, annotations map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]any{
			"name":      name,
			"namespace": "default",
		},
	}}
	if len(annotations) > 0 {
		obj.SetAnnotations(annotations)
	}
	return obj
}

func TestWarnUnprotectedStrayResources_emitsForStrayWithoutAnnotation(t *testing.T) {
	t.Parallel()

	sm, ctrl, app := strayResourceWarningTestManager(t)
	logEntry := log.NewEntry(log.StandardLogger())

	unprotected := strayLiveObj("data-my-sts-0", nil)
	pruneProtected := strayLiveObj("data-my-sts-1", map[string]string{
		synccommon.AnnotationSyncOptions: "Prune=false",
	})
	// Delete=false alone does not stop gitops-engine from pruning this
	// resource in a normal sync, so it must still warn (see
	// liveObjHasPruneFalse's comment and TestLiveObjHasPruneFalse above).
	deleteFalseOnly := strayLiveObj("data-my-sts-2", map[string]string{
		synccommon.AnnotationSyncOptions: "Delete=false",
	})
	notStray := strayLiveObj("data-my-sts-3", nil)
	notStrayTarget := notStray.DeepCopy()

	reconciliationResult := sync.ReconciliationResult{
		Live:   []*unstructured.Unstructured{unprotected, pruneProtected, deleteFalseOnly, notStray},
		Target: []*unstructured.Unstructured{nil, nil, nil, notStrayTarget},
	}

	sm.warnUnprotectedStrayResources(t.Context(), app, logEntry, reconciliationResult, nil, false, nil)

	events, err := ctrl.kubeClientset.CoreV1().Events(app.Namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)

	assert.Len(t, events.Items, 2, "exactly the unprotected and the Delete=false-only resources should warn")
	warnedMessages := warnedMessages(events.Items)
	assert.Contains(t, warnedMessages, "data-my-sts-0")
	assert.Contains(t, warnedMessages, "data-my-sts-2")
	assert.NotContains(t, warnedMessages, "data-my-sts-1")
	assert.NotContains(t, warnedMessages, "data-my-sts-3")
}

func TestWarnUnprotectedStrayResources_skipsResourcesOutsideScopedSync(t *testing.T) {
	t.Parallel()

	sm, ctrl, app := strayResourceWarningTestManager(t)
	logEntry := log.NewEntry(log.StandardLogger())

	inScope := strayLiveObj("in-scope", nil)
	outOfScope := strayLiveObj("out-of-scope", nil)

	reconciliationResult := sync.ReconciliationResult{
		Live:   []*unstructured.Unstructured{inScope, outOfScope},
		Target: []*unstructured.Unstructured{nil, nil},
	}
	scopedResources := []v1alpha1.SyncOperationResource{
		{Kind: "PersistentVolumeClaim", Name: "in-scope", Namespace: "default"},
	}

	sm.warnUnprotectedStrayResources(t.Context(), app, logEntry, reconciliationResult, nil, false, scopedResources)

	events, err := ctrl.kubeClientset.CoreV1().Events(app.Namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, events.Items, 1, "only the resource actually included in the scoped sync should warn")
	assert.Contains(t, warnedMessages(events.Items), "in-scope")
}

func TestWarnUnprotectedStrayResources_capsEventVolume(t *testing.T) {
	t.Parallel()

	sm, ctrl, app := strayResourceWarningTestManager(t)
	logEntry := log.NewEntry(log.StandardLogger())

	var live, target []*unstructured.Unstructured
	total := maxStrayResourceWarningsPerSync + 5
	for i := range total {
		live = append(live, strayLiveObj(fmt.Sprintf("stray-%d", i), nil))
		target = append(target, nil)
	}
	reconciliationResult := sync.ReconciliationResult{Live: live, Target: target}

	sm.warnUnprotectedStrayResources(t.Context(), app, logEntry, reconciliationResult, nil, false, nil)

	events, err := ctrl.kubeClientset.CoreV1().Events(app.Namespace).List(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	assert.Len(t, events.Items, maxStrayResourceWarningsPerSync,
		"event volume must be capped even when far more stray resources qualify")
}

// warnedMessages joins every event's message into one string so tests can
// assert a resource name is (or isn't) mentioned with a plain Contains
// check; the audit event's InvolvedObject is always the Application
// itself, not the stray resource, so the resource identity only shows up
// in the message text.
func warnedMessages(events []corev1.Event) string {
	var sb strings.Builder
	for _, ev := range events {
		sb.WriteString(ev.Message)
		sb.WriteString("\n")
	}
	return sb.String()
}
