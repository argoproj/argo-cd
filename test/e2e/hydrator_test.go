package e2e

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	argoutil "github.com/argoproj/argo-cd/v3/util/argo"

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

func TestHydratorSharedKeyAcrossControllerShards(t *testing.T) {
	if fixture.IsRemote() {
		t.Skip("this test requires control of the local application-controller processes")
	}

	const (
		app1Name = "hydrator-cross-shard-1"
		app2Name = "hydrator-cross-shard-2"
		branch   = "env/test"
		// Each round is an independent chance for the shards to interleave
		// badly, and hydration is only requested mid-flight within a round.
		hydrationRounds    = 5
		dryCommitsPerRound = 3
	)

	ctx := Given(t)
	shard0ClusterName := createClusterSecretWithShard(ctx, 0, ctx.DeploymentNamespace())
	shard1ClusterName := createClusterSecretWithShard(ctx, 1, ctx.DeploymentNamespace())

	require.NoError(t, fixture.StopProcess(fixture.ApplicationControllerProcName))

	app1 := ctx.
		Name(app1Name).
		Timeout(60).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook-1").
		SyncSourceBranch(branch).
		DestName(shard0ClusterName)
	app1.When().CreateApp()
	t.Cleanup(func() {
		_, _ = fixture.RunCli("app", "delete", app1.AppName(), "--cascade", "--yes")
	})

	app2 := GivenWithSameState(ctx).
		Name(app2Name).
		Timeout(60).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook-2").
		SyncSourceBranch(branch).
		DestName(shard1ClusterName)
	app2.When().CreateApp()
	t.Cleanup(func() {
		_, _ = fixture.RunCli("app", "delete", app2.AppName(), "--cascade", "--yes")
	})

	fixture.StartControllerShards(t, 2)

	// Both apps already exist when the two controllers start, so the group is
	// live on both shards before the first hydration request.
	app1.When().Refresh(RefreshTypeNormal)
	app2.When().Refresh(RefreshTypeNormal)
	app1.When().Wait("--hydrated")
	app2.When().Wait("--hydrated")

	// Whether two shards collide on a shared hydration group depends on how
	// their hydrate requests interleave, so make several dry commits and give
	// each one a fresh chance to race.
	group := []*Context{app1, app2}
	for round := range hydrationRounds {
		// Request hydration of a new dry commit while the previous one is
		// still being hydrated, so the shards are working from different dry
		// commits when they push.
		var drySHA string
		var revisionHistoryLimit int
		for burst := range dryCommitsPerRound {
			if burst > 0 {
				time.Sleep(2 * time.Second)
			}
			revisionHistoryLimit = round*dryCommitsPerRound + burst + 4
			drySHA = commitDryChange(t, revisionHistoryLimit)
			requestHydration(t, group...)
		}

		hydratedSHA := requireGroupHydrated(t, drySHA, group...)
		requireHydratedManifests(t, hydratedSHA, revisionHistoryLimit, "guestbook-1", "guestbook-2")

		// The commit the group reports must be the one the branch actually
		// holds. A late push from a second hydration of the group leaves the
		// branch somewhere the apps never agreed on.
		tip, err := fixture.Run(fixture.LocalRepoRoot(), "git", "rev-parse", branch)
		require.NoError(t, err)
		require.Equal(t, hydratedSHA, strings.TrimSpace(tip),
			"%s points at a commit the group did not hydrate", branch)

		requireHydratedBranchNeverRegressed(t, branch)
	}
}

// requireHydratedBranchNeverRegressed checks that the hydrated branch only ever
// moved forward. Every dry commit raises revisionHistoryLimit, so a hydrated
// commit that lowers it is a hydration of an older dry commit landing on top of
// a newer one, which reverts the manifests apps are syncing.
func requireHydratedBranchNeverRegressed(t *testing.T, branch string) {
	t.Helper()
	commits, err := fixture.Run(fixture.LocalRepoRoot(), "git", "rev-list", "--reverse", branch)
	require.NoError(t, err)

	highest := 0
	for commit := range strings.FieldsSeq(commits) {
		manifest, err := fixture.Run(fixture.LocalRepoRoot(), "git", "show", commit+":guestbook-1/manifest.yaml")
		if err != nil {
			// Commits made before the app was first hydrated.
			continue
		}
		limit := revisionHistoryLimitIn(t, manifest)
		require.GreaterOrEqual(t, limit, highest,
			"hydrated commit %s reverted %s from dry change %d back to %d", commit, branch, highest, limit)
		highest = limit
	}
	require.Positive(t, highest, "%s carries no hydrated manifests", branch)
}

