package e2e

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// Secret values shouldn't be exposed in error messages and the diff view
// when invalid secret is synced.
func TestMaskValuesInInvalidSecret(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|U0VDUkVUVkFM|12345`)

	Given(t).
		Path("empty-dir").
		When().
		// valid secret
		AddFile("secrets.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: secret
  annotations:
    app: test
stringData:
  username: SECRETVAL
data:
  password: U0VDUkVUVkFM
`).
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		// secret data shouldn't be exposed in manifests output
		And(func(app *Application) {
			mnfs, _ := RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))
		}).
		When().
		// invalidate secret
		PatchFile("secrets.yaml", `[{"op": "replace", "path": "/data/password", "value": 12345}]`).
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(OperationPhaseIs(common.OperationFailed)).
		// secret data shouldn't be exposed in manifests, diff & error output for invalid secret
		And(func(app *Application) {
			mnfs, _ := RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))

			diff, _ := RunCli("app", "diff", app.Name)
			assert.False(t, sensitiveData.MatchString(diff))

			msg := app.Status.OperationState.Message
			assert.False(t, sensitiveData.MatchString(msg))
		})
}
