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
			assert.Equal(t, Name(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
				assert.Equal(t, sources[i].Name, source.Name)
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(app *Application) {
			// we remove the first source
			output, err := RunCli("app", "remove-source", Name(), "--source-name", sources[0].Name)
			require.NoError(t, err)
			assert.Contains(t, output, "updated successfully")
		}).
		Expect(Success("")).
		And(func(app *Application) {
			assert.Len(t, app.Spec.GetSources(), 1)
			// we add a source
			output, err := RunCli("app", "add-source", Name(), "--source-name", sources[0].Name, "--repo", RepoURL(RepoURLTypeFile), "--path", guestbookPath)
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
			assert.Equal(t, Name(), app.Name)
			for i, source := range app.Spec.GetSources() {
				assert.Equal(t, sources[i].RepoURL, source.RepoURL)
				assert.Equal(t, sources[i].Path, source.Path)
				assert.Equal(t, sources[i].Name, source.Name)
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(app *Application) {
			_, err := RunCli("app", "set", Name(), "--source-name", sources[1].Name, "--path", "deployment")
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
		And(func(app *Application) {
			_, err := RunCli("app", "get", Name(), "--source-name", sources[1].Name, "--source-position", "1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Only one of source-position and source-name can be specified.")
		}).
		And(func(app *Application) {
			_, err := RunCli("app", "manifests", Name(), "--revisions", "0.0.2", "--source-names", sources[0].Name, "--revisions", "0.0.2", "--source-positions", "1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Only one of source-positions and source-names can be specified.")
		})
}
