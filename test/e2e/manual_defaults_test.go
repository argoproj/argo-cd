package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestManualDefaultsPruneOnSync verifies that syncPolicy.manualDefaults.prune is applied
// when the CLI omits --prune (request Prune stays nil).
func TestManualDefaultsPruneOnSync(t *testing.T) {
	Given(t).
		// Fixture defaults Prune(true); disable so Sync() omits --prune and
		// manualDefaults.prune is what enables pruning.
		Prune(false).
		Path("two-nice-pods").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				ManualDefaults: &SyncPolicyManualDefaults{Prune: new(true)},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Status.OperationState)
			require.NotNil(t, app.Status.OperationState.Operation.Sync)
			assert.True(t, app.Status.OperationState.Operation.Sync.Prune)
		}).
		When().
		DeleteFile("pod-2.yaml").
		// Do not pass --prune; manualDefaults.prune must prune pod-2.
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-2", health.HealthStatusMissing))
}

// TestManualDefaultsDryRunAllowsLocalSyncWithAutosync verifies CLI/server effective dry-run:
// when --dry-run is omitted, manualDefaults.dryRun still allows local sync with autosync.
func TestManualDefaultsDryRunAllowsLocalSyncWithAutosync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{},
				ManualDefaults: &SyncPolicyManualDefaults{
					Prune:  new(false),
					DryRun: new(true),
				},
			}
		}).
		Then().
		// Wait for the controller's auto-sync to finish before starting a manual local sync.
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_, err := fixture.RunCli("app", "sync", app.Name, "--local-repo-root", ".", "--local", guestbookPathLocal)
			require.NoError(t, err)

			updated, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.NotNil(t, updated.Status.OperationState)
			require.NotNil(t, updated.Status.OperationState.Operation.Sync)
			assert.True(t, updated.Status.OperationState.Operation.Sync.DryRun)
		})
}
