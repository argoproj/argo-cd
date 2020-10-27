package app

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util/errors"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
}

func (c *Consequences) Expect(e Expectation) *Consequences {
	// this invocation makes sure this func is not reported as the cause of the failure - we are a "test helper"
	c.context.t.Helper()
	var message string
	var state state
	timeout := time.Duration(15) * time.Second
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(3 * time.Second) {
		state, message = e(c)
		switch state {
		case succeeded:
			log.Infof("expectation succeeded: %s", message)
			return c
		case failed:
			c.context.t.Fatalf("failed expectation: %s", message)
			return c
		}
		log.Infof("pending: %s", message)
	}
	c.context.t.Fatal("timeout waiting for: " + message)
	return c
}

func (c *Consequences) And(block func(app *Application)) *Consequences {
	c.context.t.Helper()
	block(c.app())
	return c
}

func (c *Consequences) Given() *Context {
	return c.context
}

func (c *Consequences) When() *Actions {
	return c.actions
}

func (c *Consequences) app() *Application {
	app, err := c.get()
	errors.CheckError(err)
	return app
}

func (c *Consequences) get() (*Application, error) {
	return fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(context.Background(), c.context.name, v1.GetOptions{})
}

func (c *Consequences) resource(kind, name, namespace string) ResourceStatus {
	for _, r := range c.app().Status.Resources {
		if r.Kind == kind && r.Name == name && (namespace == "" || namespace == r.Namespace) {
			return r
		}
	}
	return ResourceStatus{
		Health: &HealthStatus{
			Status:  health.HealthStatusMissing,
			Message: "not found",
		},
	}
}
