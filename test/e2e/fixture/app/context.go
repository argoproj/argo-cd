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

// Context implements the "given" part of given/when/then.
// It embeds fixture.TestState to provide test-specific state that enables parallel test execution.
type Context struct {
	*fixture.TestState
	path            string
	chart           string
	ociRegistry     string
	ociRegistryPath string
	repoURLType     fixture.RepoURLType
	// seconds
	timeout int

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
	state := fixture.EnsureCleanState(t, opts...)
	return GivenWithSameState(state)
}

func GivenWithNamespace(t *testing.T, namespace string) *Context {
	t.Helper()
	ctx := Given(t)
	ctx.appNamespace = namespace
	return ctx
}

// GivenWithSameState creates a new Context that shares the same TestState as an existing context.
// Use this when you need multiple fixture contexts within the same test.
func GivenWithSameState(ctx fixture.TestContext) *Context {
	ctx.T().Helper()
	// ARGOCD_E2E_DEFAULT_TIMEOUT can be used to override the default timeout
	// for any context.
	timeout := env.ParseNumFromEnv("ARGOCD_E2E_DEFAULT_TIMEOUT", 20, 0, 180)
	return &Context{
		TestState:      fixture.NewTestStateFromContext(ctx),
		destServer:     v1alpha1.KubernetesInternalAPIServerAddr,
		destName:       "in-cluster",
		repoURLType:    fixture.RepoURLTypeFile,
		timeout:        timeout,
		project:        "default",
		prune:          true,
		trackingMethod: v1alpha1.TrackingMethodLabel,
	}
}

func (c *Context) Name(name string) *Context {
	c.SetName(name)
	return c
}

// AppName returns the unique application name for the test context.
// Unique application names protects from potential conflicts between test run
// caused by the tracking annotation on existing objects
func (c *Context) AppName() string {
	return c.GetName()
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
	gpgkeys.AddGPGPublicKey(c.T())
	return c
}

func (c *Context) GPGPublicKeyRemoved() *Context {
	gpgkeys.DeleteGPGPublicKey(c.T())
	return c
}

func (c *Context) CustomCACertAdded() *Context {
	certs.AddCustomCACert(c.T())
	return c
}

func (c *Context) CustomSSHKnownHostsAdded() *Context {
	certs.AddCustomSSHKnownHostsKeys(c.T())
	return c
}

func (c *Context) HTTPSRepoURLAdded(withCreds bool, opts ...repos.AddRepoOpts) *Context {
	repos.AddHTTPSRepo(c.T(), false, withCreds, "", fixture.RepoURLTypeHTTPS, opts...)
	return c
}

func (c *Context) HTTPSInsecureRepoURLAdded(withCreds bool, opts ...repos.AddRepoOpts) *Context {
	repos.AddHTTPSRepo(c.T(), true, withCreds, "", fixture.RepoURLTypeHTTPS, opts...)
	return c
}

func (c *Context) HTTPSInsecureRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(c.T(), true)
	return c
}

func (c *Context) HTTPSRepoURLWithClientCertAdded() *Context {
	repos.AddHTTPSRepoClientCert(c.T(), false)
	return c
}

func (c *Context) SubmoduleHTTPSRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos(c.T(), "https")
	repos.AddHTTPSRepo(c.T(), false, withCreds, "", fixture.RepoURLTypeHTTPSSubmoduleParent)
	return c
}

func (c *Context) SSHRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(c.T(), false, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SSHInsecureRepoURLAdded(withCreds bool) *Context {
	repos.AddSSHRepo(c.T(), true, withCreds, fixture.RepoURLTypeSSH)
	return c
}

func (c *Context) SubmoduleSSHRepoURLAdded(withCreds bool) *Context {
	fixture.CreateSubmoduleRepos(c.T(), "ssh")
	repos.AddSSHRepo(c.T(), false, withCreds, fixture.RepoURLTypeSSHSubmoduleParent)
	return c
}

func (c *Context) HelmRepoAdded(name string) *Context {
	repos.AddHelmRepo(c.T(), name)
	return c
}

func (c *Context) HelmOCIRepoAdded(name string) *Context {
	repos.AddHelmOCIRepo(c.T(), name)
	return c
}

func (c *Context) PushImageToOCIRegistry(pathName, tag string) *Context {
	repos.PushImageToOCIRegistry(c.T(), pathName, tag)
	return c
}

func (c *Context) PushImageToAuthenticatedOCIRegistry(pathName, tag string) *Context {
	repos.PushImageToAuthenticatedOCIRegistry(c.T(), pathName, tag)
	return c
}

func (c *Context) PushChartToOCIRegistry(chartPathName, chartName, chartVersion string) *Context {
	repos.PushChartToOCIRegistry(c.T(), chartPathName, chartName, chartVersion)
	return c
}

func (c *Context) PushChartToAuthenticatedOCIRegistry(chartPathName, chartName, chartVersion string) *Context {
	repos.PushChartToAuthenticatedOCIRegistry(c.T(), chartPathName, chartName, chartVersion)
	return c
}

func (c *Context) HTTPSCredentialsUserPassAdded() *Context {
	repos.AddHTTPSCredentialsUserPass(c.T())
	return c
}

func (c *Context) HelmHTTPSCredentialsUserPassAdded() *Context {
	repos.AddHelmHTTPSCredentialsTLSClientCert(c.T())
	return c
}

func (c *Context) HelmoOCICredentialsWithoutUserPassAdded() *Context {
	repos.AddHelmoOCICredentialsWithoutUserPass(c.T())
	return c
}

func (c *Context) HTTPSCredentialsTLSClientCertAdded() *Context {
	repos.AddHTTPSCredentialsTLSClientCert(c.T())
	return c
}

func (c *Context) SSHCredentialsAdded() *Context {
	repos.AddSSHCredentials(c.T())
	return c
}

func (c *Context) OCIRepoAdded(name, imagePath string) *Context {
	repos.AddOCIRepo(c.T(), name, imagePath)
	return c
}

func (c *Context) AuthenticatedOCIRepoAdded(name, imagePath string) *Context {
	repos.AddAuthenticatedOCIRepo(c.T(), name, imagePath)
	return c
}

func (c *Context) OCIRegistry(registry string) *Context {
	c.ociRegistry = registry
	return c
}

func (c *Context) ProjectSpec(spec v1alpha1.AppProjectSpec) *Context {
	c.T().Helper()
	require.NoError(c.T(), fixture.SetProjectSpec(c.project, spec))
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

func (c *Context) OCIRegistryPath(ociPath string) *Context {
	c.ociRegistryPath = ociPath
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
	c.T().Helper()
	require.NoError(c.T(), fixture.SetResourceOverrides(overrides))
	return c
}

func (c *Context) ResourceFilter(filter settings.ResourcesFilter) *Context {
	c.T().Helper()
	require.NoError(c.T(), fixture.SetResourceFilter(filter))
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
	c.T().Helper()
	require.NoError(c.T(), fixture.SetTrackingMethod(trackingMethod))
	return c
}

func (c *Context) SetInstallationID(installationID string) *Context {
	c.T().Helper()
	require.NoError(c.T(), fixture.SetInstallationID(installationID))
	return c
}

func (c *Context) GetTrackingMethod() v1alpha1.TrackingMethod {
	return c.trackingMethod
}

func (c *Context) Sources(sources []v1alpha1.ApplicationSource) *Context {
	c.sources = sources
	return c
}

func (c *Context) RegisterKustomizeVersion(version, path string) *Context {
	c.T().Helper()
	require.NoError(c.T(), fixture.RegisterKustomizeVersion(version, path))
	return c
}
