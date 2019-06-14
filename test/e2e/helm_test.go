package e2e

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestHelmHooksAreNotCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "pre-install"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		// important check, Helm hooks should be ignored for sync status
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NotPod(func(p v1.Pod) bool {
			return p.Name == "hook"
		}))
}

func TestHelmCrdInstallIsCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "crd-install"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool {
			return p.Name == "hook"
		}))
}

func TestServiceCatalog(t *testing.T) {
	t.Skip("slow")
	Given(t).
		HelmRepoCredential("svc-cat", "https://svc-catalog-charts.storage.googleapis.com").
		Path("service-catalog").
		Timeout(900).
		When().
		Create().
		Then().
		Expect(Success("")).
		When().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy))
}

func TestSonatypeNexus(t *testing.T) {
	t.Skip("slow")
	Given(t).
		Path("sonatype-nexus").
		Timeout(900).
		When().
		Create().
		Then().
		Expect(Success("")).
		When().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy))
}
