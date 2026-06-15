package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

const wardleAPIServiceName = "v1alpha1.wardle.example.com"

// TestAPIServiceLateRegistrationIsDiscovered verifies that when an aggregated
// (extension) API server is registered AFTER Argo CD is already running, the
// destination cluster cache picks up the newly served group/kind WITHOUT a manual
// cache invalidation or hard refresh.
//
// The scenario reproduces the original bug: starting Argo CD without the
// aggregated apiserver running means its group/kind is absent from discovery, so
// the cluster cache never watches it. Argo CD watches APIService objects and, on
// the fix, reacts to an APIService becoming Available by re-running discovery and
// starting the missing watches - mirroring the existing CRD handling.
//
// The assertion deliberately checks the application RESOURCE TREE (which is built
// purely from resources the cluster cache is watching) rather than the resource
// sync status. The sync status is not a reliable signal here because
// GetManagedLiveObjs falls back to a direct live API GET for kinds that are not
// watched, so a resource can show as Synced even though the cache never observed
// it - which is exactly why it would be missing from the UI/tree.
func TestAPIServiceLateRegistrationIsDiscovered(t *testing.T) {
	const serverManifests = "testdata/aggregated-apiserver-server/manifests.yaml"

	// The aggregated apiserver infrastructure is cluster-scoped (Namespace,
	// APIService, ClusterRole(Binding)s) and is not managed by an Argo CD app, so
	// it must be cleaned up explicitly. A dangling APIService backed by a deleted
	// service would otherwise degrade discovery for subsequent tests.
	t.Cleanup(func() {
		_, _ = Run("", "kubectl", "delete", "-f", serverManifests, "--ignore-not-found=true", "--wait=false")
	})

	flunderManifest := `apiVersion: wardle.example.com/v1alpha1
kind: Flunder
metadata:
  name: e2e-flunder
`

	ctx := Given(t)
	ctx.
		Path("aggregated-apiserver").
		When().
		CreateApp().
		Sync().
		Then().
		// Baseline: the app - and therefore the destination cluster cache - is
		// synced and running before the aggregated API exists.
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("ConfigMap", "aggregated-apiserver-canary", SyncStatusCodeSynced)).
		When().
		And(func() {
			// Register the extension apiserver AFTER Argo CD is up and wait until it
			// is actually serving its API group.
			_, err := Run("", "kubectl", "apply", "-f", serverManifests)
			require.NoError(t, err)
			_, err = Run("", "kubectl", "-n", "wardle", "rollout", "status", "deployment/wardle-server", "--timeout=180s")
			require.NoError(t, err)
			_, err = Run("", "kubectl", "wait", "--for=condition=Available", "apiservice/"+wardleAPIServiceName, "--timeout=180s")
			require.NoError(t, err)
		}).
		// Deliberately NO InvalidateCache and NO RefreshTypeHard here - the cache must
		// discover the new kind on its own in response to the APIService event.
		AddFile("flunder.yaml", flunderManifest).
		Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// The Flunder appears in the resource tree only if the cluster cache
			// discovered the aggregated kind and is watching it. This is the behavior
			// under test: without reacting to the APIService event, the cache never
			// watches wardle.example.com and the Flunder is absent from the tree (and
			// the UI) until a manual invalidation.
			closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
			defer utilio.Close(closer)

			require.Eventually(t, func() bool {
				// Trigger a normal (soft) refresh so the controller recomputes the
				// resource tree from the live cluster cache. This does NOT invalidate
				// the cache or re-run discovery: without the APIService fix the kind
				// remains unwatched, so the Flunder never enters the tree regardless of
				// how many times we refresh.
				if _, err := RunCli("app", "get", ctx.AppQualifiedName(), "--refresh"); err != nil {
					t.Logf("app refresh failed: %v", err)
				}
				tree, err := cdClient.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{
					ApplicationName: &app.Name,
					AppNamespace:    &app.Namespace,
					Project:         &app.Spec.Project,
				})
				if err != nil {
					t.Logf("failed to get resource tree: %v", err)
					return false
				}
				for _, node := range tree.Nodes {
					if node.Group == "wardle.example.com" && node.Kind == "Flunder" && node.Name == "e2e-flunder" {
						return true
					}
				}
				return false
			}, 90*time.Second, 3*time.Second, "Flunder should appear in the application resource tree without a cache invalidation")
		})
}
