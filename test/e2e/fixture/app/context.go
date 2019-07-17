package app

import (
	"testing"

	. "github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/util/settings"
)

// this implements the "given" part of given/when/then
type Context struct {
	t           *testing.T
	path        string
	repoURLType fixture.RepoURLType
	// seconds
	timeout                int
	name                   string
	destServer             string
	env                    string
	parameters             []string
	jsonnetTLAS            []string
	namePrefix             string
	resource               string
	prune                  bool
	configManagementPlugin string
	async                  bool
	localPath              string
	project                string
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return &Context{t: t, destServer: KubernetesInternalAPIServerAddr, repoURLType: fixture.RepoURLTypeFile, name: fixture.Name(), timeout: 5, project: "default", prune: true}
}

func (c *Context) HTTPSRepoURLAdded() *Context {
	repos.AddHTTPSRepo()
	return c
}

func (c *Context) SSHRepoURLAdded() *Context {
	repos.AddSSHRepo()
	return c
}

func (c *Context) RepoURLType(urlType fixture.RepoURLType) *Context {
	c.repoURLType = urlType
	return c
}

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) Timeout(timeout int) *Context {
	c.timeout = timeout
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

func (c *Context) JsonnetTlaParameter(parameter string) *Context {
	c.jsonnetTLAS = append(c.jsonnetTLAS, parameter)
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

func (c *Context) ResourceFilter(filter settings.ResourcesFilter) *Context {
	fixture.SetResourceFilter(filter)
	return c
}

// this both configures the plugin, but forces use of it
func (c *Context) ConfigManagementPlugin(plugin v1alpha1.ConfigManagementPlugin) *Context {
	fixture.SetConfigManagementPlugins(plugin)
	c.configManagementPlugin = plugin.Name
	return c
}

func (c *Context) HelmRepoCredential(name, url string) *Context {
	fixture.SetHelmRepoCredential(settings.HelmRepoCredentials{Name: name, URL: url})
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

func (c *Context) Async(async bool) *Context {
	c.async = async
	return c
}

func (c *Context) LocalPath(localPath string) *Context {
	c.localPath = localPath
	return c
}

func (c *Context) Project(project string) *Context {
	c.project = project
	return c
}
