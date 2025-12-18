package project

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Context implements the "given" part of given/when/then.
// It embeds fixture.TestState to provide test-specific state that enables parallel test execution.
type Context struct {
	*fixture.TestState

	destination                string
	destinationServiceAccounts []string
	repos                      []string
	sourceNamespaces           []string
}

func Given(t *testing.T) *Context {
	t.Helper()
	state := fixture.EnsureCleanState(t)
	return GivenWithSameState(state)
}

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
// Use this when you need multiple fixture contexts within the same test.
func GivenWithSameState(ctx fixture.TestContext) *Context {
	ctx.T().Helper()
	state := ctx.(*fixture.TestState)
	return &Context{TestState: state}
}

func (c *Context) GetName() string {
	return c.Name()
}

// SetName sets the project name for this context
func (c *Context) SetName(name string) *Context {
	c.TestState.SetName(name)
	return c
}

func (c *Context) Destination(destination string) *Context {
	c.destination = destination
	return c
}

func (c *Context) DestinationServiceAccounts(destinationServiceAccounts []string) *Context {
	c.destinationServiceAccounts = destinationServiceAccounts
	return c
}

func (c *Context) SourceRepositories(repos []string) *Context {
	c.repos = repos
	return c
}

func (c *Context) SourceNamespaces(namespaces []string) *Context {
	c.sourceNamespaces = namespaces
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
