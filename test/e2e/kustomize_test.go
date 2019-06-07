package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestKustomize2AppSource(t *testing.T) {

	patchLabelMatchesFor := func(kind string) func(app *Application) {
		return func(app *Application) {
			name := "k2-patched-guestbook-ui"
			labelValue, err := fixture.Run(
				"", "kubectl", "-n="+fixture.DeploymentNamespace(),
				"get", kind, name,
				"-ojsonpath={.metadata.labels.patched-by}")
			assert.NoError(t, err)
			assert.Equal(t, "argo-cd", labelValue, "wrong value of 'patched-by' label of %s %s", kind, name)
		}
	}

	Given(t).
		Path(guestbookPath).
		NamePrefix("k2-").
		When().
		Create().
		Refresh(RefreshTypeHard).
		PatchApp(`[
			{
				"op": "replace",
				"path": "/spec/source/kustomize/namePrefix",
				"value": "k2-patched-"
			},
			{
				"op": "add",
				"path": "/spec/source/kustomize/commonLabels",
				"value": {
					"patched-by": "argo-cd"
				}
			}
		]`).
		Then().
		Expect(Success("")).
		When().
		Sync().
		Then().
		And(patchLabelMatchesFor("Service")).
		And(patchLabelMatchesFor("Deployment"))
}

// when we have a config map generator, AND the ignore annotation, it is ignored in the app's sync status
func TestSyncStatusOptionIgnore(t *testing.T) {
	var mapName string
	Given(t).
		Path("kustomize-cm-gen").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		And(func(app *Application) {
			resourceStatus := app.Status.Resources[0]
			assert.Contains(t, resourceStatus.Name, "my-map-")
			assert.Equal(t, SyncStatusCodeSynced, resourceStatus.Status)

			mapName = resourceStatus.Name
		}).
		When().
		// we now force generation of a second CM
		PatchFile("kustomization.yaml", `[{"op": "replace", "path": "/configMapGenerator/0/literals/0", "value": "foo=baz"}]`).
		Refresh(RefreshTypeHard).
		Then().
		// this is standard logging from the command - tough one - true statement
		When().
		Sync().
		Then().
		Expect(Error("1 resources require pruning")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		// this is a key check - we expect the app to be healthy because, even though we have a resources that needs
		// pruning, because it is annotated with IgnoreExtraneous it should not contribute to the sync status
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		And(func(app *Application) {
			assert.Equal(t, 2, len(app.Status.Resources))
			// new map in-sync
			{
				resourceStatus := app.Status.Resources[0]
				assert.Contains(t, resourceStatus.Name, "my-map-")
				// make sure we've a new map with changed name
				assert.NotEqual(t, mapName, resourceStatus.Name)
				assert.Equal(t, SyncStatusCodeSynced, resourceStatus.Status)
			}
			// old map is out of sync
			{
				resourceStatus := app.Status.Resources[1]
				assert.Equal(t, mapName, resourceStatus.Name)
				assert.Equal(t, SyncStatusCodeOutOfSync, resourceStatus.Status)
			}
		})
}
