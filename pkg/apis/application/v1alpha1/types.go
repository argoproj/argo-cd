package v1alpha1

import (
	"encoding/json"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/git"
)

// SyncOperation contains sync operation details.
type SyncOperation struct {
	// Revision is the git revision in which to sync the application to
	Revision string `json:"revision,omitempty" protobuf:"bytes,1,opt,name=revision"`
	// Prune deletes resources that are no longer tracked in git
	Prune bool `json:"prune,omitempty" protobuf:"bytes,2,opt,name=prune"`
	// DryRun will perform a `kubectl apply --dry-run` without actually performing the sync
	DryRun bool `json:"dryRun,omitempty" protobuf:"bytes,3,opt,name=dryRun"`
	// SyncStrategy describes how to perform the sync
	SyncStrategy *SyncStrategy `json:"syncStrategy,omitempty" protobuf:"bytes,4,opt,name=syncStrategy"`
}

type RollbackOperation struct {
	ID     int64 `json:"id" protobuf:"bytes,1,opt,name=id"`
	Prune  bool  `json:"prune,omitempty" protobuf:"bytes,2,opt,name=prune"`
	DryRun bool  `json:"dryRun,omitempty" protobuf:"bytes,3,opt,name=dryRun"`
}

// Operation contains requested operation parameters.
type Operation struct {
	Sync     *SyncOperation     `json:"sync,omitempty" protobuf:"bytes,1,opt,name=sync"`
	Rollback *RollbackOperation `json:"rollback,omitempty" protobuf:"bytes,2,opt,name=rollback"`
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
	// RollbackResult is the result of a Rollback operation
	RollbackResult *SyncOperationResult `json:"rollbackResult,omitempty" protobuf:"bytes,5,opt,name=rollbackResult"`
	// StartedAt contains time of operation start
	StartedAt metav1.Time `json:"startedAt" protobuf:"bytes,6,opt,name=startedAt"`
	// FinishedAt contains time of operation completion
	FinishedAt *metav1.Time `json:"finishedAt" protobuf:"bytes,7,opt,name=finishedAt"`
}

