package applicationsets

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/applicationsets/utils"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/gpgkeys"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
)

// Context implements the "given" part of given/when/then
type Context struct {
	t *testing.T

	// name is the ApplicationSet's name, created by a Create action
	name              string
	namespace         string
	switchToNamespace utils.ExternalNamespace
	project           string
	path              string
}

func Given(t *testing.T) *Context {
	t.Helper()
	utils.EnsureCleanState(t)
	return &Context{t: t}
}

func (c *Context) When() *Actions {
	time.Sleep(fixture.WhenThenSleepInterval)
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

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) GPGPublicKeyAdded() *Context {
	gpgkeys.AddGPGPublicKey(c.t)
	return c
}

func (c *Context) HTTPSInsecureRepoURLAdded(project string) *Context {
	repos.AddHTTPSRepo(c.t, true, true, project, fixture.RepoURLTypeHTTPS)
	return c
}
