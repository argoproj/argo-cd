package project

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
	// seconds
	timeout                    int
	name                       string
	destination                string
	destinationServiceAccounts []string
	repos                      []string
	sourceNamespaces           []string
}

func Given(t *testing.T) *Context {
	t.Helper()
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

func GivenWithSameState(t *testing.T) *Context {
	t.Helper()
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{t: t, name: fixture.Name(), timeout: timeout}
}

func (c *Context) GetName() string {
	return c.name
}

func (c *Context) Name(name string) *Context {
	c.name = name
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
