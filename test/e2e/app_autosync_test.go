package e2e

import (
	"testing"
	"time"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func TestAutoSyncSelfHealDisabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		// app should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: false}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should be auto-synced if git change detected
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 1}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should not be auto-synced if k8s change detected
		When().
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 0}}`), metav1.PatchOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestAutoSyncSelfHealEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		// app should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{SelfHeal: true},
				Retry:     &RetryStrategy{Limit: 0},
			}
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// app should be auto-synced once k8s change detected
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 0}}`), metav1.PatchOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// app should be attempted to auto-synced once and marked with error after failed attempt detected
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": "badValue"}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		When().
		// Trigger refresh again to make sure controller notices previously failed sync attempt before expectation timeout expires
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionSyncError, "Failed last sync attempt")).
		When().
		// SyncError condition should be removed after successful sync
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 1}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		When().
		// Trigger refresh twice to make sure controller notices successful attempt and removes condition
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Empty(t, app.Status.Conditions)
		})
}

// TestAutoSyncRetryAndRefreshEnabled verifies that auto-sync+refresh picks up fixed commits automatically
func TestAutoSyncRetryAndRefreshEnabled(t *testing.T) {
	limits := []int64{
		100, // Repeat enough times to see we move on to the 3rd commit without reaching the limit
		-1,  // Repeat forever
	}

	for _, limit := range limits {
		Given(t).
			Path(guestbookPath).
			When(). // I create an app with auto-sync and Refresh
			CreateFromFile(func(app *Application) {
				app.Spec.SyncPolicy = &SyncPolicy{
					Automated: &SyncPolicyAutomated{},
					Retry: &RetryStrategy{
						Limit:   limit,
						Refresh: true,
					},
				}
			}).
			Then(). // It should auto-sync correctly
			Expect(OperationPhaseIs(OperationSucceeded)).
			Expect(SyncStatusIs(SyncStatusCodeSynced)).
			Expect(NoConditions()).
			When(). // Auto-sync encounters broken commit
			PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": "badValue"}]`).
			Refresh(RefreshTypeNormal).
			Then(). // It should keep on trying to sync it
			Expect(OperationPhaseIs(OperationRunning)).
			Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
			Expect(OperationRetriedTimes(1)).
			// Wait to make sure the condition is consistent
			And(func(_ *Application) {
				time.Sleep(10 * time.Second)
			}).
			Expect(OperationPhaseIs(OperationRunning)).
			Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
			Expect(OperationRetriedTimes(2)).
			When(). // I push a fixed commit (while auto-sync in progress)
			PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 42}]`).
			Refresh(RefreshTypeNormal).
			Then(). // Argo CD should pick it up and sync it successfully
			// Wait for the sync retry to pick up a new commit
			And(func(_ *Application) {
				time.Sleep(10 * time.Second)
			}).
			Expect(NoConditions()).
			Expect(SyncStatusIs(SyncStatusCodeSynced)).
			Expect(OperationPhaseIs(OperationSucceeded))
	}
}

// TestAutoSyncRetryAndRefreshManualSync verifies that auto-sync+refresh do not pick new commits on manual sync
func TestAutoSyncRetryAndRefreshManualSync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When(). // I create an app with auto-sync and Refresh
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{},
				Retry: &RetryStrategy{
					Limit:   -1,
					Refresh: true,
				},
			}
		}).
		Then(). // It should auto-sync correctly
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		When(). // I manually sync the app on a broken commit
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": "badValue"}]`).
		Sync("--async").
		Then(). // Argo should keep on retrying
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(OperationRetriedTimes(1)).
		And(func(_ *Application) {
			// Wait to make sure the condition is consistent
			time.Sleep(10 * time.Second)
		}).
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(OperationRetriedTimes(2)).
		When(). // I push a fixed commit (during manual sync)
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 42}]`).
		Then(). // Argo CD should keep on retrying the one from the tyme of the sync start
		And(func(_ *Application) {
			// Wait to make sure the condition is consistent
			time.Sleep(10 * time.Second)
		}).
		Expect(OperationRetriedTimes(3)).
		When(). // I terminate the stuck sync and start a new manual one (when ref points to fixed commit)
		TerminateOp().
		And(func() {
			// Wait for the operation to terminate before starting new sync
			time.Sleep(1 * time.Second)
		}).
		Sync("--async").
		Then(). // Argo CD syncs successfully
		Expect(NoConditions()).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(OperationSucceeded))
}
