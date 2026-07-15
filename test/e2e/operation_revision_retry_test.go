package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// triggerAppSetStyleSync mimics applicationset RollingSync: it writes an
// Operation with an empty Sync.Revision (and optional retry), instead of going
// through `argocd app sync` which always resolves a concrete SHA into
// operation.sync.revision before creating the operation.
func triggerAppSetStyleSync(t *testing.T, appName string, retryLimit int64, backoffDuration string) {
	t.Helper()
	patch := fmt.Sprintf(`{
		"operation": {
			"initiatedBy": {"username": "applicationset-controller", "automated": true},
			"info": [{"name": "Reason", "value": "ApplicationSet RollingSync triggered a sync of this Application resource"}],
			"sync": {},
			"retry": {"limit": %d, "backoff": {"duration": %q}}
		}
	}`, retryLimit, backoffDuration)
	_, err := fixture.Run("", "kubectl", "-n", fixture.TestNamespace(), "patch", "application", appName,
		"--type", "merge", "-p", patch)
	require.NoError(t, err)
}

func waitForSyncResultRevision(t *testing.T, appName, wantRevision string) *Application {
	t.Helper()
	var app *Application
	require.Eventually(t, func() bool {
		var err error
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Get(
			context.Background(), appName, metav1.GetOptions{})
		if err != nil || app.Status.OperationState == nil || app.Status.OperationState.SyncResult == nil {
			return false
		}
		return app.Status.OperationState.SyncResult.Revision == wantRevision &&
			app.Status.OperationState.Phase.Completed()
	}, 90*time.Second, time.Second)
	return app
}

// TestStaleOperationRevisionSurvivesSyncWithoutRevision validates the core
// hypothesis of https://github.com/argoproj/argo-cd/issues/26530:
//
// setOperationState uses JSON Merge Patch while SyncOperation.Revision is
// omitempty. An AppSet-style sync that does not set revision therefore leaves
// the previous operation.sync.revision on the CR, even when syncResult.revision
// is the newly resolved HEAD.
//
// This test currently expects the buggy persisted state so CI documents the
// bug. After the fix, invert the operation.sync.revision assertion to
// require it to be empty (or equal to syncResult.revision).
func TestStaleOperationRevisionSurvivesSyncWithoutRevision(t *testing.T) {
	var staleRevision string
	var headAfterGoodSync string

	ctx := Given(t)
	ctx.
		Path("hook").
		When().
		CreateApp().
		And(func() {
			sha, err := fixture.Run(fixture.TmpDir()+"/testdata.git", "git", "rev-parse", "HEAD")
			require.NoError(t, err)
			staleRevision = strings.TrimSpace(sha)
		}).
		Sync("--revision", staleRevision).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Status.OperationState)
			require.NotNil(t, app.Status.OperationState.Operation.Sync)
			require.Equal(t, staleRevision, app.Status.OperationState.Operation.Sync.Revision)
		}).
		When().
		// Create a new HEAD commit, then sync the AppSet way (empty revision).
		// Do NOT use `argocd app sync` here: the API always resolves a concrete
		// SHA into operation.sync.revision, which hides the merge-patch bug.
		PatchFile("pod.yaml", `[{"op": "add", "path": "/metadata/labels", "value": {"e2e-rev": "good"}}]`).
		And(func() {
			sha, err := fixture.Run(fixture.TmpDir()+"/testdata.git", "git", "rev-parse", "HEAD")
			require.NoError(t, err)
			headAfterGoodSync = strings.TrimSpace(sha)
			require.NotEqual(t, staleRevision, headAfterGoodSync)
			triggerAppSetStyleSync(t, ctx.AppName(), 0, "1s")
		}).
		Then().
		And(func(_ *Application) {
			app := waitForSyncResultRevision(t, ctx.AppName(), headAfterGoodSync)
			require.True(t, app.Status.OperationState.Phase.Successful())
			require.NotNil(t, app.Status.OperationState.Operation.Sync)

			assert.Equal(t, headAfterGoodSync, app.Status.OperationState.SyncResult.Revision,
				"syncResult records the commit that was actually applied")

			// Bug observation: empty revision was omitted from the merge patch.
			assert.Equal(t, staleRevision, app.Status.OperationState.Operation.Sync.Revision,
				"operation.sync.revision kept the previous --revision value after an AppSet-style sync (#26530)")
			assert.NotEqual(t, app.Status.OperationState.SyncResult.Revision, app.Status.OperationState.Operation.Sync.Revision,
				"syncResult and operation.sync.revision disagree after merge-patch omitempty")
		})
}

