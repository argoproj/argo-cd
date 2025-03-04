package application

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util"

	"k8s.io/apimachinery/pkg/labels"
)

// deepCopyApplicationLister wraps an ApplicationLister and returns deep copies of the applications.
type deepCopyApplicationLister struct {
	applisters.ApplicationLister
}

// List lists all Applications in the indexer and returns deep copies.
func (d *deepCopyApplicationLister) List(selector labels.Selector) ([]*v1alpha1.Application, error) {
	apps, err := d.ApplicationLister.List(selector)
	if err != nil {
		return nil, err
	}
	deepCopiedApps := make([]*v1alpha1.Application, len(apps))
	for i, app := range apps {
		deepCopiedApps[i] = app.DeepCopy()
	}
	return deepCopiedApps, nil
}

// Applications return an object that can list and get Applications and returns deep copies.
func (d *deepCopyApplicationLister) Applications(namespace string) applisters.ApplicationNamespaceLister {
	return &deepCopyApplicationNamespaceLister{
		ApplicationNamespaceLister: d.ApplicationLister.Applications(namespace),
	}
}

// deepCopyApplicationNamespaceLister wraps an ApplicationNamespaceLister and returns deep copies of the applications.
type deepCopyApplicationNamespaceLister struct {
	applisters.ApplicationNamespaceLister
}

// List lists all Applications in the indexer for a given namespace and returns deep copies.
func (d *deepCopyApplicationNamespaceLister) List(selector labels.Selector) ([]*v1alpha1.Application, error) {
	apps, err := d.ApplicationNamespaceLister.List(selector)
	if err != nil {
		return nil, err
	}
	deepCopiedApps := util.SliceCopy(apps)
	return deepCopiedApps, nil
}

// Get retrieves the Application from the indexer for a given namespace and name and returns a deep copy.
func (d *deepCopyApplicationNamespaceLister) Get(name string) (*v1alpha1.Application, error) {
	app, err := d.ApplicationNamespaceLister.Get(name)
	if err != nil {
		return nil, err
	}
	return app.DeepCopy(), nil
}
