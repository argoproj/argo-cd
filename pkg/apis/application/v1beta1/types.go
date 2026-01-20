package v1beta1

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application is a definition of Application resource.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applications,shortName=app;apps
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Sync Status",type=string,JSONPath=`.status.sync.status`
// +kubebuilder:printcolumn:name="Health Status",type=string,JSONPath=`.status.health.status`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.sync.revision`,priority=10
// +kubebuilder:printcolumn:name="Project",type=string,JSONPath=`.spec.project`,priority=10
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSpec            `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            v1alpha1.ApplicationStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
	Operation         *v1alpha1.Operation        `json:"operation,omitempty" protobuf:"bytes,4,opt,name=operation"`
}

// ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.
// +kubebuilder:validation:XValidation:rule="(has(self.sources) && size(self.sources) > 0) || has(self.sourceHydrator)",message="sources is required (or sourceHydrator must be set)"
// +kubebuilder:validation:XValidation:rule="!has(self.sourceHydrator) || !has(self.sources) || size(self.sources) == 0",message="cannot have both sources and sourceHydrator defined"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, has(s.repoURL) && size(s.repoURL) > 0)",message="all sources must have a repoURL"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, has(s.targetRevision) && size(s.targetRevision) > 0)",message="all sources must have a targetRevision"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, !has(s.chart) || !has(s.path) || size(s.chart) == 0 || size(s.path) == 0)",message="sources cannot have both chart and path defined"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, !has(s.ref) || size(s.ref) == 0 || (!has(s.path) || size(s.path) == 0) && (!has(s.chart) || size(s.chart) == 0))",message="ref sources cannot have path or chart defined"
// +kubebuilder:validation:XValidation:rule="has(self.project) && size(self.project) > 0",message="project is required"
// +kubebuilder:validation:XValidation:rule="size(self.project) <= 253 && self.project.matches('^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$')",message="project must be a valid DNS subdomain (lowercase alphanumeric, '-', or '.', must start and end with alphanumeric, max 253 chars)"
// +kubebuilder:validation:XValidation:rule="(has(self.destination.server) && size(self.destination.server) > 0) || (has(self.destination.name) && size(self.destination.name) > 0)",message="destination must have either server or name set"
// +kubebuilder:validation:XValidation:rule="!((has(self.destination.server) && size(self.destination.server) > 0) && (has(self.destination.name) && size(self.destination.name) > 0))",message="destination can't have both name and server defined"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.retry) || !has(self.syncPolicy.retry.limit) || self.syncPolicy.retry.limit >= 0",message="retry limit must be >= 0"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.retry) || !has(self.syncPolicy.retry.backoff) || !has(self.syncPolicy.retry.backoff.factor) || self.syncPolicy.retry.backoff.factor >= 1",message="retry backoff factor must be >= 1"
// +kubebuilder:validation:XValidation:rule="!has(self.revisionHistoryLimit) || self.revisionHistoryLimit >= 0",message="revisionHistoryLimit must be >= 0"
// +kubebuilder:validation:XValidation:rule="!has(self.ignoreDifferences) || self.ignoreDifferences.all(d, has(d.kind) && size(d.kind) > 0)",message="ignoreDifferences entries must have a kind"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !has(self.syncPolicy.syncOptions.pruneLast) || !self.syncPolicy.syncOptions.pruneLast || !has(self.syncPolicy.syncOptions.prune) || self.syncPolicy.syncOptions.prune != 'disabled'",message="cannot have pruneLast=true with prune=disabled"
type ApplicationSpec struct {
	// Destination is a reference to the target Kubernetes server and namespace
	Destination v1alpha1.ApplicationDestination `json:"destination" protobuf:"bytes,2,name=destination"`
	// Project is a reference to the project this application belongs to.
	// The empty string means that application belongs to the 'default' project.
	Project string `json:"project" protobuf:"bytes,3,name=project"`
	// SyncPolicy controls when and how a sync will be performed
	SyncPolicy *SyncPolicy `json:"syncPolicy,omitempty" protobuf:"bytes,4,name=syncPolicy"`
	// IgnoreDifferences is a list of resources and their fields which should be ignored during comparison
	IgnoreDifferences IgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
	// Info contains a list of information (URLs, email addresses, and plain text) that relates to the application
	Info []v1alpha1.Info `json:"info,omitempty" protobuf:"bytes,6,name=info"`
	// RevisionHistoryLimit limits the number of items kept in the application's revision history, which is used for informational purposes as well as for rollbacks to previous versions.
	// This should only be changed in exceptional circumstances.
	// Setting to zero will store no history. This will reduce storage used.
	// Increasing will increase the space used to store the history, so we do not recommend increasing it.
	// Default is 10.
	RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty" protobuf:"bytes,7,name=revisionHistoryLimit"`
	// Sources is a reference to the location of the application's manifests or chart
	Sources ApplicationSources `json:"sources,omitempty" protobuf:"bytes,8,opt,name=sources"`
	// SourceHydrator provides a way to push hydrated manifests back to git before syncing them to the cluster.
	SourceHydrator *v1alpha1.SourceHydrator `json:"sourceHydrator,omitempty" protobuf:"bytes,9,opt,name=sourceHydrator"`
}

