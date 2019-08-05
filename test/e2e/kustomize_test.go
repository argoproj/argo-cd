package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/errors"

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
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
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
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		And(patchLabelMatchesFor("Service")).
		And(patchLabelMatchesFor("Deployment"))
}

// when we have a config map generator, AND the ignore annotation, it is ignored in the app's sync status
func TestSyncStatusOptionIgnore(t *testing.T) {
	var mapName string
	Given(t).
		Path("kustomize-cm-gen").
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
		PatchFile("kustomization.yaml", `[{"op": "replace", "path": "/configMapGenerator/0/literals/0", "value": "foo=baz"}]`).
		Refresh(RefreshTypeHard).
		Then().
		// this is standard logging from the command - tough one - true statement
		When().
		IgnoreErrors().
		Sync().
		Then().
		Expect(Error("", "1 resources require pruning")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		// this is a key check - we expect the app to be healthy because, even though we have a resources that needs
		// pruning, because it is annotated with IgnoreExtraneous it should not contribute to the sync status
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

// make sure we can create an app which has a SSH remote base
func TestKustomizeSSHRemoteBase(t *testing.T) {
	Given(t).
		// not the best test, as we should have two remote repos both with the same SSH private key
		SSHInsecureRepoURLAdded().
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("ssh-kustomize-base").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceSyncStatusIs("ConfigMap", "my-map", SyncStatusCodeSynced))
}

// make sure we can create an app which has a SSH remote base
func TestKustomizeDeclarativeInvalidApp(t *testing.T) {
	Given(t).
		Path("invalid-kustomize").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		Expect(Condition(ApplicationConditionComparisonError, "invalid-kustomize/does-not-exist.yaml: no such file or directory"))
}

func TestKustomizeBuildOptionsLoadRestrictor(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		And(func() {
			errors.FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm",
				"-n", fixture.ArgoCDNamespace,
				"-p", `{ "data": { "kustomize.buildOptions": "--load_restrictor none" } }`))
		}).
		When().
		PatchFile("kustomization.yaml", `[{"op": "replace", "path": "/resources/1", "value": "../guestbook_local/guestbook-ui-svc.yaml"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		And(func() {
			errors.FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm",
				"-n", fixture.ArgoCDNamespace,
				"-p", `{ "data": { "kustomize.buildOptions": "" } }`))
		})
}

// make sure we we can invoke the CLI to replace images
func TestKustomizeImages(t *testing.T) {
	Given(t).
		Path("kustomize").
		When().
		Create().
		// pass two flags to check the multi flag logic works
		AppSet("--kustomize-image", "alpine:foo", "--kustomize-image", "alpine:bar").
		Then().
		And(func(app *Application) {
			assert.Contains(t, app.Spec.Source.Kustomize.Images, KustomizeImage("alpine:bar"))
		})
}
