package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/argo"
)

func TestMultiSourceFromAppDir(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "guestbook-envspec",
		From: []CopyFromSpec{{
			SourcePath:               "$envvalues/copyfrom-envfiles/dev",
			DestinationPath:          "guestbook-envspec/env",
			FailOnOutOfBoundsSymlink: false,
		},
		},
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Ref:     "envvalues",
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
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
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
			assert.Len(t, statusByName, 2)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["copyfrom-envspec-cm"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])
		})
}

func TestMultiSourceFromAppFile(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "guestbook-envspec",
		From: []CopyFromSpec{{
			SourcePath:               "$envvalues/copyfrom-envfiles/dev/cm.yaml",
			DestinationPath:          "guestbook-envspec/env/cm.yaml",
			FailOnOutOfBoundsSymlink: false,
		},
		},
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Ref:     "envvalues",
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
			}
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
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
			assert.Len(t, statusByName, 2)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["copyfrom-envspec-cm"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])
		})
}
