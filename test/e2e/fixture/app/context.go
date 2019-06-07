package app

import (
	"testing"

	. "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

// this implements the "given" part of given/when/then
type Context struct {
	t                      *testing.T
	path                   string
	name                   string
	destServer             string
	env                    string
	parameters             []string
	namePrefix             string
	resource               string
	prune                  bool
	configManagementPlugin string
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return &Context{t: t, destServer: KubernetesInternalAPIServerAddr, name: fixture.Name(), prune: true}
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

// group:kind:name
func (c *Context) SelectedResource(resource string) *Context {
	c.resource = resource
	return c
}

func (c *Context) NamePrefix(namePrefix string) *Context {
	c.namePrefix = namePrefix
	return c
}

func (c *Context) ResourceOverrides(overrides map[string]v1alpha1.ResourceOverride) *Context {
	fixture.SetResourceOverrides(overrides)
	return c
}

// this both configures the plugin, but forces use of it
func (c *Context) ConfigManagementPlugin(plugin v1alpha1.ConfigManagementPlugin) *Context {
	fixture.SetConfigManagementPlugin(plugin)
	c.configManagementPlugin = plugin.Name
	return c
}

func (c *Context) And(block func()) *Context {
	block()
	return c
}

func (c *Context) When() *Actions {
	return &Actions{context: c}
}

func (c *Context) Prune(prune bool) *Context {
	c.prune = prune
	return c
}
