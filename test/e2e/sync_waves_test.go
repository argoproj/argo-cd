package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
)

func TestFixingDegradedApp(t *testing.T) {
	Given(t).
		Path("sync-waves").
		When().
		IgnoreErrors().
		Create().
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"ConfigMap": {
					HealthLua: `return { status = obj.metadata.annotations and obj.metadata.annotations['health'] or 'Degraded' }`,
				},
			})
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceSyncStatusIs("ConfigMap", "cm-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("ConfigMap", "cm-1", health.HealthStatusDegraded)).
		Expect(ResourceSyncStatusIs("ConfigMap", "cm-2", SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("ConfigMap", "cm-2", health.HealthStatusMissing)).
		When().
		PatchFile("cm-1.yaml", `[{"op": "replace", "path": "/metadata/annotations/health", "value": "Healthy"}]`).
		PatchFile("cm-2.yaml", `[{"op": "replace", "path": "/metadata/annotations/health", "value": "Healthy"}]`).
		// need to force a refresh here
		Refresh(RefreshTypeNormal).
		Then().
		Expect(ResourceSyncStatusIs("ConfigMap", "cm-1", SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceSyncStatusIs("ConfigMap", "cm-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("ConfigMap", "cm-1", health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("ConfigMap", "cm-2", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("ConfigMap", "cm-2", health.HealthStatusHealthy))
}

func TestOneProgressingDeploymentIsSucceededAndSynced(t *testing.T) {
	Given(t).
		Path("one-deployment").
		When().
		// make this deployment get stuck in progressing due to "invalidimagename"
		PatchFile("deployment.yaml", `[
    {
        "op": "replace",
        "path": "/spec/template/spec/containers/0/image",
        "value": "alpine:ops!"
    }
]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusProgressing)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}

func TestDegradedDeploymentIsSucceededAndSynced(t *testing.T) {
	Given(t).
		Path("one-deployment").
		When().
		// make this deployment get stuck in progressing due to "invalidimagename"
		PatchFile("deployment.yaml", `[
    {
        "op": "replace",
        "path": "/spec/progressDeadlineSeconds",
        "value": 1
    },
    {
        "op": "replace",
        "path": "/spec/template/spec/containers/0/image",
        "value": "alpine:ops!"
    }
]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusDegraded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}
