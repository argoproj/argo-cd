// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// ApplicationSetLister helps list ApplicationSets.
// All objects returned here must be treated as read-only.
type ApplicationSetLister interface {
	// List lists all ApplicationSets in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ApplicationSet, err error)
	// ApplicationSets returns an object that can list and get ApplicationSets.
	ApplicationSets(namespace string) ApplicationSetNamespaceLister
	ApplicationSetListerExpansion
}

// applicationSetLister implements the ApplicationSetLister interface.
type applicationSetLister struct {
	listers.ResourceIndexer[*v1alpha1.ApplicationSet]
}

// NewApplicationSetLister returns a new ApplicationSetLister.
func NewApplicationSetLister(indexer cache.Indexer) ApplicationSetLister {
	return &applicationSetLister{listers.New[*v1alpha1.ApplicationSet](indexer, v1alpha1.Resource("applicationset"))}
}

// ApplicationSets returns an object that can list and get ApplicationSets.
func (s *applicationSetLister) ApplicationSets(namespace string) ApplicationSetNamespaceLister {
	return applicationSetNamespaceLister{listers.NewNamespaced[*v1alpha1.ApplicationSet](s.ResourceIndexer, namespace)}
}

// ApplicationSetNamespaceLister helps list and get ApplicationSets.
// All objects returned here must be treated as read-only.
type ApplicationSetNamespaceLister interface {
	// List lists all ApplicationSets in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ApplicationSet, err error)
	// Get retrieves the ApplicationSet from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ApplicationSet, error)
	ApplicationSetNamespaceListerExpansion
}

// applicationSetNamespaceLister implements the ApplicationSetNamespaceLister
// interface.
type applicationSetNamespaceLister struct {
	listers.ResourceIndexer[*v1alpha1.ApplicationSet]
}
