package v1beta1

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applications,shortName=app;apps
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
// +kubebuilder:validation:XValidation:rule="!(has(self.source) && self.source != null)",message="source is not supported in v1beta1, use sources instead"
// +kubebuilder:validation:XValidation:rule="(has(self.sources) && size(self.sources) > 0) || has(self.sourceHydrator)",message="sources is required (or sourceHydrator must be set)"
// +kubebuilder:validation:XValidation:rule="!has(self.sourceHydrator) || !has(self.sources) || size(self.sources) == 0",message="cannot have both sources and sourceHydrator defined"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, has(s.repoURL) && size(s.repoURL) > 0)",message="all sources must have a repoURL"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, has(s.targetRevision) && size(s.targetRevision) > 0)",message="all sources must have a targetRevision"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, !has(s.chart) || !has(s.path) || size(s.chart) == 0 || size(s.path) == 0)",message="sources cannot have both chart and path defined"
// +kubebuilder:validation:XValidation:rule="!has(self.sources) || size(self.sources) == 0 || self.sources.all(s, !has(s.ref) || size(s.ref) == 0 || (!has(s.path) || size(s.path) == 0) && (!has(s.chart) || size(s.chart) == 0))",message="ref sources cannot have path or chart defined"
// +kubebuilder:validation:XValidation:rule="has(self.project) && size(self.project) > 0",message="project is required"
// +kubebuilder:validation:XValidation:rule="(has(self.destination.server) && size(self.destination.server) > 0) || (has(self.destination.name) && size(self.destination.name) > 0)",message="destination must have either server or name set"
// +kubebuilder:validation:XValidation:rule="!((has(self.destination.server) && size(self.destination.server) > 0) && (has(self.destination.name) && size(self.destination.name) > 0))",message="destination can't have both name and server defined"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.retry) || !has(self.syncPolicy.retry.limit) || self.syncPolicy.retry.limit >= 0",message="retry limit must be >= 0"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.retry) || !has(self.syncPolicy.retry.backoff) || !has(self.syncPolicy.retry.backoff.factor) || self.syncPolicy.retry.backoff.factor >= 1",message="retry backoff factor must be >= 1"
// +kubebuilder:validation:XValidation:rule="!has(self.revisionHistoryLimit) || self.revisionHistoryLimit >= 0",message="revisionHistoryLimit must be >= 0"
// +kubebuilder:validation:XValidation:rule="!has(self.ignoreDifferences) || self.ignoreDifferences.all(d, has(d.kind) && size(d.kind) > 0)",message="ignoreDifferences entries must have a kind"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !self.syncPolicy.syncOptions.exists(s, s.startsWith('PrunePropagationPolicy=')) || self.syncPolicy.syncOptions.exists(s, s in ['PrunePropagationPolicy=background', 'PrunePropagationPolicy=foreground', 'PrunePropagationPolicy=orphan'])",message="PrunePropagationPolicy must be one of: background, foreground, orphan"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'Replace=true') && self.syncPolicy.syncOptions.exists(s, s == 'Replace=false'))",message="cannot have both Replace=true and Replace=false"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'ServerSideApply=true') && self.syncPolicy.syncOptions.exists(s, s == 'ServerSideApply=false'))",message="cannot have both ServerSideApply=true and ServerSideApply=false"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'ApplyOutOfSyncOnly=true') && self.syncPolicy.syncOptions.exists(s, s == 'ApplyOutOfSyncOnly=false'))",message="cannot have both ApplyOutOfSyncOnly=true and ApplyOutOfSyncOnly=false"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'ClientSideApplyMigration=true') && self.syncPolicy.syncOptions.exists(s, s == 'ClientSideApplyMigration=false'))",message="cannot have both ClientSideApplyMigration=true and ClientSideApplyMigration=false"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'Prune=false') && self.syncPolicy.syncOptions.exists(s, s == 'Prune=confirm'))",message="cannot have both Prune=false and Prune=confirm"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'Delete=false') && self.syncPolicy.syncOptions.exists(s, s == 'Delete=confirm'))",message="cannot have both Delete=false and Delete=confirm"
// +kubebuilder:validation:XValidation:rule="!has(self.syncPolicy) || !has(self.syncPolicy.syncOptions) || !(self.syncPolicy.syncOptions.exists(s, s == 'PruneLast=true') && self.syncPolicy.syncOptions.exists(s, s == 'Prune=false'))",message="cannot have PruneLast=true with Prune=false"
type ApplicationSpec struct {
	// Deprecated: Source is not supported in v1beta1. Use Sources instead.
	Source *v1alpha1.ApplicationSource `json:"source,omitempty" protobuf:"bytes,1,opt,name=source"`
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
	// Options allow you to specify whole app sync-options
	SyncOptions SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,2,opt,name=syncOptions"`
	// Retry controls failed sync retry behavior
	Retry *v1alpha1.RetryStrategy `json:"retry,omitempty" protobuf:"bytes,3,opt,name=retry"`
	// ManagedNamespaceMetadata controls metadata in the given namespace (if CreateNamespace=true)
	ManagedNamespaceMetadata *v1alpha1.ManagedNamespaceMetadata `json:"managedNamespaceMetadata,omitempty" protobuf:"bytes,4,opt,name=managedNamespaceMetadata"`
}

// SyncOptions is a list of sync options
// +kubebuilder:validation:MaxItems=20
// +kubebuilder:validation:XValidation:rule="self.all(s, s in ['SkipDryRunOnMissingResource=true','Prune=false','Validate=false','PruneLast=true','Replace=true','Replace=false','Force=true','ServerSideApply=true','ServerSideApply=false','Delete=false','Delete=confirm','ApplyOutOfSyncOnly=true','ApplyOutOfSyncOnly=false','Prune=confirm','ClientSideApplyMigration=true','ClientSideApplyMigration=false','PrunePropagationPolicy=background','PrunePropagationPolicy=foreground','PrunePropagationPolicy=orphan','CreateNamespace=true','RespectIgnoreDifferences=true','FailOnSharedResource=true'])",message="invalid syncOption, must be one of the predefined options"
type SyncOptions []string

// ApplicationSources is a list of application sources
// +kubebuilder:validation:MaxItems=20
type ApplicationSources []v1alpha1.ApplicationSource

// IgnoreDifferences is a list of resource ignore differences
// +kubebuilder:validation:MaxItems=100
type IgnoreDifferences []v1alpha1.ResourceIgnoreDifferences
