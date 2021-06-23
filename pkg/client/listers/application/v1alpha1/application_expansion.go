package v1alpha1

import (
	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
)

// ApplicationListerExpansion allows custom methods to be added to
// ApplicationLister.
type ApplicationListerExpansion interface{}

// ApplicationNamespaceListerExpansion allows custom methods to be added to
// ApplicationNamespaceLister.
type ApplicationNamespaceListerExpansion interface {
	NamespacedGet(appName string) (*v1alpha1.Application, error)
}

// NamespacedGet retrieves the Application from the indexer for a given namespace and name.
// Name must be specified as "namespace/name".
// It will use the namespace specified in name, instead the one specified in the lister.
func (s applicationNamespaceLister) NamespacedGet(name string) (*v1alpha1.Application, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("application"), name)
	}
	return obj.(*v1alpha1.Application), nil
}
