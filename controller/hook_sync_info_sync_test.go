package controller

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
)

func syncInfoHookManifest(phase, inject string) string {
	annotations := fmt.Sprintf(`"argocd.argoproj.io/hook":%q`, phase)
	if inject != "" {
		annotations += fmt.Sprintf(`,"argocd.argoproj.io/inject-sync-info":%q`, inject)
	}
	return `{"apiVersion":"batch/v1","kind":"Job","metadata":{"generateName":"verify-","namespace":"` + test.FakeDestNamespace +
		`","annotations":{` + annotations + `}},"spec":{"template":{"metadata":{"annotations":{"keep":"me"}},"spec":{"restartPolicy":"Never","containers":[{"name":"c","image":"busybox"}]}}}}`
}

// runSyncWithHooks runs one SyncAppState pass and returns the operation state.
func runSyncWithHooks(t *testing.T, app *v1alpha1.Application, responses []*apiclient.ManifestResponse, op *v1alpha1.SyncOperation) *v1alpha1.OperationState {
	t.Helper()
	app.Status.OperationState = nil
	app.Status.History = nil
	proj := &v1alpha1.AppProject{Namespace: test.FakeArgoCDNamespace, Name: "default"}
	data := fakeData{
		apps:              []runtime.Object{app, proj},
		manifestResponses: responses,
		managedLiveObjs:   make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(t.Context(), &data, nil)
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{Sync: op}}
	ctrl.appStateManager.SyncAppState(t.Context(), app, proj, opState)
	return opState
}

// These tests only check whether hook preparation accepts or rejects the operation. The sync engine
// discovers API resources against a real server, so applied hooks and their revisions are verified by
// the e2e tests in test/e2e/hook_test.go.
func TestSyncAppState_SyncInfo_InvalidOptInFailsOperation(t *testing.T) {
	app := newFakeApp()
	opState := runSyncWithHooks(t, app, []*apiclient.ManifestResponse{{
		Manifests: []string{syncInfoHookManifest("PostSync", "revision,bogus")},
		Namespace: test.FakeDestNamespace, Server: test.FakeClusterURL, Revision: "sha-A",
	}}, &v1alpha1.SyncOperation{})
	assert.Equal(t, synccommon.OperationError, opState.Phase)
	assert.Contains(t, opState.Message, `Failed to prepare sync hooks: unsupported argocd.argoproj.io/inject-sync-info field "bogus" on Job "verify-*"`)
}

func TestSyncAppState_SyncInfo_LocalManifestsHaveNoRevision(t *testing.T) {
	app := newFakeApp()
	opState := runSyncWithHooks(t, app, []*apiclient.ManifestResponse{{
		Manifests: []string{syncInfoHookManifest("PostSync", "revision")},
		Namespace: test.FakeDestNamespace, Server: test.FakeClusterURL,
	}}, &v1alpha1.SyncOperation{Manifests: []string{syncInfoHookManifest("PostSync", "revision")}})
	assert.Equal(t, synccommon.OperationError, opState.Phase)
	assert.Contains(t, opState.Message, "Failed to prepare sync hooks: cannot inject revision")
	assert.Contains(t, opState.Message, "no resolved revision")
}

func TestSyncAppState_SyncInfo_NotOptedInDoesNotFailPreparation(t *testing.T) {
	app := newFakeApp()
	opState := runSyncWithHooks(t, app, []*apiclient.ManifestResponse{{
		Manifests: []string{syncInfoHookManifest("PostSync", "")},
		Namespace: test.FakeDestNamespace, Server: test.FakeClusterURL,
	}}, &v1alpha1.SyncOperation{Manifests: []string{syncInfoHookManifest("PostSync", "")}})
	assert.NotContains(t, opState.Message, "Failed to prepare sync hooks")
}

// Hooks are skipped for selective syncs and the apply strategy, so their opt-in must not fail the operation.
func TestSyncAppState_SyncInfo_SkippedHooksDoNotFailPreparation(t *testing.T) {
	tests := map[string]*v1alpha1.SyncOperation{
		"selective sync": {Resources: []v1alpha1.SyncOperationResource{{Kind: "ConfigMap", Name: "other"}}},
		"apply strategy": {SyncStrategy: &v1alpha1.SyncStrategy{Apply: &v1alpha1.SyncStrategyApply{}}},
	}
	for name, op := range tests {
		t.Run(name, func(t *testing.T) {
			app := newFakeApp()
			opState := runSyncWithHooks(t, app, []*apiclient.ManifestResponse{{
				Manifests: []string{syncInfoHookManifest("PostSync", "revision,bogus")},
				Namespace: test.FakeDestNamespace, Server: test.FakeClusterURL, Revision: "sha-A",
			}}, op)
			assert.NotContains(t, opState.Message, "Failed to prepare sync hooks")
		})
	}
}

func TestSyncAppState_SyncInfo_NullTemplateMetadataDoesNotFailPreparation(t *testing.T) {
	manifest := `{"apiVersion":"batch/v1","kind":"Job","metadata":{"generateName":"verify-","namespace":"` + test.FakeDestNamespace +
		`","annotations":{"argocd.argoproj.io/hook":"PostSync","argocd.argoproj.io/inject-sync-info":"revision"}},` +
		`"spec":{"template":{"metadata":null,"spec":{"restartPolicy":"Never","containers":[{"name":"c","image":"busybox"}]}}}}`
	app := newFakeApp()
	opState := runSyncWithHooks(t, app, []*apiclient.ManifestResponse{{
		Manifests: []string{manifest},
		Namespace: test.FakeDestNamespace, Server: test.FakeClusterURL, Revision: "sha-A",
	}}, &v1alpha1.SyncOperation{})
	assert.NotContains(t, opState.Message, "Failed to prepare sync hooks")
}
