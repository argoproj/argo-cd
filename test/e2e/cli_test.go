package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestCliAppCommand(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		Create().
		And(func() {
			output, err := RunCli("app", "sync", Name())
			assert.NoError(t, err)
			expected := Tmpl(
				`GROUP KIND NAMESPACE NAME STATUS HEALTH HOOK MESSAGE
 Pod {{.Namespace}} pod Synced Healthy pod/pod created
 Pod {{.Namespace}} hook Succeeded Sync pod/hook created`,
				map[string]interface{}{"Name": Name(), "Namespace": DeploymentNamespace()})
			assert.Contains(t, NormalizeOutput(output), expected)
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		And(func(_ *Application) {
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
			expected := Tmpl(
				`{{.Name}} https://kubernetes.default.svc {{.Namespace}} default Synced Healthy <none> <none>`,
				map[string]interface{}{"Name": Name(), "Namespace": DeploymentNamespace()})
			assert.Contains(t, NormalizeOutput(output), expected)
		})
}
