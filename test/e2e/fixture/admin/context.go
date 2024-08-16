package admin

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
	// seconds
	timeout int
	name    string
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

func GivenWithSameState(t *testing.T) *Context {
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 20, 0, 180)
	return &Context{
		t:       t,
		name:    fixture.Name(),
		timeout: timeout,
	}
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	return &Actions{context: c}
}
