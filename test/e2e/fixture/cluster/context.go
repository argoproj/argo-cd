package cluster

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Context implements the "given" part of given/when/then.
// It embeds fixture.TestState to provide test-specific state that enables parallel test execution.
type Context struct {
	*fixture.TestState
	// clusterName is the name of the cluster being tested (shadows TestState.Name())
	clusterName string
	project     string
	server      string
	upsert      bool
	namespaces  []string
	bearerToken string
}

func Given(t *testing.T) *Context {
	t.Helper()
	state := fixture.EnsureCleanState(t)
	return GivenWithSameState(state)
}

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
// Use this when you need multiple fixture contexts within the same test.
// For backward compatibility, also accepts *testing.T (deprecated - pass a TestContext instead).
func GivenWithSameState(ctxOrT any) *Context {
	var state *fixture.TestState
	switch v := ctxOrT.(type) {
	case *testing.T:
		v.Helper()
		state = fixture.NewTestState(v)
	case fixture.TestContext:
		v.T().Helper()
		state = v.(*fixture.TestState)
	default:
		panic("GivenWithSameState: expected *testing.T or fixture.TestContext")
	}
	return &Context{TestState: state, clusterName: state.Name(), project: "default"}
}

func (c *Context) GetName() string {
	return c.clusterName
}

// Name sets the cluster name for this context
func (c *Context) Name(name string) *Context {
	c.clusterName = name
	return c
}

func (c *Context) Server(server string) *Context {
	c.server = server
	return c
}

func (c *Context) Namespaces(namespaces []string) *Context {
	c.namespaces = namespaces
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

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}

func (c *Context) BearerToken(bearerToken string) *Context {
	c.bearerToken = bearerToken
	return c
}

func (c *Context) Upsert(upsert bool) *Context {
	c.upsert = upsert
	return c
}
