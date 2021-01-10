package v1alpha1

import (
	"encoding/json"
	"fmt"
	math "math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/cert"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/glob"
	"github.com/argoproj/argo-cd/util/helm"
)

// Application is a definition of Application resource.
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:path=applications,shortName=app;apps
// +kubebuilder:printcolumn:name="Sync Status",type=string,JSONPath=`.status.sync.status`
// +kubebuilder:printcolumn:name="Health Status",type=string,JSONPath=`.status.health.status`
// +kubebuilder:printcolumn:name="Revision",type=string,JSONPath=`.status.sync.revision`,priority=10
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
	// This limits this number of items kept in the apps revision history.
	// This should only be changed in exceptional circumstances.
	// Setting to zero will store no history. This will reduce storage used.
	// Increasing will increase the space used to store the history, so we do not recommend increasing it.
	// Default is 10.
	RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty" protobuf:"bytes,7,name=revisionHistoryLimit"`
}

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group        string   `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
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

func NewEnvEntry(text string) (*EnvEntry, error) {
	parts := strings.SplitN(text, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Expected env entry of the form: param=value. Received: %s", text)
	}
	return &EnvEntry{
		Name:  parts[0],
		Value: parts[1],
	}, nil
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

// does an operation similar to `envsubst` tool,
func (e Env) Envsubst(s string) string {
	valByEnv := map[string]string{}
	for _, item := range e {
		valByEnv[item.Name] = item.Value
	}
	return os.Expand(s, func(s string) string {
		return valByEnv[s]
	})
}

// ApplicationSource contains information about github repository, path within repository and target application environment.
type ApplicationSource struct {
	// RepoURL is the repository URL of the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the Git repository
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
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
	// Chart is a Helm chart name
	Chart string `json:"chart,omitempty" protobuf:"bytes,12,opt,name=chart"`
}

// AllowsConcurrentProcessing returns true if given application source can be processed concurrently
func (a *ApplicationSource) AllowsConcurrentProcessing() bool {
	switch {
	// Kustomize with parameters requires changing kustomization.yaml file
	case a.Kustomize != nil:
		return a.Kustomize.AllowsConcurrentProcessing()
	// Kustomize with parameters requires changing params.libsonnet file
	case a.Ksonnet != nil:
		return a.Ksonnet.AllowsConcurrentProcessing()
	}
	return true
}

func (a *ApplicationSource) IsHelm() bool {
	return a.Chart != ""
}

func (a *ApplicationSource) IsHelmOci() bool {
	if a.Chart == "" {
		return false
	}
	return helm.IsHelmOciChart(a.Chart)
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
	// Values is Helm values, typically defined as a block
	Values string `json:"values,omitempty" protobuf:"bytes,4,opt,name=values"`
	// FileParameters are file parameters to the helm template
	FileParameters []HelmFileParameter `json:"fileParameters,omitempty" protobuf:"bytes,5,opt,name=fileParameters"`
	// Version is the Helm version to use for templating with
	Version string `json:"version,omitempty" protobuf:"bytes,6,opt,name=version"`
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

// HelmFileParameter is a file parameter to a helm template
type HelmFileParameter struct {
	// Name is the name of the helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Path is the path value for the helm parameter
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
}

var helmParameterRx = regexp.MustCompile(`([^\\]),`)

func NewHelmParameter(text string, forceString bool) (*HelmParameter, error) {
	parts := strings.SplitN(text, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Expected helm parameter of the form: param=value. Received: %s", text)
	}
	return &HelmParameter{
		Name:        parts[0],
		Value:       helmParameterRx.ReplaceAllString(parts[1], `$1\,`),
		ForceString: forceString,
	}, nil
}

func NewHelmFileParameter(text string) (*HelmFileParameter, error) {
	parts := strings.SplitN(text, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Expected helm file parameter of the form: param=path. Received: %s", text)
	}
	return &HelmFileParameter{
		Name: parts[0],
		Path: helmParameterRx.ReplaceAllString(parts[1], `$1\,`),
	}, nil
}

func (in *ApplicationSourceHelm) AddParameter(p HelmParameter) {
	found := false
	for i, cp := range in.Parameters {
		if cp.Name == p.Name {
			found = true
			in.Parameters[i] = p
			break
		}
	}
	if !found {
		in.Parameters = append(in.Parameters, p)
	}
}

func (in *ApplicationSourceHelm) AddFileParameter(p HelmFileParameter) {
	found := false
	for i, cp := range in.FileParameters {
		if cp.Name == p.Name {
			found = true
			in.FileParameters[i] = p
			break
		}
	}
	if !found {
		in.FileParameters = append(in.FileParameters, p)
	}
}

func (h *ApplicationSourceHelm) IsZero() bool {
	return h == nil || (h.Version == "") && (h.ReleaseName == "") && len(h.ValueFiles) == 0 && len(h.Parameters) == 0 && len(h.FileParameters) == 0 && h.Values == ""
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
	// NameSuffix is a suffix appended to resources for kustomize apps
	NameSuffix string `json:"nameSuffix,omitempty" protobuf:"bytes,2,opt,name=nameSuffix"`
	// Images are kustomize image overrides
	Images KustomizeImages `json:"images,omitempty" protobuf:"bytes,3,opt,name=images"`
	// CommonLabels adds additional kustomize commonLabels
	CommonLabels map[string]string `json:"commonLabels,omitempty" protobuf:"bytes,4,opt,name=commonLabels"`
	// Version contains optional Kustomize version
	Version string `json:"version,omitempty" protobuf:"bytes,5,opt,name=version"`
	// CommonAnnotations adds additional kustomize commonAnnotations
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty" protobuf:"bytes,6,opt,name=commonAnnotations"`
}

func (k *ApplicationSourceKustomize) AllowsConcurrentProcessing() bool {
	return len(k.Images) == 0 &&
		len(k.CommonLabels) == 0 &&
		k.NamePrefix == "" &&
		k.NameSuffix == ""
}

