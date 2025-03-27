package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	v1 "k8s.io/api/core/v1"
)

func TestFixingDegradedApp(t *testing.T) {
	Given(t).
		Path("sync-waves").
		When().
		IgnoreErrors().
		CreateApp().
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
		Expect(HealthIs(health.HealthStatusDegraded)).
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
		CreateApp().
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
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusDegraded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}

// resources should be pruned in reverse of creation order(syncwaves order)
func TestSyncPruneOrderWithSyncWaves(t *testing.T) {
	ctx := Given(t).Timeout(60)

	// remove finalizer to ensure proper cleanup if test fails at early stage
	defer func() {
		_, _ = RunCli("app", "patch-resource", ctx.AppQualifiedName(),
			"--kind", "Pod",
			"--resource-name", "pod-with-finalizers",
			"--patch", `[{"op": "remove", "path": "/metadata/finalizers"}]`,
			"--patch-type", "application/json-patch+json", "--all",
		)
	}()

	ctx.Path("syncwaves-prune-order").
		When().
		CreateApp().
		// creation order: sa & role -> rolebinding -> pod
		Sync().
		Wait().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		// delete files to remove resources
		DeleteFile("pod.yaml").
		DeleteFile("rbac.yaml").
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		// prune order: pod -> rolebinding -> sa & role
		Sync("--prune").
		Wait().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "pod-with-finalizers" })).
		Expect(ResourceResultNumbering(4))
}
