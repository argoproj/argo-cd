package cluster

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
	// seconds
	timeout     int
	name        string
	project     string
	server      string
	upsert      bool
	namespaces  []string
	bearerToken string
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

func GivenWithSameState(t *testing.T) *Context {
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{t: t, name: fixture.Name(), timeout: timeout, project: "default"}
}

func (c *Context) GetName() string {
	return c.name
}

func (c *Context) Name(name string) *Context {
	c.name = name
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
	// in case any settings have changed, pause for 1s, not great, but fine
	time.Sleep(1 * time.Second)
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
