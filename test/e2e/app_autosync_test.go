package e2e

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func TestAutoSyncSelfHealDisabled(t *testing.T) {
	ctx := Given(t)
	ctx.Path(guestbookPath).
		When().
		// app should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: new(false)}}
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
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 0}}`), metav1.PatchOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestAutoSyncSelfHealEnabled(t *testing.T) {
	ctx := Given(t)
	ctx.Path(guestbookPath).
		When().
		// app should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{SelfHeal: new(true)},
				Retry:     &RetryStrategy{Limit: 0},
			}
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// app should be auto-synced once k8s change detected
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Patch(t.Context(),
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
						Factor:      new(int64(1)),
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
						Factor:      new(int64(1)),
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

// TestAutoSyncAllowEmptyCanBeDisabled verifies that setting allowEmpty=false via CLI is persisted.
// Regression test: with bool+omitempty, false was silently dropped from JSON so the field could never be unset.
func TestAutoSyncAllowEmptyCanBeDisabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{AllowEmpty: new(true)},
			}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Spec.SyncPolicy.Automated.AllowEmpty)
			assert.True(t, *app.Spec.SyncPolicy.Automated.AllowEmpty)
		}).
		When().
		AppSet("--allow-empty=false").
		Then().
		And(func(app *Application) {
			require.NotNil(t, app.Spec.SyncPolicy.Automated.AllowEmpty, "allowEmpty should not be nil after being explicitly set to false")
			assert.False(t, *app.Spec.SyncPolicy.Automated.AllowEmpty, "allowEmpty=false should be persisted, not silently dropped")
		})
}

type pausedRollbackApp struct {
	// firstRevision is the revision the application was rolled back to, and the one now deployed.
	firstRevision string
	// badRevision is the revision the application was rolled back from, and the one now recorded.
	badRevision string
	// firstHistoryID identifies the history entry holding firstRevision.
	firstHistoryID int64
	// historyLen is the length of the revision history once the rollback has been recorded.
	historyLen int
	// rollbackToFirst rolls the application back to firstHistoryID again.
	rollbackToFirst func()
}

func givenPausedRollbackAwareApp(t *testing.T, ctx *Context, automated *SyncPolicyAutomated) *pausedRollbackApp {
	t.Helper()
	paused := &pausedRollbackApp{}
	paused.rollbackToFirst = func() {
		_, err := fixture.RunCli("app", "rollback", ctx.AppName(), strconv.FormatInt(paused.firstHistoryID, 10))
		require.NoError(t, err)
	}
	ctx.Path(guestbookPath).
		When().
		SetParamInSettingConfigMap("application.rollbackAwareAutoSyncEnabled", "true").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: automated}
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Len(t, app.Status.History, 1)
			paused.firstRevision = app.Status.Sync.Revision
			paused.firstHistoryID = app.Status.History[0].ID
		}).
		// the offending commit, which automated sync deploys as usual
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 1}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Len(t, app.Status.History, 2)
			require.NotEqual(t, paused.firstRevision, app.Status.Sync.Revision)
			paused.badRevision = app.Status.Sync.Revision
		}).
		// the rollback is accepted with automated sync still enabled, and pauses it
		When().
		And(paused.rollbackToFirst).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionAutoSyncPausedWarning, "was rolled back")).
		And(func(app *Application) {
			require.Equal(t, paused.badRevision, app.Status.RolledBackRevision)
			require.NotNil(t, app.Status.OperationState.SyncResult)
			assert.Equal(t, paused.firstRevision, app.Status.OperationState.SyncResult.Revision, "the rollback deployed the earlier revision")
			assert.Nil(t, app.Operation, "automated sync must not re-deploy the rolled-back revision")
			paused.historyLen = len(app.Status.History)
		})
	return paused
}

func TestAutoSyncRollbackAware(t *testing.T) {
	ctx := Given(t)
	paused := givenPausedRollbackAwareApp(t, ctx, &SyncPolicyAutomated{})
	ctx.When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 2}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Empty(t, app.Status.RolledBackRevision)
			assert.NotEqual(t, paused.badRevision, app.Status.Sync.Revision)
			assert.Equal(t, app.Status.Sync.Revision, app.Status.OperationState.SyncResult.Revision)
		})
}

func TestAutoSyncRollbackAwareRevertClearsRecord(t *testing.T) {
	ctx := Given(t)
	paused := givenPausedRollbackAwareApp(t, ctx, &SyncPolicyAutomated{})
	// restoring the original value is a revert: the manifests match what the rollback already applied
	ctx.When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 3}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		Expect(Status(func(status ApplicationStatus) (bool, string) {
			return status.RolledBackRevision == "", fmt.Sprintf("rolled back revision to be cleared, is %q", status.RolledBackRevision)
		})).
		And(func(app *Application) {
			assert.NotEqual(t, paused.badRevision, app.Status.Sync.Revision, "the revert is a new revision")
			// Nothing needed applying, so the last operation is still the rollback itself.
			assert.Equal(t, paused.firstRevision, app.Status.OperationState.SyncResult.Revision)
			assert.Len(t, app.Status.History, paused.historyLen, "no sync runs when the revert leaves the application Synced")
		})
}

func TestAutoSyncRollbackAwarePausesSelfHeal(t *testing.T) {
	ctx := Given(t)
	paused := givenPausedRollbackAwareApp(t, ctx, &SyncPolicyAutomated{SelfHeal: new(true)})
	deployments := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace())
	liveRevisionHistoryLimit := func() int32 {
		deploy, err := deployments.Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, deploy.Spec.RevisionHistoryLimit)
		return *deploy.Spec.RevisionHistoryLimit
	}
	// change the live cluster behind Argo CD's back
	ctx.When().
		And(func() {
			errors.NewHandler(t).FailOnErr(deployments.Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 0}}`), metav1.PatchOptions{}))
		}).
		// Two refreshes, so that more than one reconciliation has seen both the drift and self-heal
		// enabled. Refresh is synchronous: the API server holds the request open until the controller
		// has processed the refresh annotation and removed it again, so each call is one completed
		// reconciliation. Waiting on status.reconciledAt instead cannot work, because metav1.Time
		// serialises at second precision, so two reconciliations within the same second are not
		// After() one another and the wait can never be satisfied.
		Refresh(RefreshTypeNormal).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionAutoSyncPausedWarning, "was rolled back")).
		And(func(app *Application) {
			assert.Equal(t, paused.badRevision, app.Status.RolledBackRevision, "self-heal must not clear the record")
			assert.Len(t, app.Status.History, paused.historyLen, "self-heal must not sync while paused")
			assert.Equal(t, paused.firstRevision, app.Status.OperationState.SyncResult.Revision)
			assert.Equal(t, int32(0), liveRevisionHistoryLimit(), "the live change must survive while automated sync is paused")
		}).
		// once the source moves on, the pause lifts and the drift is corrected along with it
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 2}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Empty(t, app.Status.RolledBackRevision)
			assert.Equal(t, int32(2), liveRevisionHistoryLimit(), "self-heal works again once the pause lifts")
		})
}

