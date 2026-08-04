package progressivesync

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Regression tests for the progressive-sync reconcile loop.
//
// updateApplicationSetApplicationStatus must only report a spec change when the ApplicationSet
// controller would actually rewrite the Application's spec. If it reports a change the write path
// will never act on, the Application is never corrected, so the "pending change" is still there on
// the next reconcile: status flips to Waiting, then Pending, then triggers a sync, which writes the
// Application, which triggers another reconcile. Measured at ~137 reconciles/second for a single
// ApplicationSet with two Applications, with no backoff.
//
// Both cases below place the Application in a state where it is NOT (Synced and Healthy). That
// matters: a Synced+Healthy Application has its Waiting status immediately overwritten with Healthy,
// which masks the bug entirely and is why it is not universal.

// helper: a progressive-sync ApplicationSet with a single RollingSync step.
func regressionAppSet(ignore argov1alpha1.ApplicationSetIgnoreDifferences) argov1alpha1.ApplicationSet {
	return argov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "storm", Namespace: "argocd"},
		Spec: argov1alpha1.ApplicationSetSpec{
			IgnoreApplicationDifferences: ignore,
			Strategy: &argov1alpha1.ApplicationSetStrategy{
				Type: "RollingSync",
				RollingSync: &argov1alpha1.ApplicationSetRolloutStrategy{
					Steps: []argov1alpha1.ApplicationSetRolloutStep{{}},
				},
			},
		},
		Status: argov1alpha1.ApplicationSetStatus{
			ApplicationStatus: []argov1alpha1.ApplicationSetApplicationStatus{{
				Application: "storm-a",
				Status:      argov1alpha1.ProgressiveSyncHealthy,
				Message:     "Application resource has synced, updating status to Healthy",
				Step:        "1",
				// Pinned to the Application's observed revision so revisionsChanged is false and these
				// tests isolate specChanged.
				TargetRevisions: []string{"abc123"},
			}},
		},
	}
}

// helper: an Application that is deliberately not Synced, so a Waiting status is not overwritten.
func regressionApp(rev string, automatedEnabled *bool) argov1alpha1.Application {
	app := argov1alpha1.Application{
		TypeMeta:   metav1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "Application"},
		ObjectMeta: metav1.ObjectMeta{Name: "storm-a", Namespace: "argocd"},
		Spec: argov1alpha1.ApplicationSpec{
			Project: "default",
			Source: &argov1alpha1.ApplicationSource{
				RepoURL:        "git://example/repo",
				Path:           "leaves/x",
				TargetRevision: rev,
			},
			Destination: argov1alpha1.ApplicationDestination{
				Server: "https://kubernetes.default.svc", Namespace: "storm-a",
			},
			SyncPolicy: &argov1alpha1.SyncPolicy{
				Automated:   &argov1alpha1.SyncPolicyAutomated{Prune: new(true), SelfHeal: new(true), Enabled: automatedEnabled},
				SyncOptions: []string{"CreateNamespace=true"},
			},
		},
		Status: argov1alpha1.ApplicationStatus{
			Sync:   argov1alpha1.SyncStatus{Status: argov1alpha1.SyncStatusCodeUnknown, Revision: "abc123"},
			Health: argov1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
		},
	}
	return app
}

// regressionDeps is a no-op Dependencies: these tests assert on the statuses returned by
// UpdateApplicationSetApplicationStatus, not on their persistence.
type regressionDeps struct{}

func (regressionDeps) SetAppSetApplicationStatus(_ context.Context, _ *log.Entry, _ *argov1alpha1.ApplicationSet, _ []argov1alpha1.ApplicationSetApplicationStatus) error {
	return nil
}

func (regressionDeps) SetApplicationSetStatusCondition(_ context.Context, _ *argov1alpha1.ApplicationSet, _ []argov1alpha1.ApplicationSetCondition, _ bool) error {
	return nil
}

func regressionManager(t *testing.T, appSet *argov1alpha1.ApplicationSet) *Manager {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, argov1alpha1.AddToScheme(scheme))

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(appSet).WithStatusSubresource(appSet).Build()
	return NewManager(c, regressionDeps{})
}

