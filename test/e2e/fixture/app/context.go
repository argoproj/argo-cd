package app

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/certs"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/gpgkeys"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// this implements the "given" part of given/when/then
type Context struct {
	t           *testing.T
	path        string
	chart       string
	repoURLType fixture.RepoURLType
	// seconds
	timeout                int
	name                   string
	destServer             string
	destName               string
	env                    string
	parameters             []string
	namePrefix             string
	nameSuffix             string
	resource               string
	prune                  bool
	configManagementPlugin string
	async                  bool
	localPath              string
	project                string
	revision               string
	force                  bool
	directoryRecurse       bool
	replace                bool
	helmPassCredentials    bool
	helmSkipCrds           bool
}

func Given(t *testing.T) *Context {
	fixture.EnsureCleanState(t)
	return GivenWithSameState(t)
}

func GivenWithSameState(t *testing.T) *Context {
	// ARGOCE_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 10, 0, 180)
	return &Context{t: t, destServer: v1alpha1.KubernetesInternalAPIServerAddr, repoURLType: fixture.RepoURLTypeFile, name: fixture.Name(), timeout: timeout, project: "default", prune: true}
}

func (c *Context) GPGPublicKeyAdded() *Context {
	gpgkeys.AddGPGPublicKey()
	return c
}

func (c *Context) GPGPublicKeyRemoved() *Context {
	gpgkeys.DeleteGPGPublicKey()
	return c
}

func (c *Context) CustomCACertAdded() *Context {
	certs.AddCustomCACert()
	return c
}

func (c *Context) CustomSSHKnownHostsAdded() *Context {
	certs.AddCustomSSHKnownHostsKeys()
	return c
}

func (c *Context) HTTPSRepoURLAdded(withCreds bool) *Context {
	repos.AddHTTPSRepo(false, withCreds, fixture.RepoURLTypeHTTPS)
	return c
}

func (c *Context) HTTPSInsecureRepoURLAdded(withCreds bool) *Context {
	repos.AddHTTPSRepo(true, withCreds, fixture.RepoURLTypeHTTPS)
	return c
}

func (c *Context) HTTPSInsecureRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(true)
	return c
}

func (c *Context) HTTPSRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(false)
	return c
}

func (c *Context) SubmoduleHTTPSRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos("https")
	repos.AddHTTPSRepo(false, withCreds, fixture.RepoURLTypeHTTPSSubmoduleParent)
	return c
}

func (c *Context) SSHRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(false, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SSHInsecureRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(true, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SubmoduleSSHRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos("ssh")
	repos.AddSSHRepo(false, withCreds, fixture.RepoURLTypeSSHSubmoduleParent)
	return c
}

func (c *Context) HelmRepoAdded(name string) *Context {
	repos.AddHelmRepo(name)
	return c
}

func (c *Context) HelmOCIRepoAdded(name string) *Context {
	repos.AddHelmOCIRepo(name)
	return c
}

func (c *Context) PushChartToOCIRegistry(chartPathName, chartName, chartVersion string) *Context {
	repos.PushChartToOCIRegistry(chartPathName, chartName, chartVersion)
	return c
}

func (c *Context) HTTPSCredentialsUserPassAdded() *Context {
	repos.AddHTTPSCredentialsUserPass()
	return c
}

func (c *Context) HelmHTTPSCredentialsUserPassAdded() *Context {
	repos.AddHelmHTTPSCredentialsTLSClientCert()
	return c
}

func (c *Context) HelmoOCICredentialsWithoutUserPassAdded() *Context {
	repos.AddHelmoOCICredentialsWithoutUserPass()
	return c
}

func (c *Context) HTTPSCredentialsTLSClientCertAdded() *Context {
	repos.AddHTTPSCredentialsTLSClientCert()
	return c
}

func (c *Context) SSHCredentialsAdded() *Context {
	repos.AddSSHCredentials()
	return c
}

func (c *Context) ProjectSpec(spec v1alpha1.AppProjectSpec) *Context {
	fixture.SetProjectSpec(c.project, spec)
	return c
}

func (c *Context) Replace() *Context {
	c.replace = true
	return c
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

func (c *Context) Path(path string) *Context {
	c.path = path
	return c
}

func (c *Context) Recurse() *Context {
	c.directoryRecurse = true
	return c
}

func (c *Context) Chart(chart string) *Context {
	c.chart = chart
	return c
}

func (c *Context) Revision(revision string) *Context {
	c.revision = revision
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

func (c *Context) DestName(destName string) *Context {
	c.destName = destName
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

func (c *Context) NameSuffix(nameSuffix string) *Context {
	c.nameSuffix = nameSuffix
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

func (c *Context) And(block func()) *Context {
	block()
	return c
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

func (c *Context) Force() *Context {
	c.force = true
	return c
}

func (c *Context) HelmPassCredentials() *Context {
	c.helmPassCredentials = true
	return c
}

func (c *Context) HelmSkipCrds() *Context {
	c.helmSkipCrds = true
	return c
}
