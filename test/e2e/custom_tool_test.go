package e2e

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/util/errors"
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
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitAskpass\": \"$GIT_ASKPASS\", \"GitUsername\": \"$GIT_USERNAME\", \"GitPassword\": \"$GIT_PASSWORD\"}}}"`},
				},
			},
		).
		CustomCACertAdded().
		// add the private repo with credentials
		HTTPSRepoURLAdded(true).
		RepoURLType(RepoURLTypeHTTPS).
		Path("https-kustomize-base").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			assert.NoError(t, err)
			assert.Equal(t, "git-ask-pass.sh", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			assert.NoError(t, err)
			assert.Equal(t, GitUsername, output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			assert.NoError(t, err)
			assert.Equal(t, GitPassword, output)
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
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			assert.NoError(t, err)
			assert.Equal(t, "git-ask-pass.sh", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			assert.NoError(t, err)
			assert.Equal(t, GitUsername, output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			assert.NoError(t, err)
			assert.Equal(t, GitPassword, output)
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
			expectedApiVersion := GetApiVersions()
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
		Create("--config-management-plugin", Name()).
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
