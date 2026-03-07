package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
	. "github.com/argoproj/argo-cd/v3/util/argo"
)

func TestMultiSourceAppCreation(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
	}}
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.GetName())
		}).
		Expect(Success("")).
		Given().Timeout(60).
		When().Wait().Then().
		Expect(Success("")).
		And(func(app *Application) {
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			// check if the app has 3 resources, guestbook and 2 pods
			assert.Len(t, statusByName, 3)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-1"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-2"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])
		})
}

func TestMultiSourceAppWithHelmExternalValueFiles(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Ref:     "values",
	}, {
		RepoURL:        RepoURL(RepoURLTypeFile),
		TargetRevision: "HEAD",
		Path:           "helm-guestbook",
		Helm: &ApplicationSourceHelm{
			ReleaseName: "helm-guestbook",
			ValueFiles: []string{
				"$values/multiple-source-values/values.yaml",
			},
		},
	}}
	fmt.Printf("sources: %v\n", sources)
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.GetName())
		}).
		Expect(Success("")).
		Given().Timeout(60).
		When().Wait().Then().
		Expect(Success("")).
		And(func(app *Application) {
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			assert.Len(t, statusByName, 1)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])

			// Confirm that the deployment has 3 replicas.
			output, err := Run("", "kubectl", "get", "deployment", "guestbook-ui", "-n", ctx.DeploymentNamespace(), "-o", "jsonpath={.spec.replicas}")
			require.NoError(t, err)
			assert.Equal(t, "3", output, "Expected 3 replicas for the helm-guestbook deployment")
		})
}

func TestMultiSourceAppWithSourceOverride(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "multiple-source-values",
	}}
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.GetName())
		}).
		Expect(Success("")).
		Given().Timeout(60).
		When().Wait().Then().
		Expect(Success("")).
		And(func(app *Application) {
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			// check if the app has 3 resources, guestbook and 2 pods
			assert.Len(t, statusByName, 3)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-1"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-2"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])

			// check if label was added to the pod to make sure resource was taken from the later source
			output, err := Run("", "kubectl", "describe", "pods", "pod-1", "-n", ctx.DeploymentNamespace())
			require.NoError(t, err)
			assert.Contains(t, output, "foo=bar")
		})
}

func TestMultiSourceAppWithSourceName(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "guestbook",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
		Name:    "dynamic duo",
	}}
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
				assert.Equal(t, sources[i].Name, source.Name)
			}
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// we remove the first source
			output, err := RunCli("app", "remove-source", ctx.GetName(), "--source-name", sources[0].Name)
			require.NoError(t, err)
			assert.Contains(t, output, "updated successfully")
		}).
		Expect(Success("")).
		And(func(app *Application) {
			assert.Len(t, app.Spec.GetSources(), 1)
			// we add a source
			output, err := RunCli("app", "add-source", ctx.GetName(), "--source-name", sources[0].Name, "--repo", RepoURL(RepoURLTypeFile), "--path", guestbookPath)
			require.NoError(t, err)
			assert.Contains(t, output, "updated successfully")
		}).
		Expect(Success("")).
		Given().Timeout(60).
		When().Wait().Then().
		Expect(Success("")).
		And(func(app *Application) {
			assert.Len(t, app.Spec.GetSources(), 2)
			// sources order has been inverted
			assert.Equal(t, sources[1].Name, app.Spec.GetSources()[0].Name)
			assert.Equal(t, sources[0].Name, app.Spec.GetSources()[1].Name)
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			// check if the app has 3 resources, guestbook and 2 pods
			assert.Len(t, statusByName, 3)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-1"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-2"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])
		})
}

func TestMultiSourceAppSetWithSourceName(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "guestbook",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
		Name:    "dynamic duo",
	}}
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
				assert.Equal(t, sources[i].Name, source.Name)
			}
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			_, err := RunCli("app", "set", ctx.GetName(), "--source-name", sources[1].Name, "--path", "deployment")
			require.NoError(t, err)
		}).
		Expect(Success("")).
		And(func(app *Application) {
			assert.Equal(t, "deployment", app.Spec.GetSources()[1].Path)
		})
}

func TestMultiSourceApptErrorWhenSourceNameAndSourcePosition(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "guestbook",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
		Name:    "dynamic duo",
	}}
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			_, err := RunCli("app", "get", ctx.GetName(), "--source-name", sources[1].Name, "--source-position", "1")
			assert.ErrorContains(t, err, "Only one of source-position and source-name can be specified.")
		}).
		And(func(_ *Application) {
			_, err := RunCli("app", "manifests", ctx.GetName(), "--revisions", "0.0.2", "--source-names", sources[0].Name, "--revisions", "0.0.2", "--source-positions", "1")
			assert.ErrorContains(t, err, "Only one of source-positions and source-names can be specified.")
		})
}

