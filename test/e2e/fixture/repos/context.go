package repos

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// this implements the "given" part of given/when/then
type Context struct {
	t       *testing.T
	path    string
	name    string
	project string
}

func Given(t *testing.T) *Context {
	t.Helper()
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

// GivenWithSameState skips cleaning state. Use this when you've already ensured you have a clean
// state in your test setup don't want to waste time by doing so again.
func GivenWithSameState(t *testing.T) *Context {
	t.Helper()
	return &Context{t: t, name: fixture.Name(), project: "default"}
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
	time.Sleep(fixture.WhenThenSleepInterval)
	return &Actions{context: c}
}

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}
