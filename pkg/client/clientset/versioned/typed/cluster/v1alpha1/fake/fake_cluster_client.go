package fake

import (
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeArgoprojV1alpha1 struct {
	*testing.Fake
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeArgoprojV1alpha1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
