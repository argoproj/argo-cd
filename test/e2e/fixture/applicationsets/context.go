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
	*fixture.TestState

	namespace         string
	switchToNamespace utils.ExternalNamespace
	path              string
}

func Given(t *testing.T) *Context {
	t.Helper()

	state := fixture.EnsureCleanState(t)

	// TODO: Appset EnsureCleanState specific logic should be moved to the main EnsureCleanState function (https://github.com/argoproj/argo-cd/issues/24307)
	utils.EnsureCleanState(t)

	return &Context{TestState: state}
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

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) GPGPublicKeyAdded() *Context {
	gpgkeys.AddGPGPublicKey(c.T())
	return c
}

func (c *Context) HTTPSInsecureRepoURLAdded(project string) *Context {
	repos.AddHTTPSRepo(c.T(), true, true, project, fixture.RepoURLTypeHTTPS)
	return c
}

// PushOCIArtifact pushes an OCI artifact to the local registry for testing
func (c *Context) PushOCIArtifact(pathName, tag, pushPath string) *Context {
	c.T().Helper()
	repos.PushImageToOCIRegistry(c.T(), pathName, tag, pushPath)
	return c
}

// AddOCIRepository adds an OCI repository to Argo CD
func (c *Context) AddOCIRepository(name, imagePath string) *Context {
	c.T().Helper()
	repos.AddOCIRepo(c.T(), name, imagePath)
	return c
}
