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

func rollingSyncStatusRevisions(status argov1alpha1.ProgressiveSyncStatusCode, message string, revisions []string, transition time.Time) argov1alpha1.ApplicationSetApplicationStatus {
	return argov1alpha1.ApplicationSetApplicationStatus{
		Application:        "storm-a",
		Message:            message,
		Status:             status,
		Step:               "1",
		TargetRevisions:    revisions,
		LastTransitionTime: &metav1.Time{Time: transition},
	}
}

func rollingMultiSourceApp(syncStatus argov1alpha1.SyncStatusCode, healthStatus health.HealthStatusCode, revisions []string, op *argov1alpha1.OperationState, reconciledAt *metav1.Time) argov1alpha1.Application {
	return argov1alpha1.Application{
		APIVersion: "argoproj.io/v1alpha1", Kind: "Application",
		Name: "storm-a", Namespace: "argocd",
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Sources: argov1alpha1.ApplicationSources{
				{
					RepoURL:        "git://example/repo-one",
					Path:           "leaves/x",
					TargetRevision: "HEAD",
				},
				{
					RepoURL:        "git://example/repo-two",
					Path:           "leaves/y",
					TargetRevision: "HEAD",
				},
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc", Namespace: "storm-a",
			},
		},
		Status: argov1alpha1.ApplicationStatus{
			Sync:           argov1alpha1.SyncStatus{Status: syncStatus, Revisions: revisions},
			Health:         argov1alpha1.AppHealthStatus{Status: healthStatus},
			OperationState: op,
			ReconciledAt:   reconciledAt,
		},
	}
}

func opSyncedToRevisions(revisions []string, startedAt time.Time) *argov1alpha1.OperationState {
	finishedAt := startedAt.Add(time.Minute)
	return &argov1alpha1.OperationState{
		Phase:      common.OperationSucceeded,
		SyncResult: &argov1alpha1.SyncOperationResult{Revisions: revisions},
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

func TestApplicationSyncedToTargetMultiSource(t *testing.T) {
	t.Parallel()

	started := time.Now().Add(-time.Minute)
	target := []string{"a", "b"}

	opWith := func(revisions []string) *argov1alpha1.OperationState {
		return opSyncedToRevisions(revisions, started)
	}
	opWithoutResult := func() *argov1alpha1.OperationState {
		opState := opSyncedToRevisions(nil, started)
		opState.SyncResult = nil
		return opState
	}
	opRunning := func() *argov1alpha1.OperationState {
		return &argov1alpha1.OperationState{
			Phase:      common.OperationRunning,
			SyncResult: &argov1alpha1.SyncOperationResult{Revisions: []string{"a", "b"}},
			StartedAt:  metav1.Time{Time: started},
		}
	}

	for _, tc := range []struct {
		name     string
		op       *argov1alpha1.OperationState
		expected bool
	}{
		{"revisions match target in order", opWith([]string{"a", "b"}), true},
		{"revisions reordered relative to target", opWith([]string{"b", "a"}), false},
		{"revisions missing", opWith(nil), false},
		{"revisions partial", opWith([]string{"a"}), false},
		{"revisions extra", opWith([]string{"a", "b", "c"}), false},
		{"sync result missing", opWithoutResult(), false},
		{"operation not successful", opRunning(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			app := rollingMultiSourceApp(argov1alpha1.SyncStatusCodeSynced, health.HealthStatusHealthy, target, tc.op, nil)
			assert.Equal(t, tc.expected, applicationSyncedToTarget(&app, target))
		})
	}
}

func TestMultiSourceWaitingPromotionRequiresExactSyncEvidence(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)
	target := []string{"a", "b"}

	for _, tc := range []struct {
		name          string
		syncResult    []string
		expectedState argov1alpha1.ProgressiveSyncStatusCode
	}{
		{"exact match promotes to Healthy", []string{"a", "b"}, argov1alpha1.ProgressiveSyncHealthy},
		{"reordered revisions stay Waiting", []string{"b", "a"}, argov1alpha1.ProgressiveSyncWaiting},
		{"missing revisions stay Waiting", nil, argov1alpha1.ProgressiveSyncWaiting},
		{"mismatched revisions stay Waiting", []string{"a", "c"}, argov1alpha1.ProgressiveSyncWaiting},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appSet := rollingAppSet(rollingSyncStatusRevisions(
				argov1alpha1.ProgressiveSyncWaiting, applicationOutOfSyncMsg, target, transition))
			app := rollingMultiSourceApp(argov1alpha1.SyncStatusCodeSynced, health.HealthStatusHealthy, target,
				opSyncedToRevisions(tc.syncResult, transition.Add(time.Minute)), &metav1.Time{Time: time.Now().Add(-time.Minute)})

			m := regressionManager(t, &appSet)
			statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
				&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
				map[string]int{"storm-a": 0})
			require.NoError(t, err)
			require.Len(t, statuses, 1)

			assert.Equal(t, tc.expectedState, statuses[0].Status,
				"multi-source SyncResult.Revisions must exactly match the recorded target revisions to promote")
		})
	}
}

func TestMultiSourceHealthyRollsBackWhenSyncEvidenceDoesNotReachTarget(t *testing.T) {
	t.Parallel()

	transition := time.Now().Add(-5 * time.Minute)
	target := []string{"a", "b"}

	for _, tc := range []struct {
		name          string
		syncResult    []string
		expectedState argov1alpha1.ProgressiveSyncStatusCode
	}{
		{"exact sync evidence keeps Healthy", []string{"a", "b"}, argov1alpha1.ProgressiveSyncHealthy},
		{"partial sync evidence rolls back to Waiting", []string{"a"}, argov1alpha1.ProgressiveSyncWaiting},
		{"reordered sync evidence rolls back to Waiting", []string{"b", "a"}, argov1alpha1.ProgressiveSyncWaiting},
		{"missing sync evidence rolls back to Waiting", nil, argov1alpha1.ProgressiveSyncWaiting},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			appSet := rollingAppSet(rollingSyncStatusRevisions(
				argov1alpha1.ProgressiveSyncHealthy, "Application resource has synced, updating status to Healthy", target, transition))
			app := rollingMultiSourceApp(argov1alpha1.SyncStatusCodeOutOfSync, health.HealthStatusHealthy, target,
				opSyncedToRevisions(tc.syncResult, transition.Add(time.Minute)), &metav1.Time{Time: time.Now().Add(-time.Minute)})

			m := regressionManager(t, &appSet)
			statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
				&appSet, []argov1alpha1.Application{app}, []argov1alpha1.Application{app},
				map[string]int{"storm-a": 0})
			require.NoError(t, err)
			require.Len(t, statuses, 1)

			assert.Equal(t, tc.expectedState, statuses[0].Status,
				"multi-source sync evidence must reach every recorded target revision to avoid rollback")
		})
	}
}
