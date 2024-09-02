package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/argo"
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
			assert.Equal(t, Name(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, Name())
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
		RepoURL:        "https://github.com/argoproj/argocd-example-apps.git",
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
			assert.Equal(t, Name(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, Name())
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
			assert.Len(t, statusByName, 1)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["helm-guestbook"])
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
			assert.Equal(t, Name(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, Name())
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
			output, err := Run("", "kubectl", "describe", "pods", "pod-1", "-n", DeploymentNamespace())
			require.NoError(t, err)
			assert.Contains(t, output, "foo=bar")
		})
}
