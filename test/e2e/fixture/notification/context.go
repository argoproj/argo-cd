package notification

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return &Context{t: t}
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	return &Actions{context: c}
}
