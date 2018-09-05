package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func unsortedManifest() []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
	}
}

func sortedManifest() []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "ConfigMap",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "PersistentVolume",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Service",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
				"kind":         "Pod",
			},
		},
		{
			Object: map[string]interface{}{
				"GroupVersion": apiv1.SchemeGroupVersion.String(),
			},
		},
	}
}

func TestSortKubernetesResourcesSuccessfully(t *testing.T) {
	sorted := SortManifestByKind(unsortedManifest())
	expectedOrder := sortedManifest()
	assert.Equal(t, len(sorted), len(expectedOrder))
	for i, sorted := range sorted {
		assert.Equal(t, expectedOrder[i], sorted)
	}

}

func TestSortManifestHandleNil(t *testing.T) {
	service := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"GroupVersion": apiv1.SchemeGroupVersion.String(),
			"kind":         "Service",
		},
	}
	manifest := []*unstructured.Unstructured{
		nil,
		service,
	}
	sortedManifest := SortManifestByKind(manifest)
	assert.Equal(t, service, sortedManifest[0])
	assert.Nil(t, sortedManifest[1])

}
