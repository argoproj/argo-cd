package notification

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

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
// Use this when you need multiple fixture contexts within the same test.
func GivenWithSameState(ctx fixture.TestContext) *Context {
	ctx.T().Helper()
	return &Context{TestState: fixture.NewTestStateFromContext(ctx)}
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Actions{context: c}
}
