package progressivesync

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func rollingSyncStatus(status argov1alpha1.ProgressiveSyncStatusCode, message, revision string, transition time.Time) argov1alpha1.ApplicationSetApplicationStatus {
	return argov1alpha1.ApplicationSetApplicationStatus{
		Application:        "storm-a",
		Message:            message,
		Status:             status,
		Step:               "1",
		TargetRevisions:    []string{revision},
		LastTransitionTime: &metav1.Time{Time: transition},
	}
}

func rollingAppSet(status argov1alpha1.ApplicationSetApplicationStatus) argov1alpha1.ApplicationSet {
	return argov1alpha1.ApplicationSet{
		Name: "storm", Namespace: "argocd",
		Spec: argov1alpha1.ApplicationSetSpec{
			Strategy: &argov1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
					Steps: []argov1alpha1.ApplicationSetRolloutStep{{}},
				},
			},
		},
		Status: argov1alpha1.ApplicationSetStatus{
			ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{status},
		},
	}
}

func rollingApp(syncStatus argov1alpha1.SyncStatusCode, healthStatus health.HealthStatusCode, revision string, op *argov1alpha1.OperationState, reconciledAt *metav1.Time) argov1alpha1.Application {
	return argov1alpha1.Application{
		APIVersion: "argoproj.io/v1alpha1", Kind: "Application",
		Name: "storm-a", Namespace: "argocd",
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "git://example/repo",
				Path:           "leaves/x",
				TargetRevision: revision,
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc", Namespace: "storm-a",
			},
		},
		Status: argov1alpha1.ApplicationStatus{
			Sync:           argov1alpha1.SyncStatus{Status: syncStatus, Revision: revision},
			Health:         argov1alpha1.AppHealthStatus{Status: healthStatus},
			OperationState: op,
			ReconciledAt:   reconciledAt,
		},
	}
}

func opSyncedTo(revision string, startedAt time.Time) *argov1alpha1.OperationState {
	finishedAt := startedAt.Add(time.Minute)
	return &argov1alpha1.OperationState{
		Phase:      common.OperationSucceeded,
		SyncResult: &argov1alpha1.SyncOperationResult{Revision: revision},
		StartedAt:  metav1.Time{Time: startedAt},
		FinishedAt: &metav1.Time{Time: finishedAt},
	}
}

func TestRevisionChangeWithNoSyncEvidenceStaysWaiting(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)
	appSet := rollingAppSet(rollingSyncStatus(
		argov1alpha1.ProgressiveSyncHealthy, "Application resource has synced, updating status to Healthy", "a", transition))
	app := rollingApp(argov1alpha1.SyncStatusCodeSynced, health.HealthStatusHealthy, "b",
		opSyncedTo("a", transition.Add(-time.Minute)), nil)

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.Equal(t, argov1alpha1.ProgressiveSyncWaiting, statuses[0].Status,
		"Synced/Healthy at the new revision is stale evidence from the previous sync and must not promote")
	assert.Equal(t, revisionChangedMsg, statuses[0].Message)
	assert.Equal(t, []string{"b"}, statuses[0].TargetRevisions)
}

func TestHealthyOutOfSyncAtRecordedRevisionRecoversToWaiting(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)
	appSet := rollingAppSet(rollingSyncStatus(
		argov1alpha1.ProgressiveSyncHealthy, "Application resource has synced, updating status to Healthy", "b", transition))
	app := rollingApp(argov1alpha1.SyncStatusCodeOutOfSync, health.HealthStatusHealthy, "b",
		opSyncedTo("a", transition.Add(time.Minute)), &metav1.Time{Time: time.Now().Add(-time.Minute)})

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.Equal(t, argov1alpha1.ProgressiveSyncWaiting, statuses[0].Status,
		"OutOfSync at the recorded target with no sync evidence must not be reported Healthy")
	assert.Equal(t, applicationOutOfSyncMsg, statuses[0].Message)
	assert.Equal(t, []string{"b"}, statuses[0].TargetRevisions)

	appSet.Status.ApplicationStatus = statuses
	progress, err := m.UpdateApplicationSetApplicationStatusProgress(t.Context(), log.NewEntry(log.New()),
		&appSet, map[string]bool{"storm-a": true}, map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, progress, 1)

	assert.Equal(t, argov1alpha1.ProgressiveSyncPending, progress[0].Status,
		"after rolling back to Waiting the Application must be sync-eligible again")
}

func TestWaitingPromotesToHealthyOnlyAfterSyncToTargetAfterTransition(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)

	for _, tc := range []struct {
		name          string
		startedAt     time.Time
		expectedState argov1alpha1.ProgressiveSyncStatusCode
	}{
		{"sync started before the transition must not promote", transition.Add(-time.Minute), argov1alpha1.ProgressiveSyncWaiting},
		{"sync started after the transition promotes", transition.Add(time.Minute), argov1alpha1.ProgressiveSyncHealthy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appSet := rollingAppSet(rollingSyncStatus(
				argov1alpha1.ProgressiveSyncWaiting, applicationOutOfSyncMsg, "b", transition))
			app := rollingApp(argov1alpha1.SyncStatusCodeSynced, health.HealthStatusHealthy, "b",
				opSyncedTo("b", tc.startedAt), &metav1.Time{Time: time.Now().Add(-time.Minute)})

			m := regressionManager(t, &appSet)
			statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
				&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
				map[string]int{"storm-a": 0})
			require.NoError(t, err)
			require.Len(t, statuses, 1)

			assert.Equal(t, tc.expectedState, statuses[0].Status,
				"SyncResult matching the target is only valid evidence if it started after the transition to Waiting")
		})
	}
}

func TestHealthyOutOfSyncStaysHealthyWhenLastSyncReachedTarget(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)
	appSet := rollingAppSet(rollingSyncStatus(
		argov1alpha1.ProgressiveSyncHealthy, "Application resource has synced, updating status to Healthy", "b", transition))
	app := rollingApp(argov1alpha1.SyncStatusCodeOutOfSync, health.HealthStatusHealthy, "b",
		opSyncedTo("b", transition.Add(time.Minute)), &metav1.Time{Time: time.Now().Add(-time.Minute)})

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.Equal(t, argov1alpha1.ProgressiveSyncHealthy, statuses[0].Status,
		"the last successful operation reached the recorded target, so the OutOfSync view is transient")
	assert.NotEqual(t, applicationOutOfSyncMsg, statuses[0].Message)
}
