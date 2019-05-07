package v1alpha1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/git"
)

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            ApplicationStatus `json:"status" protobuf:"bytes,3,opt,name=status"`
	Operation         *Operation        `json:"operation,omitempty" protobuf:"bytes,4,opt,name=operation"`
}

// ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.
type ApplicationSpec struct {
	// Source is a reference to the location ksonnet application definition
	Source ApplicationSource `json:"source" protobuf:"bytes,1,opt,name=source"`
	// Destination overrides the kubernetes server and namespace defined in the environment ksonnet app.yaml
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,name=destination"`
	// Project is a application project name. Empty name means that application belongs to 'default' project.
	Project string `json:"project" protobuf:"bytes,3,name=project"`
	// SyncPolicy controls when a sync will be performed
	SyncPolicy *SyncPolicy `json:"syncPolicy,omitempty" protobuf:"bytes,4,name=syncPolicy"`
	// IgnoreDifferences controls resources fields which should be ignored during comparison
	IgnoreDifferences []ResourceIgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
}

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group        string   `json:"group" protobuf:"bytes,1,opt,name=group"`
	Kind         string   `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name         string   `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
	Namespace    string   `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	JSONPointers []string `json:"jsonPointers" protobuf:"bytes,5,opt,name=jsonPointers"`
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the git repository URL of the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the repository containing a
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`
	// Environment is a ksonnet application environment name
	// TargetRevision defines the commit, tag, or branch in which to sync the application to.
	// If omitted, will sync to HEAD
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// Helm holds helm specific options
	Helm *ApplicationSourceHelm `json:"helm,omitempty" protobuf:"bytes,7,opt,name=helm"`
	// Kustomize holds kustomize specific options
	Kustomize *ApplicationSourceKustomize `json:"kustomize,omitempty" protobuf:"bytes,8,opt,name=kustomize"`
	// Ksonnet holds ksonnet specific options
	Ksonnet *ApplicationSourceKsonnet `json:"ksonnet,omitempty" protobuf:"bytes,9,opt,name=ksonnet"`
	// Directory holds path/directory specific options
	Directory *ApplicationSourceDirectory `json:"directory,omitempty" protobuf:"bytes,10,opt,name=directory"`
	// ConfigManagementPlugin holds config management plugin specific options
	Plugin *ApplicationSourcePlugin `json:"plugin,omitempty" protobuf:"bytes,11,opt,name=plugin"`
}

func (a ApplicationSource) IsZero() bool {
	return a.RepoURL == "" &&
		a.Path == "" &&
		a.TargetRevision == "" &&
		a.Helm.IsZero() &&
		a.Kustomize.IsZero() &&
		a.Ksonnet.IsZero() &&
		a.Directory.IsZero() &&
		a.Plugin.IsZero()
}

type ApplicationSourceType string

const (
	ApplicationSourceTypeHelm      ApplicationSourceType = "Helm"
	ApplicationSourceTypeKustomize ApplicationSourceType = "Kustomize"
	ApplicationSourceTypeKsonnet   ApplicationSourceType = "Ksonnet"
	ApplicationSourceTypeDirectory ApplicationSourceType = "Directory"
	ApplicationSourceTypePlugin    ApplicationSourceType = "Plugin"
)

type RefreshType string

const (
	RefreshTypeNormal RefreshType = "normal"
	RefreshTypeHard   RefreshType = "hard"
)

// ApplicationSourceHelm holds helm specific options
type ApplicationSourceHelm struct {
	// ValuesFiles is a list of Helm value files to use when generating a template
	ValueFiles []string `json:"valueFiles,omitempty" protobuf:"bytes,1,opt,name=valueFiles"`
	// Parameters are parameters to the helm template
	Parameters []HelmParameter `json:"parameters,omitempty" protobuf:"bytes,2,opt,name=parameters"`
}

