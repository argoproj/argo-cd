package common

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

const (
	// AnnotationSyncOptions is a comma-separated list of options for syncing
	AnnotationSyncOptions = "argocd.argoproj.io/sync-options"
	// AnnotationSyncWave indicates which wave of the sync the resource or hook should be in
	AnnotationSyncWave = "argocd.argoproj.io/sync-wave"
	// AnnotationKeyHook contains the hook type of a resource
	AnnotationKeyHook = "argocd.argoproj.io/hook"
	// AnnotationKeyHookDeletePolicy is the policy of deleting a hook
	AnnotationKeyHookDeletePolicy = "argocd.argoproj.io/hook-delete-policy"

	// Sync option that disables dry run in resource is missing in the cluster
	SyncOptionSkipDryRunOnMissingResource = "SkipDryRunOnMissingResource=true"
	// Sync option that disables resource pruning
	SyncOptionDisablePrune = "Prune=false"
	// Sync option that disables resource validation
	SyncOptionsDisableValidation = "Validate=false"
	// Sync option that enables pruneLast
	SyncOptionPruneLast = "PruneLast=true"
	// Sync option that enables use of replace or create command instead of apply
	SyncOptionReplace = "Replace=true"
	// Sync option that enables use of --server-side flag instead of client-side
	SyncOptionServerSideApply = "ServerSideApply=true"
)

type PermissionValidator func(un *unstructured.Unstructured, res *metav1.APIResource) error

type SyncPhase string

// SyncWaveHook is a callback function which will be invoked after each sync wave is successfully
// applied during a sync operation. The callback indicates which phase and wave it had just
// executed, and whether or not that wave was the final one.
type SyncWaveHook func(phase SyncPhase, wave int, final bool) error

const (
	SyncPhasePreSync  = "PreSync"
	SyncPhaseSync     = "Sync"
	SyncPhasePostSync = "PostSync"
	SyncPhaseSyncFail = "SyncFail"
)

type OperationPhase string

const (
	OperationRunning     OperationPhase = "Running"
	OperationTerminating OperationPhase = "Terminating"
	OperationFailed      OperationPhase = "Failed"
	OperationError       OperationPhase = "Error"
	OperationSucceeded   OperationPhase = "Succeeded"
)

func (os OperationPhase) Completed() bool {
	switch os {
	case OperationFailed, OperationError, OperationSucceeded:
		return true
	}
	return false
}

func (os OperationPhase) Running() bool {
	return os == OperationRunning
}

func (os OperationPhase) Successful() bool {
	return os == OperationSucceeded
}

func (os OperationPhase) Failed() bool {
	return os == OperationFailed
}

type ResultCode string

const (
	ResultCodeSynced       ResultCode = "Synced"
	ResultCodeSyncFailed   ResultCode = "SyncFailed"
	ResultCodePruned       ResultCode = "Pruned"
	ResultCodePruneSkipped ResultCode = "PruneSkipped"
)

type HookType string

const (
	HookTypePreSync  HookType = "PreSync"
	HookTypeSync     HookType = "Sync"
	HookTypePostSync HookType = "PostSync"
	HookTypeSkip     HookType = "Skip"
	HookTypeSyncFail HookType = "SyncFail"
)

func NewHookType(t string) (HookType, bool) {
	return HookType(t),
		t == string(HookTypePreSync) ||
			t == string(HookTypeSync) ||
			t == string(HookTypePostSync) ||
			t == string(HookTypeSyncFail) ||
			t == string(HookTypeSkip)

}

type HookDeletePolicy string

const (
	HookDeletePolicyHookSucceeded      HookDeletePolicy = "HookSucceeded"
	HookDeletePolicyHookFailed         HookDeletePolicy = "HookFailed"
	HookDeletePolicyBeforeHookCreation HookDeletePolicy = "BeforeHookCreation"
)

func NewHookDeletePolicy(p string) (HookDeletePolicy, bool) {
	return HookDeletePolicy(p),
		p == string(HookDeletePolicyHookSucceeded) ||
			p == string(HookDeletePolicyHookFailed) ||
			p == string(HookDeletePolicyBeforeHookCreation)
}

type ResourceSyncResult struct {
	// holds associated resource key
	ResourceKey kube.ResourceKey
	// holds resource version
	Version string
	// holds the execution order
	Order int
	// result code
	Status ResultCode
	// message for the last sync OR operation
	Message string
	// the type of the hook, empty for non-hook resources
	HookType HookType
	// the state of any operation associated with this resource OR hook
	// note: can contain values for non-hook resources
	HookPhase OperationPhase
	// indicates the particular phase of the sync that this is for
	SyncPhase SyncPhase
}
