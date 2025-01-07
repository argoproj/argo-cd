package repos

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/util/env"
)

// this implements the "given" part of given/when/then
type Context struct {
	t           *testing.T
	path        string
	repoURLType fixture.RepoURLType
	// seconds
	timeout int
	name    string
	project string
}

func Given(t *testing.T, sameState bool) *Context {
	if !sameState {
		fixture.EnsureCleanState(t)
	}
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{t: t, repoURLType: fixture.RepoURLTypeFile, name: fixture.Name(), timeout: timeout, project: "default"}
}

func (c *Context) RepoURLType(urlType fixture.RepoURLType) *Context {
	c.repoURLType = urlType
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
	// in case any settings have changed, pause for 1s, not great, but fine
	time.Sleep(1 * time.Second)
	return &Actions{context: c}
}

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}
