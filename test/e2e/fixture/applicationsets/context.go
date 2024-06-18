package applicationsets

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/applicationsets/utils"
)

// Context implements the "given" part of given/when/then
type Context struct {
	t *testing.T

	// name is the ApplicationSet's name, created by a Create action
	name              string
	namespace         string
	switchToNamespace utils.ExternalNamespace
}

func Given(t *testing.T) *Context {
	utils.EnsureCleanState(t)
	return &Context{t: t}
}

func (c *Context) When() *Actions {
	// in case any settings have changed, pause for 1s, not great, but fine
	time.Sleep(1 * time.Second)
	return &Actions{context: c}
}

func (c *Context) Sleep(seconds time.Duration) *Context {
	time.Sleep(seconds * time.Second)
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}
