package e2e

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// make sure we can echo back the Git creds
func TestCustomToolWithGitCreds(t *testing.T) {
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-gitcreds").
		CustomCACertAdded().
		// add the private repo with credentials
		HTTPSRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path("cmp-gitcreds").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			require.NoError(t, err)
			assert.Equal(t, "argocd", output)
		})
}

// make sure we can echo back the Git creds
func TestCustomToolWithGitCredsTemplate(t *testing.T) {
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-gitcredstemplate").
		CustomCACertAdded().
		// add the git creds template
		HTTPSCredentialsUserPassAdded().
		// add the private repo without credentials
		HTTPSRepoURLAdded(false).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path("cmp-gitcredstemplate").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			require.NoError(t, err)
			assert.Equal(t, "argocd", output)
		}).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			require.NoError(t, err)
			assert.Empty(t, output)
		}).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			require.NoError(t, err)
			assert.Empty(t, output)
		})
}

// make sure we can read the Git creds stored in a temporary file
func TestCustomToolWithSSHGitCreds(t *testing.T) {
	ctx := Given(t)
	// path does not matter, we ignore it
	ctx.
		RunningCMPServer("./testdata/cmp-gitsshcreds").
		// add the private repo with ssh credentials
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("cmp-gitsshcreds").
		When().
		CreateApp().
		Sync().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.GetName(), "-o", "jsonpath={.metadata.annotations.GitSSHCommand}")
			require.NoError(t, err)
			assert.Regexp(t, `-i [^ ]+`, output, "test plugin expects $GIT_SSH_COMMAND to contain the option '-i <path to ssh private key>'")
		}).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.GetName(), "-o", "jsonpath={.metadata.annotations.GitSSHCredsFileSHA}")
			require.NoError(t, err)
			assert.Regexp(t, `\w+\s+[\/\w]+`, output, "git ssh credentials file should be able to be read, hashing the contents")
		})
}

func TestCustomToolWithSSHGitCredsDisabled(t *testing.T) {
	ctx := Given(t)
	// path does not matter, we ignore it
	ctx.
		RunningCMPServer("./testdata/cmp-gitsshcreds-disable-provide").
		CustomCACertAdded().
		// add the private repo with ssh credentials
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("cmp-gitsshcreds").
		When().
		IgnoreErrors().
		CreateApp("--validate=false").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown))
}

// make sure we can echo back the env
func TestCustomToolWithEnv(t *testing.T) {
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-fileName").
		// does not matter what the path is
		Path("cmp-fileName").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source.Plugin = &ApplicationSourcePlugin{
				Env: Env{{
					Name:  "FOO",
					Value: "bar",
				}, {
					Name:  "EMPTY",
					Value: "",
				}},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Bar}")
			require.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Foo}")
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
		}).
		And(func(_ *Application) {
			expectedKubeVersion := fixture.GetVersions(t).ServerVersion.String()
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			require.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(_ *Application) {
			expectedAPIVersion := fixture.GetApiResources(t)
			expectedAPIVersionSlice := strings.Split(expectedAPIVersion, ",")
			sort.Strings(expectedAPIVersionSlice)

			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			require.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.Equal(t, expectedAPIVersionSlice, outputSlice)
		})
}

// make sure we can sync and diff with --local
func TestCustomToolSyncAndDiffLocal(t *testing.T) {
	testdataPath, err := filepath.Abs("testdata")
	require.NoError(t, err)
	ctx := Given(t)
	appPath := filepath.Join(testdataPath, "guestbook")
	ctx.
		RunningCMPServer("./testdata/cmp-kustomize").
		// does not matter what the path is
		Path("guestbook").
		When().
		CreateApp("--config-management-plugin", "cmp-kustomize-v1.0").
		Sync("--local", appPath, "--local-repo-root", testdataPath).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "sync", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		}).
		And(func(_ *Application) {
			errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "diff", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		})
}

// Discover by fileName
func TestCMPDiscoverWithFileName(t *testing.T) {
	pluginName := "cmp-fileName"
	Given(t).
		RunningCMPServer("./testdata/cmp-fileName").
		Path(pluginName + "/subdir").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

// Discover by Find glob
func TestCMPDiscoverWithFindGlob(t *testing.T) {
	Given(t).
		RunningCMPServer("./testdata/cmp-find-glob").
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

// Discover by Plugin Name
func TestCMPDiscoverWithPluginName(t *testing.T) {
	Given(t).
		RunningCMPServer("./testdata/cmp-find-glob").
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			// specifically mention the plugin to use (name is based on <plugin name>-<version>
			app.Spec.Source.Plugin = &ApplicationSourcePlugin{Name: "cmp-find-glob-v1.0"}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

// Discover by Find command
func TestCMPDiscoverWithFindCommandWithEnv(t *testing.T) {
	pluginName := "cmp-find-command"
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-find-command").
		Path(pluginName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Bar}")
			require.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(_ *Application) {
			expectedKubeVersion := fixture.GetVersions(t).ServerVersion.String()
			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			require.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(_ *Application) {
			expectedAPIVersion := fixture.GetApiResources(t)
			expectedAPIVersionSlice := strings.Split(expectedAPIVersion, ",")
			sort.Strings(expectedAPIVersionSlice)

			output, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			require.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.Equal(t, expectedAPIVersionSlice, outputSlice)
		})
}

func TestPruneResourceFromCMP(t *testing.T) {
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-find-glob").
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		AndAction(func() {
			_, err := fixture.Run("", "kubectl", "-n", ctx.DeploymentNamespace(), "get", "deployment", "guestbook-ui")
			require.Error(t, err)
		})
}

func TestPreserveFileModeForCMP(t *testing.T) {
	Given(t).
		RunningCMPServer("./testdata/cmp-preserve-file-mode").
		Path("cmp-preserve-file-mode").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source.Plugin = &ApplicationSourcePlugin{Name: "cmp-preserve-file-mode-v1.0"}
		}).
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			require.Len(t, app.Status.Resources, 1)
			assert.Equal(t, "ConfigMap", app.Status.Resources[0].Kind)
		})
}

func TestCMPWithSymlinkPartialFiles(t *testing.T) {
	Given(t, fixture.WithTestData("testdata2")).
		RunningCMPServer("./testdata2/cmp-symlink").
		Path("guestbook-partial-symlink-files").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestCMPWithSymlinkFiles(t *testing.T) {
	Given(t, fixture.WithTestData("testdata2")).
		RunningCMPServer("./testdata2/cmp-symlink").
		Path("guestbook-symlink-files").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestCMPWithSymlinkFolder(t *testing.T) {
	Given(t, fixture.WithTestData("testdata2")).
		RunningCMPServer("./testdata2/cmp-symlink").
		Path("guestbook-symlink-folder").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}