// TestMultiSourceRevisionResolutions verifies that .status.sync.resolutions[] and
// .status.operationState.syncResult.resolutions[] are populated correctly for a
// multi-source application that exercises every distinct code path:
//
//   - sources[0]: OCI Helm semver constraint  → entry populated (constraint resolved)
//   - sources[1]: OCI Helm pinned version     → entry empty    (versions.IsVersion early-return)
//   - sources[2]: Git HEAD                    → entry empty    (git non-semver path)
//
// The three sources produce non-overlapping resources so the sync succeeds cleanly:
// sources[0] → ConfigMap my-map, sources[1] → Deployment+Service guestbook-ui,
// sources[2] → two bare Pods.
func TestMultiSourceRevisionResolutions(t *testing.T) {
	repos.PushChartToOCIRegistry(t, "testdata/helm-values", "helm-values", "1.0.0")
	repos.PushChartToOCIRegistry(t, "testdata/helm-guestbook", "helm-guestbook", "1.0.0")

	sources := []ApplicationSource{
		{
			// Semver constraint: resolution IS populated (constraint + resolvedSymbol + revision).
			RepoURL:        HelmOCIRegistryURL,
			Chart:          "helm-values",
			TargetRevision: ">=1.0.0",
		},
		{
			// Pinned OCI version: resolution is NOT populated because versions.IsVersion("1.0.0")
			// returns true and newHelmClientResolveRevision short-circuits before resolving tags.
			RepoURL:        HelmOCIRegistryURL,
			Chart:          "helm-guestbook",
			TargetRevision: "1.0.0",
		},
		{
			// Plain git HEAD: resolution is NOT populated (newClientResolveRevisionWithResolution
			// only sets resolution for semver-looking revisions).
			RepoURL:        RepoURL(RepoURLTypeFile),
			Path:           "two-nice-pods",
			TargetRevision: "HEAD",
		},
	}

	Given(t).
		HelmOCIRepoAdded("myrepo").
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			syncResolutions := app.Status.Sync.Resolutions
			require.Len(t, syncResolutions, 3, "Resolutions must be index-aligned with Sources")

			// sources[0]: semver constraint → full resolution.
			assert.Equal(t, "1.0.0", syncResolutions[0].ResolvedSymbol)
			assert.Equal(t, ">=1.0.0", syncResolutions[0].Constraint)
			assert.Equal(t, "1.0.0", syncResolutions[0].Revision)

			// sources[1]: pinned OCI version → revision only, no constraint resolution.
			assert.Equal(t, "1.0.0", syncResolutions[1].Revision, "pinned OCI version should have revision")
			assert.Empty(t, syncResolutions[1].ResolvedSymbol, "pinned OCI version should have no resolvedSymbol")
			assert.Empty(t, syncResolutions[1].Constraint, "pinned OCI version should have no constraint")

			// sources[2]: git HEAD → revision (SHA) only, no constraint resolution.
			assert.NotEmpty(t, syncResolutions[2].Revision, "git HEAD should have a resolved SHA")
			assert.Empty(t, syncResolutions[2].ResolvedSymbol, "git HEAD should have no resolvedSymbol")
			assert.Empty(t, syncResolutions[2].Constraint, "git HEAD should have no constraint")

			// SyncResult.Resolutions must mirror the same structure.
			require.NotNil(t, app.Status.OperationState)
			require.NotNil(t, app.Status.OperationState.SyncResult)
			syncResultResolutions := app.Status.OperationState.SyncResult.Resolutions
			require.Len(t, syncResultResolutions, 3, "SyncResult.Resolutions must be index-aligned with Sources")

			assert.Equal(t, "1.0.0", syncResultResolutions[0].ResolvedSymbol)
			assert.Equal(t, ">=1.0.0", syncResultResolutions[0].Constraint)
			assert.Equal(t, "1.0.0", syncResultResolutions[0].Revision)
			assert.Equal(t, "1.0.0", syncResultResolutions[1].Revision, "pinned OCI version should have revision")
			assert.Empty(t, syncResultResolutions[1].ResolvedSymbol, "pinned OCI version should have no resolvedSymbol")
			assert.NotEmpty(t, syncResultResolutions[2].Revision, "git HEAD should have a resolved SHA")
			assert.Empty(t, syncResultResolutions[2].ResolvedSymbol, "git HEAD should have no resolvedSymbol")

			// Singular Resolution must be nil for multi-source apps.
			assert.Nil(t, app.Status.Sync.Resolution, "singular Resolution must be nil for multi-source apps")
			assert.Nil(t, app.Status.OperationState.SyncResult.Resolution)
		})
}
