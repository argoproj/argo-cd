package notification

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "given" part of given/when/then
type Context struct {
	t *testing.T
}

func Given(t *testing.T) *Context {
	t.Helper()
	fixture.EnsureCleanState(t)
	return &Context{t: t}
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	// Account for batch events processing (set to 1ms in e2e tests)
	time.Sleep(5 * time.Millisecond)
	return &Actions{context: c}
}
