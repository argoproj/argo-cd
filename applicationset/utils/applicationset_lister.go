package utils

import (
	"context"

	"k8s.io/apimachinery/pkg/labels"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	listers "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
)

// Implements AppsetLister interface with controller-runtime client
type AppsetLister struct {
	Client ctrlclient.Client
}

func NewAppsetLister(client ctrlclient.Client) listers.ApplicationSetLister {
	return &AppsetLister{Client: client}
}

func (l *AppsetLister) List(_ labels.Selector) (ret []*v1alpha1.ApplicationSet, err error) {
	return clientListAppsets(l.Client, ctrlclient.ListOptions{})
}

// ApplicationSets returns an object that can list and get ApplicationSets.
func (l *AppsetLister) ApplicationSets(namespace string) listers.ApplicationSetNamespaceLister {
	return &appsetNamespaceLister{
		Client:    l.Client,
		Namespace: namespace,
	}
}

// Implements ApplicationSetNamespaceLister
type appsetNamespaceLister struct {
	Client    ctrlclient.Client
	Namespace string
}

func (n *appsetNamespaceLister) List(_ labels.Selector) (ret []*v1alpha1.ApplicationSet, err error) {
	return clientListAppsets(n.Client, ctrlclient.ListOptions{Namespace: n.Namespace})
}

func (n *appsetNamespaceLister) Get(_ string) (*v1alpha1.ApplicationSet, error) {
	appset := v1alpha1.ApplicationSet{}
	err := n.Client.Get(context.TODO(), ctrlclient.ObjectKeyFromObject(&appset), &appset)
	return &appset, err
}

func clientListAppsets(client ctrlclient.Client, listOptions ctrlclient.ListOptions) (ret []*v1alpha1.ApplicationSet, err error) {
	var appsetlist v1alpha1.ApplicationSetList
	var results []*v1alpha1.ApplicationSet

	err = client.List(context.TODO(), &appsetlist, &listOptions)

	if err == nil {
		for _, appset := range appsetlist.Items {
			results = append(results, appset.DeepCopy())
		}
	}

	return results, err
}