// HelmParameter is a parameter to a helm template
type HelmParameter struct {
	// Name is the name of the helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is the value for the helm parameter
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

func (h *ApplicationSourceHelm) IsZero() bool {
	return len(h.ValueFiles) == 0 && len(h.Parameters) == 0
}

// ApplicationSourceKustomize holds kustomize specific options
type ApplicationSourceKustomize struct {
	// NamePrefix is a prefix appended to resources for kustomize apps
	NamePrefix string `json:"namePrefix,omitempty" protobuf:"bytes,1,opt,name=namePrefix"`
	// ImageTags are kustomize 1.0 image tag overrides
	ImageTags []KustomizeImageTag `json:"imageTags,omitempty" protobuf:"bytes,2,opt,name=imageTags"`
	// Images are kustomize 2.0 image overrides
	Images []string `json:"images,omitempty" protobuf:"bytes,3,opt,name=images"`
}

// KustomizeImageTag is a kustomize image tag
type KustomizeImageTag struct {
	// Name is the name of the image (e.g. nginx)
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is the value for the new tag (e.g. 1.8.0)
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

func (k *ApplicationSourceKustomize) IsZero() bool {
	return k.NamePrefix == "" && len(k.ImageTags) == 0 && len(k.Images) == 0
}

// JsonnetVar is a jsonnet variable
type JsonnetVar struct {
	Name  string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
	Code  bool   `json:"code,omitempty" protobuf:"bytes,3,opt,name=code"`
}

// ApplicationSourceJsonnet holds jsonnet specific options
type ApplicationSourceJsonnet struct {
	// ExtVars is a list of Jsonnet External Variables
	ExtVars []JsonnetVar `json:"extVars,omitempty" protobuf:"bytes,1,opt,name=extVars"`
	// TLAS is a list of Jsonnet Top-level Arguments
	TLAs []JsonnetVar `json:"tlas,omitempty" protobuf:"bytes,2,opt,name=tlas"`
}

func (j *ApplicationSourceJsonnet) IsZero() bool {
	return len(j.ExtVars) == 0 && len(j.TLAs) == 0
}

// ApplicationSourceKsonnet holds ksonnet specific options
type ApplicationSourceKsonnet struct {
	// Environment is a ksonnet application environment name
	Environment string `json:"environment,omitempty" protobuf:"bytes,1,opt,name=environment"`
	// Parameters are a list of ksonnet component parameter override values
	Parameters []KsonnetParameter `json:"parameters,omitempty" protobuf:"bytes,2,opt,name=parameters"`
}

// KsonnetParameter is a ksonnet component parameter
type KsonnetParameter struct {
	Component string `json:"component,omitempty" protobuf:"bytes,1,opt,name=component"`
	Name      string `json:"name" protobuf:"bytes,2,opt,name=name"`
	Value     string `json:"value" protobuf:"bytes,3,opt,name=value"`
}

func (k *ApplicationSourceKsonnet) IsZero() bool {
	return k.Environment == "" && len(k.Parameters) == 0
}

type ApplicationSourceDirectory struct {
	Recurse bool                     `json:"recurse,omitempty" protobuf:"bytes,1,opt,name=recurse"`
	Jsonnet ApplicationSourceJsonnet `json:"jsonnet,omitempty" protobuf:"bytes,2,opt,name=jsonnet"`
}

func (d *ApplicationSourceDirectory) IsZero() bool {
	return !d.Recurse && d.Jsonnet.IsZero()
}

// ApplicationSourcePlugin holds config management plugin specific options
type ApplicationSourcePlugin struct {
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
}

func (c *ApplicationSourcePlugin) IsZero() bool {
	return c.Name == ""
}

// ApplicationDestination contains deployment destination information
type ApplicationDestination struct {
	// Server overrides the environment server value in the ksonnet app.yaml
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Namespace overrides the environment namespace value in the ksonnet app.yaml
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

// ApplicationStatus contains information about application sync, health status
type ApplicationStatus struct {
	Resources      []ResourceStatus       `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	Sync           SyncStatus             `json:"sync,omitempty" protobuf:"bytes,2,opt,name=sync"`
	Health         HealthStatus           `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`
	History        []RevisionHistory      `json:"history,omitempty" protobuf:"bytes,4,opt,name=history"`
	Conditions     []ApplicationCondition `json:"conditions,omitempty" protobuf:"bytes,5,opt,name=conditions"`
	ReconciledAt   metav1.Time            `json:"reconciledAt,omitempty" protobuf:"bytes,6,opt,name=reconciledAt"`
	OperationState *OperationState        `json:"operationState,omitempty" protobuf:"bytes,7,opt,name=operationState"`
	ObservedAt     metav1.Time            `json:"observedAt,omitempty" protobuf:"bytes,8,opt,name=observedAt"`
	SourceType     ApplicationSourceType  `json:"sourceType,omitempty" protobuf:"bytes,9,opt,name=sourceType"`
	Summary        ApplicationSummary     `json:"summary,omitempty" protobuf:"bytes,10,opt,name=summary"`
}

// Operation contains requested operation parameters.
type Operation struct {
	Sync *SyncOperation `json:"sync,omitempty" protobuf:"bytes,1,opt,name=sync"`
}

// SyncOperationResource contains resources to sync.
type SyncOperationResource struct {
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind  string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name  string `json:"name" protobuf:"bytes,3,opt,name=name"`
}

// HasIdentity determines whether a sync operation is identified by a manifest.
func (r SyncOperationResource) HasIdentity(name string, gvk schema.GroupVersionKind) bool {
	if name == r.Name && gvk.Kind == r.Kind && gvk.Group == r.Group {
		return true
	}
	return false
}

// SyncOperation contains sync operation details.
type SyncOperation struct {
	// Revision is the git revision in which to sync the application to.
	// If omitted, will use the revision specified in app spec.
	Revision string `json:"revision,omitempty" protobuf:"bytes,1,opt,name=revision"`
	// Prune deletes resources that are no longer tracked in git
	Prune bool `json:"prune,omitempty" protobuf:"bytes,2,opt,name=prune"`
	// DryRun will perform a `kubectl apply --dry-run` without actually performing the sync
	DryRun bool `json:"dryRun,omitempty" protobuf:"bytes,3,opt,name=dryRun"`
	// SyncStrategy describes how to perform the sync
	SyncStrategy *SyncStrategy `json:"syncStrategy,omitempty" protobuf:"bytes,4,opt,name=syncStrategy"`
	// Resources describes which resources to sync
	Resources []SyncOperationResource `json:"resources,omitempty" protobuf:"bytes,6,opt,name=resources"`
	// Source overrides the source definition set in the application.
	// This is typically set in a Rollback operation and nil during a Sync operation
	Source *ApplicationSource `json:"source,omitempty" protobuf:"bytes,7,opt,name=source"`
}

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

func (os OperationPhase) Successful() bool {
	return os == OperationSucceeded
}

// OperationState contains information about state of currently performing operation on application.
type OperationState struct {
	// Operation is the original requested operation
	Operation Operation `json:"operation" protobuf:"bytes,1,opt,name=operation"`
	// Phase is the current phase of the operation
	Phase OperationPhase `json:"phase" protobuf:"bytes,2,opt,name=phase"`
	// Message hold any pertinent messages when attempting to perform operation (typically errors).
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// SyncResult is the result of a Sync operation
	SyncResult *SyncOperationResult `json:"syncResult,omitempty" protobuf:"bytes,4,opt,name=syncResult"`
	// StartedAt contains time of operation start
	StartedAt metav1.Time `json:"startedAt" protobuf:"bytes,6,opt,name=startedAt"`
	// FinishedAt contains time of operation completion
	FinishedAt *metav1.Time `json:"finishedAt,omitempty" protobuf:"bytes,7,opt,name=finishedAt"`
}

// SyncPolicy controls when a sync will be performed in response to updates in git
type SyncPolicy struct {
	// Automated will keep an application synced to the target revision
	Automated *SyncPolicyAutomated `json:"automated,omitempty" protobuf:"bytes,1,opt,name=automated"`
}

// SyncPolicyAutomated controls the behavior of an automated sync
type SyncPolicyAutomated struct {
	// Prune will prune resources automatically as part of automated sync (default: false)
	Prune bool `json:"prune,omitempty" protobuf:"bytes,1,opt,name=prune"`
}

// SyncStrategy controls the manner in which a sync is performed
type SyncStrategy struct {
	// Apply wil perform a `kubectl apply` to perform the sync.
	Apply *SyncStrategyApply `json:"apply,omitempty" protobuf:"bytes,1,opt,name=apply"`
	// Hook will submit any referenced resources to perform the sync. This is the default strategy
	Hook *SyncStrategyHook `json:"hook,omitempty" protobuf:"bytes,2,opt,name=hook"`
}

// SyncStrategyApply uses `kubectl apply` to perform the apply
type SyncStrategyApply struct {
	// Force indicates whether or not to supply the --force flag to `kubectl apply`.
	// The --force flag deletes and re-create the resource, when PATCH encounters conflict and has
	// retried for 5 times.
	Force bool `json:"force,omitempty" protobuf:"bytes,1,opt,name=force"`
}

// SyncStrategyHook will perform a sync using hooks annotations.
// If no hook annotation is specified falls back to `kubectl apply`.
type SyncStrategyHook struct {
	// Embed SyncStrategyApply type to inherit any `apply` options
	// +optional
	SyncStrategyApply `protobuf:"bytes,1,opt,name=syncStrategyApply"`
}

type HookType string

const (
	HookTypePreSync  HookType = "PreSync"
	HookTypeSync     HookType = "Sync"
	HookTypePostSync HookType = "PostSync"
	HookTypeSkip     HookType = "Skip"

	// NOTE: we may consider adding SyncFail hook. With a SyncFail hook, finalizer-like logic could
	// be implemented by specifying both PostSync,SyncFail in the hook annotation:
	// (e.g.: argocd.argoproj.io/hook: PostSync,SyncFail)
	//HookTypeSyncFail     HookType = "SyncFail"
)

type HookDeletePolicy string

const (
	HookDeletePolicyHookSucceeded HookDeletePolicy = "HookSucceeded"
	HookDeletePolicyHookFailed    HookDeletePolicy = "HookFailed"
)

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	// Resources holds the sync result of each individual resource
	Resources []*ResourceResult `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Revision holds the git commit SHA of the sync
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// Source records the application source information of the sync, used for comparing auto-sync
	Source ApplicationSource `json:"source" protobuf:"bytes,3,opt,name=source"`
}

type ResultCode string

const (
	ResultCodeSynced       ResultCode = "Synced"
	ResultCodeSyncFailed   ResultCode = "SyncFailed"
	ResultCodePruned       ResultCode = "Pruned"
	ResultCodePruneSkipped ResultCode = "PruneSkipped"
)

func (s ResultCode) Successful() bool {
	return s != ResultCodeSyncFailed
}

// ResourceResult holds the operation result details of a specific resource
type ResourceResult struct {
	Group     string         `json:"group" protobuf:"bytes,1,opt,name=group"`
	Version   string         `json:"version" protobuf:"bytes,2,opt,name=version"`
	Kind      string         `json:"kind" protobuf:"bytes,3,opt,name=kind"`
	Namespace string         `json:"namespace" protobuf:"bytes,4,opt,name=namespace"`
	Name      string         `json:"name" protobuf:"bytes,5,opt,name=name"`
	Status    ResultCode     `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	Message   string         `json:"message,omitempty" protobuf:"bytes,7,opt,name=message"`
	HookType  HookType       `json:"hookType,omitempty" protobuf:"bytes,8,opt,name=hookType"`
	HookPhase OperationPhase `json:"hookPhase,omitempty" protobuf:"bytes,9,opt,name=hookPhase"`
}

func (r *ResourceResult) IsHook() bool {
	return r.HookType != ""
}

func (r *ResourceResult) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

// RevisionHistory contains information relevant to an application deployment
type RevisionHistory struct {
	Revision   string            `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	DeployedAt metav1.Time       `json:"deployedAt" protobuf:"bytes,4,opt,name=deployedAt"`
	ID         int64             `json:"id" protobuf:"bytes,5,opt,name=id"`
	Source     ApplicationSource `json:"source" protobuf:"bytes,6,opt,name=source"`
}

// ApplicationWatchEvent contains information about application change.
type ApplicationWatchEvent struct {
	Type watch.EventType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=k8s.io/apimachinery/pkg/watch.EventType"`

	// Application is:
	//  * If Type is Added or Modified: the new state of the object.
	//  * If Type is Deleted: the state of the object immediately before deletion.
	//  * If Type is Error: *api.Status is recommended; other types may make sense
	//    depending on context.
	Application Application `json:"application" protobuf:"bytes,2,opt,name=application"`
}

// ApplicationList is list of Application resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Application `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ComponentParameter contains information about component parameter value
type ComponentParameter struct {
	Component string `json:"component,omitempty" protobuf:"bytes,1,opt,name=component"`
	Name      string `json:"name" protobuf:"bytes,2,opt,name=name"`
	Value     string `json:"value" protobuf:"bytes,3,opt,name=value"`
}

// SyncStatusCode is a type which represents possible comparison results
type SyncStatusCode string

// Possible comparison results
const (
	SyncStatusCodeUnknown   SyncStatusCode = "Unknown"
	SyncStatusCodeSynced    SyncStatusCode = "Synced"
	SyncStatusCodeOutOfSync SyncStatusCode = "OutOfSync"
)

// ApplicationConditionType represents type of application condition. Type name has following convention:
// prefix "Error" means error condition
// prefix "Warning" means warning condition
// prefix "Info" means informational condition
type ApplicationConditionType = string

const (
	// ApplicationConditionDeletionError indicates that controller failed to delete application
	ApplicationConditionDeletionError = "DeletionError"
	// ApplicationConditionInvalidSpecError indicates that application source is invalid
	ApplicationConditionInvalidSpecError = "InvalidSpecError"
	// ApplicationConditionComparisonError indicates controller failed to compare application state
	ApplicationConditionComparisonError = "ComparisonError"
	// ApplicationConditionSyncError indicates controller failed to automatically sync the application
	ApplicationConditionSyncError = "SyncError"
	// ApplicationConditionUnknownError indicates an unknown controller error
	ApplicationConditionUnknownError = "UnknownError"
	// ApplicationConditionSharedResourceWarning indicates that controller detected resources which belongs to more than one application
	ApplicationConditionSharedResourceWarning = "SharedResourceWarning"
	// ApplicationConditionRepeatedResourceWarning indicates that application source has resource with same Group, Kind, Name, Namespace multiple times
	ApplicationConditionRepeatedResourceWarning = "RepeatedResourceWarning"
)

// ApplicationCondition contains details about current application condition
type ApplicationCondition struct {
	// Type is an application condition type
	Type ApplicationConditionType `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Message contains human-readable message indicating details about condition
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
}

// ComparedTo contains application source and target which was used for resources comparison
type ComparedTo struct {
	Source      ApplicationSource      `json:"source" protobuf:"bytes,1,opt,name=source"`
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,opt,name=destination"`
}

// SyncStatus is a comparison result of application spec and deployed application.
type SyncStatus struct {
	Status     SyncStatusCode `json:"status" protobuf:"bytes,1,opt,name=status,casttype=SyncStatusCode"`
	ComparedTo ComparedTo     `json:"comparedTo" protobuf:"bytes,2,opt,name=comparedTo"`
	Revision   string         `json:"revision" protobuf:"bytes,3,opt,name=revision"`
}

type HealthStatus struct {
	Status  HealthStatusCode `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	Message string           `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}

type HealthStatusCode = string

const (
	HealthStatusUnknown     HealthStatusCode = "Unknown"
	HealthStatusProgressing HealthStatusCode = "Progressing"
	HealthStatusHealthy     HealthStatusCode = "Healthy"
	HealthStatusSuspended   HealthStatusCode = "Suspended"
	HealthStatusDegraded    HealthStatusCode = "Degraded"
	HealthStatusMissing     HealthStatusCode = "Missing"
)

// InfoItem contains human readable information about object
type InfoItem struct {
	// Name is a human readable title for this piece of information.
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is human readable content.
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

// ResourceNetworkingInfo holds networking resource related information
type ResourceNetworkingInfo struct {
	TargetLabels map[string]string        `json:"targetLabels,omitempty" protobuf:"bytes,1,opt,name=targetLabels"`
	TargetRefs   []ResourceRef            `json:"targetRefs,omitempty" protobuf:"bytes,2,opt,name=targetRefs"`
	Labels       map[string]string        `json:"labels,omitempty" protobuf:"bytes,3,opt,name=labels"`
	Ingress      []v1.LoadBalancerIngress `json:"ingress,omitempty" protobuf:"bytes,4,opt,name=ingress"`
	// ExternalURLs holds list of URLs which should be available externally. List is populated for ingress resources using rules hostnames.
	ExternalURLs []string `json:"externalURLs,omitempty" protobuf:"bytes,5,opt,name=externalURLs"`
}

// ApplicationTree holds nodes which belongs to the application
type ApplicationTree struct {
	Nodes []ResourceNode `json:"nodes,omitempty" protobuf:"bytes,1,rep,name=nodes"`
}

type ApplicationSummary struct {
	// ExternalURLs holds all external URLs of application child resources.
	ExternalURLs []string `json:"externalURLs,omitempty" protobuf:"bytes,1,opt,name=externalURLs"`
	// Images holds all images of application child resources.
	Images []string `json:"images,omitempty" protobuf:"bytes,2,opt,name=images"`
}

func (t *ApplicationTree) FindNode(group string, kind string, namespace string, name string) *ResourceNode {
	for _, n := range t.Nodes {
		if n.Group == group && n.Kind == kind && n.Namespace == namespace && n.Name == name {
			return &n
		}
	}
	return nil
}

func (t *ApplicationTree) GetSummary() ApplicationSummary {
	urlsSet := make(map[string]bool)
	imagesSet := make(map[string]bool)
	for _, node := range t.Nodes {
		if node.NetworkingInfo != nil {
			for _, url := range node.NetworkingInfo.ExternalURLs {
				urlsSet[url] = true
			}
		}
		for _, image := range node.Images {
			imagesSet[image] = true
		}
	}
	urls := make([]string, 0)
	for url := range urlsSet {
		urls = append(urls, url)
	}
	images := make([]string, 0)
	for image := range imagesSet {
		images = append(images, image)
	}
	return ApplicationSummary{ExternalURLs: urls, Images: images}
}

// ResourceRef includes fields which unique identify resource
type ResourceRef struct {
	Group     string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Version   string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	Kind      string `json:"kind,omitempty" protobuf:"bytes,3,opt,name=kind"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	Name      string `json:"name,omitempty" protobuf:"bytes,5,opt,name=name"`
}

// ResourceNode contains information about live resource and its children
type ResourceNode struct {
	ResourceRef     `json:",inline" protobuf:"bytes,1,opt,name=resourceRef"`
	ParentRefs      []ResourceRef           `json:"parentRefs,omitempty" protobuf:"bytes,2,opt,name=parentRefs"`
	Info            []InfoItem              `json:"info,omitempty" protobuf:"bytes,3,opt,name=info"`
	NetworkingInfo  *ResourceNetworkingInfo `json:"networkingInfo,omitempty" protobuf:"bytes,4,opt,name=networkingInfo"`
	ResourceVersion string                  `json:"resourceVersion,omitempty" protobuf:"bytes,5,opt,name=resourceVersion"`
	Images          []string                `json:"images,omitempty" protobuf:"bytes,6,opt,name=images"`
	Health          *HealthStatus           `json:"health,omitempty" protobuf:"bytes,7,opt,name=health"`
}

func (n *ResourceNode) GroupKindVersion() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   n.Group,
		Version: n.Version,
		Kind:    n.Kind,
	}
}

// ResourceStatus holds the current sync and health status of a resource
type ResourceStatus struct {
	Group     string         `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Version   string         `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	Kind      string         `json:"kind,omitempty" protobuf:"bytes,3,opt,name=kind"`
	Namespace string         `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	Name      string         `json:"name,omitempty" protobuf:"bytes,5,opt,name=name"`
	Status    SyncStatusCode `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	Health    *HealthStatus  `json:"health,omitempty" protobuf:"bytes,7,opt,name=health"`
	Hook      bool           `json:"hook,omitempty" protobuf:"bytes,8,opt,name=hook"`
}

func (r *ResourceStatus) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: r.Group, Version: r.Version, Kind: r.Kind}
}

// ResourceDiff holds the diff of a live and target resource object
type ResourceDiff struct {
	Group       string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind        string `json:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
	Namespace   string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	Name        string `json:"name,omitempty" protobuf:"bytes,4,opt,name=name"`
	TargetState string `json:"targetState,omitempty" protobuf:"bytes,5,opt,name=targetState"`
	LiveState   string `json:"liveState,omitempty" protobuf:"bytes,6,opt,name=liveState"`
	Diff        string `json:"diff,omitempty" protobuf:"bytes,7,opt,name=diff"`
}

// ConnectionStatus represents connection status
type ConnectionStatus = string

const (
	ConnectionStatusSuccessful = "Successful"
	ConnectionStatusFailed     = "Failed"
)

// ConnectionState contains information about remote resource connection state
type ConnectionState struct {
	Status     ConnectionStatus `json:"status" protobuf:"bytes,1,opt,name=status"`
	Message    string           `json:"message" protobuf:"bytes,2,opt,name=message"`
	ModifiedAt *metav1.Time     `json:"attemptedAt" protobuf:"bytes,3,opt,name=attemptedAt"`
}

// Cluster is the definition of a cluster resource
type Cluster struct {
	// Server is the API server URL of the Kubernetes cluster
	Server string `json:"server" protobuf:"bytes,1,opt,name=server"`
	// Name of the cluster. If omitted, will use the server address
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Config holds cluster information for connecting to a cluster
	Config ClusterConfig `json:"config" protobuf:"bytes,3,opt,name=config"`
	// ConnectionState contains information about cluster connection state
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,4,opt,name=connectionState"`
}

// ClusterList is a collection of Clusters.
type ClusterList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AWSAuthConfig is an AWS IAM authentication configuration
type AWSAuthConfig struct {
	// ClusterName contains AWS cluster name
	ClusterName string `json:"clusterName,omitempty" protobuf:"bytes,1,opt,name=clusterName"`

	// RoleARN contains optional role ARN. If set then AWS IAM Authenticator assume a role to perform cluster operations instead of the default AWS credential provider chain.
	RoleARN string `json:"roleARN,omitempty" protobuf:"bytes,2,opt,name=roleARN"`
}

// ClusterConfig is the configuration attributes. This structure is subset of the go-client
// rest.Config with annotations added for marshalling.
type ClusterConfig struct {
	// Server requires Basic authentication
	Username string `json:"username,omitempty" protobuf:"bytes,1,opt,name=username"`
	Password string `json:"password,omitempty" protobuf:"bytes,2,opt,name=password"`

	// Server requires Bearer authentication. This client will not attempt to use
	// refresh tokens for an OAuth2 flow.
	// TODO: demonstrate an OAuth2 compatible client.
	BearerToken string `json:"bearerToken,omitempty" protobuf:"bytes,3,opt,name=bearerToken"`

	// TLSClientConfig contains settings to enable transport layer security
	TLSClientConfig `json:"tlsClientConfig" protobuf:"bytes,4,opt,name=tlsClientConfig"`

	// AWSAuthConfig contains IAM authentication configuration
	AWSAuthConfig *AWSAuthConfig `json:"awsAuthConfig,omitempty" protobuf:"bytes,5,opt,name=awsAuthConfig"`
}

// TLSClientConfig contains settings to enable transport layer security
type TLSClientConfig struct {
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool `json:"insecure" protobuf:"bytes,1,opt,name=insecure"`
	// ServerName is passed to the server for SNI and is used in the client to check server
	// certificates against. If ServerName is empty, the hostname used to contact the
	// server is used.
	ServerName string `json:"serverName,omitempty" protobuf:"bytes,2,opt,name=serverName"`
	// CertData holds PEM-encoded bytes (typically read from a client certificate file).
	// CertData takes precedence over CertFile
	CertData []byte `json:"certData,omitempty" protobuf:"bytes,3,opt,name=certData"`
	// KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
	// KeyData takes precedence over KeyFile
	KeyData []byte `json:"keyData,omitempty" protobuf:"bytes,4,opt,name=keyData"`
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte `json:"caData,omitempty" protobuf:"bytes,5,opt,name=caData"`
}

type HelmRepository struct {
	URL      string `json:"url" protobuf:"bytes,1,opt,name=url"`
	Name     string `json:"name" protobuf:"bytes,2,opt,name=name"`
	CAData   []byte `json:"caData,omitempty" protobuf:"bytes,3,opt,name=caData"`
	CertData []byte `json:"certData,omitempty" protobuf:"bytes,4,opt,name=certData"`
	KeyData  []byte `json:"keyData,omitempty" protobuf:"bytes,5,opt,name=keyData"`
	Username string `json:"username,omitempty" protobuf:"bytes,6,opt,name=username"`
	Password string `json:"password,omitempty" protobuf:"bytes,7,opt,name=password"`
}

// ResourceOverride holds configuration to customize resource diffing and health assessment
type ResourceOverride struct {
	HealthLua         string `json:"health.lua,omitempty" protobuf:"bytes,1,opt,name=healthLua"`
	Actions           string `json:"actions,omitempty" protobuf:"bytes,3,opt,name=actions"`
	IgnoreDifferences string `json:"ignoreDifferences,omitempty" protobuf:"bytes,2,opt,name=ignoreDifferences"`
}

func (o *ResourceOverride) GetActions() (ResourceActions, error) {
	var actions ResourceActions
	err := yaml.Unmarshal([]byte(o.Actions), &actions)
	if err != nil {
		return actions, err
	}
	return actions, nil
}

type ResourceActions struct {
	ActionDiscoveryLua string                     `json:"discovery.lua,omitempty" yaml:"discovery.lua,omitempty" protobuf:"bytes,1,opt,name=actionDiscoveryLua"`
	Definitions        []ResourceActionDefinition `json:"definitions,omitEmpty" protobuf:"bytes,2,rep,name=definitions"`
}

type ResourceActionDefinition struct {
	Name      string `json:"name" protobuf:"bytes,1,opt,name=name"`
	ActionLua string `json:"action.lua" yaml:"action.lua" protobuf:"bytes,2,opt,name=actionLua"`
}

type ResourceAction struct {
	Name   string                `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Params []ResourceActionParam `json:"params,omitempty" protobuf:"bytes,2,rep,name=params"`
}

type ResourceActionParam struct {
	Name    string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Value   string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	Type    string `json:"type,omitempty" protobuf:"bytes,3,opt,name=type"`
	Default string `json:"default,omitempty" protobuf:"bytes,4,opt,name=default"`
}

// Repository is a Git repository holding application configurations
type Repository struct {
	Repo                  string          `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	Username              string          `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	Password              string          `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	SSHPrivateKey         string          `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	ConnectionState       ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,5,opt,name=connectionState"`
	InsecureIgnoreHostKey bool            `json:"insecureIgnoreHostKey,omitempty" protobuf:"bytes,6,opt,name=insecureIgnoreHostKey"`
}

func (m *Repository) HasCredentials() bool {
	return m.Username != "" || m.Password != "" || m.SSHPrivateKey != "" || m.InsecureIgnoreHostKey
}

func (m *Repository) CopyCredentialsFrom(source Repository) {
	m.Username = source.Username
	m.Password = source.Password
	m.SSHPrivateKey = source.SSHPrivateKey
	m.InsecureIgnoreHostKey = source.InsecureIgnoreHostKey
}

// RepositoryList is a collection of Repositories.
type RepositoryList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Repository `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AppProjectList is list of AppProject resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Items           []AppProject `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// AppProject provides a logical grouping of applications, providing controls for:
// * where the apps may deploy to (cluster whitelist)
// * what may be deployed (repository whitelist, resource whitelist/blacklist)
// * who can access these applications (roles, OIDC group claims bindings)
// * and what they can do (RBAC policies)
// * automation access to these roles (JWT tokens)
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppProjectSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// AppProjectSpec is the specification of an AppProject
type AppProjectSpec struct {
	// SourceRepos contains list of git repository URLs which can be used for deployment
	SourceRepos []string `json:"sourceRepos,omitempty" protobuf:"bytes,1,name=sourceRepos"`
	// Destinations contains list of destinations available for deployment
	Destinations []ApplicationDestination `json:"destinations,omitempty" protobuf:"bytes,2,name=destination"`
	// Description contains optional project description
	Description string `json:"description,omitempty" protobuf:"bytes,3,opt,name=description"`
	// Roles are user defined RBAC roles associated with this project
	Roles []ProjectRole `json:"roles,omitempty" protobuf:"bytes,4,rep,name=roles"`
	// ClusterResourceWhitelist contains list of whitelisted cluster level resources
	ClusterResourceWhitelist []metav1.GroupKind `json:"clusterResourceWhitelist,omitempty" protobuf:"bytes,5,opt,name=clusterResourceWhitelist"`
	// NamespaceResourceBlacklist contains list of blacklisted namespace level resources
	NamespaceResourceBlacklist []metav1.GroupKind `json:"namespaceResourceBlacklist,omitempty" protobuf:"bytes,6,opt,name=namespaceResourceBlacklist"`
}

func (d AppProjectSpec) DestinationClusters() []string {
	servers := make([]string, 0)

	for _, d := range d.Destinations {
		servers = append(servers, d.Server)
	}

	return servers
}

// ProjectRole represents a role that has access to a project
type ProjectRole struct {
	// Name is a name for this role
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Description is a description of the role
	Description string `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
	// Policies Stores a list of casbin formated strings that define access policies for the role in the project
	Policies []string `json:"policies,omitempty" protobuf:"bytes,3,rep,name=policies"`
	// JWTTokens are a list of generated JWT tokens bound to this role
	JWTTokens []JWTToken `json:"jwtTokens,omitempty" protobuf:"bytes,4,rep,name=jwtTokens"`
	// Groups are a list of OIDC group claims bound to this role
	Groups []string `json:"groups,omitempty" protobuf:"bytes,5,rep,name=groups"`
}

// JWTToken holds the issuedAt and expiresAt values of a token
type JWTToken struct {
	IssuedAt  int64 `json:"iat" protobuf:"int64,1,opt,name=iat"`
	ExpiresAt int64 `json:"exp,omitempty" protobuf:"int64,2,opt,name=exp"`
}

// Command holds binary path and arguments list
type Command struct {
	Command []string `json:"command,omitempty" protobuf:"bytes,1,name=command"`
	Args    []string `json:"args,omitempty" protobuf:"bytes,2,rep,name=args"`
}

// ConfigManagementPlugin contains config management plugin configuration
type ConfigManagementPlugin struct {
	Name     string   `json:"name" protobuf:"bytes,1,name=name"`
	Init     *Command `json:"init,omitempty" protobuf:"bytes,2,name=init"`
	Generate Command  `json:"generate" protobuf:"bytes,3,name=generate"`
}

// ProjectPoliciesString returns Casbin formated string of a project's policies for each role
func (proj *AppProject) ProjectPoliciesString() string {
	var policies []string
	for _, role := range proj.Spec.Roles {
		projectPolicy := fmt.Sprintf("p, proj:%s:%s, projects, get, %s, allow", proj.ObjectMeta.Name, role.Name, proj.ObjectMeta.Name)
		policies = append(policies, projectPolicy)
		policies = append(policies, role.Policies...)
		for _, groupName := range role.Groups {
			policies = append(policies, fmt.Sprintf("g, %s, proj:%s:%s", groupName, proj.ObjectMeta.Name, role.Name))
		}
	}
	return strings.Join(policies, "\n")
}

func (app *Application) getFinalizerIndex(name string) int {
	for i, finalizer := range app.Finalizers {
		if finalizer == name {
			return i
		}
	}
	return -1
}

// CascadedDeletion indicates if resources finalizer is set and controller should delete app resources before deleting app
func (app *Application) CascadedDeletion() bool {
	return app.getFinalizerIndex(common.ResourcesFinalizerName) > -1
}

func (app *Application) IsRefreshRequested() (RefreshType, bool) {
	refreshType := RefreshTypeNormal
	annotations := app.GetAnnotations()
	if annotations == nil {
		return refreshType, false
	}

	typeStr, ok := annotations[common.AnnotationKeyRefresh]
	if !ok {
		return refreshType, false
	}

	if typeStr == string(RefreshTypeHard) {
		refreshType = RefreshTypeHard
	}

	return refreshType, true
}

// SetCascadedDeletion sets or remove resources finalizer
func (app *Application) SetCascadedDeletion(prune bool) {
	index := app.getFinalizerIndex(common.ResourcesFinalizerName)
	if prune != (index > -1) {
		if index > -1 {
			app.Finalizers[index] = app.Finalizers[len(app.Finalizers)-1]
			app.Finalizers = app.Finalizers[:len(app.Finalizers)-1]
		} else {
			app.Finalizers = append(app.Finalizers, common.ResourcesFinalizerName)
		}
	}
}

// GetErrorConditions returns list of application error conditions
func (status *ApplicationStatus) GetErrorConditions() []ApplicationCondition {
	result := make([]ApplicationCondition, 0)
	for i := range status.Conditions {
		condition := status.Conditions[i]
		if condition.IsError() {
			result = append(result, condition)
		}
	}
	return result
}

// IsError returns true if condition is error condition
func (condition *ApplicationCondition) IsError() bool {
	return strings.HasSuffix(condition.Type, "Error")
}

// Equals compares two instances of ApplicationSource and return true if instances are equal.
func (source *ApplicationSource) Equals(other ApplicationSource) bool {
	return reflect.DeepEqual(*source, other)
}

func (source *ApplicationSource) ExplicitType() (*ApplicationSourceType, error) {
	var appTypes []ApplicationSourceType
	if source.Kustomize != nil {
		appTypes = append(appTypes, ApplicationSourceTypeKustomize)
	}
	if source.Helm != nil {
		appTypes = append(appTypes, ApplicationSourceTypeHelm)
	}
	if source.Ksonnet != nil {
		appTypes = append(appTypes, ApplicationSourceTypeKsonnet)
	}
	if source.Directory != nil {
		appTypes = append(appTypes, ApplicationSourceTypeDirectory)
	}
	if source.Plugin != nil {
		appTypes = append(appTypes, ApplicationSourceTypePlugin)
	}
	if len(appTypes) == 0 {
		return nil, nil
	}
	if len(appTypes) > 1 {
		typeNames := make([]string, len(appTypes))
		for i := range appTypes {
			typeNames[i] = string(appTypes[i])
		}
		return nil, fmt.Errorf("multiple application sources defined: %s", strings.Join(typeNames, ","))
	}
	appType := appTypes[0]
	return &appType, nil
}

// Equals compares two instances of ApplicationDestination and return true if instances are equal.
func (dest ApplicationDestination) Equals(other ApplicationDestination) bool {
	return reflect.DeepEqual(dest, other)
}

// GetProject returns the application's project. This is preferred over spec.Project which may be empty
func (spec ApplicationSpec) GetProject() string {
	if spec.Project == "" {
		return common.DefaultAppProjectName
	}
	return spec.Project
}

func isResourceInList(res metav1.GroupKind, list []metav1.GroupKind) bool {
	for _, item := range list {
		ok, err := filepath.Match(item.Kind, res.Kind)
		if ok && err == nil {
			ok, err = filepath.Match(item.Group, res.Group)
			if ok && err == nil {
				return true
			}
		}
	}
	return false
}

// IsResourcePermitted validates if the given resource group/kind is permitted to be deployed in the project
func (proj AppProject) IsResourcePermitted(res metav1.GroupKind, namespaced bool) bool {
	if namespaced {
		return !isResourceInList(res, proj.Spec.NamespaceResourceBlacklist)
	} else {
		return isResourceInList(res, proj.Spec.ClusterResourceWhitelist)
	}
}

func globMatch(pattern string, val string) bool {
	if pattern == "*" {
		return true
	}
	if ok, err := filepath.Match(pattern, val); ok && err == nil {
		return true
	}
	return false
}

// IsSourcePermitted validates if the provided application's source is a one of the allowed sources for the project.
func (proj AppProject) IsSourcePermitted(src ApplicationSource) bool {
	srcNormalized := git.NormalizeGitURL(src.RepoURL)
	for _, repoURL := range proj.Spec.SourceRepos {
		normalized := git.NormalizeGitURL(repoURL)
		if globMatch(normalized, srcNormalized) {
			return true
		}
	}
	return false
}

// IsDestinationPermitted validates if the provided application's destination is one of the allowed destinations for the project
func (proj AppProject) IsDestinationPermitted(dst ApplicationDestination) bool {
	for _, item := range proj.Spec.Destinations {
		if globMatch(item.Server, dst.Server) && globMatch(item.Namespace, dst.Namespace) {
			return true
		}
	}
	return false
}

// RESTConfig returns a go-client REST config from cluster
func (c *Cluster) RESTConfig() *rest.Config {
	var config *rest.Config
	var err error
	if c.Server == common.KubernetesInternalAPIServerAddr && os.Getenv(common.EnvVarFakeInClusterConfig) == "true" {
		config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	} else if c.Server == common.KubernetesInternalAPIServerAddr && c.Config.Username == "" && c.Config.Password == "" && c.Config.BearerToken == "" {
		config, err = rest.InClusterConfig()
	} else {
		tlsClientConfig := rest.TLSClientConfig{
			Insecure:   c.Config.TLSClientConfig.Insecure,
			ServerName: c.Config.TLSClientConfig.ServerName,
			CertData:   c.Config.TLSClientConfig.CertData,
			KeyData:    c.Config.TLSClientConfig.KeyData,
			CAData:     c.Config.TLSClientConfig.CAData,
		}
		if c.Config.AWSAuthConfig != nil {
			args := []string{"token", "-i", c.Config.AWSAuthConfig.ClusterName}
			if c.Config.AWSAuthConfig.RoleARN != "" {
				args = append(args, "-r", c.Config.AWSAuthConfig.RoleARN)
			}
			config = &rest.Config{
				Host:            c.Server,
				TLSClientConfig: tlsClientConfig,
				ExecProvider: &api.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1alpha1",
					Command:    "aws-iam-authenticator",
					Args:       args,
				},
			}
		} else {
			config = &rest.Config{
				Host:            c.Server,
				Username:        c.Config.Username,
				Password:        c.Config.Password,
				BearerToken:     c.Config.BearerToken,
				TLSClientConfig: tlsClientConfig,
			}
		}
	}
	if err != nil {
		panic("Unable to create K8s REST config")
	}
	config.QPS = common.K8sClientConfigQPS
	config.Burst = common.K8sClientConfigBurst
	return config
}

func UnmarshalToUnstructured(resource string) (*unstructured.Unstructured, error) {
	if resource == "" || resource == "null" {
		return nil, nil
	}
	var obj unstructured.Unstructured
	err := json.Unmarshal([]byte(resource), &obj)
	if err != nil {
		return nil, err
	}
	return &obj, nil
}

func (r ResourceDiff) LiveObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.LiveState)
}

func (r ResourceDiff) TargetObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.TargetState)
}
