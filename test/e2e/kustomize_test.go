package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// when we have a config map generator, AND the ignore annotation, it is ignored in the app's sync status
func TestSyncStatusOptionIgnore(t *testing.T) {
	var mapName string
	Given(t).
		Path("kustomize-cm-gen").
		// note we don't want to prune resources, check the config maps exist
		Prune(false).
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
		Patch("kustomization.yaml", `[{"op": "replace", "path": "/configMapGenerator/0/literals/0", "value": "foo=baz"}]`).
		Refresh(RefreshTypeHard).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
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
