package project

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
	// seconds
	timeout int
	name    string
	project string
}

func Given(t *testing.T) *Context {
	t.Helper()
	fixture.EnsureCleanState(t)
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{t: t, name: fixture.Name(), timeout: timeout}
}

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}

func (c *Context) GetName() string {
	return c.name
}

func (c *Context) Name(name string) *Context {
	c.name = name
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	return &Actions{context: c}
}
