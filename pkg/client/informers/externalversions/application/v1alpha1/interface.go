// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// AppProjects returns a AppProjectInformer.
	AppProjects() AppProjectInformer
	// Applications returns a ApplicationInformer.
	Applications() ApplicationInformer
	// ApplicationSets returns a ApplicationSetInformer.
	ApplicationSets() ApplicationSetInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// AppProjects returns a AppProjectInformer.
func (v *version) AppProjects() AppProjectInformer {
	return &appProjectInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// Applications returns a ApplicationInformer.
func (v *version) Applications() ApplicationInformer {
	return &applicationInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// ApplicationSets returns a ApplicationSetInformer.
func (v *version) ApplicationSets() ApplicationSetInformer {
	return &applicationSetInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
