package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
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
		CreateMultiSourceApp().
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
		CreateMultiSourceApp().
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
		CreateMultiSourceApp().
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
		CreateMultiSourceApp().
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
		CreateMultiSourceApp().
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

func TestMultiSourceAppErrorWhenSourceNameAndSourcePosition(t *testing.T) {
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
		CreateMultiSourceApp().
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

// TestMultiSourceAppWithWorktreeSameRepoMultipleRevisions tests the scenario where
// a multi-source application uses the same git repository with different target revisions.
// This validates that git worktrees are properly used to checkout different revisions
// simultaneously and that value files are correctly resolved from the appropriate worktree.
//
// The revisions must actually differ for this to exercise the worktree path: the reposerver
// only creates a worktree when the ref source resolves the repository to a different commit
// than the primary source (see needsWorktree in reposerver/repository/repository.go). If both
// sources tracked HEAD they would resolve to the same commit and the ordinary single-checkout
// path would handle the app, which is legal on any release and would prove nothing.
func TestMultiSourceAppWithWorktreeSameRepoMultipleRevisions(t *testing.T) {
	// Tags the initial commit, where multiple-source-values/values.yaml is `replicas: 3`.
	const pinnedValuesTag = "pinned-values"
	// Committed on top of the tag, so HEAD carries a different value than the pinned revision.
	const headValuesFile = "replicas: 5\n"

	sources := []ApplicationSource{{
		RepoURL:        RepoURL(RepoURLTypeFile),
		TargetRevision: pinnedValuesTag,
		Ref:            "values",
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
	ctx := Given(t)
	ctx.
		Sources(sources).
		When().
		// Pin the tag to the current values, then move HEAD past it. The ref source stays on the
		// tag while the chart source follows HEAD, so the two sources need separate checkouts of
		// the same repository at once.
		AddTag(pinnedValuesTag).
		AddFile("multiple-source-values/values.yaml", headValuesFile).
		CreateMultiSourceAppFromFile(func(_ *Application) {}).
		Then().
		And(func(app *Application) {
			assert.Equal(t, ctx.GetName(), app.Name)
			assert.Len(t, app.Spec.GetSources(), 2)
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
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
			// The helm-guestbook should be synced with values from the ref source
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])

			// Confirm the values file was read from the worktree checked out at the pinned tag.
			// The three candidate sources of this value are distinct, so the assertion pins down
			// which checkout was used: 3 = the pinned worktree (correct), 5 = the primary HEAD
			// checkout (worktree ignored/misresolved), 1 = the chart's own values.yaml default
			// (the $values ref was dropped entirely).
			output, err := Run("", "kubectl", "get", "deployment", "guestbook-ui", "-n", ctx.DeploymentNamespace(), "-o", "jsonpath={.spec.replicas}")
			require.NoError(t, err)
			assert.Equal(t, "3", output, "expected 3 replicas from the values file at %q; 5 means the primary HEAD checkout was used instead of the worktree, 1 means the $values ref was not applied", pinnedValuesTag)
		})
}