func TestAutoSyncRollbackAwareManualSyncClearsRecord(t *testing.T) {
	ctx := Given(t)
	paused := givenPausedRollbackAwareApp(t, ctx, &SyncPolicyAutomated{})
	ctx.When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Empty(t, app.Status.RolledBackRevision, "a manual sync clears the record")
			assert.Equal(t, paused.badRevision, app.Status.Sync.Revision, "the revision that was rolled back from is deployed again")
			require.NotNil(t, app.Status.OperationState.SyncResult)
			assert.Equal(t, paused.badRevision, app.Status.OperationState.SyncResult.Revision)
		}).
		// with the record gone there is nothing left to pause on
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Empty(t, app.Status.RolledBackRevision, "automated sync must not pause again on an accepted revision")
			assert.Nil(t, app.Operation)
		})
}

func TestAutoSyncRollbackAwareSecondRollbackReplacesRecord(t *testing.T) {
	ctx := Given(t)
	paused := givenPausedRollbackAwareApp(t, ctx, &SyncPolicyAutomated{})
	var secondBadRevision string
	// a second commit moves the desired revision, so the first record is cleared and the commit is deployed
	ctx.When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 4}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			secondBadRevision = app.Status.Sync.Revision
			require.NotEqual(t, paused.badRevision, secondBadRevision)
			assert.Empty(t, app.Status.RolledBackRevision, "a new desired revision clears the record")
		}).
		// rolling back again records the second revision in place of the first
		When().
		And(paused.rollbackToFirst).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionAutoSyncPausedWarning, "was rolled back")).
		And(func(app *Application) {
			assert.Equal(t, secondBadRevision, app.Status.RolledBackRevision, "only the most recent rollback is remembered")
			assert.NotEqual(t, paused.badRevision, app.Status.RolledBackRevision, "the first rollback must not still be recorded")
			assert.Empty(t, app.Status.RolledBackRevisions, "a single-source application must not use the plural field")
		}).
		// the condition names the second revision, which is what the UI renders in the sync status panel
		Expect(Condition(ApplicationConditionAutoSyncPausedWarning, secondBadRevision)).
		// a fix clears the record for good
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 5}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Empty(t, app.Status.RolledBackRevision)
			assert.Empty(t, app.Status.RolledBackRevisions)
		})
}

