package account

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Context implements the "given" part of given/when/then.
// It embeds fixture.TestState to provide test-specific state that enables parallel test execution.
type Context struct {
	*fixture.TestState
	// accountName is the name of the account being tested (shadows TestState.Name())
	accountName string
	project     string
}

func Given(t *testing.T) *Context {
	t.Helper()
	state := fixture.EnsureCleanState(t)
	return &Context{TestState: state, accountName: state.Name()}
}

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
// Use this when you need multiple fixture contexts within the same test.
// For backward compatibility, also accepts *testing.T (deprecated - pass a TestContext instead).
func GivenWithSameState(ctxOrT any) *Context {
	var state *fixture.TestState
	switch v := ctxOrT.(type) {
	case *testing.T:
		v.Helper()
		state = fixture.GetTestState(v)
	case fixture.TestContext:
		v.T().Helper()
		state = v.(*fixture.TestState)
	default:
		panic("GivenWithSameState: expected *testing.T or fixture.TestContext")
	}
	return &Context{TestState: state, accountName: state.Name()}
}

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}

func (c *Context) GetName() string {
	return c.accountName
}

// Name sets the account name for this context
func (c *Context) Name(name string) *Context {
	c.accountName = name
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Actions{context: c}
}
