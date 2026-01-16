package e2e

import (
	"testing"

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
		// Async so we don't fail immediately on the error
		Async(true).
		When().
		Sync().
		Wait("--operation").
		Then().
		// Fails because we hydrated to env/test-next but not to env/test.
		Expect(OperationPhaseIs(OperationError)).
		When().
		// Will now hydrate to the sync source branch.
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
	// Make sure that if we add another app targeting the same sync branch, it hydrates correctly.
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
		SyncSourceBranch("env/test2").
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
		// Clean up the apps manually since we used custom names.
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
		// Skip validation, otherwise app creation will fail on the unsupported kustomize version.
		CreateApp("--validate=false").
		Refresh(RefreshTypeNormal).
		Then().
		// Expect a failure at first because the kustomize version is not supported.
		Expect(HydrationPhaseIs(HydrateOperationPhaseFailed)).
		// Now register the kustomize version override and try again.
		Given().
		RegisterKustomizeVersion("v1.2.3", "kustomize").
		When().
		// Hard refresh so we don't use the cached error.
		Refresh(RefreshTypeHard).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHydratorWithHelm(t *testing.T) {
	ctx := Given(t)
	ctx.Path("hydrator-helm").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-helm",
					TargetRevision: "HEAD",
					Helm: &ApplicationSourceHelm{
						Parameters: []HelmParameter{
							{Name: "message", Value: "helm-hydrated-with-inline-params"},
						},
					},
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-helm-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Verify that the inline helm parameter was applied
			output, err := fixture.Run("", "kubectl", "-n="+ctx.DeploymentNamespace(),
				"get", "configmap", "my-map",
				"-ojsonpath={.data.message}")
			require.NoError(t, err)
			require.Equal(t, "helm-hydrated-with-inline-params", output)

			// Verify that the namespace was passed to helm
			output, err = fixture.Run("", "kubectl", "-n="+ctx.DeploymentNamespace(),
				"get", "configmap", "my-map",
				"-ojsonpath={.data.helmns}")
			require.NoError(t, err)
			require.Equal(t, ctx.DeploymentNamespace(), output)
		})
}

func TestHydratorWithKustomize(t *testing.T) {
	ctx := Given(t)
	ctx.Path("hydrator-kustomize").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-kustomize",
					TargetRevision: "HEAD",
					Kustomize: &ApplicationSourceKustomize{
						NameSuffix: "-inline",
					},
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-kustomize-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Verify that the inline kustomize nameSuffix was applied
			// kustomization.yaml has namePrefix: kustomize-, and we added nameSuffix: -inline
			// So the ConfigMap name should be kustomize-my-map-inline
			_, err := fixture.Run("", "kubectl", "-n="+ctx.DeploymentNamespace(),
				"get", "configmap", "kustomize-my-map-inline")
			require.NoError(t, err)
		})
}

func TestHydratorWithDirectory(t *testing.T) {
	ctx := Given(t)
	ctx.Path("hydrator-directory").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-directory",
					TargetRevision: "HEAD",
					Directory: &ApplicationSourceDirectory{
						Recurse: true,
					},
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-directory-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Verify that the recurse option was applied by checking the ConfigMap from subdir
			_, err := fixture.Run("", "kubectl", "-n="+ctx.DeploymentNamespace(),
				"get", "configmap", "my-map-subdir")
			require.NoError(t, err)
		})
}

func TestHydratorWithPlugin(t *testing.T) {
	ctx := Given(t)
	ctx.Path("hydrator-plugin").
		RunningCMPServer("./testdata/hydrator-plugin").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "hydrator-plugin",
					TargetRevision: "HEAD",
					Plugin: &ApplicationSourcePlugin{
						Env: Env{
							{Name: "PLUGIN_ENV", Value: "inline-plugin-value"},
						},
					},
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "hydrator-plugin-output",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Verify that the inline plugin env was applied
			output, err := fixture.Run("", "kubectl", "-n="+ctx.DeploymentNamespace(),
				"get", "configmap", "plugin-generated-map",
				"-ojsonpath={.data.plugin-env}")
			require.NoError(t, err)
			require.Equal(t, "inline-plugin-value", output)
		})
}

func TestHydratorNoOp(t *testing.T) {
	// Test that when hydration is run for a no-op (manifests do not change),
	// the hydrated SHA is persisted to the app's source hydrator status instead of an empty string.
	var firstHydratedSHA string
	var firstDrySHA string

	Given(t).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			require.NotEmpty(t, app.Status.SourceHydrator.CurrentOperation.HydratedSHA, "First hydration should have a hydrated SHA")
			require.NotEmpty(t, app.Status.SourceHydrator.CurrentOperation.DrySHA, "First hydration should have a dry SHA")
			firstHydratedSHA = app.Status.SourceHydrator.CurrentOperation.HydratedSHA
			firstDrySHA = app.Status.SourceHydrator.CurrentOperation.DrySHA
			t.Logf("First hydration - drySHA: %s, hydratedSHA: %s", firstDrySHA, firstHydratedSHA)
		}).
		When().
		// Make a change to the dry source that doesn't affect the generated manifests.
		AddFile("guestbook/README.md", "# Guestbook\n\nThis is documentation.").
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			require.NotEmpty(t, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA must not be empty")
			require.NotEmpty(t, app.Status.SourceHydrator.CurrentOperation.DrySHA)

			// The dry SHA should be different (new commit in the dry source)
			require.NotEqual(t, firstDrySHA, app.Status.SourceHydrator.CurrentOperation.DrySHA,
				"Dry SHA should change after pushing a new commit")

			t.Logf("Second hydration - drySHA: %s, hydratedSHA: %s",
				app.Status.SourceHydrator.CurrentOperation.DrySHA,
				app.Status.SourceHydrator.CurrentOperation.HydratedSHA)

			require.Equal(t, firstHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA should remain the same for no-op hydration")
		})
}

func TestHydratorWithAuthenticatedRepo(t *testing.T) {
	// Test that hydration works with an HTTPS repository requiring authentication,
	// specifically that GetCommitNote and AddAndPushNote properly use credentials when
	// fetching git notes. This test creates an initial hydration, then makes a change
	// to trigger a second hydration. On the second hydration, the commit-server will
	// need to fetch existing git notes from the authenticated repository, which requires
	// credentials.
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		HTTPSInsecureRepoURLAdded(true).
		// Add write credentials for commit-server to push hydrated manifests
		WriteCredentials(true).
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
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// Now make a change and re-hydrate. This will trigger git notes fetch
		// operations that require credentials.
		When().
		PatchFile("guestbook/guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
