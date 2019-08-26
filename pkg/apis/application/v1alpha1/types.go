package v1alpha1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	"github.com/argoproj/argo-cd/util/rbac"
)

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applications,shortName=app;apps
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ApplicationSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            ApplicationStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
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
	// Infos contains a list of useful information (URLs, email addresses, and plain text) that relates to the application
	Info []Info `json:"info,omitempty" protobuf:"bytes,6,name=info"`
}

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group        string   `json:"group" protobuf:"bytes,1,opt,name=group"`
	Kind         string   `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name         string   `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
	Namespace    string   `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	JSONPointers []string `json:"jsonPointers" protobuf:"bytes,5,opt,name=jsonPointers"`
}

type EnvEntry struct {
	// the name, usually uppercase
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// the value
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
}

func (a *EnvEntry) IsZero() bool {
	return a == nil || a.Name == "" && a.Value == ""
}

type Env []*EnvEntry

func (e Env) IsZero() bool {
	return len(e) == 0
}

func (e Env) Environ() []string {
	var environ []string
	for _, item := range e {
		if !item.IsZero() {
			environ = append(environ, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
	}
	return environ
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the git repository URL of the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the repository containing a
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`
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

func (a *ApplicationSource) IsZero() bool {
	return a == nil ||
		a.RepoURL == "" &&
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
	// The Helm release name. If omitted it will use the application name
	ReleaseName string `json:"releaseName,omitempty" protobuf:"bytes,3,opt,name=releaseName"`
}

// HelmParameter is a parameter to a helm template
type HelmParameter struct {
	// Name is the name of the helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is the value for the helm parameter
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// ForceString determines whether to tell Helm to interpret booleans and numbers as strings
	ForceString bool `json:"forceString,omitempty" protobuf:"bytes,3,opt,name=forceString"`
}

func (h *ApplicationSourceHelm) IsZero() bool {
	return h == nil || (h.ReleaseName == "") && len(h.ValueFiles) == 0 && len(h.Parameters) == 0
}

type KustomizeImage string

func (i KustomizeImage) delim() string {
	for _, d := range []string{"=", ":", "@"} {
		if strings.Contains(string(i), d) {
			return d
		}
	}
	return ":"
}

// if the image name matches (i.e. up to the first delimiter)
func (i KustomizeImage) Match(j KustomizeImage) bool {
	delim := j.delim()
	if !strings.Contains(string(j), delim) {
		return false
	}
	return strings.HasPrefix(string(i), strings.Split(string(j), delim)[0])
}

type KustomizeImages []KustomizeImage

// find the image or -1
func (images KustomizeImages) Find(image KustomizeImage) int {
	for i, a := range images {
		if a.Match(image) {
			return i
		}
	}
	return -1
}

// ApplicationSourceKustomize holds kustomize specific options
type ApplicationSourceKustomize struct {
	// NamePrefix is a prefix appended to resources for kustomize apps
	NamePrefix string `json:"namePrefix,omitempty" protobuf:"bytes,1,opt,name=namePrefix"`
	// Images are kustomize image overrides
	Images KustomizeImages `json:"images,omitempty" protobuf:"bytes,3,opt,name=images"`
	// CommonLabels adds additional kustomize commonLabels
	CommonLabels map[string]string `json:"commonLabels,omitempty" protobuf:"bytes,4,opt,name=commonLabels"`
}

func (k *ApplicationSourceKustomize) IsZero() bool {
	return k == nil || k.NamePrefix == "" && len(k.Images) == 0 && len(k.CommonLabels) == 0
}

// either updates or adds the images
func (k *ApplicationSourceKustomize) MergeImage(image KustomizeImage) {
	i := k.Images.Find(image)
	if i >= 0 {
		k.Images[i] = image
	} else {
		k.Images = append(k.Images, image)
	}
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
	return j == nil || len(j.ExtVars) == 0 && len(j.TLAs) == 0
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
	return k == nil || k.Environment == "" && len(k.Parameters) == 0
}

