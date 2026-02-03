package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
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

func TestMultiSourceAppWithOCIRefValues(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL:        fixture.RepoURL(fixture.RepoURLTypeFile),
		TargetRevision: "HEAD",
		Path:           "helm-guestbook",
		Helm: &ApplicationSourceHelm{
			ReleaseName: "helm-guestbook",
			ValueFiles: []string{
				"$values/values.yaml",
			},
		},
	}, {
		RepoURL:        "oci://localhost:5000/oci-ref-values",
		TargetRevision: "1.0.0",
		Ref:            "values",
	}}

	ctx := Given(t)
	ctx.
		PushImageToOCIRegistry("testdata/oci-ref-values", "1.0.0").
		OCIRepoAdded("oci-ref-values", "oci-ref-values").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("oci-ref-values").
		Sources(sources).
		When().
		CreateMultiSourceAppFromFile().
		Then().
		And(func(app *Application) {
			assert.Equal(t, fixture.Name(), app.Name)
			assert.Len(t, app.Spec.GetSources(), 2)

			// Verify first source (Helm chart)
			helmSource := app.Spec.GetSources()[0]
			assert.Equal(t, fixture.RepoURL(fixture.RepoURLTypeFile), helmSource.RepoURL)
			assert.Equal(t, "helm-guestbook", helmSource.Path)
			assert.NotNil(t, helmSource.Helm)
			assert.Contains(t, helmSource.Helm.ValueFiles, "$values/values.yaml")

			// Verify second source (OCI ref values)
			ociSource := app.Spec.GetSources()[1]
			assert.Equal(t, "oci://localhost:5000/oci-ref-values", ociSource.RepoURL)
			assert.Equal(t, "1.0.0", ociSource.TargetRevision)
			assert.Equal(t, "values", ociSource.Ref)
		}).
		Expect(Event("ResourceCreated", "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := fixture.RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, fixture.Name())
		}).
		Expect(Success("")).
		Given().Timeout(60).
		When().Wait().Then().
		Expect(Success("")).
		And(func(app *Application) {
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			assert.Len(t, statusByName, 1)
			assert.Equal(t, SyncStatusCodeSynced, statusByName["guestbook-ui"])

			// Confirm that the deployment has 3 replicas (from OCI ref values)
			output, err := fixture.Run("", "kubectl", "get", "deployment", "guestbook-ui", "-n", fixture.DeploymentNamespace(), "-o", "jsonpath={.spec.replicas}")
			require.NoError(t, err)
			assert.Equal(t, "3", output, "Expected 3 replicas for the helm-guestbook deployment from OCI ref values")
		})
}
