package session

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Context implements the "given" part of given/when/then for session/logout tests.
type Context struct {
	*fixture.TestState
}

func Given(t *testing.T) *Context {
	t.Helper()
	state := fixture.EnsureCleanState(t)
	return &Context{TestState: state}
}

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
func GivenWithSameState(ctx fixture.TestContext) *Context {
	ctx.T().Helper()
	return &Context{TestState: fixture.NewTestStateFromContext(ctx)}
}

func (c *Context) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Actions{context: c}
}