func TestAutoSyncRollbackAwareRecordsDeployedRevision(t *testing.T) {
	ctx := Given(t)
	var firstHistoryID int64
	var deployedRevision, newerRevision string
	ctx.Path(guestbookPath).
		When().
		SetParamInSettingConfigMap("application.rollbackAwareAutoSyncEnabled", "true").
		// automated sync is off for now, so the desired revision can move ahead of the deployed one
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{Enabled: new(false)}}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Len(t, app.Status.History, 1)
			firstHistoryID = app.Status.History[0].ID
		}).
		// a second manual sync gives the application a revision to roll back from
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 1}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Len(t, app.Status.History, 2)
			deployedRevision = app.Status.Sync.Revision
		}).
		// a third commit is never deployed, so it is only the desired revision
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 2}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			newerRevision = app.Status.Sync.Revision
			require.NotEqual(t, deployedRevision, newerRevision)
			require.Len(t, app.Status.History, 2, "the newer revision must not have been deployed")
		}).
		// the rollback records the deployed revision, not the newer desired one
		When().
		And(func() {
			_, err := fixture.RunCli("app", "rollback", ctx.AppName(), strconv.FormatInt(firstHistoryID, 10))
			require.NoError(t, err)
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		And(func(app *Application) {
			assert.Equal(t, deployedRevision, app.Status.RolledBackRevision)
			assert.NotEqual(t, newerRevision, app.Status.RolledBackRevision, "a revision that was never deployed must not be recorded")
		}).
		// re-enabling automated sync must therefore deploy the newer revision instead of pausing on it
		When().
		PatchApp(`[{"op": "replace", "path": "/spec/syncPolicy/automated/enabled", "value": true}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Equal(t, newerRevision, app.Status.Sync.Revision, "automated sync must deploy the newer revision")
			assert.Empty(t, app.Status.RolledBackRevision)
		})
}

// TestAutoSyncRollbackRejectedWhenNotRollbackAware verifies the default behaviour is unchanged: with the feature
// disabled, a rollback is rejected while automated sync is enabled.
func TestAutoSyncRollbackRejectedWhenNotRollbackAware(t *testing.T) {
	ctx := Given(t)
	ctx.Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.Len(t, app.Status.History, 1)
			_, err := fixture.RunCli("app", "rollback", ctx.AppName(), strconv.FormatInt(app.Status.History[0].ID, 10))
			require.ErrorContains(t, err, "rollback cannot be initiated when auto-sync is enabled")
		})
}
