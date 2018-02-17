package fake

import (
	v1alpha1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeClusters implements ClusterInterface
type FakeClusters struct {
	Fake *FakeArgoprojV1alpha1
	ns   string
}

var clustersResource = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "clusters"}

var clustersKind = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Cluster"}

// Get takes name of the cluster, and returns the corresponding cluster object, and an error if there is any.
func (c *FakeClusters) Get(name string, options v1.GetOptions) (result *v1alpha1.Cluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(clustersResource, c.ns, name), &v1alpha1.Cluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cluster), err
}

// List takes label and field selectors, and returns the list of Clusters that match those selectors.
func (c *FakeClusters) List(opts v1.ListOptions) (result *v1alpha1.ClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(clustersResource, clustersKind, c.ns, opts), &v1alpha1.ClusterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ClusterList{}
	for _, item := range obj.(*v1alpha1.ClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested clusters.
func (c *FakeClusters) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(clustersResource, c.ns, opts))

}

// Create takes the representation of a cluster and creates it.  Returns the server's representation of the cluster, and an error, if there is any.
func (c *FakeClusters) Create(cluster *v1alpha1.Cluster) (result *v1alpha1.Cluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(clustersResource, c.ns, cluster), &v1alpha1.Cluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cluster), err
}

// Update takes the representation of a cluster and updates it. Returns the server's representation of the cluster, and an error, if there is any.
func (c *FakeClusters) Update(cluster *v1alpha1.Cluster) (result *v1alpha1.Cluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(clustersResource, c.ns, cluster), &v1alpha1.Cluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cluster), err
}

// Delete takes name of the cluster and deletes it. Returns an error if one occurs.
func (c *FakeClusters) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(clustersResource, c.ns, name), &v1alpha1.Cluster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeClusters) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(clustersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ClusterList{})
	return err
}

// Patch applies the patch and returns the patched cluster.
func (c *FakeClusters) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Cluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(clustersResource, c.ns, name, data, subresources...), &v1alpha1.Cluster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cluster), err
}