// Defect 1: the status path must honour ignoreApplicationDifferences, exactly as
// createOrUpdateInCluster does when deciding whether to Patch. Here the only difference between live
// and desired is a field the ApplicationSet explicitly ignores, so the write path will never correct
// it — reporting a spec change is therefore a permanent, self-sustaining loop.
//
// Without the fix this reports "Application has pending changes (spec differs)".
func TestSpecChangedHonoursIgnoreApplicationDifferences(t *testing.T) {
	t.Parallel()

	appSet := regressionAppSet(argov1alpha1.ApplicationSetIgnoreDifferences{
		{JSONPointers: []string{"/spec/source/targetRevision"}},
	})
	live := regressionApp("refs/heads/does-not-exist", new(false))
	desired := regressionApp("HEAD", new(false))

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{live}, []argov1alpha1.Application{desired},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.NotEqual(t, argov1alpha1.ProgressiveSyncWaiting, statuses[0].Status,
		"the only difference is an ignored field, which the write path will never correct, so "+
			"reporting a pending spec change loops forever: %s", statuses[0].Message)
	assert.NotContains(t, statuses[0].Message, "spec differs")
}

// Defect 2: applyRolloutStrategy sets Automated.Enabled=false on progressive-sync-managed
// Applications, because the ApplicationSet controller drives those syncs itself. That mutation lands
// on the copy written to the cluster, not on the freshly generated Application compared here — so the
// live object permanently carries automated.enabled=false while the generated one leaves it unset.
//
// This needs no ignore rules and no unusual configuration: it applies to every ApplicationSet whose
// template sets syncPolicy.automated, which is the common case.
//
// Without the fix this reports "Application has pending changes (spec differs)".
func TestSpecChangedIgnoresControllerManagedAutomatedEnabled(t *testing.T) {
	t.Parallel()

	appSet := regressionAppSet(nil)
	live := regressionApp("HEAD", new(false)) // written by the controller with Enabled=false
	desired := regressionApp("HEAD", nil)     // freshly generated: Enabled unset

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{live}, []argov1alpha1.Application{desired},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.NotEqual(t, argov1alpha1.ProgressiveSyncWaiting, statuses[0].Status,
		"automated.enabled is set by the controller itself on the written copy only; treating it as a "+
			"user-visible spec change loops forever for any ApplicationSet using syncPolicy.automated: %s",
		statuses[0].Message)
	assert.NotContains(t, statuses[0].Message, "spec differs")
}

// An invalid ignore rule must not manufacture a spec change.
//
// Note where such a rule actually fails. BuildIgnoreDiffConfig only validates structural invariants,
// so an unparseable JQ expression passes it; the failure surfaces later, from applyIgnoreDifferences
// inside SpecsEquivalent. The write path turns that into a reconcile error. The status path cannot
// usefully do the same for a single Application, so it must at least not report a change it cannot
// substantiate -- reporting one would trigger a sync the write path is failing to perform, which is
// the loop this file exists to prevent.
func TestInvalidIgnoreRuleDoesNotReportSpecChange(t *testing.T) {
	t.Parallel()

	appSet := regressionAppSet(argov1alpha1.ApplicationSetIgnoreDifferences{
		// Unparseable JQ: accepted by BuildIgnoreDiffConfig, rejected by gojq during normalization.
		{JQPathExpressions: []string{"this is not( valid jq"}},
	})
	live := regressionApp("refs/heads/does-not-exist", new(false))
	desired := regressionApp("HEAD", nil)

	m := regressionManager(t, &appSet)
	statuses, err := m.UpdateApplicationSetApplicationStatus(t.Context(), log.NewEntry(log.New()),
		&appSet, []argov1alpha1.Application{live}, []argov1alpha1.Application{desired},
		map[string]int{"storm-a": 0})
	require.NoError(t, err)
	require.Len(t, statuses, 1)

	assert.NotContains(t, statuses[0].Message, "spec differs",
		"with an unusable ignore rule the comparison cannot be trusted, so the status must not claim "+
			"a spec change; the write path is erroring, so a change reported here can never be acted on")
}
