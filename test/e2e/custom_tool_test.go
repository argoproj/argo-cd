package e2e

import (
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

// make sure we can echo back the Git creds
func TestCustomToolWithGitCreds(t *testing.T) {
	Given(t).
		// path does not matter, we ignore it
		ConfigManagementPlugin(
			ConfigManagementPlugin{
				Name: Name(),
				Generate: Command{
					Command: []string{"sh", "-c"},
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitAskpass\": \"$GIT_ASKPASS\"}}}"`},
				},
			},
		).
		CustomCACertAdded().
		// add the private repo with credentials
		HTTPSRepoURLAdded(true).
		RepoURLType(RepoURLTypeHTTPS).
		Path("https-kustomize-base").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			assert.NoError(t, err)
			assert.Equal(t, "argocd", output)
		})
}

// make sure we can echo back the Git creds
func TestCustomToolWithGitCredsTemplate(t *testing.T) {
	Given(t).
		// path does not matter, we ignore it
		ConfigManagementPlugin(
			ConfigManagementPlugin{
				Name: Name(),
				Generate: Command{
					Command: []string{"sh", "-c"},
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitAskpass\": \"$GIT_ASKPASS\", \"GitUsername\": \"$GIT_USERNAME\", \"GitPassword\": \"$GIT_PASSWORD\"}}}"`},
				},
			},
		).
		CustomCACertAdded().
		// add the git creds template
		HTTPSCredentialsUserPassAdded().
		// add the private repo without credentials
		HTTPSRepoURLAdded(false).
		RepoURLType(RepoURLTypeHTTPS).
		Path("https-kustomize-base").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			assert.NoError(t, err)
			assert.Equal(t, "argocd", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			assert.NoError(t, err)
			assert.Empty(t, output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			assert.NoError(t, err)
			assert.Empty(t, output)
		})
}

// make sure we can echo back the env
func TestCustomToolWithEnv(t *testing.T) {
	Given(t).
		// path does not matter, we ignore it
		ConfigManagementPlugin(
			ConfigManagementPlugin{
				Name: Name(),
				Generate: Command{
					Command: []string{"sh", "-c"},
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"`},
				},
			},
		).
		// does not matter what the path is
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source.Plugin.Env = Env{{
				Name:  "FOO",
				Value: "bar",
			}}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			time.Sleep(1 * time.Second)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.Bar}")
			assert.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.Foo}")
			assert.NoError(t, err)
			assert.Equal(t, "bar", output)
		}).
		And(func(app *Application) {
			expectedKubeVersion := GetVersions().ServerVersion.Format("%s.%s")
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			assert.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(app *Application) {
			expectedApiVersion := GetApiResources()
			expectedApiVersionSlice := strings.Split(expectedApiVersion, ",")
			sort.Strings(expectedApiVersionSlice)

			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			assert.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.EqualValues(t, expectedApiVersionSlice, outputSlice)
		})
}

//make sure we can sync and diff with --local
func TestCustomToolSyncAndDiffLocal(t *testing.T) {
	Given(t).
		// path does not matter, we ignore it
		ConfigManagementPlugin(
			ConfigManagementPlugin{
				Name: Name(),
				Generate: Command{
					Command: []string{"sh", "-c"},
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"Foo\": \"$FOO\", \"KubeVersion\": \"$KUBE_VERSION\", \"KubeApiVersion\": \"$KUBE_API_VERSIONS\",\"Bar\": \"baz\"}}}"`},
				},
			},
		).
		// does not matter what the path is
		Path("guestbook").
		When().
		CreateApp("--config-management-plugin", Name()).
		Sync("--local", "testdata/guestbook").
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			time.Sleep(1 * time.Second)
		}).
		And(func(app *Application) {
			FailOnErr(RunCli("app", "sync", app.Name, "--local", "testdata/guestbook"))
		}).
		And(func(app *Application) {
			FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/guestbook"))
		})
}

func startCMPServer(configFile string) {
	pluginSockFilePath := TmpDir + PluginSockFilePath
	os.Setenv("ARGOCD_BINARY_NAME", "argocd-cmp-server")
	// ARGOCD_PLUGINSOCKFILEPATH should be set as the same value as repo server env var
	os.Setenv("ARGOCD_PLUGINSOCKFILEPATH", pluginSockFilePath)
	if _, err := os.Stat(pluginSockFilePath); os.IsNotExist(err) {
		// path/to/whatever does not exist
		err := os.Mkdir(pluginSockFilePath, 0700)
		CheckError(err)
	}
	FailOnErr(RunWithStdin("", "", "../../dist/argocd", "--config-dir-path", configFile))
}

//Discover by fileName
func TestCMPDiscoverWithFileName(t *testing.T) {
	pluginName := "cmp-fileName"
	Given(t).
		And(func() {
			go startCMPServer("./testdata/cmp-fileName")
			time.Sleep(1 * time.Second)
			os.Setenv("ARGOCD_BINARY_NAME", "argocd")
		}).
		Path(pluginName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

//Discover by Find glob
func TestCMPDiscoverWithFindGlob(t *testing.T) {
	Given(t).
		And(func() {
			go startCMPServer("./testdata/cmp-find-glob")
			time.Sleep(1 * time.Second)
			os.Setenv("ARGOCD_BINARY_NAME", "argocd")
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

//Discover by Find command
func TestCMPDiscoverWithFindCommandWithEnv(t *testing.T) {
	pluginName := "cmp-find-command"
	Given(t).
		And(func() {
			go startCMPServer("./testdata/cmp-find-command")
			time.Sleep(1 * time.Second)
			os.Setenv("ARGOCD_BINARY_NAME", "argocd")
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
			time.Sleep(1 * time.Second)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.Bar}")
			assert.NoError(t, err)
			assert.Equal(t, "baz", output)
		}).
		And(func(app *Application) {
			expectedKubeVersion := GetVersions().ServerVersion.Format("%s.%s")
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.KubeVersion}")
			assert.NoError(t, err)
			assert.Equal(t, expectedKubeVersion, output)
		}).
		And(func(app *Application) {
			expectedApiVersion := GetApiResources()
			expectedApiVersionSlice := strings.Split(expectedApiVersion, ",")
			sort.Strings(expectedApiVersionSlice)

			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.KubeApiVersion}")
			assert.NoError(t, err)
			outputSlice := strings.Split(output, ",")
			sort.Strings(outputSlice)

			assert.EqualValues(t, expectedApiVersionSlice, outputSlice)
		})
}

func TestPruneResourceFromCMP(t *testing.T) {
	Given(t).
		And(func() {
			go startCMPServer("./testdata/cmp-find-glob")
			time.Sleep(1 * time.Second)
			os.Setenv("ARGOCD_BINARY_NAME", "argocd")
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
			assert.Error(t, err)
		})
}
