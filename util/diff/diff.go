package diff

import (
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/ignore"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ComparisonObject represents a resource pair for comparison
type ComparisonObject struct {
	Key    kube.ResourceKey
	Live   *unstructured.Unstructured
	Target *unstructured.Unstructured
}

// getObjectMap builds a map of objects by resource key, filtering hooks, ignored resources, and secrets
func getObjectMap(objects []*unstructured.Unstructured) map[kube.ResourceKey]*unstructured.Unstructured {
	objectMap := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for i := range objects {
		obj := objects[i]
		if obj == nil {
			continue
		}

		// Skip hooks and ignored resources
		if hook.IsHook(obj) || ignore.Ignore(obj) {
			continue
		}

		key := kube.GetResourceKey(obj)
		objectMap[key] = obj
	}
	return objectMap
}

// GetComparisonObjects pairs target and live manifests by resource key
func GetComparisonObjects(
	targetManifests []*unstructured.Unstructured,
	liveManifests []*unstructured.Unstructured,
) []ComparisonObject {
	// Build map of target objects by key
	targetByKey := getObjectMap(targetManifests)
	liveByKey := getObjectMap(liveManifests)

	// Build result list by pairing live and target objects
	items := make([]ComparisonObject, 0)

	// Process live objects and match with targets
	for key := range liveByKey {
		items = append(items, ComparisonObject{
			Key:    key,
			Live:   liveByKey[key],
			Target: targetByKey[key],
		})
		delete(targetByKey, key)
	}
	// Add remaining target objects that don't have live counterparts
	for key := range targetByKey {
		items = append(items, ComparisonObject{
			Key:    key,
			Live:   nil,
			Target: targetByKey[key],
		})
	}

	return items
}
