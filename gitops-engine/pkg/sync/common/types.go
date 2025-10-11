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
	// AnnotationSyncWaveGroup indicates which wave of the sync the resource or hook should be in
	AnnotationSyncWaveGroup = "argocd.argoproj.io/sync-wave-group"
	// AnnotationSyncWaveGroupDependencies indicates which wave of the sync the resource or hook should be in
	AnnotationSyncWaveGroupDependencies = "argocd.argoproj.io/sync-wave-group-dependencies"
	// AnnotationKeyHook contains the hook type of a resource
	AnnotationKeyHook = "argocd.argoproj.io/hook"
	// AnnotationKeyHookDeletePolicy is the policy of deleting a hook
	AnnotationKeyHookDeletePolicy = "argocd.argoproj.io/hook-delete-policy"
	AnnotationDeletionApproved    = "argocd.argoproj.io/deletion-approved"

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
	// Sync option that disables use of replace or create command instead of apply
	SyncOptionDisableReplace = "Replace=false"
	// Sync option that enables use of --force flag, delete and re-create
	SyncOptionForce = "Force=true"
	// Sync option that enables use of --server-side flag instead of client-side
	SyncOptionServerSideApply = "ServerSideApply=true"
	// Sync option that disables use of --server-side flag instead of client-side
	SyncOptionDisableServerSideApply = "ServerSideApply=false"
	// Sync option that disables resource deletion
	SyncOptionDisableDeletion = "Delete=false"
	// Sync option that sync only out of sync resources
	SyncOptionApplyOutOfSyncOnly = "ApplyOutOfSyncOnly=true"
	// Sync option that disables sync only out of sync resources
	SyncOptionDisableApplyOutOfSyncOnly = "ApplyOutOfSyncOnly=false"
	// Sync option that requires confirmation before deleting the resource
	SyncOptionDeleteRequireConfirm = "Delete=confirm"
	// Sync option that requires confirmation before deleting the resource
	SyncOptionPruneRequireConfirm = "Prune=confirm"
	// Sync option that enables client-side apply migration
	SyncOptionClientSideApplyMigration = "ClientSideApplyMigration=true"
	// Sync option that disables client-side apply migration
	SyncOptionDisableClientSideApplyMigration = "ClientSideApplyMigration=false"

	// Default field manager for client-side apply migration
	DefaultClientSideApplyMigrationManager = "kubectl-client-side-apply"
)

type PermissionValidator func(un *unstructured.Unstructured, res *metav1.APIResource) error

type SyncPhase string

type SyncIdentity struct {
	Phase     SyncPhase
	Wave      int
	WaveGroup int
}

// Will be used when using Dependency graph
type GroupIdentity struct {
	Phase     SyncPhase
	WaveGroup int
}

// Will be used when using Dependency graph
type WaveDependency struct {
	Origin      GroupIdentity
	Destination GroupIdentity
}

// Will be used when using Dependency graph
type WaveDependencyGraph struct {
	Dependencies []WaveDependency
}

// SyncWaveHook is a callback function which will be invoked after each sync wave is successfully
// applied during a sync operation. The callback indicates which phase and wave it had just
// executed, and whether or not that wave was the final one.
type SyncWaveHook func(t []SyncIdentity, final bool) error

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
	// Images holds the images associated with the resource. These images are collected on a best-effort basis
	// from fields used by known workload resources. This does not necessarily reflect the exact list of images
	// used by workloads in the application.
	Images []string
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
