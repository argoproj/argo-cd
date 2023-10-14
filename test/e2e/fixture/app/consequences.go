package app

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context *Context
	actions *Actions
	timeout int
}

func (c *Consequences) Expect(e Expectation) *Consequences {
	// this invocation makes sure this func is not reported as the cause of the failure - we are a "test helper"
	c.context.t.Helper()
	var message string
	var state state
	timeout := time.Duration(c.timeout) * time.Second
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

func (c *Consequences) AndAction(block func()) *Consequences {
	c.context.t.Helper()
	block()
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
	return fixture.AppClientset.ArgoprojV1alpha1().Applications(c.context.AppNamespace()).Get(context.Background(), c.context.AppName(), v1.GetOptions{})
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

func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.t.Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}
