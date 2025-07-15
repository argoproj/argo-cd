package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// TestSyncWithCreateNamespace verifies that the namespace is created when the
// CreateNamespace=true is provided as part of the normal sync resources
func TestSyncWithCreateNamespace(t *testing.T) {
	newNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			errors.NewHandler(t).FailOnErr(Run("", "kubectl", "delete", "namespace", newNamespace))
		}
	}()

	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Destination.Namespace = newNamespace
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{
					"CreateNamespace=true",
				},
			}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceResultNumbering(3))
}

// TestSyncWithCreateNamespaceAndDryRunError verifies that the namespace is created before the
// DryRun validation is made on the resources, even if the sync fails. This allows transient errors
// to be resolved on sync retries
func TestSyncWithCreateNamespaceAndDryRunError(t *testing.T) {
	newNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			errors.NewHandler(t).FailOnErr(Run("", "kubectl", "delete", "namespace", newNamespace))
		}
	}()

	Given(t).
		Path("failure-during-sync").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Destination.Namespace = newNamespace
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{
					"CreateNamespace=true",
				},
			}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceResultMatches(ResourceResult{Version: "v1", Kind: "Namespace", Name: newNamespace, Status: ResultCodeSynced, Message: fmt.Sprintf("namespace/%s created", newNamespace), HookPhase: OperationRunning, SyncPhase: SyncPhasePreSync})).
		Expect(ResourceResultMatches(ResourceResult{Version: "v1", Kind: "ServiceAccount", Namespace: newNamespace, Name: "failure-during-sync", Status: ResultCodeSyncFailed, Message: `ServiceAccount "failure-during-sync" is invalid: metadata.labels: Invalid value`, HookPhase: OperationFailed, SyncPhase: SyncPhaseSync}))
}

// TestSyncOptionsValidateFalse verifies we can disable validation during kubectl apply, using the
// 'argocd.argoproj.io/sync-options: Validate=false' sync option
func TestSyncOptionsValidateFalse(t *testing.T) {
	Given(t).
		Path("sync-options-validate-false").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
	// NOTE: it is a bug that we do not detect this as OutOfSync. This is because we
	// are dropping fields as part of remarshalling. See: https://github.com/argoproj/argo-cd/issues/1787
	// Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

// TestSyncOptionsValidateTrue verifies when 'argocd.argoproj.io/sync-options: Validate=false' is
// not present, then validation is performed and we fail during the apply
func TestSyncOptionsValidateTrue(t *testing.T) {
	// k3s does not validate at all, so this test does not work
	if os.Getenv("ARGOCD_E2E_K3S") == "true" {
		t.SkipNow()
	}
	Given(t).
		Path("sync-options-validate-false").
		When().
		IgnoreErrors().
		CreateApp().
		PatchFile("invalid-cm.yaml", `[{"op": "remove", "path": "/metadata/annotations"}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed))
}

func TestSyncWithStatusIgnored(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		And(func() {
			require.NoError(t, SetResourceOverrides(map[string]ResourceOverride{
				"/": {
					IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/status"}},
				},
			}))
		}).
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: true}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced if git change detected
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/status", "value": { "observedGeneration": 1 }}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced if k8s change detected
		When().
		And(func() {
			errors.NewHandler(t).FailOnErr(KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/status/observedGeneration", "value": 2 }]`), metav1.PatchOptions{}))
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestSyncWithApplyOutOfSyncOnly(t *testing.T) {
	var ns string
	Given(t).
		Path(guestbookPath).
		ApplyOutOfSyncOnly().
		When().
		CreateFromFile(func(app *Application) {
			ns = app.Spec.Destination.Namespace
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "replace", "path": "/spec/replicas", "value": 1 }]`).
		Sync().
		Then().
		// Only one resource should be in sync result
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceResultIs(ResourceResult{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: ns, Name: "guestbook-ui", Message: "deployment.apps/guestbook-ui configured", SyncPhase: SyncPhaseSync, HookPhase: OperationRunning, Status: ResultCodeSynced, Images: []string{"quay.io/argoprojlabs/argocd-e2e-container:0.2"}}))
}

func TestSyncWithSkipHook(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: true}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced when app has skipped annotation even if git change detected
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/metadata/annotations", "value": { "argocd.argoproj.io/hook": "Skip" }}]`).
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "replace", "path": "/spec/replicas", "value": 1 }]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should not remain synced if skipped annotation removed
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "remove", "path": "/metadata/annotations" }]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestSyncWithForceReplace(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app having `Replace=true` and `Force=true` annotation should sync succeed if change in immutable field
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/metadata/annotations", "value": { "argocd.argoproj.io/sync-options": "Force=true,Replace=true" }}]`).
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/spec/selector/matchLabels/env", "value": "e2e" }, { "op": "add", "path": "/spec/template/metadata/labels/env", "value": "e2e" }]`).
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "replace", "path": "/spec/replicas", "value": 1 }]`).
		Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

// Given application is set with --sync-option CreateNamespace=true and --sync-option ServerSideApply=true
//
//		application --dest-namespace exists
//
//	Then, --dest-namespace is created with server side apply
//		  	application is synced and healthy with resource
//		  	application resources created with server side apply in the newly created namespace.
func TestNamespaceCreationWithSSA(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	namespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			errors.NewHandler(t).FailOnErr(Run("", "kubectl", "delete", "namespace", namespace))
		}
	}()

	Given(t).
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.Destination.Namespace = namespace
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{"CreateNamespace=true", "ServerSideApply=true"},
			}
		}).
		Then().
		Expect(NoNamespace(namespace)).
		When().
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(namespace, func(_ *Application, ns *corev1.Namespace) {
			assert.NotContains(t, ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		})).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", namespace, SyncStatusCodeSynced)).
		Expect(ResourceHealthWithNamespaceIs("Service", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Service", "guestbook-ui", namespace, SyncStatusCodeSynced))
}
