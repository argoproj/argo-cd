package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
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

func TestOCIBuildEnvironment(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeOCI).
		PushImageToOCIRegistry("testdata/helm-values", "1.0.0").
		OCIRepoAdded("helm-values", "helm-values").
		Revision("1.0.*").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("helm-values").
		Path(".").
		When().
		CreateApp().
		AppSet("--helm-set", "foo=$ARGOCD_RESOLVED_TAG").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Success("")).
		When().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			assert.Equal(t, "1.0.0", errors.NewHandler(t).FailOnErr(fixture.Run(".", "kubectl", "-n", fixture.DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
		})
}
