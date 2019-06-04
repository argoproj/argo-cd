package app

import (
	"testing"

	. "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// this implements the "given" part of given/when/then
type Context struct {
	t          *testing.T
	path       string
	name       string
	destServer string
	env        string
	parameters []string
	namePrefix string
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState()
	return &Context{t: t, destServer: KubernetesInternalAPIServerAddr, name: fixture.Name()}
}

func (c *Context) Repo(url string) *Context {
	fixture.SetRepoURL(url)
	return c
}

func (c *Context) Path(path string) *Context {
	c.path = path
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

func (c *Context) NamePrefix(namePrefix string) *Context {
	c.namePrefix = namePrefix
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	return &Actions{context: c}
}
