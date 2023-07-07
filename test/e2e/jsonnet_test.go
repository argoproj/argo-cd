package e2e

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

func TestJsonnetAppliedCorrectly(t *testing.T) {
	Given(t).
		Path("jsonnet-tla").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML([]byte(manifests))
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
		When().
		CreateApp("--jsonnet-tla-str", "name=testing-tla", "--jsonnet-tla-code", "replicas=0").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML([]byte(manifests))
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
			assert.Equal(t, int64(0), *kube.GetDeploymentReplicas(deployment))
		})
}

func TestJsonnetTlaEnv(t *testing.T) {
	Given(t).
		Path("jsonnet-tla-cm").
		When().
		CreateApp("--jsonnet-tla-str", "foo=$ARGOCD_APP_NAME", "--jsonnet-tla-code", "bar='$ARGOCD_APP_NAME'").
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
		When().
		CreateApp("--jsonnet-ext-var-str", "foo=$ARGOCD_APP_NAME", "--jsonnet-ext-var-code", "bar='$ARGOCD_APP_NAME'").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.bar}")).(string))
		})
}

//Jsonnet file located in nested sub directory uses import
func TestJsonnetNestedDirWithImports(t *testing.T) {
	Given(t).
		Path("jsonnet-nested-dir-with-imports/apps").
		When().
		CreateApp("--directory-recurse").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("Namespace", "hello-world", SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("Namespace", "hello-root", SyncStatusCodeSynced))
}
