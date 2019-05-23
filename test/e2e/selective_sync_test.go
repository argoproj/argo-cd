package e2e

import (
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/stretchr/testify/assert"
	"testing"
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
		And(func(app *Application) {
			// service
			assert.Equal(t, HealthStatusHealthy, app.Status.Resources[0].Health.Status)
			// deployment
			assert.Equal(t, HealthStatusMissing, app.Status.Resources[1].Health.Status)
		})
}