// ApplicationList is list of Application resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Application `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// SyncPolicy controls when and how a sync will be performed
type SyncPolicy struct {
	// Automated will keep an application synced to the target revision
	Automated *v1alpha1.SyncPolicyAutomated `json:"automated,omitempty" protobuf:"bytes,1,opt,name=automated"`
	// SyncOptions provide per-sync sync-options
	SyncOptions *SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,2,opt,name=syncOptions"`
	// Retry controls failed sync retry behavior
	Retry *v1alpha1.RetryStrategy `json:"retry,omitempty" protobuf:"bytes,3,opt,name=retry"`
	// ManagedNamespaceMetadata controls metadata in the given namespace (if CreateNamespace=true)
	ManagedNamespaceMetadata *v1alpha1.ManagedNamespaceMetadata `json:"managedNamespaceMetadata,omitempty" protobuf:"bytes,4,opt,name=managedNamespaceMetadata"`
}

// SyncOptions provides structured sync options for an application.
// This replaces the v1alpha1 []string format with properly typed fields.
type SyncOptions struct {
	// Validate enables/disables kubectl validation (default: true)
	// +optional
	Validate *bool `json:"validate,omitempty" protobuf:"varint,1,opt,name=validate"`

	// CreateNamespace creates the namespace if it doesn't exist (default: false)
	// +optional
	CreateNamespace *bool `json:"createNamespace,omitempty" protobuf:"varint,2,opt,name=createNamespace"`

	// PrunePropagationPolicy sets the propagation policy for pruning
	// +kubebuilder:validation:Enum=background;foreground;orphan
	// +optional
	PrunePropagationPolicy *PrunePropagationPolicy `json:"prunePropagationPolicy,omitempty" protobuf:"bytes,3,opt,name=prunePropagationPolicy"`

	// Prune controls whether pruning is enabled, disabled, or requires confirmation
	// +kubebuilder:validation:Enum=enabled;disabled;confirm
	// +optional
	Prune *SyncOptionPruneDelete `json:"prune,omitempty" protobuf:"bytes,4,opt,name=prune"`

	// PruneLast causes resources to be pruned after all other resources have been synced (default: false)
	// +optional
	PruneLast *bool `json:"pruneLast,omitempty" protobuf:"varint,5,opt,name=pruneLast"`

	// Delete controls whether deletion is enabled, disabled, or requires confirmation
	// +kubebuilder:validation:Enum=enabled;disabled;confirm
	// +optional
	Delete *SyncOptionPruneDelete `json:"delete,omitempty" protobuf:"bytes,6,opt,name=delete"`

	// Replace uses `kubectl replace` instead of `kubectl apply` (default: false)
	// +optional
	Replace *bool `json:"replace,omitempty" protobuf:"varint,7,opt,name=replace"`

	// Force uses `kubectl apply --force` which deletes and recreates resources (default: false)
	// +optional
	Force *bool `json:"force,omitempty" protobuf:"varint,8,opt,name=force"`

	// ServerSideApply uses server-side apply for syncing (default: false)
	// +optional
	ServerSideApply *bool `json:"serverSideApply,omitempty" protobuf:"varint,9,opt,name=serverSideApply"`

	// ApplyOutOfSyncOnly only applies resources that are out of sync (default: false)
	// +optional
	ApplyOutOfSyncOnly *bool `json:"applyOutOfSyncOnly,omitempty" protobuf:"varint,10,opt,name=applyOutOfSyncOnly"`

	// SkipDryRunOnMissingResource skips dry-run when CRDs are missing (default: false)
	// +optional
	SkipDryRunOnMissingResource *bool `json:"skipDryRunOnMissingResource,omitempty" protobuf:"varint,11,opt,name=skipDryRunOnMissingResource"`

	// RespectIgnoreDifferences respects ignoreDifferences config during sync (default: false)
	// +optional
	RespectIgnoreDifferences *bool `json:"respectIgnoreDifferences,omitempty" protobuf:"varint,12,opt,name=respectIgnoreDifferences"`

	// FailOnSharedResource fails sync if a resource is already applied by another application (default: false)
	// +optional
	FailOnSharedResource *bool `json:"failOnSharedResource,omitempty" protobuf:"varint,13,opt,name=failOnSharedResource"`

	// ClientSideApplyMigration enables migration from client-side to server-side apply (default: false)
	// +optional
	ClientSideApplyMigration *bool `json:"clientSideApplyMigration,omitempty" protobuf:"varint,14,opt,name=clientSideApplyMigration"`
}

// PrunePropagationPolicy defines the propagation policy for pruning resources
// +kubebuilder:validation:Enum=background;foreground;orphan
type PrunePropagationPolicy string

const (
	// PrunePropagationPolicyBackground deletes dependent objects in the background
	PrunePropagationPolicyBackground PrunePropagationPolicy = "background"
	// PrunePropagationPolicyForeground deletes dependent objects in the foreground
	PrunePropagationPolicyForeground PrunePropagationPolicy = "foreground"
	// PrunePropagationPolicyOrphan orphans dependent objects
	PrunePropagationPolicyOrphan PrunePropagationPolicy = "orphan"
)

// SyncOptionPruneDelete defines the behavior for prune and delete operations
// +kubebuilder:validation:Enum=enabled;disabled;confirm
type SyncOptionPruneDelete string

const (
	// SyncOptionEnabled enables the operation
	SyncOptionEnabled SyncOptionPruneDelete = "enabled"
	// SyncOptionDisabled disables the operation
	SyncOptionDisabled SyncOptionPruneDelete = "disabled"
	// SyncOptionConfirm requires confirmation for the operation
	SyncOptionConfirm SyncOptionPruneDelete = "confirm"
)

// ApplicationSources is a list of application sources
// +kubebuilder:validation:MaxItems=20
type ApplicationSources []v1alpha1.ApplicationSource

// IgnoreDifferences is a list of resource ignore differences
// +kubebuilder:validation:MaxItems=100
type IgnoreDifferences []v1alpha1.ResourceIgnoreDifferences