type ApplicationSourceDirectory struct {
	Recurse bool                     `json:"recurse,omitempty" protobuf:"bytes,1,opt,name=recurse"`
	Jsonnet ApplicationSourceJsonnet `json:"jsonnet,omitempty" protobuf:"bytes,2,opt,name=jsonnet"`
}

func (d *ApplicationSourceDirectory) IsZero() bool {
	return d == nil || !d.Recurse && d.Jsonnet.IsZero()
}

// ApplicationSourcePlugin holds config management plugin specific options
type ApplicationSourcePlugin struct {
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Env  `json:"env,omitempty" protobuf:"bytes,2,opt,name=env"`
}

func (c *ApplicationSourcePlugin) IsZero() bool {
	return c == nil || c.Name == "" && c.Env.IsZero()
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
	ReconciledAt   *metav1.Time           `json:"reconciledAt,omitempty" protobuf:"bytes,6,opt,name=reconciledAt"`
	OperationState *OperationState        `json:"operationState,omitempty" protobuf:"bytes,7,opt,name=operationState"`
	ObservedAt     *metav1.Time           `json:"observedAt,omitempty" protobuf:"bytes,8,opt,name=observedAt"`
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
	// Manifests is an optional field that overrides sync source with a local directory for development
	Manifests []string `json:"manifests,omitempty" protobuf:"bytes,8,opt,name=manifests"`
}

