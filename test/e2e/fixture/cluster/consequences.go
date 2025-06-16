package cluster

import (
	"context"
	"fmt"

	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) Expect() *Consequences {
	return c
}

func (c *Consequences) And(block func(cluster *v1alpha1.Cluster, err error)) *Consequences {
	c.context.t.Helper()
	block(c.cluster())
	return c
}

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

func (c *Consequences) cluster() (*v1alpha1.Cluster, error) {
	app, err := c.get()
	return app, err
}

func (c *Consequences) get() (*v1alpha1.Cluster, error) {
	_, clusterClient, _ := fixture.ArgoCDClientset.NewClusterClient()

	cluster, _ := clusterClient.List(context.Background(), &clusterpkg.ClusterQuery{})
	for i := range cluster.Items {
		if cluster.Items[i].Server == c.context.server {
			return &cluster.Items[i], nil
		}
	}

	return nil, fmt.Errorf("cluster not found")
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}
