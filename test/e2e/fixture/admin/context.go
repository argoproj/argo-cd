package admin

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Context implements the "given" part of given/when/then.
// It embeds fixture.TestState to provide test-specific state that enables parallel test execution.
type Context struct {
	*fixture.TestState
}

func Given(t *testing.T) *Context {
	t.Helper()
	state := fixture.EnsureCleanState(t)
	return &Context{TestState: state}
}

func GivenWithSameState(t *testing.T) *Context {
	t.Helper()
	return &Context{TestState: fixture.GetTestState(t)}
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Actions{context: c}
}
