package normalizers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestNormalizeAsNormalizer(t *testing.T) {
	overrides := map[string]v1alpha1.ResourceOverride{
		"mygroup/AnotherSecret": {
			NormalizeAs: "Secret",
		},
	}
	normalizer := NewNormalizeAsNormalizer(overrides)

	un := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "mygroup/v1",
			"kind":       "AnotherSecret",
			"metadata": map[string]any{
				"name": "test",
			},
		},
	}

	require.NoError(t, normalizer.Normalize(un))

	annotations := un.GetAnnotations()
	assert.NotNil(t, annotations)
	assert.Equal(t, "Secret", annotations[common.AnnotationKeyNormalizeAs])
}

func TestNormalizeAsNormalizer_NoOverride(t *testing.T) {
	overrides := map[string]v1alpha1.ResourceOverride{
		"mygroup/SomeOtherKind": {
			NormalizeAs: "Secret",
		},
	}
	normalizer := NewNormalizeAsNormalizer(overrides)

	un := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "mygroup/v1",
			"kind":       "AnotherSecret",
			"metadata": map[string]any{
				"name": "test",
			},
		},
	}

	require.NoError(t, normalizer.Normalize(un))

	annotations := un.GetAnnotations()
	assert.Empty(t, annotations[common.AnnotationKeyNormalizeAs])
}
