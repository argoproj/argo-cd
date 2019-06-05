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
