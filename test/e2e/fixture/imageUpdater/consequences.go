package imageUpdater

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) app() *v1alpha1.Application {
	app, err := c.get()
	errors.CheckError(err)
	return app
}

func (c *Consequences) get() (*v1alpha1.Application, error) {
	return fixture.AppClientset.ArgoprojV1alpha1().Applications(c.context.AppNamespace()).Get(context.Background(), c.context.AppName(), v1.GetOptions{})
}

func (c *Consequences) When() *Actions {
	return c.actions
}
