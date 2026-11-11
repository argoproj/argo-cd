package progressivesync

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Unit tests for the drift-detection helpers that gate RollingSync drift re-queue.
// They cover every branch of driftResyncSettledOp / isDriftResyncCandidate /
// isApplicationDrifted directly, including the rate-limiting backoff that keeps a
// difference a sync cannot resolve (for example a prune-disabled orphan) from being
// re-queued on every reconcile.
func TestIsApplicationDrifted(t *testing.T) {
	t.Parallel()

	// opState builds an OperationState with the given phase whose sync finished
	// finishedAgo before now (negative durations are in the future).
	opState := func(phase common.OperationPhase, finishedAgo time.Duration) *argov1alpha1.OperationState {
		return &argov1alpha1.OperationState{
			Phase:      phase,
			StartedAt:  metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
			FinishedAt: &metav1.Time{Time: time.Now().Add(-finishedAgo)},
		}
	}

	app := func(sync argov1alpha1.SyncStatusCode, op *argov1alpha1.OperationState, reconciledAgo *time.Duration, pendingOp bool) argov1alpha1.Application {
		a := argov1alpha1.Application{
			Status: argov1alpha1.ApplicationStatus{
				Health:         argov1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
				OperationState: op,
				Sync:           argov1alpha1.SyncStatus{Status: sync},
			},
		}
		if reconciledAgo != nil {
			a.Status.ReconciledAt = &metav1.Time{Time: time.Now().Add(-*reconciledAgo)}
		}
		if pendingOp {
			a.Operation = &argov1alpha1.Operation{}
		}
		return a
	}

	dur := func(d time.Duration) *time.Duration { return &d }

	cases := []struct {
		name          string
		app           argov1alpha1.Application
		wantSettledOp bool // driftResyncSettledOp returns non-nil
		wantCandidate bool // isDriftResyncCandidate
		wantDrifted   bool // isApplicationDrifted (candidate + backoff elapsed)
	}{
		{
			name: "synced application is not drift",
			app:  app(argov1alpha1.SyncStatusCodeSynced, opState(common.OperationSucceeded, time.Minute), dur(0), false),
		},
		{
			name: "a sync already requested is left alone",
			app:  app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationSucceeded, time.Minute), dur(0), true),
		},
		{
			name: "no recorded operation is not treated as settled drift",
			app:  app(argov1alpha1.SyncStatusCodeOutOfSync, nil, dur(0), false),
		},
		{
			name: "in-flight sync is not drift",
			app: func() argov1alpha1.Application {
				a := app(argov1alpha1.SyncStatusCodeOutOfSync, nil, dur(0), false)
				a.Status.OperationState = &argov1alpha1.OperationState{Phase: common.OperationRunning, StartedAt: metav1.Time{Time: time.Now()}}
				return a
			}(),
		},
		{
			name: "failed sync is left to the Application retry policy",
			app:  app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationFailed, time.Minute), dur(0), false),
		},
		{
			name: "successful sync without a finish time is not settled",
			app: func() argov1alpha1.Application {
				a := app(argov1alpha1.SyncStatusCodeOutOfSync, nil, dur(0), false)
				a.Status.OperationState = &argov1alpha1.OperationState{Phase: common.OperationSucceeded, StartedAt: metav1.Time{Time: time.Now().Add(-time.Minute)}}
				return a
			}(),
		},
		{
			name: "never reconciled is not a candidate",
			app:  app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationSucceeded, time.Minute), nil, false),
		},
		{
			name:          "drift observed before the sync finished is not a candidate",
			app:           app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationSucceeded, time.Minute), dur(2*time.Minute), false),
			wantSettledOp: true,
		},
		{
			name:          "drift within the backoff window is a candidate but not yet re-queued",
			app:           app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationSucceeded, time.Second), dur(0), false),
			wantSettledOp: true,
			wantCandidate: true,
		},
		{
			name:          "drift observed after the sync and past the backoff is re-queued",
			app:           app(argov1alpha1.SyncStatusCodeOutOfSync, opState(common.OperationSucceeded, 2*driftResyncBackoff), dur(0), false),
			wantSettledOp: true,
			wantCandidate: true,
			wantDrifted:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.wantSettledOp, driftResyncSettledOp(tc.app) != nil, "driftResyncSettledOp")
			assert.Equal(t, tc.wantCandidate, isDriftResyncCandidate(tc.app), "isDriftResyncCandidate")
			assert.Equal(t, tc.wantDrifted, isApplicationDrifted(tc.app), "isApplicationDrifted")
		})
	}
}
