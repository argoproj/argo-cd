package app

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

// this implements the "then" part of given/when/then
type Consequences struct {
	context    *Context
	actionable *Actions
}

func (c *Consequences) Expect(e Expectation) *Consequences {
	var err error
	for start := time.Now(); time.Since(start) < 30*time.Second; time.Sleep(3 * time.Second) {
		state, message := e(c)
		log.WithFields(log.Fields{"message": message, "state": state}).Info("polling for expectation")
		switch state {
		case succeeded:
			return c
		case failed:
			c.context.t.Error(message)
			return c
		}
	}
	c.context.t.Error(err)
	return c
}

func (c *Consequences) And(block func(app *Application)) *Consequences {
	block(c.app())
	return c
}

func (c *Consequences) When() *Actions {
	return c.actionable
}

func (c *Consequences) app() *Application {
	app, err := c.get()
	assert.NoError(c.context.t, err)
	return app
}

func (c *Consequences) get() (*Application, error) {
	return c.context.fixture.AppClientset.ArgoprojV1alpha1().Applications(c.context.fixture.ArgoCDNamespace).Get(c.context.name, v1.GetOptions{})
}

func (c *Consequences) resource(name string) ResourceStatus {
	for _, r := range c.app().Status.Resources {
		if r.Name == name {
			return r
		}
	}
	return ResourceStatus{
		Health: &HealthStatus{Status: HealthStatusUnknown},
	}
}
