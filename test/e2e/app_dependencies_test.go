package e2e

import (
	"testing"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
)

func TestSyncWithBlockingDependencies(t *testing.T) {
	// Parent app
	parent := Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.DependsOn = &ApplicationDependency{
				Selectors: []ApplicationSelector{
					{
						LabelSelector: &v1.LabelSelector{
							MatchLabels: map[string]string{"name": "dep1"},
						},
					},
				},
				BlockOnEmpty: pointer.Bool(true),
			}
		})

	// We sync the parent and make sure it's OutOfSync after some time, and
	// that the sync operation is still ongoing, and that the application is
	// waiting for any of its dependencies to be created.
	parent.
		Sync("--async").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(OperationPhaseIs(common.OperationRunning)).
		Expect(OperationMessageContains("Waiting for any app to be created"))

	// Dependency app
	dep1 := GivenWithSameState(t).
		Name("dep1").
		Path("kustomize").
		When().
		CreateFromFile(func(app *Application) {
			app.Labels = map[string]string{
				"name": "dep1",
			}
		})

	// Once the dependency is created (but not synced), the parent app should
	// change its status to indicate that it's found some dependencies and is
	// now waiting for them.
	parent.Then().
		Expect(OperationMessageContains("Waiting for dependencies"))

	// We sync the dependency app
	dep1.Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(common.OperationSucceeded))

	// After the dependency has been synced, the parent's in-progress sync
	// operation must resume and the sync eventually finishes.
	parent.Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(common.OperationSucceeded))

}

func TestSyncWaitingForDependencies(t *testing.T) {
	// Dependency app
	dep1 := Given(t).
		Name("dep1").
		Path("kustomize").
		When().
		CreateFromFile(func(app *Application) {
			app.Labels = map[string]string{
				"name": "dep1",
			}
		})

	// Parent app
	parent := GivenWithSameState(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.DependsOn = &ApplicationDependency{
				Selectors: []ApplicationSelector{
					{
						LabelSelector: &v1.LabelSelector{
							MatchLabels: map[string]string{"name": "dep1"},
						},
					},
				},
			}
		})

	// Parent should be waiting for dependencies after sync
	parent.
		Sync("--async").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(OperationPhaseIs(common.OperationRunning)).
		Expect(OperationMessageContains("Waiting for dependencies"))

	// We sync the dependency app
	dep1.Sync("--async").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(common.OperationSucceeded))

	// After the dependency has been synced, the parent's in-progress sync
	// operation must resume and the sync eventually finishes.
	parent.Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(common.OperationSucceeded))
}

func TestSyncBlockingTimeout(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.DependsOn = &ApplicationDependency{
				Selectors: []ApplicationSelector{
					{
						LabelSelector: &v1.LabelSelector{
							MatchLabels: map[string]string{"name": "dep1"},
						},
					},
				},
				BlockOnEmpty: pointer.Bool(true),
				Timeout:      pointer.Duration(3 * time.Second),
			}
		}).
		Sync("--async").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(OperationPhaseIs(common.OperationFailed)).
		Expect(OperationMessageContains("Timeout waiting for dependencies"))
}