// SyncStrategy indicates the
type SyncStrategy struct {
	// Apply wil perform a `kubectl apply` to perform the sync. This is the default strategy
	Apply *SyncStrategyApply `json:"apply,omitempty" protobuf:"bytes,1,opt,name=apply"`
	// Hook will submit any referenced resources to perform the sync
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

// HookStatus contains status about a hook invocation
type HookStatus struct {
	// Name is the resource name
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Kind is the resource kind
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	// APIVersion is the resource API version
	APIVersion string `json:"apiVersion" protobuf:"bytes,3,opt,name=apiVersion"`
	// Type is the type of hook (e.g. PreSync, Sync, PostSync, Skip)
	Type HookType `json:"type" protobuf:"bytes,4,opt,name=type"`
	// Status a simple, high-level summary of where the resource is in its lifecycle
	Status OperationPhase `json:"status" protobuf:"bytes,5,opt,name=status"`
	// A human readable message indicating details about why the resource is in this condition.
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	// Resources holds the sync result of each individual resource
	Resources []*ResourceDetails `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Revision holds the git commit SHA of the sync
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// Hooks contains list of hook resource statuses associated with this operation
	Hooks []*HookStatus `json:"hooks,omitempty" protobuf:"bytes,3,opt,name=hooks"`
}

type ResourceSyncStatus string

const (
	ResourceDetailsSynced          ResourceSyncStatus = "Synced"
	ResourceDetailsSyncFailed      ResourceSyncStatus = "SyncFailed"
	ResourceDetailsSyncedAndPruned ResourceSyncStatus = "SyncedAndPruned"
	ResourceDetailsPruningRequired ResourceSyncStatus = "PruningRequired"
)

func (s ResourceSyncStatus) Successful() bool {
	return s != ResourceDetailsSyncFailed
}

type ResourceDetails struct {
	Name      string             `json:"name" protobuf:"bytes,1,opt,name=name"`
	Kind      string             `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Namespace string             `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`
	Message   string             `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
	Status    ResourceSyncStatus `json:"status,omitempty" protobuf:"bytes,5,opt,name=status"`
}

// DeploymentInfo contains information relevant to an application deployment
type DeploymentInfo struct {
	Params                      []ComponentParameter `json:"params" protobuf:"bytes,1,name=params"`
	Revision                    string               `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	ComponentParameterOverrides []ComponentParameter `json:"componentParameterOverrides,omitempty" protobuf:"bytes,3,opt,name=componentParameterOverrides"`
	DeployedAt                  metav1.Time          `json:"deployedAt" protobuf:"bytes,4,opt,name=deployedAt"`
	ID                          int64                `json:"id" protobuf:"bytes,5,opt,name=id"`
}

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

// ApplicationSpec represents desired application state. Contains link to repository with application definition and additional parameters link definition revision.
type ApplicationSpec struct {
	// Source is a reference to the location ksonnet application definition
	Source ApplicationSource `json:"source" protobuf:"bytes,1,opt,name=source"`
	// Destination overrides the kubernetes server and namespace defined in the environment ksonnet app.yaml
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,name=destination"`
	// Project is a application project name. Empty name means that application belongs to 'default' project.
	Project string `json:"project" protobuf:"bytes,3,name=project"`
}

// ComponentParameter contains information about component parameter value
type ComponentParameter struct {
	Component string `json:"component,omitempty" protobuf:"bytes,1,opt,name=component"`
	Name      string `json:"name" protobuf:"bytes,2,opt,name=name"`
	Value     string `json:"value" protobuf:"bytes,3,opt,name=value"`
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the git repository URL of the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the repository containing a
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`
	// Environment is a ksonnet application environment name
	Environment string `json:"environment,omitempty" protobuf:"bytes,3,opt,name=environment"`
	// TargetRevision defines the commit, tag, or branch in which to sync the application to.
	// If omitted, will sync to HEAD
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// ComponentParameterOverrides are a list of parameter override values
	ComponentParameterOverrides []ComponentParameter `json:"componentParameterOverrides,omitempty" protobuf:"bytes,5,opt,name=componentParameterOverrides"`
	// ValuesFiles is a list of Helm values files to use when generating a template
	ValuesFiles []string `json:"valuesFiles,omitempty" protobuf:"bytes,6,opt,name=valuesFiles"`
}

// ApplicationDestination contains deployment destination information
type ApplicationDestination struct {
	// Server overrides the environment server value in the ksonnet app.yaml
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Namespace overrides the environment namespace value in the ksonnet app.yaml
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

// ComparisonStatus is a type which represents possible comparison results
type ComparisonStatus string

// Possible comparison results
const (
	ComparisonStatusUnknown   ComparisonStatus = "Unknown"
	ComparisonStatusSynced    ComparisonStatus = "Synced"
	ComparisonStatusOutOfSync ComparisonStatus = "OutOfSync"
)

// ApplicationStatus contains information about application status in target environment.
type ApplicationStatus struct {
	ComparisonResult ComparisonResult       `json:"comparisonResult" protobuf:"bytes,1,opt,name=comparisonResult"`
	History          []DeploymentInfo       `json:"history" protobuf:"bytes,2,opt,name=history"`
	Parameters       []ComponentParameter   `json:"parameters,omitempty" protobuf:"bytes,3,opt,name=parameters"`
	Health           HealthStatus           `json:"health,omitempty" protobuf:"bytes,4,opt,name=health"`
	OperationState   *OperationState        `json:"operationState,omitempty" protobuf:"bytes,5,opt,name=operationState"`
	Conditions       []ApplicationCondition `json:"conditions,omitempty" protobuf:"bytes,6,opt,name=conditions"`
}

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
	// ApplicationComparisonError indicates controller failed to compare application state
	ApplicationConditionComparisonError = "ComparisonError"
	// ApplicationConditionUnknownError indicates an unknown controller error
	ApplicationConditionUnknownError = "UnknownError"
	// ApplicationConditionSharedResourceWarning indicates that controller detected resources which belongs to more than one application
	ApplicationConditionSharedResourceWarning = "SharedResourceWarning"
)

// ApplicationCondition contains details about current application condition
type ApplicationCondition struct {
	// Type is an application condition type
	Type ApplicationConditionType `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Message contains human-readable message indicating details about condition
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
}

// ComparisonResult is a comparison result of application spec and deployed application.
type ComparisonResult struct {
	ComparedAt metav1.Time       `json:"comparedAt" protobuf:"bytes,1,opt,name=comparedAt"`
	ComparedTo ApplicationSource `json:"comparedTo" protobuf:"bytes,2,opt,name=comparedTo"`
	Status     ComparisonStatus  `json:"status" protobuf:"bytes,5,opt,name=status,casttype=ComparisonStatus"`
	Resources  []ResourceState   `json:"resources" protobuf:"bytes,6,opt,name=resources"`
}

type HealthStatus struct {
	Status        HealthStatusCode `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	StatusDetails string           `json:"statusDetails,omitempty" protobuf:"bytes,2,opt,name=statusDetails"`
}

type HealthStatusCode = string

const (
	HealthStatusUnknown     = "Unknown"
	HealthStatusProgressing = "Progressing"
	HealthStatusHealthy     = "Healthy"
	HealthStatusDegraded    = "Degraded"
	HealthStatusMissing     = "Missing"
)

// ResourceNode contains information about live resource and its children
type ResourceNode struct {
	State    string         `json:"state,omitempty" protobuf:"bytes,1,opt,name=state"`
	Children []ResourceNode `json:"children,omitempty" protobuf:"bytes,2,opt,name=children"`
}

// ResourceState holds the target state of a resource and live state of a resource
type ResourceState struct {
	TargetState        string           `json:"targetState,omitempty" protobuf:"bytes,1,opt,name=targetState"`
	LiveState          string           `json:"liveState,omitempty" protobuf:"bytes,2,opt,name=liveState"`
	Status             ComparisonStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
	ChildLiveResources []ResourceNode   `json:"childLiveResources,omitempty" protobuf:"bytes,4,opt,name=childLiveResources"`
	Health             HealthStatus     `json:"health,omitempty" protobuf:"bytes,5,opt,name=health"`
}

// ConnectionStatus represents connection status
type ConnectionStatus = string

const (
	ConnectionStatusUnknown    = "Unknown"
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
}

// TLSClientConfig contains settings to enable transport layer security
type TLSClientConfig struct {
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool `json:"insecure" protobuf:"bytes,1,opt,name=insecure"`
	// ServerName is passed to the server for SNI and is used in the client to check server
	// ceritificates against. If ServerName is empty, the hostname used to contact the
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

// Repository is a Git repository holding application configurations
type Repository struct {
	Repo            string          `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	Username        string          `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	Password        string          `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	SSHPrivateKey   string          `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,5,opt,name=connectionState"`
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

// AppProject is a definition of AppProject resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type AppProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata" protobuf:"bytes,1,opt,name=metadata"`
	Spec              AppProjectSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// ProjectPoliciesString returns Casbin formated string of a project's polcies for each role
func (proj *AppProject) ProjectPoliciesString() string {
	var policies []string
	for _, role := range proj.Spec.Roles {
		policies = append(policies, role.Policies...)
	}
	return strings.Join(policies, "\n")
}

// AppProjectSpec represents
type AppProjectSpec struct {
	// SourceRepos contains list of git repository URLs which can be used for deployment
	SourceRepos []string `json:"sourceRepos" protobuf:"bytes,1,name=sourceRepos"`

	// Destinations contains list of destinations available for deployment
	Destinations []ApplicationDestination `json:"destinations" protobuf:"bytes,2,name=destination"`

	// Description contains optional project description
	Description string `json:"description,omitempty" protobuf:"bytes,3,opt,name=description"`

	Roles []ProjectRole `json:"roles,omitempty" protobuf:"bytes,4,rep,name=roles"`
}

// ProjectRole represents a role that has access to a project
type ProjectRole struct {
	Name        string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Description string `json:"description" protobuf:"bytes,2,opt,name=description"`
	// Policies Stores a list of casbin formated strings that define access policies for the role in the project.
	Policies  []string   `json:"policies" protobuf:"bytes,3,rep,name=policies"`
	JWTTokens []JWTToken `json:"jwtTokens" protobuf:"bytes,4,rep,name=jwtTokens"`
}

// JWTToken holds the issuedAt and expiresAt values of a token
type JWTToken struct {
	IssuedAt  int64 `json:"iat,omitempty" protobuf:"int64,1,opt,name=iat"`
	ExpiresAt int64 `json:"exp,omitempty" protobuf:"int64,2,opt,name=exp"`
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
func (source ApplicationSource) Equals(other ApplicationSource) bool {
	return reflect.DeepEqual(source, other)
}

func (spec ApplicationSpec) BelongsToDefaultProject() bool {
	return spec.GetProject() == common.DefaultAppProjectName
}

func (spec ApplicationSpec) GetProject() string {
	if spec.Project == "" {
		return common.DefaultAppProjectName
	}
	return spec.Project
}

// IsSourcePermitted validiates if the provided application's source is a one of the allowed sources for the project.
func (proj AppProject) IsSourcePermitted(src ApplicationSource) bool {

	normalizedURL := git.NormalizeGitURL(src.RepoURL)
	for _, repoURL := range proj.Spec.SourceRepos {
		if repoURL == "*" {
			return true
		}
		if git.NormalizeGitURL(repoURL) == normalizedURL {
			return true
		}
	}
	return false
}

// IsDestinationPermitted validiates if the provided application's destination is one of the allowed destinations for the project
func (proj AppProject) IsDestinationPermitted(dst ApplicationDestination) bool {

	for _, item := range proj.Spec.Destinations {
		if item.Server == dst.Server || item.Server == "*" {
			if item.Namespace == dst.Namespace || item.Namespace == "*" {
				return true
			}
		}
	}
	return false
}

// RESTConfig returns a go-client REST config from cluster
func (c *Cluster) RESTConfig() *rest.Config {
	return &rest.Config{
		Host:        c.Server,
		Username:    c.Config.Username,
		Password:    c.Config.Password,
		BearerToken: c.Config.BearerToken,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure:   c.Config.TLSClientConfig.Insecure,
			ServerName: c.Config.TLSClientConfig.ServerName,
			CertData:   c.Config.TLSClientConfig.CertData,
			KeyData:    c.Config.TLSClientConfig.KeyData,
			CAData:     c.Config.TLSClientConfig.CAData,
		},
	}
}

// TargetObjects deserializes the list of target states into unstructured objects
func (cr *ComparisonResult) TargetObjects() ([]*unstructured.Unstructured, error) {
	objs := make([]*unstructured.Unstructured, len(cr.Resources))
	for i, resState := range cr.Resources {
		obj, err := resState.TargetObject()
		if err != nil {
			return nil, err
		}
		objs[i] = obj
	}
	return objs, nil
}

// LiveObjects deserializes the list of live states into unstructured objects
func (cr *ComparisonResult) LiveObjects() ([]*unstructured.Unstructured, error) {
	objs := make([]*unstructured.Unstructured, len(cr.Resources))
	for i, resState := range cr.Resources {
		obj, err := resState.LiveObject()
		if err != nil {
			return nil, err
		}
		objs[i] = obj
	}
	return objs, nil
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

func (r ResourceState) LiveObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.LiveState)
}

func (r ResourceState) TargetObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.TargetState)
}
