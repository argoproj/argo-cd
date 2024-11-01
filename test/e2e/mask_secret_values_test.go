package e2e

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/health"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// Values of `.data` & `.stringData“ fields in Secret resources are masked in UI/CLI
// Optionally, values of `.metadata.annotations` can also be masked, if needed.
func TestMaskSecretValues(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|NEWSECRETVAL|U0VDUkVUVkFM`)

	Given(t).
		Path("empty-dir").
		When().
		SetParamInSettingConfigMap("resource.sensitive.mask.annotations", "token"). // hide sensitive annotation
		AddFile("secrets.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: secret
  annotations:
    token: SECRETVAL
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
		// sensitive data should be masked in manifests output
		And(func(app *Application) {
			mnfs, _ := RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))
		}).
		When().
		PatchFile("secrets.yaml", `[{"op": "replace", "path": "/stringData/username", "value": "NEWSECRETVAL"}]`).
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"token": "NEWSECRETVAL"}}]`).
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		// sensitive data should be masked in diff output
		And(func(app *Application) {
			diff, _ := RunCli("app", "diff", app.Name)
			assert.False(t, sensitiveData.MatchString(diff))
		})
}
