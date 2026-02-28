package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestOCIImage(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushImageToOCIRegistry("testdata/guestbook", "1.0.0").
		OCIRepoAdded("guestbook", "guestbook").
		Revision("1.0.0").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("guestbook").
		Path(".").
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Success("")).
		When().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestOCIWithOCIHelmRegistryDependencies(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushChartToOCIRegistry("testdata/helm-values", "helm-values", "1.0.0").
		PushImageToOCIRegistry("testdata/helm-oci-with-dependencies", "1.0.0").
		OCIRegistry(fixture.OCIHostURL).
		OCIRepoAdded("helm-oci-with-dependencies", "helm-oci-with-dependencies").
		OCIRegistryPath("helm-oci-with-dependencies").
		Revision("1.0.0").
		Path(".").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestOCIWithAuthedOCIHelmRegistryDeps(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushChartToAuthenticatedOCIRegistry("testdata/helm-values", "helm-values", "1.0.0").
		PushImageToOCIRegistry("testdata/helm-oci-authed-with-dependencies", "1.0.0").
		OCIRepoAdded("helm-oci-authed-with-dependencies", "helm-oci-authed-with-dependencies").
		AuthenticatedOCIRepoAdded("helm-values", "myrepo/helm-values").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("helm-oci-authed-with-dependencies").
		Revision("1.0.0").
		Path(".").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestOCIImageWithOutOfBoundsSymlink(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushImageToOCIRegistry("testdata3/symlink-out-of-bounds", "1.0.0").
		OCIRepoAdded("symlink-out-of-bounds", "symlink-out-of-bounds").
		Revision("1.0.0").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("symlink-out-of-bounds").
		Path(".").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "could not decompress layer: illegal filepath in symlink"))
}

// TestOCIRevisionResolution verifies that when a semver constraint is used as the
// revision for an OCI image source, the sync result captures the intermediate resolution
// — which concrete tag the constraint resolved to.
func TestOCIRevisionResolution(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushImageToOCIRegistry("testdata/guestbook", "1.0.0").
		OCIRepoAdded("guestbook", "guestbook").
		Revision("^1.0.0").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("guestbook").
		Path(".").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Status.OperationState, "OperationState should be set after sync")
			require.NotNil(t, app.Status.OperationState.SyncResult, "SyncResult should be set after sync")
			require.NotNil(t, app.Status.OperationState.SyncResult.Resolution,
				"Resolution should be populated when a semver constraint was resolved")
			assert.Equal(t, "1.0.0", app.Status.OperationState.SyncResult.Resolution.ResolvedSymbol,
				"ResolvedSymbol should be the concrete tag selected by the constraint")
			assert.Equal(t, "^1.0.0", app.Status.OperationState.SyncResult.Resolution.Constraint,
				"Constraint should be the original revision expression")
		})
}

// TestOCIPinnedTagNoRevisionResolution verifies that a pinned OCI tag (not a constraint)
// produces no RevisionResolution.
func TestOCIPinnedTagNoRevisionResolution(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushImageToOCIRegistry("testdata/guestbook", "1.0.0").
		OCIRepoAdded("guestbook", "guestbook").
		Revision("1.0.0").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("guestbook").
		Path(".").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			require.NotNil(t, app.Status.OperationState, "OperationState should be set after sync")
			require.NotNil(t, app.Status.OperationState.SyncResult, "SyncResult should be set after sync")
			assert.Nil(t, app.Status.OperationState.SyncResult.Resolution,
				"Resolution should be nil when a pinned tag (not a constraint) was specified")
		})
}