func (k *ApplicationSourceKustomize) IsZero() bool {
	return k == nil ||
		k.NamePrefix == "" &&
			k.NameSuffix == "" &&
			k.Version == "" &&
			len(k.Images) == 0 &&
			len(k.CommonLabels) == 0 &&
			len(k.CommonAnnotations) == 0
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

func NewJsonnetVar(s string, code bool) JsonnetVar {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 2 {
		return JsonnetVar{Name: parts[0], Value: parts[1], Code: code}
	} else {
		return JsonnetVar{Name: s, Code: code}
	}
}

// ApplicationSourceJsonnet holds jsonnet specific options
type ApplicationSourceJsonnet struct {
	// ExtVars is a list of Jsonnet External Variables
	ExtVars []JsonnetVar `json:"extVars,omitempty" protobuf:"bytes,1,opt,name=extVars"`
	// TLAS is a list of Jsonnet Top-level Arguments
	TLAs []JsonnetVar `json:"tlas,omitempty" protobuf:"bytes,2,opt,name=tlas"`
	// Additional library search dirs
	Libs []string `json:"libs,omitempty" protobuf:"bytes,3,opt,name=libs"`
}

func (j *ApplicationSourceJsonnet) IsZero() bool {
	return j == nil || len(j.ExtVars) == 0 && len(j.TLAs) == 0 && len(j.Libs) == 0
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

func (k *ApplicationSourceKsonnet) AllowsConcurrentProcessing() bool {
	return len(k.Parameters) == 0
}

func (k *ApplicationSourceKsonnet) IsZero() bool {
	return k == nil || k.Environment == "" && len(k.Parameters) == 0
}

type ApplicationSourceDirectory struct {
	Recurse bool                     `json:"recurse,omitempty" protobuf:"bytes,1,opt,name=recurse"`
	Jsonnet ApplicationSourceJsonnet `json:"jsonnet,omitempty" protobuf:"bytes,2,opt,name=jsonnet"`
	Exclude string                   `json:"exclude,omitempty" protobuf:"bytes,3,opt,name=exclude"`
	Include string                   `json:"include,omitempty" protobuf:"bytes,4,opt,name=include"`
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

func (c *ApplicationSourcePlugin) AddEnvEntry(e *EnvEntry) {
	found := false
	for i, ce := range c.Env {
		if ce.Name == e.Name {
			found = true
			c.Env[i] = e
			break
		}
	}
	if !found {
		c.Env = append(c.Env, e)
	}
}

// ApplicationDestination contains deployment destination information
type ApplicationDestination struct {
	// Server overrides the environment server value in the ksonnet app.yaml
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Namespace overrides the environment namespace value in the ksonnet app.yaml
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	// Name of the destination cluster which can be used instead of server (url) field
	Name string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`

	// nolint:govet
	isServerInferred bool `json:"-"`
}

// ApplicationStatus contains information about application sync, health status
type ApplicationStatus struct {
	Resources  []ResourceStatus       `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	Sync       SyncStatus             `json:"sync,omitempty" protobuf:"bytes,2,opt,name=sync"`
	Health     HealthStatus           `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`
	History    RevisionHistories      `json:"history,omitempty" protobuf:"bytes,4,opt,name=history"`
	Conditions []ApplicationCondition `json:"conditions,omitempty" protobuf:"bytes,5,opt,name=conditions"`
	// ReconciledAt indicates when the application state was reconciled using the latest git version
	ReconciledAt   *metav1.Time    `json:"reconciledAt,omitempty" protobuf:"bytes,6,opt,name=reconciledAt"`
	OperationState *OperationState `json:"operationState,omitempty" protobuf:"bytes,7,opt,name=operationState"`
	// ObservedAt indicates when the application state was updated without querying latest git state
	// Deprecated: controller no longer updates ObservedAt field
	ObservedAt *metav1.Time          `json:"observedAt,omitempty" protobuf:"bytes,8,opt,name=observedAt"`
	SourceType ApplicationSourceType `json:"sourceType,omitempty" protobuf:"bytes,9,opt,name=sourceType"`
	Summary    ApplicationSummary    `json:"summary,omitempty" protobuf:"bytes,10,opt,name=summary"`
}

type JWTTokens struct {
	Items []JWTToken `json:"items,omitempty" protobuf:"bytes,1,opt,name=items"`
}

// AppProjectStatus contains information about appproj
type AppProjectStatus struct {
	JWTTokensByRole map[string]JWTTokens `json:"jwtTokensByRole,omitempty" protobuf:"bytes,1,opt,name=jwtTokensByRole"`
}

// OperationInitiator holds information about the operation initiator
type OperationInitiator struct {
	// Name of a user who started operation.
	Username string `json:"username,omitempty" protobuf:"bytes,1,opt,name=username"`
	// Automated is set to true if operation was initiated automatically by the application controller.
	Automated bool `json:"automated,omitempty" protobuf:"bytes,2,opt,name=automated"`
}

// Operation contains requested operation parameters.
type Operation struct {
	Sync        *SyncOperation     `json:"sync,omitempty" protobuf:"bytes,1,opt,name=sync"`
	InitiatedBy OperationInitiator `json:"initiatedBy,omitempty" protobuf:"bytes,2,opt,name=initiatedBy"`
	Info        []*Info            `json:"info,omitempty" protobuf:"bytes,3,name=info"`
	// Retry controls failed sync retry behavior
	Retry RetryStrategy `json:"retry,omitempty" protobuf:"bytes,4,opt,name=retry"`
}

func (o *Operation) DryRun() bool {
	if o.Sync != nil {
		return o.Sync.DryRun
	}
	return false
}

// SyncOperationResource contains resources to sync.
type SyncOperationResource struct {
	Group     string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind      string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name      string `json:"name" protobuf:"bytes,3,opt,name=name"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
}

// RevisionHistories is a array of history, oldest first and newest last
type RevisionHistories []RevisionHistory

func (in RevisionHistories) LastRevisionHistory() RevisionHistory {
	return in[len(in)-1]
}

func (in RevisionHistories) Trunc(n int) RevisionHistories {
	i := len(in) - n
	if i > 0 {
		in = in[i:]
	}
	return in
}

// HasIdentity determines whether a sync operation is identified by a manifest
func (r SyncOperationResource) HasIdentity(name string, namespace string, gvk schema.GroupVersionKind) bool {
	if name == r.Name && gvk.Kind == r.Kind && gvk.Group == r.Group && (r.Namespace == "" || namespace == r.Namespace) {
		return true
	}
	return false
}

// SyncOperation contains sync operation details.
type SyncOperation struct {
	// Revision is the revision in which to sync the application to.
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
	// SyncOptions provide per-sync sync-options, e.g. Validate=false
	SyncOptions SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,9,opt,name=syncOptions"`
}

func (o *SyncOperation) IsApplyStrategy() bool {
	return o.SyncStrategy != nil && o.SyncStrategy.Apply != nil
}

// OperationState contains information about state of currently performing operation on application.
type OperationState struct {
	// Operation is the original requested operation
	Operation Operation `json:"operation" protobuf:"bytes,1,opt,name=operation"`
	// Phase is the current phase of the operation
	Phase synccommon.OperationPhase `json:"phase" protobuf:"bytes,2,opt,name=phase"`
	// Message hold any pertinent messages when attempting to perform operation (typically errors).
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// SyncResult is the result of a Sync operation
	SyncResult *SyncOperationResult `json:"syncResult,omitempty" protobuf:"bytes,4,opt,name=syncResult"`
	// StartedAt contains time of operation start
	StartedAt metav1.Time `json:"startedAt" protobuf:"bytes,6,opt,name=startedAt"`
	// FinishedAt contains time of operation completion
	FinishedAt *metav1.Time `json:"finishedAt,omitempty" protobuf:"bytes,7,opt,name=finishedAt"`
	// RetryCount contains time of operation retries
	RetryCount int64 `json:"retryCount,omitempty" protobuf:"bytes,8,opt,name=retryCount"`
}

type Info struct {
	Name  string `json:"name" protobuf:"bytes,1,name=name"`
	Value string `json:"value" protobuf:"bytes,2,name=value"`
}

type SyncOptions []string

func (o SyncOptions) AddOption(option string) SyncOptions {
	for _, j := range o {
		if j == option {
			return o
		}
	}
	return append(o, option)
}

func (o SyncOptions) RemoveOption(option string) SyncOptions {
	for i, j := range o {
		if j == option {
			return append(o[:i], o[i+1:]...)
		}
	}
	return o
}

func (o SyncOptions) HasOption(option string) bool {
	for _, i := range o {
		if option == i {
			return true
		}
	}
	return false
}

// SyncPolicy controls when a sync will be performed in response to updates in git
type SyncPolicy struct {
	// Automated will keep an application synced to the target revision
	Automated *SyncPolicyAutomated `json:"automated,omitempty" protobuf:"bytes,1,opt,name=automated"`
	// Options allow you to specify whole app sync-options
	SyncOptions SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,2,opt,name=syncOptions"`
	// Retry controls failed sync retry behavior
	Retry *RetryStrategy `json:"retry,omitempty" protobuf:"bytes,3,opt,name=retry"`
}

func (p *SyncPolicy) IsZero() bool {
	return p == nil || (p.Automated == nil && len(p.SyncOptions) == 0)
}

type RetryStrategy struct {
	// Limit is the maximum number of attempts when retrying a container
	Limit int64 `json:"limit,omitempty" protobuf:"bytes,1,opt,name=limit"`

	// Backoff is a backoff strategy
	Backoff *Backoff `json:"backoff,omitempty" protobuf:"bytes,2,opt,name=backoff,casttype=Backoff"`
}

func parseStringToDuration(durationString string) (time.Duration, error) {
	var suspendDuration time.Duration
	// If no units are attached, treat as seconds
	if val, err := strconv.Atoi(durationString); err == nil {
		suspendDuration = time.Duration(val) * time.Second
	} else if duration, err := time.ParseDuration(durationString); err == nil {
		suspendDuration = duration
	} else {
		return 0, fmt.Errorf("unable to parse %s as a duration", durationString)
	}
	return suspendDuration, nil
}

func (r *RetryStrategy) NextRetryAt(lastAttempt time.Time, retryCounts int64) (time.Time, error) {
	maxDuration := common.DefaultSyncRetryMaxDuration
	duration := common.DefaultSyncRetryDuration
	factor := common.DefaultSyncRetryFactor
	var err error
	if r.Backoff != nil {
		if r.Backoff.Duration != "" {
			if duration, err = parseStringToDuration(r.Backoff.Duration); err != nil {
				return time.Time{}, err
			}
		}
		if r.Backoff.MaxDuration != "" {
			if maxDuration, err = parseStringToDuration(r.Backoff.MaxDuration); err != nil {
				return time.Time{}, err
			}
		}
		if r.Backoff.Factor != nil {
			factor = *r.Backoff.Factor
		}

	}
	// Formula: timeToWait = duration * factor^retry_number
	// Note that timeToWait should equal to duration for the first retry attempt.
	timeToWait := duration * time.Duration(math.Pow(float64(factor), float64(retryCounts)))
	if maxDuration > 0 {
		timeToWait = time.Duration(math.Min(float64(maxDuration), float64(timeToWait)))
	}
	return lastAttempt.Add(timeToWait), nil
}

// Backoff is a backoff strategy to use within retryStrategy
type Backoff struct {
	// Duration is the amount to back off. Default unit is seconds, but could also be a duration (e.g. "2m", "1h")
	Duration string `json:"duration,omitempty" protobuf:"bytes,1,opt,name=duration"`
	// Factor is a factor to multiply the base duration after each failed retry
	Factor *int64 `json:"factor,omitempty" protobuf:"bytes,2,name=factor"`
	// MaxDuration is the maximum amount of time allowed for the backoff strategy
	MaxDuration string `json:"maxDuration,omitempty" protobuf:"bytes,3,opt,name=maxDuration"`
}

// SyncPolicyAutomated controls the behavior of an automated sync
type SyncPolicyAutomated struct {
	// Prune will prune resources automatically as part of automated sync (default: false)
	Prune bool `json:"prune,omitempty" protobuf:"bytes,1,opt,name=prune"`
	// SelfHeal enables auto-syncing if  (default: false)
	SelfHeal bool `json:"selfHeal,omitempty" protobuf:"bytes,2,opt,name=selfHeal"`
	// AllowEmpty allows apps have zero live resources (default: false)
	AllowEmpty bool `json:"allowEmpty,omitempty" protobuf:"bytes,3,opt,name=allowEmpty"`
}

// SyncStrategy controls the manner in which a sync is performed
type SyncStrategy struct {
	// Apply will perform a `kubectl apply` to perform the sync.
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

// data about a specific revision within a repo
type RevisionMetadata struct {
	// who authored this revision,
	// typically their name and email, e.g. "John Doe <john_doe@my-company.com>",
	// but might not match this example
	Author string `json:"author,omitempty" protobuf:"bytes,1,opt,name=author"`
	// when the revision was authored
	Date metav1.Time `json:"date" protobuf:"bytes,2,opt,name=date"`
	// tags on the revision,
	// note - tags can move from one revision to another
	Tags []string `json:"tags,omitempty" protobuf:"bytes,3,opt,name=tags"`
	// the message associated with the revision,
	// probably the commit message,
	// this is truncated to the first newline or 64 characters (which ever comes first)
	Message string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
	// If revision was signed with GPG, and signature verification is enabled,
	// this contains a hint on the signer
	SignatureInfo string `json:"signatureInfo,omitempty" protobuf:"bytes,5,opt,name=signatureInfo"`
}

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	// Resources holds the sync result of each individual resource
	Resources ResourceResults `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Revision holds the revision of the sync
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// Source records the application source information of the sync, used for comparing auto-sync
	Source ApplicationSource `json:"source,omitempty" protobuf:"bytes,3,opt,name=source"`
}

// ResourceResult holds the operation result details of a specific resource
type ResourceResult struct {
	Group     string `json:"group" protobuf:"bytes,1,opt,name=group"`
	Version   string `json:"version" protobuf:"bytes,2,opt,name=version"`
	Kind      string `json:"kind" protobuf:"bytes,3,opt,name=kind"`
	Namespace string `json:"namespace" protobuf:"bytes,4,opt,name=namespace"`
	Name      string `json:"name" protobuf:"bytes,5,opt,name=name"`
	// the final result of the sync, this is be empty if the resources is yet to be applied/pruned and is always zero-value for hooks
	Status synccommon.ResultCode `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	// message for the last sync OR operation
	Message string `json:"message,omitempty" protobuf:"bytes,7,opt,name=message"`
	// the type of the hook, empty for non-hook resources
	HookType synccommon.HookType `json:"hookType,omitempty" protobuf:"bytes,8,opt,name=hookType"`
	// the state of any operation associated with this resource OR hook
	// note: can contain values for non-hook resources
	HookPhase synccommon.OperationPhase `json:"hookPhase,omitempty" protobuf:"bytes,9,opt,name=hookPhase"`
	// indicates the particular phase of the sync that this is for
	SyncPhase synccommon.SyncPhase `json:"syncPhase,omitempty" protobuf:"bytes,10,opt,name=syncPhase"`
}

func (r *ResourceResult) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

type ResourceResults []*ResourceResult

func (r ResourceResults) Find(group string, kind string, namespace string, name string, phase synccommon.SyncPhase) (int, *ResourceResult) {
	for i, res := range r {
		if res.Group == group && res.Kind == kind && res.Namespace == namespace && res.Name == name && res.SyncPhase == phase {
			return i, res
		}
	}
	return 0, nil
}

func (r ResourceResults) PruningRequired() (num int) {
	for _, res := range r {
		if res.Status == synccommon.ResultCodePruneSkipped {
			num++
		}
	}
	return num
}

// RevisionHistory contains information relevant to an application deployment
type RevisionHistory struct {
	// Revision holds the revision of the sync
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// DeployedAt holds the time the deployment completed
	DeployedAt metav1.Time `json:"deployedAt" protobuf:"bytes,4,opt,name=deployedAt"`
	// ID is an auto incrementing identifier of the RevisionHistory
	ID     int64             `json:"id" protobuf:"bytes,5,opt,name=id"`
	Source ApplicationSource `json:"source,omitempty" protobuf:"bytes,6,opt,name=source"`
	// DeployStartedAt holds the time the deployment started
	DeployStartedAt *metav1.Time `json:"deployStartedAt,omitempty" protobuf:"bytes,7,opt,name=deployStartedAt"`
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
	// ApplicationConditionOrphanedResourceWarning indicates that application has orphaned resources
	ApplicationConditionOrphanedResourceWarning = "OrphanedResourceWarning"
)

// ApplicationCondition contains details about current application condition
type ApplicationCondition struct {
	// Type is an application condition type
	Type ApplicationConditionType `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Message contains human-readable message indicating details about condition
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
	// LastTransitionTime is the time the condition was first observed.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
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
	Status  health.HealthStatusCode `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	Message string                  `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}

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

type HostResourceInfo struct {
	ResourceName         v1.ResourceName `json:"resourceName,omitempty" protobuf:"bytes,1,name=resourceName"`
	RequestedByApp       int64           `json:"requestedByApp,omitempty" protobuf:"bytes,2,name=requestedByApp"`
	RequestedByNeighbors int64           `json:"requestedByNeighbors,omitempty" protobuf:"bytes,3,name=requestedByNeighbors"`
	Available            int64           `json:"available,omitempty" protobuf:"bytes,4,name=available"`
}

// HostInfo holds host name and resources metrics
type HostInfo struct {
	Name          string             `json:"name,omitempty" protobuf:"bytes,1,name=name"`
	ResourcesInfo []HostResourceInfo `json:"resourcesInfo,omitempty" protobuf:"bytes,2,name=resourcesInfo"`
	SystemInfo    v1.NodeSystemInfo  `json:"systemInfo,omitempty" protobuf:"bytes,3,opt,name=systemInfo"`
}

// ApplicationTree holds nodes which belongs to the application
type ApplicationTree struct {
	// Nodes contains list of nodes which either directly managed by the application and children of directly managed nodes.
	Nodes []ResourceNode `json:"nodes,omitempty" protobuf:"bytes,1,rep,name=nodes"`
	// OrphanedNodes contains if or orphaned nodes: nodes which are not managed by the app but in the same namespace. List is populated only if orphaned resources enabled in app project.
	OrphanedNodes []ResourceNode `json:"orphanedNodes,omitempty" protobuf:"bytes,2,rep,name=orphanedNodes"`
	// Hosts holds list of Kubernetes nodes that run application related pods
	Hosts []HostInfo `json:"hosts,omitempty" protobuf:"bytes,3,rep,name=hosts"`
}

type ApplicationSummary struct {
	// ExternalURLs holds all external URLs of application child resources.
	ExternalURLs []string `json:"externalURLs,omitempty" protobuf:"bytes,1,opt,name=externalURLs"`
	// Images holds all images of application child resources.
	Images []string `json:"images,omitempty" protobuf:"bytes,2,opt,name=images"`
}

func (t *ApplicationTree) FindNode(group string, kind string, namespace string, name string) *ResourceNode {
	for _, n := range append(t.Nodes, t.OrphanedNodes...) {
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
	sort.Slice(urls, func(i, j int) bool {
		return urls[i] < urls[j]
	})
	images := make([]string, 0)
	for image := range imagesSet {
		images = append(images, image)
	}
	sort.Slice(images, func(i, j int) bool {
		return images[i] < images[j]
	})
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
	CreatedAt       *metav1.Time            `json:"createdAt,omitempty" protobuf:"bytes,8,opt,name=createdAt"`
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
	Group     string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind      string `json:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,3,opt,name=namespace"`
	Name      string `json:"name,omitempty" protobuf:"bytes,4,opt,name=name"`
	// TargetState contains the JSON serialized resource manifest defined in the Git/Helm
	TargetState string `json:"targetState,omitempty" protobuf:"bytes,5,opt,name=targetState"`
	// TargetState contains the JSON live resource manifest
	LiveState string `json:"liveState,omitempty" protobuf:"bytes,6,opt,name=liveState"`
	// Diff contains the JSON patch between target and live resource
	// Deprecated: use NormalizedLiveState and PredictedLiveState to render the difference
	Diff string `json:"diff,omitempty" protobuf:"bytes,7,opt,name=diff"`
	Hook bool   `json:"hook,omitempty" protobuf:"bytes,8,opt,name=hook"`
	// NormalizedLiveState contains JSON serialized live resource state with applied normalizations
	NormalizedLiveState string `json:"normalizedLiveState,omitempty" protobuf:"bytes,9,opt,name=normalizedLiveState"`
	// PredictedLiveState contains JSON serialized resource state that is calculated based on normalized and target resource state
	PredictedLiveState string `json:"predictedLiveState,omitempty" protobuf:"bytes,10,opt,name=predictedLiveState"`
}

// ConnectionStatus represents connection status
type ConnectionStatus = string

const (
	ConnectionStatusSuccessful = "Successful"
	ConnectionStatusFailed     = "Failed"
	ConnectionStatusUnknown    = "Unknown"
)

// ConnectionState contains information about remote resource connection state
type ConnectionState struct {
	Status     ConnectionStatus `json:"status" protobuf:"bytes,1,opt,name=status"`
	Message    string           `json:"message" protobuf:"bytes,2,opt,name=message"`
	ModifiedAt *metav1.Time     `json:"attemptedAt" protobuf:"bytes,3,opt,name=attemptedAt"`
}

// Cluster is the definition of a cluster resource
type Cluster struct {
	// ID is an internal field cluster identifier. Not exposed via API.
	ID string `json:"-"`
	// Server is the API server URL of the Kubernetes cluster
	Server string `json:"server" protobuf:"bytes,1,opt,name=server"`
	// Name of the cluster. If omitted, will use the server address
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Config holds cluster information for connecting to a cluster
	Config ClusterConfig `json:"config" protobuf:"bytes,3,opt,name=config"`
	// DEPRECATED: use Info.ConnectionState field instead.
	// ConnectionState contains information about cluster connection state
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,4,opt,name=connectionState"`
	// DEPRECATED: use Info.ServerVersion field instead.
	// The server version
	ServerVersion string `json:"serverVersion,omitempty" protobuf:"bytes,5,opt,name=serverVersion"`
	// Holds list of namespaces which are accessible in that cluster. Cluster level resources would be ignored if namespace list is not empty.
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,6,opt,name=namespaces"`
	// RefreshRequestedAt holds time when cluster cache refresh has been requested
	RefreshRequestedAt *metav1.Time `json:"refreshRequestedAt,omitempty" protobuf:"bytes,7,opt,name=refreshRequestedAt"`
	// Holds information about cluster cache
	Info ClusterInfo `json:"info,omitempty" protobuf:"bytes,8,opt,name=info"`
	// Shard contains optional shard number. Calculated on the fly by the application controller if not specified.
	Shard *int64 `json:"shard,omitempty" protobuf:"bytes,9,opt,name=shard"`
}

func (c *Cluster) Equals(other *Cluster) bool {
	if c.Server != other.Server {
		return false
	}
	if c.Name != other.Name {
		return false
	}
	if strings.Join(c.Namespaces, ",") != strings.Join(other.Namespaces, ",") {
		return false
	}
	var shard int64 = -1
	if c.Shard != nil {
		shard = *c.Shard
	}
	var otherShard int64 = -1
	if other.Shard != nil {
		otherShard = *other.Shard
	}
	if shard != otherShard {
		return false
	}
	return reflect.DeepEqual(c.Config, other.Config)
}

type ClusterInfo struct {
	ConnectionState   ConnectionState  `json:"connectionState,omitempty" protobuf:"bytes,1,opt,name=connectionState"`
	ServerVersion     string           `json:"serverVersion,omitempty" protobuf:"bytes,2,opt,name=serverVersion"`
	CacheInfo         ClusterCacheInfo `json:"cacheInfo,omitempty" protobuf:"bytes,3,opt,name=cacheInfo"`
	ApplicationsCount int64            `json:"applicationsCount" protobuf:"bytes,4,opt,name=applicationsCount"`
}

type ClusterCacheInfo struct {
	// ResourcesCount holds number of observed Kubernetes resources
	ResourcesCount int64 `json:"resourcesCount,omitempty" protobuf:"bytes,1,opt,name=resourcesCount"`
	// APIsCount holds number of observed Kubernetes API count
	APIsCount int64 `json:"apisCount,omitempty" protobuf:"bytes,2,opt,name=apisCount"`
	// LastCacheSyncTime holds time of most recent cache synchronization
	LastCacheSyncTime *metav1.Time `json:"lastCacheSyncTime,omitempty" protobuf:"bytes,3,opt,name=lastCacheSyncTime"`
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

// ExecProviderConfig is config used to call an external command to perform cluster authentication
// See: https://godoc.org/k8s.io/client-go/tools/clientcmd/api#ExecConfig
type ExecProviderConfig struct {
	// Command to execute
	Command string `json:"command,omitempty" protobuf:"bytes,1,opt,name=command"`

	// Arguments to pass to the command when executing it
	Args []string `json:"args,omitempty" protobuf:"bytes,2,rep,name=args"`

	// Env defines additional environment variables to expose to the process
	Env map[string]string `json:"env,omitempty" protobuf:"bytes,3,opt,name=env"`

	// Preferred input version of the ExecInfo
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,4,opt,name=apiVersion"`

	// This text is shown to the user when the executable doesn't seem to be present
	InstallHint string `json:"installHint,omitempty" protobuf:"bytes,5,opt,name=installHint"`
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

	// ExecProviderConfig contains configuration for an exec provider
	ExecProviderConfig *ExecProviderConfig `json:"execProviderConfig,omitempty" protobuf:"bytes,6,opt,name=execProviderConfig"`
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

// KnownTypeField contains mapping between CRD field and known Kubernetes type
type KnownTypeField struct {
	Field string `json:"field,omitempty" protobuf:"bytes,1,opt,name=field"`
	Type  string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
}

type OverrideIgnoreDiff struct {
	JSONPointers []string `json:"jsonPointers" protobuf:"bytes,1,rep,name=jSONPointers"`
}

type rawResourceOverride struct {
	HealthLua         string           `json:"health.lua,omitempty"`
	Actions           string           `json:"actions,omitempty"`
	IgnoreDifferences string           `json:"ignoreDifferences,omitempty"`
	KnownTypeFields   []KnownTypeField `json:"knownTypeFields,omitempty"`
}

// ResourceOverride holds configuration to customize resource diffing and health assessment
type ResourceOverride struct {
	HealthLua         string             `protobuf:"bytes,1,opt,name=healthLua"`
	Actions           string             `protobuf:"bytes,3,opt,name=actions"`
	IgnoreDifferences OverrideIgnoreDiff `protobuf:"bytes,2,opt,name=ignoreDifferences"`
	KnownTypeFields   []KnownTypeField   `protobuf:"bytes,4,opt,name=knownTypeFields"`
}

func (s *ResourceOverride) UnmarshalJSON(data []byte) error {
	raw := &rawResourceOverride{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.KnownTypeFields = raw.KnownTypeFields
	s.HealthLua = raw.HealthLua
	s.Actions = raw.Actions
	return yaml.Unmarshal([]byte(raw.IgnoreDifferences), &s.IgnoreDifferences)
}

func (s ResourceOverride) MarshalJSON() ([]byte, error) {
	ignoreDifferencesData, err := yaml.Marshal(s.IgnoreDifferences)
	if err != nil {
		return nil, err
	}
	raw := &rawResourceOverride{s.HealthLua, s.Actions, string(ignoreDifferencesData), s.KnownTypeFields}
	return json.Marshal(raw)
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
	Definitions        []ResourceActionDefinition `json:"definitions,omitempty" protobuf:"bytes,2,rep,name=definitions"`
}

type ResourceActionDefinition struct {
	Name      string `json:"name" protobuf:"bytes,1,opt,name=name"`
	ActionLua string `json:"action.lua" yaml:"action.lua" protobuf:"bytes,2,opt,name=actionLua"`
}

type ResourceAction struct {
	Name     string                `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Params   []ResourceActionParam `json:"params,omitempty" protobuf:"bytes,2,rep,name=params"`
	Disabled bool                  `json:"disabled,omitempty" protobuf:"varint,3,opt,name=disabled"`
}

type ResourceActionParam struct {
	Name    string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Value   string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	Type    string `json:"type,omitempty" protobuf:"bytes,3,opt,name=type"`
	Default string `json:"default,omitempty" protobuf:"bytes,4,opt,name=default"`
}

// RepoCreds holds a repository credentials definition
type RepoCreds struct {
	// URL is the URL that this credentials matches to
	URL string `json:"url" protobuf:"bytes,1,opt,name=url"`
	// Username for authenticating at the repo server
	Username string `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	// Password for authenticating at the repo server
	Password string `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	// SSH private key data for authenticating at the repo server (only Git repos)
	SSHPrivateKey string `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	// TLS client cert data for authenticating at the repo server
	TLSClientCertData string `json:"tlsClientCertData,omitempty" protobuf:"bytes,5,opt,name=tlsClientCertData"`
	// TLS client cert key for authenticating at the repo server
	TLSClientCertKey string `json:"tlsClientCertKey,omitempty" protobuf:"bytes,6,opt,name=tlsClientCertKey"`
}

// Repository is a repository holding application configurations
type Repository struct {
	// URL of the repo
	Repo string `json:"repo" protobuf:"bytes,1,opt,name=repo"`
	// Username for authenticating at the repo server
	Username string `json:"username,omitempty" protobuf:"bytes,2,opt,name=username"`
	// Password for authenticating at the repo server
	Password string `json:"password,omitempty" protobuf:"bytes,3,opt,name=password"`
	// SSH private key data for authenticating at the repo server
	// only for Git repos
	SSHPrivateKey string `json:"sshPrivateKey,omitempty" protobuf:"bytes,4,opt,name=sshPrivateKey"`
	// Current state of repository server connecting
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,5,opt,name=connectionState"`
	// InsecureIgnoreHostKey should not be used anymore, Insecure is favoured
	// only for Git repos
	InsecureIgnoreHostKey bool `json:"insecureIgnoreHostKey,omitempty" protobuf:"bytes,6,opt,name=insecureIgnoreHostKey"`
	// Whether the repo is insecure
	Insecure bool `json:"insecure,omitempty" protobuf:"bytes,7,opt,name=insecure"`
	// Whether git-lfs support should be enabled for this repo
	EnableLFS bool `json:"enableLfs,omitempty" protobuf:"bytes,8,opt,name=enableLfs"`
	// TLS client cert data for authenticating at the repo server
	TLSClientCertData string `json:"tlsClientCertData,omitempty" protobuf:"bytes,9,opt,name=tlsClientCertData"`
	// TLS client cert key for authenticating at the repo server
	TLSClientCertKey string `json:"tlsClientCertKey,omitempty" protobuf:"bytes,10,opt,name=tlsClientCertKey"`
	// type of the repo, maybe "git or "helm, "git" is assumed if empty or absent
	Type string `json:"type,omitempty" protobuf:"bytes,11,opt,name=type"`
	// only for Helm repos
	Name string `json:"name,omitempty" protobuf:"bytes,12,opt,name=name"`
	// Whether credentials were inherited from a credential set
	InheritedCreds bool `json:"inheritedCreds,omitempty" protobuf:"bytes,13,opt,name=inheritedCreds"`
	// Whether helm-oci support should be enabled for this repo
	EnableOCI bool `json:"enableOCI,omitempty" protobuf:"bytes,14,opt,name=enableOCI"`
}

// IsInsecure returns true if receiver has been configured to skip server verification
func (repo *Repository) IsInsecure() bool {
	return repo.InsecureIgnoreHostKey || repo.Insecure
}

// IsLFSEnabled returns true if LFS support is enabled on receiver
func (repo *Repository) IsLFSEnabled() bool {
	return repo.EnableLFS
}

// HasCredentials returns true when the receiver has been configured any credentials
func (m *Repository) HasCredentials() bool {
	return m.Username != "" || m.Password != "" || m.SSHPrivateKey != "" || m.TLSClientCertData != ""
}

func (repo *Repository) CopyCredentialsFromRepo(source *Repository) {
	if source != nil {
		if repo.Username == "" {
			repo.Username = source.Username
		}
		if repo.Password == "" {
			repo.Password = source.Password
		}
		if repo.SSHPrivateKey == "" {
			repo.SSHPrivateKey = source.SSHPrivateKey
		}
		if repo.TLSClientCertData == "" {
			repo.TLSClientCertData = source.TLSClientCertData
		}
		if repo.TLSClientCertKey == "" {
			repo.TLSClientCertKey = source.TLSClientCertKey
		}
	}
}

// CopyCredentialsFrom copies all credentials from source to receiver
func (repo *Repository) CopyCredentialsFrom(source *RepoCreds) {
	if source != nil {
		if repo.Username == "" {
			repo.Username = source.Username
		}
		if repo.Password == "" {
			repo.Password = source.Password
		}
		if repo.SSHPrivateKey == "" {
			repo.SSHPrivateKey = source.SSHPrivateKey
		}
		if repo.TLSClientCertData == "" {
			repo.TLSClientCertData = source.TLSClientCertData
		}
		if repo.TLSClientCertKey == "" {
			repo.TLSClientCertKey = source.TLSClientCertKey
		}
	}
}

func (repo *Repository) GetGitCreds() git.Creds {
	if repo == nil {
		return git.NopCreds{}
	}
	if repo.Username != "" && repo.Password != "" {
		return git.NewHTTPSCreds(repo.Username, repo.Password, repo.TLSClientCertData, repo.TLSClientCertKey, repo.IsInsecure())
	}
	if repo.SSHPrivateKey != "" {
		return git.NewSSHCreds(repo.SSHPrivateKey, getCAPath(repo.Repo), repo.IsInsecure())
	}
	return git.NopCreds{}
}

func (repo *Repository) GetHelmCreds() helm.Creds {
	return helm.Creds{
		Username:           repo.Username,
		Password:           repo.Password,
		CAPath:             getCAPath(repo.Repo),
		CertData:           []byte(repo.TLSClientCertData),
		KeyData:            []byte(repo.TLSClientCertKey),
		InsecureSkipVerify: repo.Insecure,
	}
}

func getCAPath(repoURL string) string {
	if git.IsHTTPSURL(repoURL) {
		if parsedURL, err := url.Parse(repoURL); err == nil {
			if caPath, err := cert.GetCertBundlePathForRepository(parsedURL.Host); err == nil {
				return caPath
			} else {
				log.Warnf("Could not get cert bundle path for host '%s'", parsedURL.Host)
			}
		} else {
			// We don't fail if we cannot parse the URL, but log a warning in that
			// case. And we execute the command in a verbatim way.
			log.Warnf("Could not parse repo URL '%s'", repoURL)
		}
	}
	return ""
}

// CopySettingsFrom copies all repository settings from source to receiver
func (m *Repository) CopySettingsFrom(source *Repository) {
	if source != nil {
		m.EnableLFS = source.EnableLFS
		m.InsecureIgnoreHostKey = source.InsecureIgnoreHostKey
		m.Insecure = source.Insecure
		m.InheritedCreds = source.InheritedCreds
	}
}

type Repositories []*Repository

func (r Repositories) Filter(predicate func(r *Repository) bool) Repositories {
	var res Repositories
	for i := range r {
		repo := r[i]
		if predicate(repo) {
			res = append(res, repo)
		}
	}
	return res
}

// RepositoryList is a collection of Repositories.
type RepositoryList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           Repositories `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// RepositoryList is a collection of Repositories.
type RepoCredsList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []RepoCreds `json:"items" protobuf:"bytes,2,rep,name=items"`
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

// GnuPGPublicKey is a representation of a GnuPG public key
type GnuPGPublicKey struct {
	// KeyID in hexadecimal string format
	KeyID string `json:"keyID" protobuf:"bytes,1,opt,name=keyID"`
	// Fingerprint of the key
	Fingerprint string `json:"fingerprint,omitempty" protobuf:"bytes,2,opt,name=fingerprint"`
	// Owner identification
	Owner string `json:"owner,omitempty" protobuf:"bytes,3,opt,name=owner"`
	// Trust level
	Trust string `json:"trust,omitempty" protobuf:"bytes,4,opt,name=trust"`
	// Key sub type (e.g. rsa4096)
	SubType string `json:"subType,omitempty" protobuf:"bytes,5,opt,name=subType"`
	// Key data
	KeyData string `json:"keyData,omitempty" protobuf:"bytes,6,opt,name=keyData"`
}

// GnuPGPublicKeyList is a collection of GnuPGPublicKey objects
type GnuPGPublicKeyList struct {
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []GnuPGPublicKey `json:"items" protobuf:"bytes,2,rep,name=items"`
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
	Spec              AppProjectSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status            AppProjectStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
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

// GetJWTToken looks up the index of a JWTToken in a project by id (new token), if not then by the issue at time (old token)
func (p *AppProject) GetJWTTokenFromSpec(roleName string, issuedAt int64, id string) (*JWTToken, int, error) {
	// This is for backward compatibility. In the oder version, JWTTokens are stored under spec.role
	role, _, err := p.GetRoleByName(roleName)
	if err != nil {
		return nil, -1, err
	}

	if id != "" {
		for i, token := range role.JWTTokens {
			if id == token.ID {
				return &token, i, nil
			}
		}
	}

	if issuedAt != -1 {
		for i, token := range role.JWTTokens {
			if issuedAt == token.IssuedAt {
				return &token, i, nil
			}
		}
	}

	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", role.Name, issuedAt, p.Name)
}

// GetJWTToken looks up the index of a JWTToken in a project by id (new token), if not then by the issue at time (old token)
func (p *AppProject) GetJWTToken(roleName string, issuedAt int64, id string) (*JWTToken, int, error) {
	// This is for newer version, JWTTokens are stored under status
	if id != "" {
		for i, token := range p.Status.JWTTokensByRole[roleName].Items {
			if id == token.ID {
				return &token, i, nil
			}
		}

	}

	if issuedAt != -1 {
		for i, token := range p.Status.JWTTokensByRole[roleName].Items {
			if issuedAt == token.IssuedAt {
				return &token, i, nil
			}
		}
	}

	return nil, -1, fmt.Errorf("JWT token for role '%s' issued at '%d' does not exist in project '%s'", roleName, issuedAt, p.Name)
}

func (p AppProject) RemoveJWTToken(roleIndex int, issuedAt int64, id string) error {
	roleName := p.Spec.Roles[roleIndex].Name
	// For backward compatibility
	_, jwtTokenIndex, err1 := p.GetJWTTokenFromSpec(roleName, issuedAt, id)
	if err1 == nil {
		p.Spec.Roles[roleIndex].JWTTokens[jwtTokenIndex] = p.Spec.Roles[roleIndex].JWTTokens[len(p.Spec.Roles[roleIndex].JWTTokens)-1]
		p.Spec.Roles[roleIndex].JWTTokens = p.Spec.Roles[roleIndex].JWTTokens[:len(p.Spec.Roles[roleIndex].JWTTokens)-1]
	}

	// New location for storing JWTToken
	_, jwtTokenIndex, err2 := p.GetJWTToken(roleName, issuedAt, id)
	if err2 == nil {
		p.Status.JWTTokensByRole[roleName].Items[jwtTokenIndex] = p.Status.JWTTokensByRole[roleName].Items[len(p.Status.JWTTokensByRole[roleName].Items)-1]
		p.Status.JWTTokensByRole[roleName] = JWTTokens{Items: p.Status.JWTTokensByRole[roleName].Items[:len(p.Status.JWTTokensByRole[roleName].Items)-1]}
	}

	if err1 == nil || err2 == nil {
		//If we find this token from either places, we can say there are no error
		return nil
	} else {
		//If we could not locate this taken from either places, we can return any of the errors
		return err2
	}
}

func (p *AppProject) ValidateJWTTokenID(roleName string, id string) error {
	role, _, err := p.GetRoleByName(roleName)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	for _, token := range role.JWTTokens {
		if id == token.ID {
			return status.Errorf(codes.InvalidArgument, "Token id '%s' has been used. ", id)
		}
	}
	return nil
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

	if p.Spec.SyncWindows.HasWindows() {
		existingWindows := make(map[string]bool)
		for _, window := range p.Spec.SyncWindows {
			if _, ok := existingWindows[window.Kind+window.Schedule+window.Duration]; ok {
				return status.Errorf(codes.AlreadyExists, "window '%s':'%s':'%s' already exists, update or edit", window.Kind, window.Schedule, window.Duration)
			}
			err := window.Validate()
			if err != nil {
				return err
			}
			if len(window.Applications) == 0 && len(window.Namespaces) == 0 && len(window.Clusters) == 0 {
				return status.Errorf(codes.OutOfRange, "window '%s':'%s':'%s' requires one of application, cluster or namespace", window.Kind, window.Schedule, window.Duration)
			}
			existingWindows[window.Kind+window.Schedule+window.Duration] = true
		}
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

var validActionPatterns = []*regexp.Regexp{
	regexp.MustCompile("action/.*"),
}

func isValidAction(action string) bool {
	if validActions[action] {
		return true
	}
	for i := range validActionPatterns {
		if validActionPatterns[i].MatchString(action) {
			return true
		}
	}
	return false
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
	objectRegexp, err := regexp.Compile(fmt.Sprintf(`^%s/[*\w-.]+$`, proj))
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

// OrphanedResourcesMonitorSettings holds settings of orphaned resources monitoring
type OrphanedResourcesMonitorSettings struct {
	// Warn indicates if warning condition should be created for apps which have orphaned resources
	Warn   *bool                 `json:"warn,omitempty" protobuf:"bytes,1,name=warn"`
	Ignore []OrphanedResourceKey `json:"ignore,omitempty" protobuf:"bytes,2,opt,name=ignore"`
}

type OrphanedResourceKey struct {
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind  string `json:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
	Name  string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
}

func (s *OrphanedResourcesMonitorSettings) IsWarn() bool {
	return s.Warn == nil || *s.Warn
}

// SignatureKey is the specification of a key required to verify commit signatures with
type SignatureKey struct {
	// The ID of the key in hexadecimal notation
	KeyID string `json:"keyID" protobuf:"bytes,1,name=keyID"`
}

// AppProjectSpec is the specification of an AppProject
type AppProjectSpec struct {
	// SourceRepos contains list of repository URLs which can be used for deployment
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
	// OrphanedResources specifies if controller should monitor orphaned resources of apps in this project
	OrphanedResources *OrphanedResourcesMonitorSettings `json:"orphanedResources,omitempty" protobuf:"bytes,7,opt,name=orphanedResources"`
	// SyncWindows controls when syncs can be run for apps in this project
	SyncWindows SyncWindows `json:"syncWindows,omitempty" protobuf:"bytes,8,opt,name=syncWindows"`
	// NamespaceResourceWhitelist contains list of whitelisted namespace level resources
	NamespaceResourceWhitelist []metav1.GroupKind `json:"namespaceResourceWhitelist,omitempty" protobuf:"bytes,9,opt,name=namespaceResourceWhitelist"`
	// List of PGP key IDs that commits to be synced to must be signed with
	SignatureKeys []SignatureKey `json:"signatureKeys,omitempty" protobuf:"bytes,10,opt,name=signatureKeys"`
	// ClusterResourceBlacklist contains list of blacklisted cluster level resources
	ClusterResourceBlacklist []metav1.GroupKind `json:"clusterResourceBlacklist,omitempty" protobuf:"bytes,11,opt,name=clusterResourceBlacklist"`
}

// SyncWindows is a collection of sync windows in this project
type SyncWindows []*SyncWindow

// SyncWindow contains the kind, time, duration and attributes that are used to assign the syncWindows to apps
type SyncWindow struct {
	// Kind defines if the window allows or blocks syncs
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	// Schedule is the time the window will begin, specified in cron format
	Schedule string `json:"schedule,omitempty" protobuf:"bytes,2,opt,name=schedule"`
	// Duration is the amount of time the sync window will be open
	Duration string `json:"duration,omitempty" protobuf:"bytes,3,opt,name=duration"`
	// Applications contains a list of applications that the window will apply to
	Applications []string `json:"applications,omitempty" protobuf:"bytes,4,opt,name=applications"`
	// Namespaces contains a list of namespaces that the window will apply to
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,5,opt,name=namespaces"`
	// Clusters contains a list of clusters that the window will apply to
	Clusters []string `json:"clusters,omitempty" protobuf:"bytes,6,opt,name=clusters"`
	// ManualSync enables manual syncs when they would otherwise be blocked
	ManualSync bool `json:"manualSync,omitempty" protobuf:"bytes,7,opt,name=manualSync"`
}

func (s *SyncWindows) HasWindows() bool {
	return s != nil && len(*s) > 0
}

func (s *SyncWindows) Active() *SyncWindows {
	return s.active(time.Now())
}

func (s *SyncWindows) active(currentTime time.Time) *SyncWindows {

	// If SyncWindows.Active() is called outside of a UTC locale, it should be
	// first converted to UTC before we scan through the SyncWindows.
	currentTime = currentTime.In(time.UTC)

	if s.HasWindows() {
		var active SyncWindows
		specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		for _, w := range *s {
			schedule, _ := specParser.Parse(w.Schedule)
			duration, _ := time.ParseDuration(w.Duration)
			nextWindow := schedule.Next(currentTime.Add(-duration))
			if nextWindow.Before(currentTime) {
				active = append(active, w)
			}
		}
		if len(active) > 0 {
			return &active
		}
	}
	return nil
}

func (s *SyncWindows) InactiveAllows() *SyncWindows {
	return s.inactiveAllows(time.Now())
}

func (s *SyncWindows) inactiveAllows(currentTime time.Time) *SyncWindows {

	// If SyncWindows.InactiveAllows() is called outside of a UTC locale, it should be
	// first converted to UTC before we scan through the SyncWindows.
	currentTime = currentTime.In(time.UTC)

	if s.HasWindows() {
		var inactive SyncWindows
		specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		for _, w := range *s {
			if w.Kind == "allow" {
				schedule, sErr := specParser.Parse(w.Schedule)
				duration, dErr := time.ParseDuration(w.Duration)
				nextWindow := schedule.Next(currentTime.Add(-duration))
				if !nextWindow.Before(currentTime) && sErr == nil && dErr == nil {
					inactive = append(inactive, w)
				}
			}
		}
		if len(inactive) > 0 {
			return &inactive
		}
	}
	return nil
}

func (s *AppProjectSpec) AddWindow(knd string, sch string, dur string, app []string, ns []string, cl []string, ms bool) error {
	if len(knd) == 0 || len(sch) == 0 || len(dur) == 0 {
		return fmt.Errorf("cannot create window: require kind, schedule, duration and one or more of applications, namespaces and clusters")

	}
	window := &SyncWindow{
		Kind:       knd,
		Schedule:   sch,
		Duration:   dur,
		ManualSync: ms,
	}

	if len(app) > 0 {
		window.Applications = app
	}
	if len(ns) > 0 {
		window.Namespaces = ns
	}
	if len(cl) > 0 {
		window.Clusters = cl
	}

	err := window.Validate()
	if err != nil {
		return err
	}

	s.SyncWindows = append(s.SyncWindows, window)

	return nil

}

func (s *AppProjectSpec) DeleteWindow(id int) error {
	var exists bool
	for i := range s.SyncWindows {
		if i == id {
			exists = true
			s.SyncWindows = append(s.SyncWindows[:i], s.SyncWindows[i+1:]...)
			break
		}
	}
	if !exists {
		return fmt.Errorf("window with id '%s' not found", strconv.Itoa(id))
	}
	return nil
}

func (w *SyncWindows) Matches(app *Application) *SyncWindows {
	if w.HasWindows() {
		var matchingWindows SyncWindows
		for _, w := range *w {
			if len(w.Applications) > 0 {
				for _, a := range w.Applications {
					if globMatch(a, app.Name) {
						matchingWindows = append(matchingWindows, w)
						break
					}
				}
			}
			if len(w.Clusters) > 0 {
				for _, c := range w.Clusters {
					if globMatch(c, app.Spec.Destination.Server) {
						matchingWindows = append(matchingWindows, w)
						break
					}
				}
			}
			if len(w.Namespaces) > 0 {
				for _, n := range w.Namespaces {
					if globMatch(n, app.Spec.Destination.Namespace) {
						matchingWindows = append(matchingWindows, w)
						break
					}
				}
			}
		}
		if len(matchingWindows) > 0 {
			return &matchingWindows
		}
	}
	return nil
}

func (w *SyncWindows) CanSync(isManual bool) bool {
	if !w.HasWindows() {
		return true
	}

	var allowActive, denyActive, manualEnabled bool
	active := w.Active()
	denyActive, manualEnabled = active.hasDeny()
	allowActive = active.hasAllow()

	if !denyActive {
		if !allowActive {
			if isManual && w.InactiveAllows().manualEnabled() {
				return true
			}
		} else {
			return true
		}
	} else {
		if isManual && manualEnabled {
			return true
		}
	}

	return false
}

func (w *SyncWindows) hasDeny() (bool, bool) {
	if !w.HasWindows() {
		return false, false
	}
	var denyActive, manualEnabled bool
	for _, a := range *w {
		if a.Kind == "deny" {
			if !denyActive {
				manualEnabled = a.ManualSync
			} else {
				if manualEnabled {
					if !a.ManualSync {
						manualEnabled = a.ManualSync
					}
				}
			}
			denyActive = true
		}
	}
	return denyActive, manualEnabled
}

func (w *SyncWindows) hasAllow() bool {
	if !w.HasWindows() {
		return false
	}
	for _, a := range *w {
		if a.Kind == "allow" {
			return true
		}
	}
	return false
}

func (w *SyncWindows) manualEnabled() bool {
	if !w.HasWindows() {
		return false
	}
	for _, s := range *w {
		if s.ManualSync {
			return true
		}
	}
	return false
}

func (w SyncWindow) Active() bool {
	return w.active(time.Now())
}

func (w SyncWindow) active(currentTime time.Time) bool {

	// If SyncWindow.Active() is called outside of a UTC locale, it should be
	// first converted to UTC before search
	currentTime = currentTime.UTC()

	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, _ := specParser.Parse(w.Schedule)
	duration, _ := time.ParseDuration(w.Duration)

	nextWindow := schedule.Next(currentTime.Add(-duration))
	return nextWindow.Before(currentTime)
}

func (w *SyncWindow) Update(s string, d string, a []string, n []string, c []string) error {
	if len(s) == 0 && len(d) == 0 && len(a) == 0 && len(n) == 0 && len(c) == 0 {
		return fmt.Errorf("cannot update: require one or more of schedule, duration, application, namespace, or cluster")
	}

	if len(s) > 0 {
		w.Schedule = s
	}

	if len(d) > 0 {
		w.Duration = d
	}

	if len(a) > 0 {
		w.Applications = a
	}
	if len(n) > 0 {
		w.Namespaces = n
	}
	if len(c) > 0 {
		w.Clusters = c
	}

	return nil
}

func (w *SyncWindow) Validate() error {
	if w.Kind != "allow" && w.Kind != "deny" {
		return fmt.Errorf("kind '%s' mismatch: can only be allow or deny", w.Kind)
	}
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := specParser.Parse(w.Schedule)
	if err != nil {
		return fmt.Errorf("cannot parse schedule '%s': %s", w.Schedule, err)
	}
	_, err = time.ParseDuration(w.Duration)
	if err != nil {
		return fmt.Errorf("cannot parse duration '%s': %s", w.Duration, err)
	}
	return nil
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
	IssuedAt  int64  `json:"iat" protobuf:"int64,1,opt,name=iat"`
	ExpiresAt int64  `json:"exp,omitempty" protobuf:"int64,2,opt,name=exp"`
	ID        string `json:"id,omitempty" protobuf:"bytes,3,opt,name=id"`
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
	// BinaryPath holds optional path to kustomize binary
	BinaryPath string `protobuf:"bytes,2,opt,name=binaryPath"`
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

// CascadedDeletion indicates if resources finalizer is set and controller should delete app resources before deleting app
func (app *Application) CascadedDeletion() bool {
	return getFinalizerIndex(app.ObjectMeta, common.ResourcesFinalizerName) > -1
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
	setFinalizer(&app.ObjectMeta, common.ResourcesFinalizerName, prune)
}

// SetConditions updates the application status conditions for a subset of evaluated types.
// If the application has a pre-existing condition of a type that is not in the evaluated list,
// it will be preserved. If the application has a pre-existing condition of a type that
// is in the evaluated list, but not in the incoming conditions list, it will be removed.
func (status *ApplicationStatus) SetConditions(conditions []ApplicationCondition, evaluatedTypes map[ApplicationConditionType]bool) {
	appConditions := make([]ApplicationCondition, 0)
	now := metav1.Now()
	for i := 0; i < len(status.Conditions); i++ {
		condition := status.Conditions[i]
		if _, ok := evaluatedTypes[condition.Type]; !ok {
			if condition.LastTransitionTime == nil {
				condition.LastTransitionTime = &now
			}
			appConditions = append(appConditions, condition)
		}
	}
	for i := range conditions {
		condition := conditions[i]
		if condition.LastTransitionTime == nil {
			condition.LastTransitionTime = &now
		}
		eci := findConditionIndexByType(status.Conditions, condition.Type)
		if eci >= 0 && status.Conditions[eci].Message == condition.Message {
			// If we already have a condition of this type, only update the timestamp if something
			// has changed.
			appConditions = append(appConditions, status.Conditions[eci])
		} else {
			// Otherwise we use the new incoming condition with an updated timestamp:
			appConditions = append(appConditions, condition)
		}
	}
	sort.Slice(appConditions, func(i, j int) bool {
		left := appConditions[i]
		right := appConditions[j]
		return fmt.Sprintf("%s/%s/%v", left.Type, left.Message, left.LastTransitionTime) < fmt.Sprintf("%s/%s/%v", right.Type, right.Message, right.LastTransitionTime)
	})
	status.Conditions = appConditions
}

func findConditionIndexByType(conditions []ApplicationCondition, t ApplicationConditionType) int {
	for i := range conditions {
		if conditions[i].Type == t {
			return i
		}
	}
	return -1
}

// GetErrorConditions returns list of application error conditions
func (status *ApplicationStatus) GetConditions(conditionTypes map[ApplicationConditionType]bool) []ApplicationCondition {
	result := make([]ApplicationCondition, 0)
	for i := range status.Conditions {
		condition := status.Conditions[i]
		if ok := conditionTypes[condition.Type]; ok {
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
	// ignore destination cluster name and isServerInferred fields during comparison
	// since server URL is inferred from cluster name
	if dest.isServerInferred {
		dest.Server = ""
		dest.isServerInferred = false
	}

	if other.isServerInferred {
		other.Server = ""
		other.isServerInferred = false
	}
	return reflect.DeepEqual(dest, other)
}

// GetProject returns the application's project. This is preferred over spec.Project which may be empty
func (spec ApplicationSpec) GetProject() string {
	if spec.Project == "" {
		return common.DefaultAppProjectName
	}
	return spec.Project
}

func (spec ApplicationSpec) GetRevisionHistoryLimit() int {
	if spec.RevisionHistoryLimit != nil {
		return int(*spec.RevisionHistoryLimit)
	}
	return common.RevisionHistoryLimit
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

// IsGroupKindPermitted validates if the given resource group/kind is permitted to be deployed in the project
func (proj AppProject) IsGroupKindPermitted(gk schema.GroupKind, namespaced bool) bool {
	var isWhiteListed, isBlackListed bool
	res := metav1.GroupKind{Group: gk.Group, Kind: gk.Kind}

	if namespaced {
		isWhiteListed = len(proj.Spec.NamespaceResourceWhitelist) == 0 || isResourceInList(res, proj.Spec.NamespaceResourceWhitelist)
		isBlackListed = isResourceInList(res, proj.Spec.NamespaceResourceBlacklist)
		return isWhiteListed && !isBlackListed
	}

	isWhiteListed = len(proj.Spec.ClusterResourceWhitelist) == 0 || isResourceInList(res, proj.Spec.ClusterResourceWhitelist)
	isBlackListed = isResourceInList(res, proj.Spec.ClusterResourceBlacklist)
	return isWhiteListed && !isBlackListed
}

func (proj AppProject) IsLiveResourcePermitted(un *unstructured.Unstructured, server string) bool {
	if !proj.IsGroupKindPermitted(un.GroupVersionKind().GroupKind(), un.GetNamespace() != "") {
		return false
	}
	if un.GetNamespace() != "" {
		return proj.IsDestinationPermitted(ApplicationDestination{Server: server, Namespace: un.GetNamespace()})
	}
	return true
}

// getFinalizerIndex returns finalizer index in the list of object finalizers or -1 if finalizer does not exist
func getFinalizerIndex(meta metav1.ObjectMeta, name string) int {
	for i, finalizer := range meta.Finalizers {
		if finalizer == name {
			return i
		}
	}
	return -1
}

// setFinalizer adds or removes finalizer with the specified name
func setFinalizer(meta *metav1.ObjectMeta, name string, exist bool) {
	index := getFinalizerIndex(*meta, name)
	if exist != (index > -1) {
		if index > -1 {
			meta.Finalizers[index] = meta.Finalizers[len(meta.Finalizers)-1]
			meta.Finalizers = meta.Finalizers[:len(meta.Finalizers)-1]
		} else {
			meta.Finalizers = append(meta.Finalizers, name)
		}
	}
}

func (proj AppProject) HasFinalizer() bool {
	return getFinalizerIndex(proj.ObjectMeta, common.ResourcesFinalizerName) > -1
}

func (proj *AppProject) RemoveFinalizer() {
	setFinalizer(&proj.ObjectMeta, common.ResourcesFinalizerName, false)
}

func globMatch(pattern string, val string, separators ...rune) bool {
	if pattern == "*" {
		return true
	}
	return glob.Match(pattern, val, separators...)
}

// IsSourcePermitted validates if the provided application's source is a one of the allowed sources for the project.
func (proj AppProject) IsSourcePermitted(src ApplicationSource) bool {
	srcNormalized := git.NormalizeGitURL(src.RepoURL)
	for _, repoURL := range proj.Spec.SourceRepos {
		normalized := git.NormalizeGitURL(repoURL)
		if globMatch(normalized, srcNormalized, '/') {
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

// SetK8SConfigDefaults sets Kubernetes REST config default settings
func SetK8SConfigDefaults(config *rest.Config) error {
	config.QPS = common.K8sClientConfigQPS
	config.Burst = common.K8sClientConfigBurst
	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return err
	}

	dial := (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport := utilnet.SetTransportDefaults(&http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        common.K8sMaxIdleConnections,
		MaxIdleConnsPerHost: common.K8sMaxIdleConnections,
		MaxConnsPerHost:     common.K8sMaxIdleConnections,
		DialContext:         dial,
		DisableCompression:  config.DisableCompression,
	})
	tr, err := rest.HTTPWrappersForConfig(config, transport)
	if err != nil {
		return err
	}

	// set default tls config and remove auth/exec provides since we use it in a custom transport
	config.TLSClientConfig = rest.TLSClientConfig{}
	config.AuthProvider = nil
	config.ExecProvider = nil

	config.Transport = tr
	return nil
}

// RawRestConfig returns a go-client REST config from cluster that might be serialized into the file using kube.WriteKubeConfig method.
func (c *Cluster) RawRestConfig() *rest.Config {
	var config *rest.Config
	var err error
	if c.Server == common.KubernetesInternalAPIServerAddr && os.Getenv(common.EnvVarFakeInClusterConfig) == "true" {
		conf, exists := os.LookupEnv("KUBECONFIG")
		if exists {
			config, err = clientcmd.BuildConfigFromFlags("", conf)
		} else {
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		}
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
			args := []string{"eks", "get-token", "--cluster-name", c.Config.AWSAuthConfig.ClusterName}
			if c.Config.AWSAuthConfig.RoleARN != "" {
				args = append(args, "--role-arn", c.Config.AWSAuthConfig.RoleARN)
			}
			config = &rest.Config{
				Host:            c.Server,
				TLSClientConfig: tlsClientConfig,
				ExecProvider: &api.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1alpha1",
					Command:    "aws",
					Args:       args,
				},
			}
		} else if c.Config.ExecProviderConfig != nil {
			var env []api.ExecEnvVar
			if c.Config.ExecProviderConfig.Env != nil {
				for key, value := range c.Config.ExecProviderConfig.Env {
					env = append(env, api.ExecEnvVar{
						Name:  key,
						Value: value,
					})
				}
			}
			config = &rest.Config{
				Host:            c.Server,
				TLSClientConfig: tlsClientConfig,
				ExecProvider: &api.ExecConfig{
					APIVersion:  c.Config.ExecProviderConfig.APIVersion,
					Command:     c.Config.ExecProviderConfig.Command,
					Args:        c.Config.ExecProviderConfig.Args,
					Env:         env,
					InstallHint: c.Config.ExecProviderConfig.InstallHint,
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
	return config
}

// RESTConfig returns a go-client REST config from cluster with tuned throttling and HTTP client settings.
func (c *Cluster) RESTConfig() *rest.Config {
	config := c.RawRestConfig()
	err := SetK8SConfigDefaults(config)
	if err != nil {
		panic(fmt.Sprintf("Unable to apply K8s REST config defaults: %v", err))
	}
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

func (d *ApplicationDestination) SetInferredServer(server string) {
	d.isServerInferred = true
	d.Server = server
}

func (d *ApplicationDestination) IsServerInferred() bool {
	return d.isServerInferred
}

func (d *ApplicationDestination) MarshalJSON() ([]byte, error) {
	type Alias ApplicationDestination
	dest := d
	if d.isServerInferred {
		dest = dest.DeepCopy()
		dest.Server = ""
	}
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(dest)})
}

func (proj *AppProject) NormalizeJWTTokens() bool {
	needNormalize := false
	for i, role := range proj.Spec.Roles {
		for j, token := range role.JWTTokens {
			if token.ID == "" {
				token.ID = strconv.FormatInt(token.IssuedAt, 10)
				role.JWTTokens[j] = token
				needNormalize = true
			}
		}
		proj.Spec.Roles[i] = role
	}
	for _, roleTokenEntry := range proj.Status.JWTTokensByRole {
		for j, token := range roleTokenEntry.Items {
			if token.ID == "" {
				token.ID = strconv.FormatInt(token.IssuedAt, 10)
				roleTokenEntry.Items[j] = token
				needNormalize = true
			}
		}
	}
	needSync := syncJWTTokenBetweenStatusAndSpec(proj)
	return needNormalize || needSync
}

func syncJWTTokenBetweenStatusAndSpec(proj *AppProject) bool {
	needSync := false
	for roleIndex, role := range proj.Spec.Roles {
		tokensInSpec := role.JWTTokens
		tokensInStatus := []JWTToken{}
		if proj.Status.JWTTokensByRole == nil {
			tokensByRole := make(map[string]JWTTokens)
			proj.Status.JWTTokensByRole = tokensByRole
		} else {
			tokensInStatus = proj.Status.JWTTokensByRole[role.Name].Items
		}
		tokens := jwtTokensCombine(tokensInStatus, tokensInSpec)

		sort.Slice(proj.Spec.Roles[roleIndex].JWTTokens, func(i, j int) bool {
			return proj.Spec.Roles[roleIndex].JWTTokens[i].IssuedAt > proj.Spec.Roles[roleIndex].JWTTokens[j].IssuedAt
		})
		sort.Slice(proj.Status.JWTTokensByRole[role.Name].Items, func(i, j int) bool {
			return proj.Status.JWTTokensByRole[role.Name].Items[i].IssuedAt > proj.Status.JWTTokensByRole[role.Name].Items[j].IssuedAt
		})
		if !cmp.Equal(tokens, proj.Spec.Roles[roleIndex].JWTTokens) || !cmp.Equal(tokens, proj.Status.JWTTokensByRole[role.Name].Items) {
			needSync = true
		}

		proj.Spec.Roles[roleIndex].JWTTokens = tokens
		proj.Status.JWTTokensByRole[role.Name] = JWTTokens{Items: tokens}

	}
	return needSync
}

func jwtTokensCombine(tokens1 []JWTToken, tokens2 []JWTToken) []JWTToken {
	tokensMap := make(map[string]JWTToken)
	for _, token := range append(tokens1, tokens2...) {
		tokensMap[token.ID] = token
	}

	var tokens []JWTToken
	for _, v := range tokensMap {
		tokens = append(tokens, v)
	}

	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].IssuedAt > tokens[j].IssuedAt
	})
	return tokens
}
