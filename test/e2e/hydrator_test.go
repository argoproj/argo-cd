package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"

	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
)

func restrictedDefaultProjectSpec() AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:              []string{"https://example.com/not-the-repo"},
		Destinations:             []ApplicationDestination{{Server: "*", Namespace: "*"}},
		ClusterResourceWhitelist: []ClusterResourceRestrictionItem{{Group: "*", Kind: "*"}},
	}
}

func permissiveDefaultProjectSpec() AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:              []string{"*"},
		Destinations:             []ApplicationDestination{{Server: "*", Namespace: "*"}},
		ClusterResourceWhitelist: []ClusterResourceRestrictionItem{{Group: "*", Kind: "*"}},
	}
}

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

func TestHydratorWebhookNoOpSyncRevisionFastForward(t *testing.T) {
	appA := Given(t).
		Name("hydrator-webhook-changed").
		RepoURLType(fixture.RepoURLTypeHTTPS).
		HTTPSInsecureRepoURLAdded(true).
		WriteCredentials(true).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-webhook-a").
		SyncSourceBranch("env/test").
		HydrateToBranch("env/test-next")
	appB := GivenWithSameState(appA).
		Name("hydrator-webhook-unchanged").
		RepoURLType(fixture.RepoURLTypeHTTPS).
		DrySourcePath("hydrator-directory").
		DrySourceRevision("HEAD").
		SyncSourcePath("hydrator-webhook-b").
		SyncSourceBranch("env/test").
		HydrateToBranch("env/test-next")

	appA.When().CreateApp()
	appB.When().CreateApp()
	t.Cleanup(func() {
		appB.When().Delete(true)
		appA.When().Delete(true)
	})

	// App A may begin hydrating before app B has been created. Advance the shared dry revision after both exist so a
	// subsequent group hydration is guaranteed to include both applications.
	fixture.AddFile(t, "hydrate-webhook-group.marker", "hydrate both applications")
	initialDryRevision := fixture.Git(t, "rev-parse", "master")
	appA.When().Refresh(RefreshTypeHard)
	appA.When().ThenWithTimeout(80).Expect(App(func(app *Application) bool {
		return app.Status.SourceHydrator.LastSuccessfulOperation != nil &&
			app.Status.SourceHydrator.LastSuccessfulOperation.DrySHA == initialDryRevision
	}))
	appB.When().ThenWithTimeout(80).Expect(App(func(app *Application) bool {
		return app.Status.SourceHydrator.LastSuccessfulOperation != nil &&
			app.Status.SourceHydrator.LastSuccessfulOperation.DrySHA == initialDryRevision
	}))

	// Simulate the first external promotion and deploy both applications. This also populates the manifest cache at
	// the first sync revision, which the no-op webhook path needs in order to warm the cache for the next revision.
	firstSyncRevision := fixture.PromoteBranch(t, "env/test-next", "env/test")
	appA.When().Refresh(RefreshTypeNormal).Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(SyncRevisionIs(firstSyncRevision))
	appB.When().Refresh(RefreshTypeNormal).Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(SyncRevisionIs(firstSyncRevision))

	// Change only app A's dry source. The group hydration advances lastComparedDryRevision for both apps, but only
	// app A's hydrated directory changes on the staging branch.
	appA.When().PatchDrySourceFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`)
	dryRevision := fixture.Git(t, "rev-parse", "master")
	appA.When().Refresh(RefreshTypeHard)
	appA.When().ThenWithTimeout(80).Expect(App(func(app *Application) bool {
		return app.Status.SourceHydrator.LastSuccessfulOperation != nil &&
			app.Status.SourceHydrator.LastSuccessfulOperation.DrySHA == dryRevision
	}))
	appB.When().ThenWithTimeout(80).Expect(App(func(app *Application) bool {
		return app.Status.SourceHydrator.LastSuccessfulOperation != nil &&
			app.Status.SourceHydrator.LastSuccessfulOperation.DrySHA == dryRevision
	}))

	// Wait for the post-hydration refresh against the still-unpromoted sync branch. This prevents that refresh from
	// racing with the promotion below and accidentally observing the new revision.
	appB.When().Then().Expect(App(func(app *Application) bool {
		return app.Status.SourceHydrator.CurrentOperation != nil &&
			app.Status.SourceHydrator.CurrentOperation.FinishedAt != nil &&
			app.Status.ReconciledAt != nil &&
			!app.Status.ReconciledAt.Time.Before(app.Status.SourceHydrator.CurrentOperation.FinishedAt.Time) &&
			app.Status.Sync.Revision == firstSyncRevision
	}))

	secondSyncRevision := fixture.PromoteBranch(t, "env/test-next", "env/test")
	require.NotEqual(t, firstSyncRevision, secondSyncRevision)
	changedFiles := fixture.GitChangedFiles(t, firstSyncRevision, secondSyncRevision)
	require.NotEmpty(t, changedFiles)
	require.Contains(t, changedFiles, "hydrator-webhook-a/manifest.yaml")
	for _, changedFile := range changedFiles {
		require.False(t, strings.HasPrefix(changedFile, "hydrator-webhook-b/"),
			"the sibling app must be a no-op in the promoted commit")
	}

	var hydrationStartedAt time.Time
	appB.When().Then().
		Expect(SyncRevisionIs(firstSyncRevision)).
		And(func(app *Application) {
			require.Equal(t, dryRevision, app.Status.SourceHydrator.LastComparedDryRevision)
			require.NotNil(t, app.Status.SourceHydrator.CurrentOperation)
			hydrationStartedAt = app.Status.SourceHydrator.CurrentOperation.StartedAt.Time
		})

	payload, err := json.Marshal(map[string]any{
		"ref":    "refs/heads/env/test",
		"before": firstSyncRevision,
		"after":  secondSyncRevision,
		"commits": []map[string]any{{
			"id":       secondSyncRevision,
			"modified": changedFiles,
		}},
		"repository": map[string]any{
			"html_url":       fixture.RepoURL(fixture.RepoURLTypeHTTPS),
			"default_branch": "master",
		},
	})
	require.NoError(t, err)
	resp, err := fixture.DoHttpRequestWithHeaders(http.MethodPost, "/api/webhook", "", map[string]string{
		"X-GitHub-Event": "push",
	}, payload...)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Before the fix this times out with B still reporting firstSyncRevision. The webhook path sees no changed files
	// under B's sync path and warms its manifest cache, but fails to request the reconcile that advances status.
	appB.When().Then().
		Expect(SyncRevisionIs(secondSyncRevision)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Equal(t, hydrationStartedAt, app.Status.SourceHydrator.CurrentOperation.StartedAt.Time,
				"a sync-source no-op webhook must not trigger hydration")
		})
}

func TestHydratorNormalRefreshRecoversFailedHydration(t *testing.T) {
	Given(t).
		Name("test-normal-refresh-recovery").
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook").
		SyncSourceBranch("env/test").
		When().
		CreateApp("--validate=false").
		And(func() {
			require.NoError(t, fixture.SetProjectSpec("default", restrictedDefaultProjectSpec()))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseFailed)).
		When().
		And(func() {
			require.NoError(t, fixture.SetProjectSpec("default", permissiveDefaultProjectSpec()))
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated))
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
		ExpectConsistently(HydrationPhaseIs(HydrateOperationPhaseHydrated), 1*time.Second, 5*time.Second).
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

func TestHydratorHydratesAutomatically_NewCommit(t *testing.T) {
	// Test that when a new commit is made to the dry source, a normal refresh (not a hydrate request)
	// detects the new revision and triggers hydration automatically.
	// This scenario has no manifest-path-annotation, so the controller always treats a new revision as
	// potentially having changes.
	var firstDrySHA string
	var firstHydratedSHA string
	var firstStartedAt time.Time

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
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Status.SourceHydrator.CurrentOperation)
			require.Equal(t, HydrateOperationPhaseHydrated, app.Status.SourceHydrator.CurrentOperation.Phase)
			firstDrySHA = app.Status.SourceHydrator.CurrentOperation.DrySHA
			firstHydratedSHA = app.Status.SourceHydrator.CurrentOperation.HydratedSHA
			firstStartedAt = app.Status.SourceHydrator.CurrentOperation.StartedAt.Time
			require.NotEmpty(t, firstDrySHA)
			require.NotEmpty(t, firstHydratedSHA)
			require.False(t, firstStartedAt.IsZero())
			t.Logf("Initial hydration - drySHA: %s, hydratedSHA: %s, startedAt: %s", firstDrySHA, firstHydratedSHA, firstStartedAt)

			// Wait a second here because we using the firstStartedAt timestamp and do not want it to be the same
			// if we refresh too fast
			time.Sleep(time.Second)
		}).
		// Verify the hydration is stable: a normal refresh should not trigger a new hydration
		// when no commits have been made.
		When().
		Refresh(RefreshTypeNormal).
		Then().
		ExpectConsistently(HydrationPhaseIs(HydrateOperationPhaseHydrated), 1*time.Second, 5*time.Second).
		And(func(app *Application) {
			require.Equal(t, firstDrySHA, app.Status.SourceHydrator.CurrentOperation.DrySHA,
				"Dry SHA should not change without a new commit")
			require.Equal(t, firstHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA should not change without a new commit")
			require.Equal(t, firstStartedAt, app.Status.SourceHydrator.CurrentOperation.StartedAt.Time,
				"StartedAt should not change — no new hydration operation should have been triggered")
		}).
		// Now make a manifest-affecting change and verify that a normal refresh triggers re-hydration.
		When().
		PatchFile("guestbook/guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			require.NotEqual(t, firstDrySHA, app.Status.SourceHydrator.CurrentOperation.DrySHA,
				"Dry SHA should change after a new commit")
			require.NotEqual(t, firstHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA should change when manifests differ")
			t.Logf("After new commit - drySHA: %s, hydratedSHA: %s",
				app.Status.SourceHydrator.CurrentOperation.DrySHA,
				app.Status.SourceHydrator.CurrentOperation.HydratedSHA)
		}).
		// Verify the new hydration is stable.
		When().
		Refresh(RefreshTypeNormal).
		Then().
		ExpectConsistently(HydrationPhaseIs(HydrateOperationPhaseHydrated), 1*time.Second, 5*time.Second)
}

func TestHydratorHydratesAutomatically_NewCommit_WithChanges(t *testing.T) {
	// Test that when a new commit is made to the dry source with manifest-generate-paths annotation,
	// and the commit affects the watched path, hydration is triggered automatically.
	var firstDrySHA string
	var firstHydratedSHA string
	var firstStartedAt time.Time

	ctx := Given(t)
	ctx.Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.ObjectMeta.Annotations = map[string]string{
				AnnotationKeyManifestGeneratePaths: ".",
			}
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "guestbook",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "guestbook",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			firstDrySHA = app.Status.SourceHydrator.CurrentOperation.DrySHA
			firstHydratedSHA = app.Status.SourceHydrator.CurrentOperation.HydratedSHA
			firstStartedAt = app.Status.SourceHydrator.CurrentOperation.StartedAt.Time
			require.NotEmpty(t, firstDrySHA)
			require.NotEmpty(t, firstHydratedSHA)
			t.Logf("Initial hydration - drySHA: %s, hydratedSHA: %s", firstDrySHA, firstHydratedSHA)

			// Wait a second here because we using the firstStartedAt timestamp and do not want it to be the same
			// if we refresh too fast
			time.Sleep(time.Second)
		}).
		// A change inside the watched path should trigger re-hydration.
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(App(func(app *Application) bool {
			op := app.Status.SourceHydrator.CurrentOperation
			return op != nil && op.DrySHA != firstDrySHA
		})).
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			require.NotEqual(t, firstDrySHA, app.Status.SourceHydrator.CurrentOperation.DrySHA,
				"Dry SHA should change after a new commit")
			require.NotEqual(t, firstHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA should change when manifests differ")
			require.NotEqual(t, firstStartedAt, app.Status.SourceHydrator.CurrentOperation.StartedAt.Time,
				"StartedAt should change — a new hydration operation should have been triggered")
			t.Logf("After change in watched path - drySHA: %s, hydratedSHA: %s",
				app.Status.SourceHydrator.CurrentOperation.DrySHA,
				app.Status.SourceHydrator.CurrentOperation.HydratedSHA)
		})
}

func TestHydratorHydratesAutomatically_NewCommit_WithoutChanges(t *testing.T) {
	// Test that when a new commit is made to the dry source with manifest-generate-paths annotation,
	// but the commit does NOT affect the watched path, hydration is NOT re-triggered.
	var firstDrySHA string
	var firstHydratedSHA string
	var firstStartedAt time.Time

	ctx := Given(t)
	ctx.Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = nil
			app.ObjectMeta.Annotations = map[string]string{
				AnnotationKeyManifestGeneratePaths: ".",
			}
			app.Spec.SourceHydrator = &SourceHydrator{
				DrySource: DrySource{
					RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
					Path:           "guestbook",
					TargetRevision: "HEAD",
				},
				SyncSource: SyncSource{
					TargetBranch: "env/test",
					Path:         "guestbook",
				},
			}
		}).
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Then().
		Expect(HydrationPhaseIs(HydrateOperationPhaseHydrated)).
		And(func(app *Application) {
			firstDrySHA = app.Status.SourceHydrator.CurrentOperation.DrySHA
			firstHydratedSHA = app.Status.SourceHydrator.CurrentOperation.HydratedSHA
			firstStartedAt = app.Status.SourceHydrator.CurrentOperation.StartedAt.Time
			require.NotEmpty(t, firstDrySHA)
			require.NotEmpty(t, firstHydratedSHA)
			t.Logf("Initial hydration - drySHA: %s, hydratedSHA: %s", firstDrySHA, firstHydratedSHA)

			// Wait a second here because we using the firstStartedAt timestamp and do not want it to be the same
			// if we refresh too fast
			time.Sleep(time.Second)
		}).
		// A change outside the watched path should NOT trigger re-hydration.
		// Use fixture.AddFile directly to write outside the context path.
		AndAction(func() {
			fixture.AddFile(t, "unrelated-file.md", "# This change is outside the manifest-generate-paths")
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		ExpectConsistently(HydrationPhaseIs(HydrateOperationPhaseHydrated), 1*time.Second, 5*time.Second).
		And(func(app *Application) {
			require.Equal(t, firstDrySHA, app.Status.SourceHydrator.CurrentOperation.DrySHA,
				"Dry SHA should not change — no new hydration operation should have been triggered")
			require.Equal(t, firstHydratedSHA, app.Status.SourceHydrator.CurrentOperation.HydratedSHA,
				"Hydrated SHA should not change when the commit is outside the watched path")
			require.Equal(t, firstStartedAt, app.Status.SourceHydrator.CurrentOperation.StartedAt.Time,
				"StartedAt should not change — no new hydration operation should have been triggered")
			t.Logf("After change outside watched path - drySHA: %s, hydratedSHA: %s",
				app.Status.SourceHydrator.CurrentOperation.DrySHA,
				app.Status.SourceHydrator.CurrentOperation.HydratedSHA)
		})
}

func TestHydratorNestedRequest(t *testing.T) {
	// Test that hydration request that arrived when application
	// was hydrating is not ignored
	dir := "slow-manifest"
	valuesFile := "values.yaml"
	ctx := Given(t)
	acts := ctx.DrySourcePath(dir).
		DrySourceRevision("HEAD").
		SyncSourcePath(dir).
		SyncSourceBranch("env/test").
		When().
		CreateApp().Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(All(OperationPhaseIs(OperationSucceeded), SyncStatusIs(SyncStatusCodeSynced))).
		When().
		// set long delay for the next helm template invocation
		PatchDrySourceFile(valuesFile, `[{"op": "replace", "path": "/iterations", "value": 400}]`)

	// runs app get --refresh asynchronously, so we do not wait for hydration to finish
	go ctx.When().Refresh(RefreshTypeNormal)

	// wait until Hydration actually runs `helm template`.  We can
	// catch it because the template is really nasty and
	// `helm template` rendering takes tens of seconds
	acts.Then().Expect(HelmTemplateRuns())
	// ps output line containing helm PID and command line
	helmProcessData := acts.GetLastOutput()

	// make another change: removing the long delay: we do not need it for the second template invocation,
	// so the test will run faster
	acts = acts.PatchDrySourceFile(valuesFile, `[{"op": "replace", "path": "/iterations", "value": 1}]`)
	// get last revision after the change
	revision := acts.GitRevList("HEAD", "-1").GetLastOutput()

	// second (nested) refresh request
	go ctx.When().Refresh(RefreshTypeNormal)

	// get process one more time and ensure the same helm process
	// still running, so the second refresh was nested
	acts.Then().Expect(All(HelmTemplateRuns(), Success(helmProcessData)))

	// in the end hydrated to the last committed revision - the second refresh worked
	// it is expected to take a long time if runner is slow
	acts.ThenWithTimeout(80).Expect(All(HydrationPhaseIs(HydrateOperationPhaseHydrated), DryRevisionIs(revision)))
}
