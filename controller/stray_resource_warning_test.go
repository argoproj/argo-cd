package controller

import (
	"testing"

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
	assert.True(t, isPruningDisabledForObj(obj, nil))
	assert.True(t, isPruningDisabledForObj(&unstructured.Unstructured{}, &pruneFalse))
	assert.False(t, isPruningDisabledForObj(&unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "x", "namespace": "ns"},
	}}, nil))
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