// TestStaleOperationRevisionUsedOnSyncRetry extends the hypothesis to the
// failure/retry path that causes old manifests to be applied in production.
//
// With the bug present, the failing sync's first attempt uses in-memory empty
// revision (HEAD with an invalid ServiceAccount), then the retry reloads the
// stale operation.sync.revision and succeeds against that old commit (which
// does not contain the invalid ServiceAccount).
//
// After the fix, this should end in OperationFailed with syncResult at HEAD
// (the invalid-manifest revision), never the earlier --revision value.
func TestStaleOperationRevisionUsedOnSyncRetry(t *testing.T) {
	var staleRevision string
	var headAfterGoodSync string
	var badHead string

	ctx := Given(t)
	ctx.
		Path("hook").
		When().
		CreateApp().
		And(func() {
			sha, err := fixture.Run(fixture.TmpDir()+"/testdata.git", "git", "rev-parse", "HEAD")
			require.NoError(t, err)
			staleRevision = strings.TrimSpace(sha)
		}).
		Sync("--revision", staleRevision).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		When().
		PatchFile("pod.yaml", `[{"op": "add", "path": "/metadata/labels", "value": {"e2e-rev": "good"}}]`).
		And(func() {
			sha, err := fixture.Run(fixture.TmpDir()+"/testdata.git", "git", "rev-parse", "HEAD")
			require.NoError(t, err)
			headAfterGoodSync = strings.TrimSpace(sha)
			triggerAppSetStyleSync(t, ctx.AppName(), 0, "1s")
		}).
		Then().
		And(func(_ *Application) {
			app := waitForSyncResultRevision(t, ctx.AppName(), headAfterGoodSync)
			require.Equal(t, staleRevision, app.Status.OperationState.Operation.Sync.Revision)
		}).
		When().
		// Invalid manifest only on HEAD: first attempt (empty revision → HEAD) fails;
		// retry that wrongly reuses staleRevision succeeds because that commit has no failing SA.
		AddFile("failure-during-sync.yaml", `apiVersion: v1
kind: ServiceAccount
metadata:
  name: failure-during-sync
  labels:
    my-label: has-inva/id-character!
`).
		And(func() {
			sha, err := fixture.Run(fixture.TmpDir()+"/testdata.git", "git", "rev-parse", "HEAD")
			require.NoError(t, err)
			badHead = strings.TrimSpace(sha)
			require.NotEqual(t, staleRevision, badHead)
			triggerAppSetStyleSync(t, ctx.AppName(), 1, "1s")
		}).
		Then().
		And(func(_ *Application) {
			// Wait until the operation finishes. With the bug, retry applies the
			// stale revision (no invalid SA) and succeeds.
			app := waitForSyncResultRevision(t, ctx.AppName(), staleRevision)
			require.True(t, app.Status.OperationState.Phase.Successful(),
				"bug #26530: retry wrongly succeeded by applying the stale revision (phase=%s message=%q syncResult=%s badHead=%s)",
				app.Status.OperationState.Phase, app.Status.OperationState.Message,
				app.Status.OperationState.SyncResult.Revision, badHead)
			assert.Equal(t, staleRevision, app.Status.OperationState.SyncResult.Revision,
				"retry applied stale operation.sync.revision from a previous sync (#26530)")
			assert.NotEqual(t, badHead, app.Status.OperationState.SyncResult.Revision,
				"syncResult must not be the broken HEAD commit")
			assert.NotEqual(t, headAfterGoodSync, app.Status.OperationState.SyncResult.Revision,
				"syncResult must not be the intermediate good HEAD either")
			assert.EqualValues(t, 1, app.Status.OperationState.RetryCount,
				"operation must have retried once after the failing HEAD attempt")
		})
}