var revisionHistoryLimitPattern = regexp.MustCompile(`revisionHistoryLimit:\s*(\d+)`)

func revisionHistoryLimitIn(t *testing.T, manifest string) int {
	t.Helper()
	match := revisionHistoryLimitPattern.FindStringSubmatch(manifest)
	require.Len(t, match, 2, "manifest has no revisionHistoryLimit")
	limit, err := strconv.Atoi(match[1])
	require.NoError(t, err)
	return limit
}

// commitDryChange makes a new dry commit that every app in the group hydrates
// from, and returns the commit it created.
func commitDryChange(t *testing.T, revisionHistoryLimit int) string {
	t.Helper()
	fixture.Patch(t, "guestbook/guestbook-ui-deployment.yaml",
		fmt.Sprintf(`[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": %d}]`, revisionHistoryLimit))
	sha, err := fixture.Run(fixture.LocalRepoRoot(), "git", "rev-parse", "HEAD")
	require.NoError(t, err)
	return strings.TrimSpace(sha)
}

// requestHydration asks every app to hydrate at once, so each shard gets the
// request before any of them has recorded that it started work.
func requestHydration(t *testing.T, apps ...*Context) {
	t.Helper()
	hydrateType := HydrateTypeNormal
	errs := make([]error, len(apps))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, app := range apps {
		wg.Go(func() {
			appIf := fixture.AppClientset.ArgoprojV1alpha1().Applications(app.AppNamespace())
			<-start
			_, errs[i] = argoutil.RefreshApp(appIf, app.AppName(), RefreshTypeNormal, &hydrateType)
		})
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "failed to request hydration of %s", apps[i].AppName())
	}
}

// requireGroupHydrated waits for every app sharing a hydration group to report
// a successful hydration of drySHA, and returns the hydrated commit they agree
// on. Hydration that stalls, fails to push, or produces a separate commit per
// shard never satisfies this.
func requireGroupHydrated(t *testing.T, drySHA string, apps ...*Context) string {
	t.Helper()
	var hydratedSHA string
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		hydratedSHA = ""
		for _, app := range apps {
			got, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(app.AppNamespace()).
				Get(context.Background(), app.AppName(), metav1.GetOptions{})
			if !assert.NoError(c, err) {
				return
			}
			status := got.Status.SourceHydrator
			if current := status.CurrentOperation; current != nil {
				assert.NotEqual(c, HydrateOperationPhaseFailed, current.Phase,
					"%s failed to hydrate %s: %s", app.AppName(), drySHA, current.Message)
			}
			if !assert.NotNil(c, status.LastSuccessfulOperation, "%s has never hydrated", app.AppName()) {
				return
			}
			assert.Equal(c, drySHA, status.LastSuccessfulOperation.DrySHA,
				"%s has not hydrated dry commit %s", app.AppName(), drySHA)
			if hydratedSHA == "" {
				hydratedSHA = status.LastSuccessfulOperation.HydratedSHA
			}
			assert.Equal(c, hydratedSHA, status.LastSuccessfulOperation.HydratedSHA,
				"%s hydrated a different commit than the rest of its group", app.AppName())
		}
	}, 2*time.Minute, time.Second)
	require.NotEmpty(t, hydratedSHA)
	return hydratedSHA
}

// requireHydratedManifests checks that the commit the group agreed on carries
// the latest dry change for every app, so a push that drops or reverts a group
// member's manifests is caught even when both apps report success.
func requireHydratedManifests(t *testing.T, hydratedSHA string, revisionHistoryLimit int, syncSourcePaths ...string) {
	t.Helper()
	for _, syncSourcePath := range syncSourcePaths {
		manifest, err := fixture.Run(fixture.LocalRepoRoot(), "git", "show", hydratedSHA+":"+syncSourcePath+"/manifest.yaml")
		require.NoError(t, err, "hydrated commit %s has no manifests for %s", hydratedSHA, syncSourcePath)
		require.Contains(t, manifest, fmt.Sprintf("revisionHistoryLimit: %d", revisionHistoryLimit),
			"%s in hydrated commit %s is missing the latest dry change", syncSourcePath, hydratedSHA)
	}
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
