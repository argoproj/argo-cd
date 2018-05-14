package v1alpha1

import (
	"encoding/json"
	"time"

	"github.com/argoproj/argo-cd/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// SyncOperation contains sync operation details.
type SyncOperation struct {
	Revision string `json:"revision,omitempty" protobuf:"bytes,1,opt,name=revision"`
	Prune    bool   `json:"prune,omitempty" protobuf:"bytes,2,opt,name=prune"`
	DryRun   bool   `json:"dryRun,omitempty" protobuf:"bytes,3,opt,name=dryRun"`
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
	OperationRunning   OperationPhase = "Running"
	OperationFailed    OperationPhase = "Failed"
	OperationError     OperationPhase = "Error"
	OperationSucceeded OperationPhase = "Succeeded"
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

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	Resources []*ResourceDetails `json:"resources" protobuf:"bytes,1,opt,name=resources"`
}

type ResourceDetails struct {
	Name      string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Kind      string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`
	Message   string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
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
}

// ComponentParameter contains information about component parameter value
type ComponentParameter struct {
	Component string `json:"component" protobuf:"bytes,1,opt,name=component"`
	Name      string `json:"name" protobuf:"bytes,2,opt,name=name"`
	Value     string `json:"value" protobuf:"bytes,3,opt,name=value"`
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the repository URL containing the ksonnet application.
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within repository which contains ksonnet application.
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`
	// Environment is a ksonnet application environment name.
	Environment string `json:"environment" protobuf:"bytes,3,opt,name=environment"`
	// TargetRevision defines the commit, tag, or branch in which to sync the application to.
	// If omitted, will sync to HEAD
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// Environment parameter override values
	ComponentParameterOverrides []ComponentParameter `json:"componentParameterOverrides,omitempty" protobuf:"bytes,5,opt,name=componentParameterOverrides"`
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
	ComparisonStatusUnknown   ComparisonStatus = ""
	ComparisonStatusError     ComparisonStatus = "Error"
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

type ApplicationConditionType = string

const (
	// ApplicationConditionDeletionError indicates that controller failed to delete application
	ApplicationConditionDeletionError = "DeletionError"
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
	Error      string            `json:"error" protobuf:"bytes,7,opt,name=error"`
}

type HealthStatus struct {
	Status        HealthStatusCode `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	StatusDetails string           `json:"statusDetails,omitempty" protobuf:"bytes,2,opt,name=statusDetails"`
}

type HealthStatusCode = string

const (
	HealthStatusUnknown     = ""
	HealthStatusProgressing = "Progressing"
	HealthStatusHealthy     = "Healthy"
	HealthStatusDegraded    = "Degraded"
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

// Cluster is the definition of a cluster resource
type Cluster struct {
	// Server is the API server URL of the Kubernetes cluster
	Server string `json:"server" protobuf:"bytes,1,opt,name=server"`

	// Name of the cluster. If omitted, will use the server address
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`

	// Config holds cluster information for connecting to a cluster
	Config ClusterConfig `json:"config" protobuf:"bytes,3,opt,name=config"`

	// Error, if not blank, holds a state error of some sort.
	Error string `json:"-" protobuf:"bytes,4,opt,name=error"`
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
	Repo          string `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	Username      string `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	Password      string `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	SSHPrivateKey string `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	Error         string `json:"-" protobuf:"bytes,5,opt,name=error"`
}

// RepositoryList is a collection of Repositories.
type RepositoryList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []Repository `json:"items" protobuf:"bytes,2,rep,name=items"`
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

// NeedRefreshAppStatus answers if application status needs to be refreshed. Returns true if application never been compared, has changed or comparison result has expired.
func (app *Application) NeedRefreshAppStatus(statusRefreshTimeout time.Duration) bool {
	return app.Status.ComparisonResult.Status == ComparisonStatusUnknown ||
		!app.Spec.Source.Equals(app.Status.ComparisonResult.ComparedTo) ||
		app.Status.ComparisonResult.ComparedAt.Add(statusRefreshTimeout).Before(time.Now())
}

// Equals compares two instances of ApplicationSource and return true if instances are equal.
func (source ApplicationSource) Equals(other ApplicationSource) bool {
	return source.TargetRevision == other.TargetRevision &&
		source.RepoURL == other.RepoURL &&
		source.Path == other.Path &&
		source.Environment == other.Environment
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
