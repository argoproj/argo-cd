package cache

import (
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type node struct {
	resourceVersion string
	ref             v1.ObjectReference
	ownerRefs       []metav1.OwnerReference
	children        map[string]*node
	parents         map[string]*node
	tags            []string
	appName         string
	resource        *unstructured.Unstructured
}

// returns true if node has application label and has no parents
func (o *node) isAppRoot() bool {
	return o.appName != "" && len(o.parents) == 0
}

func (o *node) resourceKey() string {
	return kube.FormatResourceKey(o.ref.GroupVersionKind().Group, o.ref.Kind, o.ref.Namespace, o.ref.Name)
}

func (o *node) isParentOf(child *node) bool {
	if o.ref.Namespace != child.ref.Namespace {
		return false
	}
	ownerGvk := o.ref.GroupVersionKind()
	for _, ownerRef := range child.ownerRefs {
		if kube.FormatResourceKey(ownerGvk.Group, ownerRef.Kind, o.ref.Namespace, ownerRef.Name) == o.resourceKey() {
			return true
		}
	}

	return false
}

func (o *node) fillChildren(nodes map[string]*node) {
	for k, child := range nodes {
		if o.isParentOf(child) {
			delete(nodes, k)
			child.appName = o.appName
			child.parents[o.resourceKey()] = o
			o.children[child.resourceKey()] = child
			child.fillChildren(nodes)
		}
	}
}

func (o *node) childResourceNodes() appv1.ResourceNode {
	children := make([]appv1.ResourceNode, 0)
	for i := range o.children {
		children = append(children, o.children[i].childResourceNodes())
	}
	gv, err := schema.ParseGroupVersion(o.ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return appv1.ResourceNode{
		Name:      o.ref.Name,
		Group:     gv.Group,
		Version:   gv.Version,
		Kind:      o.ref.Kind,
		Namespace: o.ref.Namespace,
		Tags:      o.tags,
		Children:  children,
	}
}
