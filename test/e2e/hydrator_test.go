package e2e

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"

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
		Path("hydrator-helm").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-helm",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-helm-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithKustomize(t *testing.T) {
	Given(t).
		Path("hydrator-kustomize").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-kustomize",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-kustomize-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithDirectory(t *testing.T) {
	Given(t).
		Path("hydrator-directory").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-directory",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-directory-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithPlugin(t *testing.T) {
	Given(t).
		Path("hydrator-plugin").
		And(func() {
			startCMPServerForHydrator(t, "./testdata/hydrator-plugin")
			time.Sleep(100 * time.Millisecond)
		}).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-plugin",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-plugin-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithMixedSources(t *testing.T) {
	Given(t).
		Name("test-mixed-sources-helm").
		Path("hydrator-helm").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-helm",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-mixed-helm",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		Name("test-mixed-sources-kustomize").
		Path("hydrator-kustomize").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-kustomize",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-mixed-kustomize",
				},
			}
		}).
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

func startCMPServerForHydrator(t *testing.T, configFile string) {
	t.Helper()
	pluginSockFilePath := fixture.TmpDir + fixture.PluginSockFilePath
	if _, err := os.Stat(pluginSockFilePath); os.IsNotExist(err) {
		err := os.Mkdir(pluginSockFilePath, 0o700)
		require.NoError(t, err)
	}

	cmd := exec.Command("../../dist/argocd", "--config-dir-path", configFile)
	cmd.Env = append(os.Environ(),
		"ARGOCD_BINARY_NAME=argocd-cmp-server",
		"ARGOCD_PLUGINSOCKFILEPATH="+pluginSockFilePath)

	if err := cmd.Start(); err != nil {
		require.NoError(t, err, "Failed to start CMP server")
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
}
