package e2e

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

// make sure we can echo back the Git creds
func TestCustomToolWithGitCreds(t *testing.T) {
	ctx := Given(t)
	ctx.
		And(func() {
			go startCMPServer(t, "./testdata/cmp-gitcreds")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		CustomCACertAdded().
		// add the private repo with credentials
		HTTPSRepoURLAdded(true).
		RepoURLType(RepoURLTypeHTTPS).
		Path("cmp-gitcreds").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			require.NoError(t, err)
			assert.Equal(t, "argocd", output)
		})
}

// make sure we can echo back the Git creds
func TestCustomToolWithGitCredsTemplate(t *testing.T) {
	ctx := Given(t)
	ctx.
		And(func() {
			go startCMPServer(t, "./testdata/cmp-gitcredstemplate")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		CustomCACertAdded().
		// add the git creds template
		HTTPSCredentialsUserPassAdded().
		// add the private repo without credentials
		HTTPSRepoURLAdded(false).
		RepoURLType(RepoURLTypeHTTPS).
		Path("cmp-gitcredstemplate").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			require.NoError(t, err)
			assert.Equal(t, "argocd", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			require.NoError(t, err)
			assert.Empty(t, output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			require.NoError(t, err)
			assert.Empty(t, output)
		})
}

// make sure we can read the Git creds stored in a temporary file
func TestCustomToolWithSSHGitCreds(t *testing.T) {
	ctx := Given(t)
	// path does not matter, we ignore it
	ctx.
		And(func() {
			go startCMPServer(t, "./testdata/cmp-gitsshcreds")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitSSHCommand}")
			require.NoError(t, err)
			assert.Regexp(t, `-i [^ ]+`, output, "test plugin expects $GIT_SSH_COMMAND to contain the option '-i <path to ssh private key>'")
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitSSHCredsFileSHA}")
			require.NoError(t, err)
			assert.Regexp(t, `\w+\s+[\/\w]+`, output, "git ssh credentials file should be able to be read, hashing the contents")
		})
}

func TestCustomToolWithSSHGitCredsDisabled(t *testing.T) {
	ctx := Given(t)
	// path does not matter, we ignore it
	ctx.
		And(func() {
			go startCMPServer(t, "./testdata/cmp-gitsshcreds-disable-provide")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		CustomCACertAdded().
		// add the private repo with ssh credentials
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(RepoURLTypeSSH).
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
		And(func() {
			go startCMPServer(t, "./testdata/cmp-fileName")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		// does not matter what the path is
		Path("cmp-fileName").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source.Plugin = &ApplicationSourcePlugin{
				Env: Env{{
					Name:  "FOO",
					Value: "bar",
				}},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Bar}")
			require.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Foo}")
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
		}).
		And(func(app *Application) {
			expectedKubeVersion := GetVersions().ServerVersion.Format("%s.%s")
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			require.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(app *Application) {
			expectedApiVersion := GetApiResources()
			expectedApiVersionSlice := strings.Split(expectedApiVersion, ",")
			sort.Strings(expectedApiVersionSlice)

			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			require.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.EqualValues(t, expectedApiVersionSlice, outputSlice)
		})
}

// make sure we can sync and diff with --local
func TestCustomToolSyncAndDiffLocal(t *testing.T) {
	testdataPath, err := filepath.Abs("testdata")
	require.NoError(t, err)
	ctx := Given(t)
	appPath := filepath.Join(testdataPath, "guestbook")
	ctx.
		And(func() {
			go startCMPServer(t, "./testdata/cmp-kustomize")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		// does not matter what the path is
		Path("guestbook").
		When().
		CreateApp("--config-management-plugin", "cmp-kustomize-v1.0").
		Sync("--local", appPath, "--local-repo-root", testdataPath).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			FailOnErr(RunCli("app", "sync", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		}).
		And(func(app *Application) {
			FailOnErr(RunCli("app", "diff", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		})
}

func startCMPServer(t *testing.T, configFile string) {
	t.Helper()
	pluginSockFilePath := TmpDir + PluginSockFilePath
	t.Setenv("ARGOCD_BINARY_NAME", "argocd-cmp-server")
	// ARGOCD_PLUGINSOCKFILEPATH should be set as the same value as repo server env var
	t.Setenv("ARGOCD_PLUGINSOCKFILEPATH", pluginSockFilePath)
	if _, err := os.Stat(pluginSockFilePath); os.IsNotExist(err) {
		// path/to/whatever does not exist
		err := os.Mkdir(pluginSockFilePath, 0o700)
		require.NoError(t, err)
	}
	FailOnErr(RunWithStdin("", "", "../../dist/argocd", "--config-dir-path", configFile))
}

// Discover by fileName
func TestCMPDiscoverWithFileName(t *testing.T) {
	pluginName := "cmp-fileName"
	Given(t).
		And(func() {
			go startCMPServer(t, "./testdata/cmp-fileName")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
		And(func() {
			go startCMPServer(t, "./testdata/cmp-find-glob")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
		And(func() {
			go startCMPServer(t, "./testdata/cmp-find-glob")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
		And(func() {
			go startCMPServer(t, "./testdata/cmp-find-command")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		Path(pluginName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.Bar}")
			require.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(app *Application) {
			expectedKubeVersion := GetVersions().ServerVersion.Format("%s.%s")
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			require.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(app *Application) {
			expectedApiVersion := GetApiResources()
			expectedApiVersionSlice := strings.Split(expectedApiVersion, ",")
			sort.Strings(expectedApiVersionSlice)

			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", ctx.AppName(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			require.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.EqualValues(t, expectedApiVersionSlice, outputSlice)
		})
}

func TestPruneResourceFromCMP(t *testing.T) {
	Given(t).
		And(func() {
			go startCMPServer(t, "./testdata/cmp-find-glob")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
			_, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "deployment", "guestbook-ui")
			require.Error(t, err)
		})
}

func TestPreserveFileModeForCMP(t *testing.T) {
	Given(t).
		And(func() {
			go startCMPServer(t, "./testdata/cmp-preserve-file-mode")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
	Given(t, WithTestData("testdata2")).
		And(func() {
			go startCMPServer(t, "./testdata2/cmp-symlink")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
	Given(t, WithTestData("testdata2")).
		And(func() {
			go startCMPServer(t, "./testdata2/cmp-symlink")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
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
	Given(t, WithTestData("testdata2")).
		And(func() {
			go startCMPServer(t, "./testdata2/cmp-symlink")
			time.Sleep(100 * time.Millisecond)
			t.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		Path("guestbook-symlink-folder").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}
