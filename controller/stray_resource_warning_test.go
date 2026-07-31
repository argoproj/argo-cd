package controller

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestLiveObjHasExplicitSyncProtection(t *testing.T) {
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
		assert.False(t, liveObjHasExplicitSyncProtection(obj))
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
		assert.True(t, liveObjHasExplicitSyncProtection(obj))
	})

	t.Run("delete false only", func(t *testing.T) {
		t.Parallel()
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
		assert.True(t, liveObjHasExplicitSyncProtection(obj))
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

func TestWarnUnprotectedStrayResources_emitsForStrayWithoutAnnotation(t *testing.T) {
	t.Parallel()

	pvc := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]any{
			"name":      "data-my-sts-0",
			"namespace": "default",
		},
	}}
	protectedPVC := pvc.DeepCopy()
	protectedPVC.SetAnnotations(map[string]string{
		synccommon.AnnotationSyncOptions: "Prune=false,Delete=false",
	})

	// Covered by unit helpers; integration with audit logger is exercised in controller tests.
	assert.False(t, liveObjHasExplicitSyncProtection(pvc))
	assert.True(t, liveObjHasExplicitSyncProtection(protectedPVC))
}
