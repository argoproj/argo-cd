package sync

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

type unknownResourceInfoProvider struct{}

func (e *unknownResourceInfoProvider) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return false, errors.New("unknown")
}

func TestReconcileWithUnknownDiscoveryDataForClusterScopedResources(t *testing.T) {
	targetObjs := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": "my-namespace",
				},
			},
		},
	}

	liveNS := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]any{
				"name": "my-namespace",
				"uid":  "c99ff56d-1921-495d-8512-d66cdfcb5740",
			},
		},
	}
	liveObjByKey := map[kube.ResourceKey]*unstructured.Unstructured{
		kube.NewResourceKey("", "Namespace", "", "my-namespace"): liveNS,
	}

	result := Reconcile(targetObjs, liveObjByKey, "some-namespace", &unknownResourceInfoProvider{})
	require.Len(t, result.Target, 1)
	require.Equal(t, result.Target[0], targetObjs[0])
	require.Len(t, result.Live, 1)
	require.Equal(t, result.Live[0], liveNS)
}
