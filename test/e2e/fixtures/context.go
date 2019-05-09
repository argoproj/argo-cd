package fixtures

import (
	"testing"

	. "github.com/argoproj/argo-cd/common"
)

type Context struct {
	fixture    *Fixture
	t          *testing.T
	path       string
	name       string
	destServer string
	env        string
	parameters []string
}

func Given(f *Fixture, t *testing.T) *Context {
	f.EnsureCleanState()
	return &Context{f, t, "", "", "", KubernetesInternalAPIServerAddr, nil}
}

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) Name(name string) *Context {
	c.name = name
	return c
}

func (c *Context) DestServer(destServer string) *Context {
	c.destServer = destServer
	return c
}

func (c *Context) Env(env string) *Context {
	c.env = env
	return c
}

func (c *Context) Parameter(parameter string) *Context {
	c.parameters = append(c.parameters, parameter)
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actionable {
	return &Actionable{c}
}
