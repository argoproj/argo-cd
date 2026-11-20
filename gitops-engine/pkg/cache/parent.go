package cache

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// Annotations defining an additional parent in the Argo CD resource tree.
const (
	AnnotationKeyParentAPIVersion = "argocd.argoproj.io/parentApiVersion"
	AnnotationKeyParentKind       = "argocd.argoproj.io/parentKind"
	AnnotationKeyParentName       = "argocd.argoproj.io/parentName"
	AnnotationKeyParentNamespace  = "argocd.argoproj.io/parentNamespace"
)

// parentReference extracts an optional display relationship, not Kubernetes ownership.
func parentReference(un *unstructured.Unstructured) (*corev1.ObjectReference, error) {
	annotations := un.GetAnnotations()
	apiVersion, kind, name := annotations[AnnotationKeyParentAPIVersion], annotations[AnnotationKeyParentKind], annotations[AnnotationKeyParentName]
	_, namespaceSet := annotations[AnnotationKeyParentNamespace]
	if apiVersion == "" && kind == "" && name == "" && !namespaceSet {
		return nil, nil
	}
	if apiVersion == "" || kind == "" || name == "" {
		return nil, errors.New("parentApiVersion, parentKind and parentName must all be set")
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	validVersion := len(validation.IsDNS1035Label(gv.Version)) == 0
	validGroup := gv.Group == "" || len(validation.IsDNS1123Subdomain(gv.Group)) == 0
	if err != nil || apiVersion != gv.String() || !validVersion || !validGroup {
		return nil, fmt.Errorf("invalid parentApiVersion %q", apiVersion)
	}
	namespace := un.GetNamespace()
	if namespaceSet {
		namespace = annotations[AnnotationKeyParentNamespace]
	}
	if kube.NewResourceKey(gv.Group, kind, namespace, name) == kube.GetResourceKey(un) {
		return nil, errors.New("a resource cannot be its own custom parent")
	}
	return &corev1.ObjectReference{APIVersion: apiVersion, Kind: kind, Namespace: namespace, Name: name}, nil
}

func parentKey(ref *corev1.ObjectReference) kube.ResourceKey {
	return kube.NewResourceKey(ref.GroupVersionKind().Group, ref.Kind, ref.Namespace, ref.Name)
}

// updateCustomParentIndex is called under the store's write lock.
func (c *store) updateCustomParentIndex(oldRes, newRes *Resource) {
	if oldRes != nil && oldRes.ParentRef != nil {
		key := parentKey(oldRes.ParentRef)
		delete(c.customParentToChildren[key], oldRes.ResourceKey())
		if len(c.customParentToChildren[key]) == 0 {
			delete(c.customParentToChildren, key)
		}
	}
	if newRes == nil || newRes.ParentRef == nil {
		return
	}
	if c.customParentToChildren == nil {
		c.customParentToChildren = make(map[kube.ResourceKey]map[kube.ResourceKey]struct{})
	}
	key := parentKey(newRes.ParentRef)
	if c.customParentToChildren[key] == nil {
		c.customParentToChildren[key] = make(map[kube.ResourceKey]struct{})
	}
	c.customParentToChildren[key][newRes.ResourceKey()] = struct{}{}
}
