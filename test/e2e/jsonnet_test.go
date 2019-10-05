package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/util/kube"
)

func TestJsonnetAppliedCorrectly(t *testing.T) {
	Given(t).
		Path("jsonnet-tla").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML(manifests)
			assert.NoError(t, err)

			index := -1
			for i := range resources {
				if resources[i].GetKind() == kube.DeploymentKind {
					index = i
					break
				}
			}

			assert.True(t, index > -1)

			deployment := resources[index]
			assert.Equal(t, "jsonnet-guestbook-ui", deployment.GetName())
			assert.Equal(t, int64(1), *kube.GetDeploymentReplicas(deployment))
		})
}

func TestJsonnetTlaParameterAppliedCorrectly(t *testing.T) {
	Given(t).
		Path("jsonnet-tla").
		JsonnetTLAStrParameter("name=testing-tla").
		JsonnetTLACodeParameter("replicas=3").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML(manifests)
			assert.NoError(t, err)

			index := -1
			for i := range resources {
				if resources[i].GetKind() == kube.DeploymentKind {
					index = i
					break
				}
			}

			assert.True(t, index > -1)

			deployment := resources[index]
			assert.Equal(t, "testing-tla", deployment.GetName())
			assert.Equal(t, int64(3), *kube.GetDeploymentReplicas(deployment))
		})
}

func TestJsonnetTlaEnv(t *testing.T) {
	Given(t).
		Path("jsonnet-tla-cm").
		JsonnetTLAStrParameter("foo=$ARGOCD_APP_NAME").
		JsonnetTLACodeParameter("bar='$ARGOCD_APP_NAME'").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.bar}")).(string))
		})
}
func TestJsonnetExtVarEnv(t *testing.T) {
	Given(t).
		Path("jsonnet-ext-var").
		JsonnetExtVarStr("foo=$ARGOCD_APP_NAME").
		JsonnetExtVarCode("bar='$ARGOCD_APP_NAME'").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.bar}")).(string))
		})
}
