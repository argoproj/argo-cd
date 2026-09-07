package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestAppDescription verifies that an optional free-text description can be set on an
// Application, is persisted through the API/CLI, and does not interfere with syncing.
func TestAppDescription(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		PatchApp(`[{"op": "add", "path": "/spec/description", "value": "guestbook example application"}]`).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "guestbook example application", app.Spec.Description)
		})
}
