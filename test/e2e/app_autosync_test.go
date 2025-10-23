package e2e

import (
	"fmt"
	"testing"
	"time"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

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

// TestAutoSyncRetryAndRefreshEnabled verifies that auto-sync+refresh picks up new commits automatically
func TestAutoSyncRetryAndRefreshEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When(). // I create an app with auto-sync and Refresh
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{},
				Retry: &RetryStrategy{
					Limit:   -1,
					Refresh: true,
					Backoff: &Backoff{
						Duration:    time.Second.String(),
						Factor:      ptr.To(int64(1)),
						MaxDuration: time.Second.String(),
					},
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
		Expect(OperationRetriedMinimumTimes(1)).
		When(). // I push a fixed commit (while auto-sync in progress)
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 42}]`).
		Refresh(RefreshTypeNormal).
		Then().
		// Argo CD should pick it up and sync it successfully
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

// TestAutoSyncRetryAndRefreshEnabled verifies that auto-sync+refresh picks up new commits automatically on the original source
// at the time the sync was triggered
func TestAutoSyncRetryAndRefreshEnabledChangedSource(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When(). // I create an app with auto-sync and Refresh
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{},
				Retry: &RetryStrategy{
					Limit:   -1, // Repeat forever
					Refresh: true,
					Backoff: &Backoff{
						Duration:    time.Second.String(),
						Factor:      ptr.To(int64(1)),
						MaxDuration: time.Second.String(),
					},
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
		Expect(OperationRetriedMinimumTimes(1)).
		When().
		PatchApp(`[{"op": "add", "path": "/spec/source/path", "value": "failure-during-sync"}]`).
		// push a fixed commit on HEAD branch
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 42}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Status(func(status ApplicationStatus) (bool, string) {
			// Validate that the history contains the sync to the previous sources
			// The history will only contain  successful sync
			if len(status.History) != 2 {
				return false, "expected len to be 2"
			}
			if status.History[1].Source.Path != guestbookPath {
				return false, fmt.Sprintf("expected source path to be '%s'", guestbookPath)
			}
			return true, ""
		}))
}
