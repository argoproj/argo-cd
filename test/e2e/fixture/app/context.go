package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/certs"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/gpgkeys"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Context implements the "given" part of given/when/then
type Context struct {
	t           *testing.T
	path        string
	chart       string
	repoURLType fixture.RepoURLType
	// seconds
	timeout                  int
	name                     string
	appNamespace             string
	destServer               string
	destName                 string
	isDestServerInferred     bool
	env                      string
	parameters               []string
	namePrefix               string
	nameSuffix               string
	resource                 string
	prune                    bool
	configManagementPlugin   string
	async                    bool
	localPath                string
	project                  string
	revision                 string
	force                    bool
	applyOutOfSyncOnly       bool
	directoryRecurse         bool
	replace                  bool
	helmPassCredentials      bool
	helmSkipCrds             bool
	helmSkipSchemaValidation bool
	helmSkipTests            bool
	trackingMethod           v1alpha1.TrackingMethod
	sources                  []v1alpha1.ApplicationSource
	drySourceRevision        string
	drySourcePath            string
	syncSourceBranch         string
	syncSourcePath           string
	hydrateToBranch          string
}

type ContextArgs struct {
	AppNamespace string
}

func Given(t *testing.T, opts ...fixture.TestOption) *Context {
	t.Helper()
	fixture.EnsureCleanState(t, opts...)
	return GivenWithSameState(t)
}

func GivenWithNamespace(t *testing.T, namespace string) *Context {
	t.Helper()
	ctx := Given(t)
	ctx.appNamespace = namespace
	return ctx
}

func GivenWithSameState(t *testing.T) *Context {
	t.Helper()
	// ARGOCD_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 20, 0, 180)
	return &Context{
		t:              t,
		destServer:     v1alpha1.KubernetesInternalAPIServerAddr,
		destName:       "in-cluster",
		repoURLType:    fixture.RepoURLTypeFile,
		name:           fixture.Name(),
		timeout:        timeout,
		project:        "default",
		prune:          true,
		trackingMethod: v1alpha1.TrackingMethodLabel,
	}
}

func (c *Context) AppName() string {
	return c.name
}

func (c *Context) AppQualifiedName() string {
	if c.appNamespace != "" {
		return c.appNamespace + "/" + c.AppName()
	}
	return c.AppName()
}

func (c *Context) AppNamespace() string {
	if c.appNamespace != "" {
		return c.appNamespace
	}
	return fixture.TestNamespace()
}

func (c *Context) SetAppNamespace(namespace string) *Context {
	c.appNamespace = namespace
	// errors.CheckError(fixture.SetParamInSettingConfigMap("application.resourceTrackingMethod", "annotation"))
	return c
}

func (c *Context) GPGPublicKeyAdded() *Context {
	gpgkeys.AddGPGPublicKey(c.t)
	return c
}

func (c *Context) GPGPublicKeyRemoved() *Context {
	gpgkeys.DeleteGPGPublicKey(c.t)
	return c
}

func (c *Context) CustomCACertAdded() *Context {
	certs.AddCustomCACert(c.t)
	return c
}

func (c *Context) CustomSSHKnownHostsAdded() *Context {
	certs.AddCustomSSHKnownHostsKeys(c.t)
	return c
}

func (c *Context) HTTPSRepoURLAdded(withCreds bool) *Context {
	repos.AddHTTPSRepo(c.t, false, withCreds, "", fixture.RepoURLTypeHTTPS)
	return c
}

func (c *Context) HTTPSInsecureRepoURLAdded(withCreds bool) *Context {
	repos.AddHTTPSRepo(c.t, true, withCreds, "", fixture.RepoURLTypeHTTPS)
	return c
}

func (c *Context) HTTPSInsecureRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(c.t, true)
	return c
}

func (c *Context) HTTPSRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(c.t, false)
	return c
}

func (c *Context) SubmoduleHTTPSRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos(c.t, "https")
	repos.AddHTTPSRepo(c.t, false, withCreds, "", fixture.RepoURLTypeHTTPSSubmoduleParent)
	return c
}

func (c *Context) SSHRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(c.t, false, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SSHInsecureRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(c.t, true, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SubmoduleSSHRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos(c.t, "ssh")
	repos.AddSSHRepo(c.t, false, withCreds, fixture.RepoURLTypeSSHSubmoduleParent)
	return c
}

func (c *Context) HelmRepoAdded(name string) *Context {
	repos.AddHelmRepo(c.t, name)
	return c
}

func (c *Context) HelmOCIRepoAdded(name string) *Context {
	repos.AddHelmOCIRepo(c.t, name)
	return c
}

func (c *Context) PushChartToOCIRegistry(chartPathName, chartName, chartVersion string) *Context {
	repos.PushChartToOCIRegistry(c.t, chartPathName, chartName, chartVersion)
	return c
}

func (c *Context) HTTPSCredentialsUserPassAdded() *Context {
	repos.AddHTTPSCredentialsUserPass(c.t)
	return c
}

func (c *Context) HelmHTTPSCredentialsUserPassAdded() *Context {
	repos.AddHelmHTTPSCredentialsTLSClientCert(c.t)
	return c
}

func (c *Context) HelmoOCICredentialsWithoutUserPassAdded() *Context {
	repos.AddHelmoOCICredentialsWithoutUserPass(c.t)
	return c
}

func (c *Context) HTTPSCredentialsTLSClientCertAdded() *Context {
	repos.AddHTTPSCredentialsTLSClientCert(c.t)
	return c
}

func (c *Context) SSHCredentialsAdded() *Context {
	repos.AddSSHCredentials(c.t)
	return c
}

func (c *Context) ProjectSpec(spec v1alpha1.AppProjectSpec) *Context {
	c.t.Helper()
	require.NoError(c.t, fixture.SetProjectSpec(c.project, spec))
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

func (c *Context) DrySourceRevision(revision string) *Context {
	c.drySourceRevision = revision
	return c
}

func (c *Context) DrySourcePath(path string) *Context {
	c.drySourcePath = path
	return c
}

func (c *Context) SyncSourceBranch(branch string) *Context {
	c.syncSourceBranch = branch
	return c
}

func (c *Context) SyncSourcePath(path string) *Context {
	c.syncSourcePath = path
	return c
}

func (c *Context) HydrateToBranch(branch string) *Context {
	c.hydrateToBranch = branch
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
	c.isDestServerInferred = false
	return c
}

func (c *Context) DestName(destName string) *Context {
	c.destName = destName
	c.isDestServerInferred = true
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
	c.t.Helper()
	require.NoError(c.t, fixture.SetResourceOverrides(overrides))
	return c
}

func (c *Context) ResourceFilter(filter settings.ResourcesFilter) *Context {
	c.t.Helper()
	require.NoError(c.t, fixture.SetResourceFilter(filter))
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

func (c *Context) ApplyOutOfSyncOnly() *Context {
	c.applyOutOfSyncOnly = true
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

func (c *Context) HelmSkipSchemaValidation() *Context {
	c.helmSkipSchemaValidation = true
	return c
}

func (c *Context) HelmSkipTests() *Context {
	c.helmSkipTests = true
	return c
}

func (c *Context) SetTrackingMethod(trackingMethod string) *Context {
	c.t.Helper()
	require.NoError(c.t, fixture.SetTrackingMethod(trackingMethod))
	return c
}

func (c *Context) SetInstallationID(installationID string) *Context {
	c.t.Helper()
	require.NoError(c.t, fixture.SetInstallationID(installationID))
	return c
}

func (c *Context) GetTrackingMethod() v1alpha1.TrackingMethod {
	return c.trackingMethod
}

func (c *Context) Sources(sources []v1alpha1.ApplicationSource) *Context {
	c.sources = sources
	return c
}
