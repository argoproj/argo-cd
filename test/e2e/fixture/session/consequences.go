package session

import (
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Consequences implements the "then" part of given/when/then for session/logout tests.
type Consequences struct {
	context *Context
	actions *Actions
}

// ActionShouldSucceed asserts that the last action completed without error.
func (c *Consequences) ActionShouldSucceed() *Consequences {
	c.context.T().Helper()
	if c.actions.lastError != nil {
		c.context.T().Errorf("expected action to succeed, got error: %v", c.actions.lastError)
	}
	return c
}

// ActionShouldFail asserts that the last action returned an error and passes
// it to the callback for further inspection.
func (c *Consequences) ActionShouldFail(block func(err error)) *Consequences {
	c.context.T().Helper()
	if c.actions.lastError == nil {
		c.context.T().Error("expected action to fail, but it succeeded")
		return c
	}
	block(c.actions.lastError)
	return c
}

// AndCLIOutput passes the CLI output and error from the last action to the
// callback for custom assertions.
func (c *Consequences) AndCLIOutput(block func(output string, err error)) *Consequences {
	c.context.T().Helper()
	block(c.actions.lastOutput, c.actions.lastError)
	return c
}

// Given returns the Context to allow chaining back to setup.
func (c *Consequences) Given() *Context {
	return c.context
}

// When returns the Actions to allow chaining back to actions.
func (c *Consequences) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return c.actions
}