func (o *SyncOperation) IsApplyStrategy() bool {
	return o.SyncStrategy != nil && o.SyncStrategy.Apply != nil
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

type Info struct {
	Name  string `json:"name" protobuf:"bytes,1,name=name"`
	Value string `json:"value" protobuf:"bytes,2,name=value"`
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
	// SelfHeal enables auto-syncing if  (default: false)
	SelfHeal bool `json:"selfHeal,omitempty" protobuf:"bytes,2,opt,name=selfHeal"`
}

// SyncStrategy controls the manner in which a sync is performed
type SyncStrategy struct {
	// Apply wil perform a `kubectl apply` to perform the sync.
	Apply *SyncStrategyApply `json:"apply,omitempty" protobuf:"bytes,1,opt,name=apply"`
	// Hook will submit any referenced resources to perform the sync. This is the default strategy
	Hook *SyncStrategyHook `json:"hook,omitempty" protobuf:"bytes,2,opt,name=hook"`
}

func (m *SyncStrategy) Force() bool {
	if m == nil {
		return false
	} else if m.Apply != nil {
		return m.Apply.Force
	} else if m.Hook != nil {
		return m.Hook.Force
	} else {
		return false
	}
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
	SyncStrategyApply `json:",inline" protobuf:"bytes,1,opt,name=syncStrategyApply"`
}

type HookType string

const (
	HookTypePreSync  HookType = "PreSync"
	HookTypeSync     HookType = "Sync"
	HookTypePostSync HookType = "PostSync"
	HookTypeSkip     HookType = "Skip"
	HookTypeSyncFail HookType = "SyncFail"

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

// data about a specific revision within a repo
type RevisionMetadata struct {
	// who authored this revision,
	// typically their name and email, e.g. "John Doe <john_doe@my-company.com>",
	// but might not match this example
	Author string `json:"author" protobuf:"bytes,1,opt,name=author"`
	// when the revision was authored
	Date metav1.Time `json:"date" protobuf:"bytes,2,opt,name=date"`
	// tags on the revision,
	// note - tags can move from one revision to another
	Tags []string `json:"tags" protobuf:"bytes,3,opt,name=tags"`
	// the message associated with the revision,
	// probably the commit message,
	// this is truncated to the first newline or 64 characters (which ever comes first)
	Message string `json:"message" protobuf:"bytes,4,opt,name=message"`
}

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	// Resources holds the sync result of each individual resource
	Resources ResourceResults `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Revision holds the git commit SHA of the sync
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// Source records the application source information of the sync, used for comparing auto-sync
	Source ApplicationSource `json:"source,omitempty" protobuf:"bytes,3,opt,name=source"`
}

type ResultCode string

const (
	ResultCodeSynced       ResultCode = "Synced"
	ResultCodeSyncFailed   ResultCode = "SyncFailed"
	ResultCodePruned       ResultCode = "Pruned"
	ResultCodePruneSkipped ResultCode = "PruneSkipped"
)

type SyncPhase = string

const (
	SyncPhasePreSync  = "PreSync"
	SyncPhaseSync     = "Sync"
	SyncPhasePostSync = "PostSync"
	SyncPhaseSyncFail = "SyncFail"
)

// ResourceResult holds the operation result details of a specific resource
type ResourceResult struct {
	Group     string `json:"group" protobuf:"bytes,1,opt,name=group"`
	Version   string `json:"version" protobuf:"bytes,2,opt,name=version"`
	Kind      string `json:"kind" protobuf:"bytes,3,opt,name=kind"`
	Namespace string `json:"namespace" protobuf:"bytes,4,opt,name=namespace"`
	Name      string `json:"name" protobuf:"bytes,5,opt,name=name"`
	// the final result of the sync, this is be empty if the resources is yet to be applied/pruned and is always zero-value for hooks
	Status ResultCode `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	// message for the last sync OR operation
	Message string `json:"message,omitempty" protobuf:"bytes,7,opt,name=message"`
	// the type of the hook, empty for non-hook resources
	HookType HookType `json:"hookType,omitempty" protobuf:"bytes,8,opt,name=hookType"`
	// the state of any operation associated with this resource OR hook
	// note: can contain values for non-hook resources
	HookPhase OperationPhase `json:"hookPhase,omitempty" protobuf:"bytes,9,opt,name=hookPhase"`
	// indicates the particular phase of the sync that this is for
	SyncPhase SyncPhase `json:"syncPhase,omitempty" protobuf:"bytes,10,opt,name=syncPhase"`
}

func (r *ResourceResult) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

type ResourceResults []*ResourceResult

func (r ResourceResults) Filter(predicate func(r *ResourceResult) bool) ResourceResults {
	results := ResourceResults{}
	for _, res := range r {
		if predicate(res) {
			results = append(results, res)
		}
	}
	return results
}
func (r ResourceResults) Find(group string, kind string, namespace string, name string, phase SyncPhase) (int, *ResourceResult) {
	for i, res := range r {
		if res.Group == group && res.Kind == kind && res.Namespace == namespace && res.Name == name && res.SyncPhase == phase {
			return i, res
		}
	}
	return 0, nil
}

func (r ResourceResults) PruningRequired() (num int) {
	for _, res := range r {
		if res.Status == ResultCodePruneSkipped {
			num++
		}
	}
	return num
}

// RevisionHistory contains information relevant to an application deployment
type RevisionHistory struct {
	Revision   string            `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	DeployedAt metav1.Time       `json:"deployedAt" protobuf:"bytes,4,opt,name=deployedAt"`
	ID         int64             `json:"id" protobuf:"bytes,5,opt,name=id"`
	Source     ApplicationSource `json:"source,omitempty" protobuf:"bytes,6,opt,name=source"`
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
	// ApplicationConditionExcludedResourceWarning indicates that application has resource which is configured to be excluded
	ApplicationConditionExcludedResourceWarning = "ExcludedResourceWarning"
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
	ComparedTo ComparedTo     `json:"comparedTo,omitempty" protobuf:"bytes,2,opt,name=comparedTo"`
	Revision   string         `json:"revision,omitempty" protobuf:"bytes,3,opt,name=revision"`
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
	UID       string `json:"uid,omitempty" protobuf:"bytes,6,opt,name=uid"`
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
	Group           string         `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Version         string         `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	Kind            string         `json:"kind,omitempty" protobuf:"bytes,3,opt,name=kind"`
	Namespace       string         `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	Name            string         `json:"name,omitempty" protobuf:"bytes,5,opt,name=name"`
	Status          SyncStatusCode `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	Health          *HealthStatus  `json:"health,omitempty" protobuf:"bytes,7,opt,name=health"`
	Hook            bool           `json:"hook,omitempty" protobuf:"bytes,8,opt,name=hook"`
	RequiresPruning bool           `json:"requiresPruning,omitempty" protobuf:"bytes,9,opt,name=requiresPruning"`
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
	Hook        bool   `json:"hook,omitempty" protobuf:"bytes,8,opt,name=hook"`
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
	// URL of the repo
	Repo string `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	// Username for authenticating at the repo server
	Username string `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	// Password for authenticating at the repo server
	Password string `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	// SSH private key data for authenticating at the repo server
	SSHPrivateKey string `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	// Current state of repository server connecting
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,5,opt,name=connectionState"`
	// InsecureIgnoreHostKey should not be used anymore, Insecure is favoured
	InsecureIgnoreHostKey bool `json:"insecureIgnoreHostKey,omitempty" protobuf:"bytes,6,opt,name=insecureIgnoreHostKey"`
	// Whether the repo is insecure
	Insecure bool `json:"insecure,omitempty" protobuf:"bytes,7,opt,name=insecure"`
	// Whether git-lfs support should be enabled for this repo
	EnableLFS bool `json:"enableLfs,omitempty" protobuf:"bytes,8,opt,name=enableLfs"`
	// TLS client cert data for authenticating at the repo server
	TLSClientCertData string `json:"tlsClientCertData,omitempty" protobuf:"bytes,9,opt,name=tlsClientCertData"`
	// TLS client cert key for authenticating at the repo server
	TLSClientCertKey string `json:"tlsClientCertKey,omitempty" protobuf:"bytes,10,opt,name=tlsClientCertKey"`
}

func (repo *Repository) IsInsecure() bool {
	return repo.InsecureIgnoreHostKey || repo.Insecure
}

func (repo *Repository) IsLFSEnabled() bool {
	return repo.EnableLFS
}

func (m *Repository) HasCredentials() bool {
	return m.Username != "" || m.Password != "" || m.SSHPrivateKey != "" || m.InsecureIgnoreHostKey
}

func (m *Repository) CopyCredentialsFrom(source *Repository) {
	if source != nil {
		m.Username = source.Username
		m.Password = source.Password
		m.SSHPrivateKey = source.SSHPrivateKey
		m.InsecureIgnoreHostKey = source.InsecureIgnoreHostKey
		m.Insecure = source.Insecure
		m.EnableLFS = source.EnableLFS
		m.TLSClientCertData = source.TLSClientCertData
		m.TLSClientCertKey = source.TLSClientCertKey
	}
}

// RepositoryList is a collection of Repositories.
type RepositoryList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Repository `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// A RepositoryCertificate is either SSH known hosts entry or TLS certificate
type RepositoryCertificate struct {
	// Name of the server the certificate is intended for
	ServerName string `json:"serverName" protobuf:"bytes,1,opt,name=serverName"`
	// Type of certificate - currently "https" or "ssh"
	CertType string `json:"certType" protobuf:"bytes,2,opt,name=certType"`
	// The sub type of the cert, i.e. "ssh-rsa"
	CertSubType string `json:"certSubType" protobuf:"bytes,3,opt,name=certSubType"`
	// Actual certificate data, protocol dependent
	CertData []byte `json:"certData" protobuf:"bytes,4,opt,name=certData"`
	// Additional certificate info (e.g. SSH fingerprint, X509 CommonName)
	CertInfo string `json:"certInfo" protobuf:"bytes,5,opt,name=certInfo"`
}

// RepositoryCertificateList is a collection of RepositoryCertificates
type RepositoryCertificateList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// List of certificates to be processed
	Items []RepositoryCertificate `json:"items" protobuf:"bytes,2,rep,name=items"`
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
// +kubebuilder:resource:path=appprojects,shortName=appproj;appprojs
type AppProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppProjectSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// GetRoleByName returns the role in a project by the name with its index
func (p *AppProject) GetRoleByName(name string) (*ProjectRole, int, error) {
	for i, role := range p.Spec.Roles {
		if name == role.Name {
			return &role, i, nil
		}
	}
	return nil, -1, fmt.Errorf("role '%s' does not exist in project '%s'", name, p.Name)
}

// GetJWTToken looks up the index of a JWTToken in a project by the issue at time
func (p *AppProject) GetJWTToken(roleName string, issuedAt int64) (*JWTToken, int, error) {
	role, _, err := p.GetRoleByName(roleName)
	if err != nil {
		return nil, -1, err
	}
	for i, token := range role.JWTTokens {
		if issuedAt == token.IssuedAt {
			return &token, i, nil
		}
	}
	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", role.Name, issuedAt, p.Name)
}

func (p *AppProject) ValidateProject() error {
	destKeys := make(map[string]bool)
	for _, dest := range p.Spec.Destinations {
		key := fmt.Sprintf("%s/%s", dest.Server, dest.Namespace)
		if _, ok := destKeys[key]; ok {
			return status.Errorf(codes.InvalidArgument, "destination '%s' already added", key)
		}
		destKeys[key] = true
	}
	srcRepos := make(map[string]bool)
	for _, src := range p.Spec.SourceRepos {
		if _, ok := srcRepos[src]; ok {
			return status.Errorf(codes.InvalidArgument, "source repository '%s' already added", src)
		}
		srcRepos[src] = true
	}

	roleNames := make(map[string]bool)
	for _, role := range p.Spec.Roles {
		if _, ok := roleNames[role.Name]; ok {
			return status.Errorf(codes.AlreadyExists, "role '%s' already exists", role.Name)
		}
		if err := validateRoleName(role.Name); err != nil {
			return err
		}
		existingPolicies := make(map[string]bool)
		for _, policy := range role.Policies {
			if _, ok := existingPolicies[policy]; ok {
				return status.Errorf(codes.AlreadyExists, "policy '%s' already exists for role '%s'", policy, role.Name)
			}
			if err := validatePolicy(p.Name, role.Name, policy); err != nil {
				return err
			}
			existingPolicies[policy] = true
		}
		existingGroups := make(map[string]bool)
		for _, group := range role.Groups {
			if _, ok := existingGroups[group]; ok {
				return status.Errorf(codes.AlreadyExists, "group '%s' already exists for role '%s'", group, role.Name)
			}
			if err := validateGroupName(group); err != nil {
				return err
			}
			existingGroups[group] = true
		}
		roleNames[role.Name] = true
	}
	if err := p.validatePolicySyntax(); err != nil {
		return err
	}
	return nil
}

// TODO: refactor to use rbacpolicy.ActionGet, rbacpolicy.ActionCreate, without import cycle
var validActions = map[string]bool{
	"get":      true,
	"create":   true,
	"update":   true,
	"delete":   true,
	"sync":     true,
	"override": true,
	"*":        true,
}

func isValidAction(action string) bool {
	return validActions[action]
}

func validatePolicy(proj string, role string, policy string) error {
	policyComponents := strings.Split(policy, ",")
	if len(policyComponents) != 6 || strings.Trim(policyComponents[0], " ") != "p" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': must be of the form: 'p, sub, res, act, obj, eft'", policy)
	}
	// subject
	subject := strings.Trim(policyComponents[1], " ")
	expectedSubject := fmt.Sprintf("proj:%s:%s", proj, role)
	if subject != expectedSubject {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': policy subject must be: '%s', not '%s'", policy, expectedSubject, subject)
	}
	// resource
	resource := strings.Trim(policyComponents[2], " ")
	if resource != "applications" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': project resource must be: 'applications', not '%s'", policy, resource)
	}
	// action
	action := strings.Trim(policyComponents[3], " ")
	if !isValidAction(action) {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': invalid action '%s'", policy, action)
	}
	// object
	object := strings.Trim(policyComponents[4], " ")
	objectRegexp, err := regexp.Compile(fmt.Sprintf(`^%s/[*\w-]+$`, proj))
	if err != nil || !objectRegexp.MatchString(object) {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': object must be of form '%s/*' or '%s/<APPNAME>', not '%s'", policy, proj, proj, object)
	}
	// effect
	effect := strings.Trim(policyComponents[5], " ")
	if effect != "allow" && effect != "deny" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': effect must be: 'allow' or 'deny'", policy)
	}
	return nil
}

var roleNameRegexp = regexp.MustCompile(`^[a-zA-Z0-9]([-_a-zA-Z0-9]*[a-zA-Z0-9])?$`)

func validateRoleName(name string) error {
	if !roleNameRegexp.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "invalid role name '%s'. Must consist of alphanumeric characters, '-' or '_', and must start and end with an alphanumeric character", name)
	}
	return nil
}

var invalidChars = regexp.MustCompile("[,\n\r\t]")

func validateGroupName(name string) error {
	if strings.TrimSpace(name) == "" {
		return status.Errorf(codes.InvalidArgument, "group '%s' is empty", name)
	}
	if invalidChars.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "group '%s' contains invalid characters", name)
	}
	return nil
}

// validatePolicySyntax verifies policy syntax is accepted by casbin
func (p *AppProject) validatePolicySyntax() error {
	err := rbac.ValidatePolicy(p.ProjectPoliciesString())
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "policy syntax error: %s", err.Error())
	}
	return nil
}

// AddGroupToRole adds an OIDC group to a role
func (p *AppProject) AddGroupToRole(roleName, group string) (bool, error) {
	role, roleIndex, err := p.GetRoleByName(roleName)
	if err != nil {
		return false, err
	}
	for _, roleGroup := range role.Groups {
		if group == roleGroup {
			return false, nil
		}
	}
	role.Groups = append(role.Groups, group)
	p.Spec.Roles[roleIndex] = *role
	return true, nil
}

// RemoveGroupFromRole removes an OIDC group from a role
func (p *AppProject) RemoveGroupFromRole(roleName, group string) (bool, error) {
	role, roleIndex, err := p.GetRoleByName(roleName)
	if err != nil {
		return false, err
	}
	for i, roleGroup := range role.Groups {
		if group == roleGroup {
			role.Groups = append(role.Groups[:i], role.Groups[i+1:]...)
			p.Spec.Roles[roleIndex] = *role
			return true, nil
		}
	}
	return false, nil
}

// NormalizePolicies normalizes the policies in the project
func (p *AppProject) NormalizePolicies() {
	for i, role := range p.Spec.Roles {
		var normalizedPolicies []string
		for _, policy := range role.Policies {
			normalizedPolicies = append(normalizedPolicies, p.normalizePolicy(policy))
		}
		p.Spec.Roles[i].Policies = normalizedPolicies
	}
}

func (p *AppProject) normalizePolicy(policy string) string {
	policyComponents := strings.Split(policy, ",")
	normalizedPolicy := ""
	for _, component := range policyComponents {
		if normalizedPolicy == "" {
			normalizedPolicy = component
		} else {
			normalizedPolicy = fmt.Sprintf("%s, %s", normalizedPolicy, strings.Trim(component, " "))
		}
	}
	return normalizedPolicy
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

// KustomizeOptions are options for kustomize to use when building manifests
type KustomizeOptions struct {
	// BuildOptions is a string of build parameters to use when calling `kustomize build`
	BuildOptions string `protobuf:"bytes,1,opt,name=buildOptions"`
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
		panic(fmt.Sprintf("Unable to create K8s REST config: %v", err))
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
