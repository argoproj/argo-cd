package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/stretchr/testify/assert"
)

func TestSelectiveSync(t *testing.T) {
	Given(t).
		Path("guestbook").
		SelectedResource(":Service:guestbook-ui").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		// TODO - create expect
		And(func(app *Application) {
			// service
			assert.Equal(t, HealthStatusHealthy, app.Status.Resources[0].Health.Status)
			// deployment
			assert.Equal(t, HealthStatusMissing, app.Status.Resources[1].Health.Status)
		})
}

func TestSelectiveSyncDoesNotRunHooks(t *testing.T) {
	Given(t).
		Path("hook").
		SelectedResource(":Pod:pod").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		// you might not expect this, but yes - it is is sync
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// overall status is missing
			assert.Equal(t, HealthStatusMissing, app.Status.Health.Status)
			// hook is missing because we skip them
			assert.Equal(t, HealthStatusMissing, app.Status.Resources[0].Health.Status)
			// pod
			assert.Equal(t, HealthStatusHealthy, app.Status.Resources[1].Health.Status)
		})
}
