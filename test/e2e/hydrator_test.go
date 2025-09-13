package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
)

func TestSimpleHydrator(t *testing.T) {
	Given(t).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydrateTo(t *testing.T) {
	Given(t).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook").
		SyncSourceBranch("env/test").
		HydrateToBranch("env/test-next").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Given().
		Async(true).
		When().
		Sync().
		Wait("--operation").
		Then().
		Expect(OperationPhaseIs(OperationError)).
		When().
		AppSet("--hydrate-to-branch", "").
		// a new git commit, that has a new revisionHistoryLimit.
		PatchFile("guestbook/guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Wait("--operation").
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestAddingApp(t *testing.T) {
	Given(t).
		Name("test-adding-app-1").
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook-1").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		Name("test-adding-app-2").
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook-2").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		// a new git commit, that has a new revisionHistoryLimit.
		PatchFile("guestbook/guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		Given().
		Name("test-adding-app-1").
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())
}

func TestKustomizeVersionOverride(t *testing.T) {
	Given(t).
		Name("test-kustomize-version-override").
		DrySourcePath("kustomize-with-version-override").
		DrySourceRevision("HEAD").
		SyncSourcePath("kustomize-with-version-override").
		SyncSourceBranch("env/test").
		When().
		CreateApp("--validate=false").
		Refresh(RefreshTypeNormal).
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseFailed)).
		Given().
		RegisterKustomizeVersion("v1.2.3", "kustomize").
		When().
		Refresh(RefreshTypeHard).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithHelm(t *testing.T) {
	Given(t).
		DrySourcePath("hydrator-helm").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-helm-output").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.message}")
			require.NoError(t, err)
			assert.Equal(t, "helm-hydrated-successfully", output)
		})
}

func TestHydratorWithKustomize(t *testing.T) {
	Given(t).
		DrySourcePath("hydrator-kustomize").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-kustomize-output").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "kustomize-my-map", "-o", "jsonpath={.data.message}")
			require.NoError(t, err)
			assert.Equal(t, "kustomize-hydrated", output)
		})
}

func TestHydratorWithDirectory(t *testing.T) {
	Given(t).
		DrySourcePath("hydrator-directory").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-directory-output").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.message}")
			require.NoError(t, err)
			assert.Equal(t, "directory-hydrated", output)
		})
}

func TestHydratorWithPlugin(t *testing.T) {
	Given(t).
		And(func() {
			go startCMPServerForHydrator(t, "./testdata/hydrator-plugin")
			time.Sleep(100 * time.Millisecond)
		}).
		DrySourcePath("hydrator-plugin").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-plugin-output").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			output, err := fixture.Run("", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "plugin-generated-map", "-o", "jsonpath={.data.message}")
			require.NoError(t, err)
			assert.Equal(t, "plugin-hydrated", output)

			contentOutput, err := fixture.Run("", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "plugin-generated-map", "-o", "jsonpath={.data.content}")
			require.NoError(t, err)
			assert.Equal(t, "This is a test for the plugin.", contentOutput)
		})
}

func TestHydratorWithMixedSources(t *testing.T) {
	Given(t).
		Name("test-mixed-sources-helm").
		DrySourcePath("hydrator-helm").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-mixed-helm").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		Name("test-mixed-sources-kustomize").
		DrySourcePath("hydrator-kustomize").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-mixed-kustomize").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		Given().
		Name("test-mixed-sources-helm").
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())
}

func TestHydratorErrorHandling(t *testing.T) {
	Given(t).
		Name("test-hydrator-error-handling").
		DrySourcePath("non-existent-path").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-error-output").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseFailed))
}

func startCMPServerForHydrator(t *testing.T, configFile string) {
	t.Helper()
	pluginSockFilePath := fixture.TmpDir + fixture.PluginSockFilePath
	t.Setenv("ARGOCD_BINARY_NAME", "argocd-cmp-server")
	t.Setenv("ARGOCD_PLUGINSOCKFILEPATH", pluginSockFilePath)
	if _, err := os.Stat(pluginSockFilePath); os.IsNotExist(err) {
		err := os.Mkdir(pluginSockFilePath, 0o700)
		require.NoError(t, err)
	}
	errors.NewHandler(t).FailOnErr(fixture.RunWithStdin("", "", "../../dist/argocd", "--config-dir-path", configFile))
}
