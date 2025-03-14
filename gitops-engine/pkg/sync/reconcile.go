package sync

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	hookutil "github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
)

func splitHooks(target []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured) {
	targetObjs := make([]*unstructured.Unstructured, 0)
	hooks := make([]*unstructured.Unstructured, 0)
	for _, obj := range target {
		if obj == nil || ignore.Ignore(obj) {
			continue
		}
		if hookutil.IsHook(obj) {
			hooks = append(hooks, obj)
		} else {
			targetObjs = append(targetObjs, obj)
		}
	}
	return targetObjs, hooks
}

// dedupLiveResources handles removes live resource duplicates with the same UID. Duplicates are created in a separate resource groups.
// E.g. apps/Deployment produces duplicate in extensions/Deployment, authorization.openshift.io/ClusterRole produces duplicate in rbac.authorization.k8s.io/ClusterRole etc.
// The method removes such duplicates unless it was defined in git ( exists in target resources list ). At least one duplicate stays.
// If non of duplicates are in git at random one stays
func dedupLiveResources(targetObjs []*unstructured.Unstructured, liveObjsByKey map[kubeutil.ResourceKey]*unstructured.Unstructured) {
	targetObjByKey := make(map[kubeutil.ResourceKey]*unstructured.Unstructured)
	for i := range targetObjs {
		targetObjByKey[kubeutil.GetResourceKey(targetObjs[i])] = targetObjs[i]
	}
	liveObjsById := make(map[types.UID][]*unstructured.Unstructured)
	for k := range liveObjsByKey {
		obj := liveObjsByKey[k]
		if obj != nil {
			liveObjsById[obj.GetUID()] = append(liveObjsById[obj.GetUID()], obj)
		}
	}
	for id := range liveObjsById {
		objs := liveObjsById[id]

		if len(objs) > 1 {
			duplicatesLeft := len(objs)
			for i := range objs {
				obj := objs[i]
				resourceKey := kubeutil.GetResourceKey(obj)
				if _, ok := targetObjByKey[resourceKey]; !ok {
					delete(liveObjsByKey, resourceKey)
					duplicatesLeft--
					if duplicatesLeft == 1 {
						break
					}
				}
			}
		}
	}
}

type ReconciliationResult struct {
	Live   []*unstructured.Unstructured
	Target []*unstructured.Unstructured
	Hooks  []*unstructured.Unstructured
}

func Reconcile(targetObjs []*unstructured.Unstructured, liveObjByKey map[kubeutil.ResourceKey]*unstructured.Unstructured, namespace string, resInfo kubeutil.ResourceInfoProvider) ReconciliationResult {
	targetObjs, hooks := splitHooks(targetObjs)
	dedupLiveResources(targetObjs, liveObjByKey)

	managedLiveObj := make([]*unstructured.Unstructured, len(targetObjs))
	for i, obj := range targetObjs {
		gvk := obj.GroupVersionKind()

		ns := text.FirstNonEmpty(obj.GetNamespace(), namespace)

		namespaced, err := resInfo.IsNamespaced(gvk.GroupKind())
		unknownScope := err != nil

		var keysToCheck []kubeutil.ResourceKey
		// If we get an error, we don't know whether the resource is namespaced. So we need to check for both in the
		// live objects. If we don't check for both, then we risk missing the object and deleting it.
		if namespaced || unknownScope {
			keysToCheck = append(keysToCheck, kubeutil.NewResourceKey(gvk.Group, gvk.Kind, ns, obj.GetName()))
		}
		if !namespaced || unknownScope {
			keysToCheck = append(keysToCheck, kubeutil.NewResourceKey(gvk.Group, gvk.Kind, "", obj.GetName()))
		}

		found := false
		for _, key := range keysToCheck {
			if liveObj, ok := liveObjByKey[key]; ok {
				managedLiveObj[i] = liveObj
				delete(liveObjByKey, key)
				found = true
				break
			}
		}
		if !found {
			managedLiveObj[i] = nil
		}
	}

	for _, obj := range liveObjByKey {
		targetObjs = append(targetObjs, nil)
		managedLiveObj = append(managedLiveObj, obj)
	}
	return ReconciliationResult{
		Target: targetObjs,
		Hooks:  hooks,
		Live:   managedLiveObj,
	}
}
