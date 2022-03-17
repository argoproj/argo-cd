package v1alpha1

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
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

	"github.com/argoproj/argo-cd/v2/util/collections"
	"github.com/argoproj/argo-cd/v2/util/helm"
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
	// Source is a reference to the location of the application's manifests or chart
	Source ApplicationSource `json:"source" protobuf:"bytes,1,opt,name=source"`
	// Destination is a reference to the target Kubernetes server and namespace
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,name=destination"`
	// Project is a reference to the project this application belongs to.
	// The empty string means that application belongs to the 'default' project.
	Project string `json:"project" protobuf:"bytes,3,name=project"`
	// SyncPolicy controls when and how a sync will be performed
	SyncPolicy *SyncPolicy `json:"syncPolicy,omitempty" protobuf:"bytes,4,name=syncPolicy"`
	// IgnoreDifferences is a list of resources and their fields which should be ignored during comparison
	IgnoreDifferences []ResourceIgnoreDifferences `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,name=ignoreDifferences"`
	// Info contains a list of information (URLs, email addresses, and plain text) that relates to the application
	Info []Info `json:"info,omitempty" protobuf:"bytes,6,name=info"`
	// RevisionHistoryLimit limits the number of items kept in the application's revision history, which is used for informational purposes as well as for rollbacks to previous versions.
	// This should only be changed in exceptional circumstances.
	// Setting to zero will store no history. This will reduce storage used.
	// Increasing will increase the space used to store the history, so we do not recommend increasing it.
	// Default is 10.
	RevisionHistoryLimit *int64 `json:"revisionHistoryLimit,omitempty" protobuf:"bytes,7,name=revisionHistoryLimit"`
}

type TrackingMethod string

// ResourceIgnoreDifferences contains resource filter and list of json paths which should be ignored during comparison with live state.
type ResourceIgnoreDifferences struct {
	Group             string   `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind              string   `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	Name              string   `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
	Namespace         string   `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	JSONPointers      []string `json:"jsonPointers,omitempty" protobuf:"bytes,5,opt,name=jsonPointers"`
	JQPathExpressions []string `json:"jqPathExpressions,omitempty" protobuf:"bytes,6,opt,name=jqPathExpressions"`
	// ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
	// desired state defined in the SCM and won't be displayed in diffs
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty" protobuf:"bytes,7,opt,name=managedFieldsManagers"`
}

// EnvEntry represents an entry in the application's environment
type EnvEntry struct {
	// Name is the name of the variable, usually expressed in uppercase
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Value is the value of the variable
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
}

// IsZero returns true if a variable is considered empty or unset
func (a *EnvEntry) IsZero() bool {
	return a == nil || a.Name == "" && a.Value == ""
}

// NewEnvEntry parses a string in format name=value and returns an EnvEntry object
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

// Env is a list of environment variable entries
type Env []*EnvEntry

// IsZero returns true if a list of variables is considered empty
func (e Env) IsZero() bool {
	return len(e) == 0
}

// Environ returns a list of environment variables in name=value format from a list of variables
func (e Env) Environ() []string {
	var environ []string
	for _, item := range e {
		if !item.IsZero() {
			environ = append(environ, fmt.Sprintf("%s=%s", item.Name, item.Value))
		}
	}
	return environ
}

// Envsubst interpolates variable references in a string from a list of variables
func (e Env) Envsubst(s string) string {
	valByEnv := map[string]string{}
	for _, item := range e {
		valByEnv[item.Name] = item.Value
	}
	return os.Expand(s, func(s string) string {
		// allow escaping $ with $$
		if s == "$" {
			return "$"
		} else {
			return valByEnv[s]
		}
	})
}

// ApplicationSource contains all required information about the source of an application
type ApplicationSource struct {
	// RepoURL is the URL to the repository (Git or Helm) that contains the application manifests
	RepoURL string `json:"repoURL" protobuf:"bytes,1,opt,name=repoURL"`
	// Path is a directory path within the Git repository, and is only valid for applications sourced from Git.
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
	// TargetRevision defines the revision of the source to sync the application to.
	// In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD.
	// In case of Helm, this is a semver tag for the Chart's version.
	TargetRevision string `json:"targetRevision,omitempty" protobuf:"bytes,4,opt,name=targetRevision"`
	// Helm holds helm specific options
	Helm *ApplicationSourceHelm `json:"helm,omitempty" protobuf:"bytes,7,opt,name=helm"`
	// Kustomize holds kustomize specific options
	Kustomize *ApplicationSourceKustomize `json:"kustomize,omitempty" protobuf:"bytes,8,opt,name=kustomize"`
	// Directory holds path/directory specific options
	Directory *ApplicationSourceDirectory `json:"directory,omitempty" protobuf:"bytes,10,opt,name=directory"`
	// ConfigManagementPlugin holds config management plugin specific options
	Plugin *ApplicationSourcePlugin `json:"plugin,omitempty" protobuf:"bytes,11,opt,name=plugin"`
	// Chart is a Helm chart name, and must be specified for applications sourced from a Helm repo.
	Chart string `json:"chart,omitempty" protobuf:"bytes,12,opt,name=chart"`
}

// AllowsConcurrentProcessing returns true if given application source can be processed concurrently
func (a *ApplicationSource) AllowsConcurrentProcessing() bool {
	switch {
	// Kustomize with parameters requires changing kustomization.yaml file
	case a.Kustomize != nil:
		return a.Kustomize.AllowsConcurrentProcessing()
	}
	return true
}

// IsHelm returns true when the application source is of type Helm
func (a *ApplicationSource) IsHelm() bool {
	return a.Chart != ""
}

// IsHelmOci returns true when the application source is of type Helm OCI
func (a *ApplicationSource) IsHelmOci() bool {
	if a.Chart == "" {
		return false
	}
	return helm.IsHelmOciRepo(a.RepoURL)
}

// IsZero returns true if the application source is considered empty
func (a *ApplicationSource) IsZero() bool {
	return a == nil ||
		a.RepoURL == "" &&
			a.Path == "" &&
			a.TargetRevision == "" &&
			a.Helm.IsZero() &&
			a.Kustomize.IsZero() &&
			a.Directory.IsZero() &&
			a.Plugin.IsZero()
}

// ApplicationSourceType specifies the type of the application's source
type ApplicationSourceType string

const (
	ApplicationSourceTypeHelm      ApplicationSourceType = "Helm"
	ApplicationSourceTypeKustomize ApplicationSourceType = "Kustomize"
	ApplicationSourceTypeDirectory ApplicationSourceType = "Directory"
	ApplicationSourceTypePlugin    ApplicationSourceType = "Plugin"
)

// RefreshType specifies how to refresh the sources of a given application
type RefreshType string

const (
	RefreshTypeNormal RefreshType = "normal"
	RefreshTypeHard   RefreshType = "hard"
)

// ApplicationSourceHelm holds helm specific options
type ApplicationSourceHelm struct {
	// ValuesFiles is a list of Helm value files to use when generating a template
	ValueFiles []string `json:"valueFiles,omitempty" protobuf:"bytes,1,opt,name=valueFiles"`
	// Parameters is a list of Helm parameters which are passed to the helm template command upon manifest generation
	Parameters []HelmParameter `json:"parameters,omitempty" protobuf:"bytes,2,opt,name=parameters"`
	// ReleaseName is the Helm release name to use. If omitted it will use the application name
	ReleaseName string `json:"releaseName,omitempty" protobuf:"bytes,3,opt,name=releaseName"`
	// Values specifies Helm values to be passed to helm template, typically defined as a block
	Values string `json:"values,omitempty" protobuf:"bytes,4,opt,name=values"`
	// FileParameters are file parameters to the helm template
	FileParameters []HelmFileParameter `json:"fileParameters,omitempty" protobuf:"bytes,5,opt,name=fileParameters"`
	// Version is the Helm version to use for templating (either "2" or "3")
	Version string `json:"version,omitempty" protobuf:"bytes,6,opt,name=version"`
	// PassCredentials pass credentials to all domains (Helm's --pass-credentials)
	PassCredentials bool `json:"passCredentials,omitempty" protobuf:"bytes,7,opt,name=passCredentials"`
	// IgnoreMissingValueFiles prevents helm template from failing when valueFiles do not exist locally by not appending them to helm template --values
	IgnoreMissingValueFiles bool `json:"ignoreMissingValueFiles,omitempty" protobuf:"bytes,8,opt,name=ignoreMissingValueFiles"`
	// SkipCrds skips custom resource definition installation step (Helm's --skip-crds)
	SkipCrds bool `json:"skipCrds,omitempty" protobuf:"bytes,9,opt,name=skipCrds"`
}

// HelmParameter is a parameter that's passed to helm template during manifest generation
type HelmParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is the value for the Helm parameter
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// ForceString determines whether to tell Helm to interpret booleans and numbers as strings
	ForceString bool `json:"forceString,omitempty" protobuf:"bytes,3,opt,name=forceString"`
}

// HelmFileParameter is a file parameter that's passed to helm template during manifest generation
type HelmFileParameter struct {
	// Name is the name of the Helm parameter
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Path is the path to the file containing the values for the Helm parameter
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
}

var helmParameterRx = regexp.MustCompile(`([^\\]),`)

// NewHelmParameter parses a string in format name=value into a HelmParameter object and returns it
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

// NewHelmFileParameter parses a string in format name=value into a HelmFileParameter object and returns it
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

// AddParameter adds a HelmParameter to the application source. If a parameter with the same name already
// exists, its value will be overwritten. Otherwise, the HelmParameter will be appended as a new entry.
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

// AddFileParameter adds a HelmFileParameter to the application source. If a file parameter with the same name already
// exists, its value will be overwritten. Otherwise, the HelmFileParameter will be appended as a new entry.
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

// IsZero Returns true if the Helm options in an application source are considered zero
func (h *ApplicationSourceHelm) IsZero() bool {
	return h == nil || (h.Version == "") && (h.ReleaseName == "") && len(h.ValueFiles) == 0 && len(h.Parameters) == 0 && len(h.FileParameters) == 0 && h.Values == "" && !h.PassCredentials && !h.IgnoreMissingValueFiles && !h.SkipCrds
}

// KustomizeImage represents a Kustomize image definition in the format [old_image_name=]<image_name>:<image_tag>
type KustomizeImage string

func (i KustomizeImage) delim() string {
	for _, d := range []string{"=", ":", "@"} {
		if strings.Contains(string(i), d) {
			return d
		}
	}
	return ":"
}

// Match returns true if the image name matches (i.e. up to the first delimiter)
func (i KustomizeImage) Match(j KustomizeImage) bool {
	delim := j.delim()
	if !strings.Contains(string(j), delim) {
		return false
	}
	return strings.HasPrefix(string(i), strings.Split(string(j), delim)[0])
}

// KustomizeImages is a list of Kustomize images
type KustomizeImages []KustomizeImage

// Find returns a positive integer representing the index in the list of images
func (images KustomizeImages) Find(image KustomizeImage) int {
	for i, a := range images {
		if a.Match(image) {
			return i
		}
	}
	return -1
}

// ApplicationSourceKustomize holds options specific to an Application source specific to Kustomize
type ApplicationSourceKustomize struct {
	// NamePrefix is a prefix appended to resources for Kustomize apps
	NamePrefix string `json:"namePrefix,omitempty" protobuf:"bytes,1,opt,name=namePrefix"`
	// NameSuffix is a suffix appended to resources for Kustomize apps
	NameSuffix string `json:"nameSuffix,omitempty" protobuf:"bytes,2,opt,name=nameSuffix"`
	// Images is a list of Kustomize image override specifications
	Images KustomizeImages `json:"images,omitempty" protobuf:"bytes,3,opt,name=images"`
	// CommonLabels is a list of additional labels to add to rendered manifests
	CommonLabels map[string]string `json:"commonLabels,omitempty" protobuf:"bytes,4,opt,name=commonLabels"`
	// Version controls which version of Kustomize to use for rendering manifests
	Version string `json:"version,omitempty" protobuf:"bytes,5,opt,name=version"`
	// CommonAnnotations is a list of additional annotations to add to rendered manifests
	CommonAnnotations map[string]string `json:"commonAnnotations,omitempty" protobuf:"bytes,6,opt,name=commonAnnotations"`
	// ForceCommonLabels specifies whether to force applying common labels to resources for Kustomize apps
	ForceCommonLabels bool `json:"forceCommonLabels,omitempty" protobuf:"bytes,7,opt,name=forceCommonLabels"`
	// ForceCommonAnnotations specifies whether to force applying common annotations to resources for Kustomize apps
	ForceCommonAnnotations bool `json:"forceCommonAnnotations,omitempty" protobuf:"bytes,8,opt,name=forceCommonAnnotations"`
}

// AllowsConcurrentProcessing returns true if multiple processes can run Kustomize builds on the same source at the same time
func (k *ApplicationSourceKustomize) AllowsConcurrentProcessing() bool {
	return len(k.Images) == 0 &&
		len(k.CommonLabels) == 0 &&
		k.NamePrefix == "" &&
		k.NameSuffix == ""
}

// IsZero returns true when the Kustomize options are considered empty
func (k *ApplicationSourceKustomize) IsZero() bool {
	return k == nil ||
		k.NamePrefix == "" &&
			k.NameSuffix == "" &&
			k.Version == "" &&
			len(k.Images) == 0 &&
			len(k.CommonLabels) == 0 &&
			len(k.CommonAnnotations) == 0
}

// MergeImage merges a new Kustomize image identifier in to a list of images
func (k *ApplicationSourceKustomize) MergeImage(image KustomizeImage) {
	i := k.Images.Find(image)
	if i >= 0 {
		k.Images[i] = image
	} else {
		k.Images = append(k.Images, image)
	}
}

// JsonnetVar represents a variable to be passed to jsonnet during manifest generation
type JsonnetVar struct {
	Name  string `json:"name" protobuf:"bytes,1,opt,name=name"`
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
	Code  bool   `json:"code,omitempty" protobuf:"bytes,3,opt,name=code"`
}

// NewJsonnetVar parses a Jsonnet variable from a string in the format name=value
func NewJsonnetVar(s string, code bool) JsonnetVar {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) == 2 {
		return JsonnetVar{Name: parts[0], Value: parts[1], Code: code}
	} else {
		return JsonnetVar{Name: s, Code: code}
	}
}

// ApplicationSourceJsonnet holds options specific to applications of type Jsonnet
type ApplicationSourceJsonnet struct {
	// ExtVars is a list of Jsonnet External Variables
	ExtVars []JsonnetVar `json:"extVars,omitempty" protobuf:"bytes,1,opt,name=extVars"`
	// TLAS is a list of Jsonnet Top-level Arguments
	TLAs []JsonnetVar `json:"tlas,omitempty" protobuf:"bytes,2,opt,name=tlas"`
	// Additional library search dirs
	Libs []string `json:"libs,omitempty" protobuf:"bytes,3,opt,name=libs"`
}

// IsZero returns true if the JSonnet options of an application are considered to be empty
func (j *ApplicationSourceJsonnet) IsZero() bool {
	return j == nil || len(j.ExtVars) == 0 && len(j.TLAs) == 0 && len(j.Libs) == 0
}

// ApplicationSourceDirectory holds options for applications of type plain YAML or Jsonnet
type ApplicationSourceDirectory struct {
	// Recurse specifies whether to scan a directory recursively for manifests
	Recurse bool `json:"recurse,omitempty" protobuf:"bytes,1,opt,name=recurse"`
	// Jsonnet holds options specific to Jsonnet
	Jsonnet ApplicationSourceJsonnet `json:"jsonnet,omitempty" protobuf:"bytes,2,opt,name=jsonnet"`
	// Exclude contains a glob pattern to match paths against that should be explicitly excluded from being used during manifest generation
	Exclude string `json:"exclude,omitempty" protobuf:"bytes,3,opt,name=exclude"`
	// Include contains a glob pattern to match paths against that should be explicitly included during manifest generation
	Include string `json:"include,omitempty" protobuf:"bytes,4,opt,name=include"`
}

// IsZero returns true if the ApplicationSourceDirectory is considered empty
func (d *ApplicationSourceDirectory) IsZero() bool {
	return d == nil || !d.Recurse && d.Jsonnet.IsZero()
}

// ApplicationSourcePlugin holds options specific to config management plugins
type ApplicationSourcePlugin struct {
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Env  `json:"env,omitempty" protobuf:"bytes,2,opt,name=env"`
}

// IsZero returns true if the ApplicationSourcePlugin is considered empty
func (c *ApplicationSourcePlugin) IsZero() bool {
	return c == nil || c.Name == "" && c.Env.IsZero()
}

// AddEnvEntry merges an EnvEntry into a list of entries. If an entry with the same name already exists,
// its value will be overwritten. Otherwise, the entry is appended to the list.
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

// RemoveEnvEntry removes an EnvEntry if present, from a list of entries.
func (c *ApplicationSourcePlugin) RemoveEnvEntry(key string) error {
	for i, ce := range c.Env {
		if ce.Name == key {
			c.Env[i] = c.Env[len(c.Env)-1]
			c.Env = c.Env[:len(c.Env)-1]
			return nil
		}
	}
	return fmt.Errorf("unable to find env variable with key %q for plugin %q", key, c.Name)
}

// ApplicationDestination holds information about the application's destination
type ApplicationDestination struct {
	// Server specifies the URL of the target cluster and must be set to the Kubernetes control plane API
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Namespace specifies the target namespace for the application's resources.
	// The namespace will only be set for namespace-scoped resources that have not set a value for .metadata.namespace
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
	// Name is an alternate way of specifying the target cluster by its symbolic name
	Name string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`

	// nolint:govet
	isServerInferred bool `json:"-"`
}

// ApplicationStatus contains status information for the application
type ApplicationStatus struct {
	// Resources is a list of Kubernetes resources managed by this application
	Resources []ResourceStatus `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Sync contains information about the application's current sync status
	Sync SyncStatus `json:"sync,omitempty" protobuf:"bytes,2,opt,name=sync"`
	// Health contains information about the application's current health status
	Health HealthStatus `json:"health,omitempty" protobuf:"bytes,3,opt,name=health"`
	// History contains information about the application's sync history
	History RevisionHistories `json:"history,omitempty" protobuf:"bytes,4,opt,name=history"`
	// Conditions is a list of currently observed application conditions
	Conditions []ApplicationCondition `json:"conditions,omitempty" protobuf:"bytes,5,opt,name=conditions"`
	// ReconciledAt indicates when the application state was reconciled using the latest git version
	ReconciledAt *metav1.Time `json:"reconciledAt,omitempty" protobuf:"bytes,6,opt,name=reconciledAt"`
	// OperationState contains information about any ongoing operations, such as a sync
	OperationState *OperationState `json:"operationState,omitempty" protobuf:"bytes,7,opt,name=operationState"`
	// ObservedAt indicates when the application state was updated without querying latest git state
	// Deprecated: controller no longer updates ObservedAt field
	ObservedAt *metav1.Time `json:"observedAt,omitempty" protobuf:"bytes,8,opt,name=observedAt"`
	// SourceType specifies the type of this application
	SourceType ApplicationSourceType `json:"sourceType,omitempty" protobuf:"bytes,9,opt,name=sourceType"`
	// Summary contains a list of URLs and container images used by this application
	Summary ApplicationSummary `json:"summary,omitempty" protobuf:"bytes,10,opt,name=summary"`
}

// JWTTokens represents a list of JWT tokens
type JWTTokens struct {
	Items []JWTToken `json:"items,omitempty" protobuf:"bytes,1,opt,name=items"`
}

// OperationInitiator contains information about the initiator of an operation
type OperationInitiator struct {
	// Username contains the name of a user who started operation
	Username string `json:"username,omitempty" protobuf:"bytes,1,opt,name=username"`
	// Automated is set to true if operation was initiated automatically by the application controller.
	Automated bool `json:"automated,omitempty" protobuf:"bytes,2,opt,name=automated"`
}

// Operation contains information about a requested or running operation
type Operation struct {
	// Sync contains parameters for the operation
	Sync *SyncOperation `json:"sync,omitempty" protobuf:"bytes,1,opt,name=sync"`
	// InitiatedBy contains information about who initiated the operations
	InitiatedBy OperationInitiator `json:"initiatedBy,omitempty" protobuf:"bytes,2,opt,name=initiatedBy"`
	// Info is a list of informational items for this operation
	Info []*Info `json:"info,omitempty" protobuf:"bytes,3,name=info"`
	// Retry controls the strategy to apply if a sync fails
	Retry RetryStrategy `json:"retry,omitempty" protobuf:"bytes,4,opt,name=retry"`
}

// DryRun returns true if an operation was requested to be performed in dry run mode
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

// LastRevisionHistory returns the latest history item from the revision history
func (in RevisionHistories) LastRevisionHistory() RevisionHistory {
	return in[len(in)-1]
}

// Trunc truncates the list of history items to size n
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

// SyncOperation contains details about a sync operation.
type SyncOperation struct {
	// Revision is the revision (Git) or chart version (Helm) which to sync the application to
	// If omitted, will use the revision specified in app spec.
	Revision string `json:"revision,omitempty" protobuf:"bytes,1,opt,name=revision"`
	// Prune specifies to delete resources from the cluster that are no longer tracked in git
	Prune bool `json:"prune,omitempty" protobuf:"bytes,2,opt,name=prune"`
	// DryRun specifies to perform a `kubectl apply --dry-run` without actually performing the sync
	DryRun bool `json:"dryRun,omitempty" protobuf:"bytes,3,opt,name=dryRun"`
	// SyncStrategy describes how to perform the sync
	SyncStrategy *SyncStrategy `json:"syncStrategy,omitempty" protobuf:"bytes,4,opt,name=syncStrategy"`
	// Resources describes which resources shall be part of the sync
	Resources []SyncOperationResource `json:"resources,omitempty" protobuf:"bytes,6,opt,name=resources"`
	// Source overrides the source definition set in the application.
	// This is typically set in a Rollback operation and is nil during a Sync operation
	Source *ApplicationSource `json:"source,omitempty" protobuf:"bytes,7,opt,name=source"`
	// Manifests is an optional field that overrides sync source with a local directory for development
	Manifests []string `json:"manifests,omitempty" protobuf:"bytes,8,opt,name=manifests"`
	// SyncOptions provide per-sync sync-options, e.g. Validate=false
	SyncOptions SyncOptions `json:"syncOptions,omitempty" protobuf:"bytes,9,opt,name=syncOptions"`
}

// IsApplyStrategy returns true if the sync strategy is "apply"
func (o *SyncOperation) IsApplyStrategy() bool {
	return o.SyncStrategy != nil && o.SyncStrategy.Apply != nil
}

// OperationState contains information about state of a running operation
type OperationState struct {
	// Operation is the original requested operation
	Operation Operation `json:"operation" protobuf:"bytes,1,opt,name=operation"`
	// Phase is the current phase of the operation
	Phase synccommon.OperationPhase `json:"phase" protobuf:"bytes,2,opt,name=phase"`
	// Message holds any pertinent messages when attempting to perform operation (typically errors).
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

// AddOption adds a sync option to the list of sync options and returns the modified list.
// If option was already set, returns the unmodified list of sync options.
func (o SyncOptions) AddOption(option string) SyncOptions {
	for _, j := range o {
		if j == option {
			return o
		}
	}
	return append(o, option)
}

// RemoveOption removes a sync option from the list of sync options and returns the modified list.
// If option has not been already set, returns the unmodified list of sync options.
func (o SyncOptions) RemoveOption(option string) SyncOptions {
	for i, j := range o {
		if j == option {
			return append(o[:i], o[i+1:]...)
		}
	}
	return o
}

// HasOption returns true if the list of sync options contains given option
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

// IsZero returns true if the sync policy is empty
func (p *SyncPolicy) IsZero() bool {
	return p == nil || (p.Automated == nil && len(p.SyncOptions) == 0 && p.Retry == nil)
}

// RetryStrategy contains information about the strategy to apply when a sync failed
type RetryStrategy struct {
	// Limit is the maximum number of attempts for retrying a failed sync. If set to 0, no retries will be performed.
	Limit int64 `json:"limit,omitempty" protobuf:"bytes,1,opt,name=limit"`
	// Backoff controls how to backoff on subsequent retries of failed syncs
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

// NextRetryAt calculates the earliest time the next retry should be performed on a failing sync
func (r *RetryStrategy) NextRetryAt(lastAttempt time.Time, retryCounts int64) (time.Time, error) {
	maxDuration := DefaultSyncRetryMaxDuration
	duration := DefaultSyncRetryDuration
	factor := DefaultSyncRetryFactor
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
	// When timeToWait is more than maxDuration retry should be performed at maxDuration.
	timeToWait := float64(duration) * (math.Pow(float64(factor), float64(retryCounts)))
	if maxDuration > 0 {
		timeToWait = math.Min(float64(maxDuration), timeToWait)
	}
	return lastAttempt.Add(time.Duration(timeToWait)), nil
}

// Backoff is the backoff strategy to use on subsequent retries for failing syncs
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
	// Prune specifies whether to delete resources from the cluster that are not found in the sources anymore as part of automated sync (default: false)
	Prune bool `json:"prune,omitempty" protobuf:"bytes,1,opt,name=prune"`
	// SelfHeal specifes whether to revert resources back to their desired state upon modification in the cluster (default: false)
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

// Force returns true if the sync strategy specifies to perform a forced sync
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

// RevisionMetadata contains metadata for a specific revision in a Git repository
type RevisionMetadata struct {
	// who authored this revision,
	// typically their name and email, e.g. "John Doe <john_doe@my-company.com>",
	// but might not match this example
	Author string `json:"author,omitempty" protobuf:"bytes,1,opt,name=author"`
	// Date specifies when the revision was authored
	Date metav1.Time `json:"date" protobuf:"bytes,2,opt,name=date"`
	// Tags specifies any tags currently attached to the revision
	// Floating tags can move from one revision to another
	Tags []string `json:"tags,omitempty" protobuf:"bytes,3,opt,name=tags"`
	// Message contains the message associated with the revision, most likely the commit message.
	// The message is truncated to the first newline or 64 characters (which ever comes first)
	Message string `json:"message,omitempty" protobuf:"bytes,4,opt,name=message"`
	// SignatureInfo contains a hint on the signer if the revision was signed with GPG, and signature verification is enabled.
	SignatureInfo string `json:"signatureInfo,omitempty" protobuf:"bytes,5,opt,name=signatureInfo"`
}

// SyncOperationResult represent result of sync operation
type SyncOperationResult struct {
	// Resources contains a list of sync result items for each individual resource in a sync operation
	Resources ResourceResults `json:"resources,omitempty" protobuf:"bytes,1,opt,name=resources"`
	// Revision holds the revision this sync operation was performed to
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// Source records the application source information of the sync, used for comparing auto-sync
	Source ApplicationSource `json:"source,omitempty" protobuf:"bytes,3,opt,name=source"`
}

// ResourceResult holds the operation result details of a specific resource
type ResourceResult struct {
	// Group specifies the API group of the resource
	Group string `json:"group" protobuf:"bytes,1,opt,name=group"`
	// Version specifies the API version of the resource
	Version string `json:"version" protobuf:"bytes,2,opt,name=version"`
	// Kind specifies the API kind of the resource
	Kind string `json:"kind" protobuf:"bytes,3,opt,name=kind"`
	// Namespace specifies the target namespace of the resource
	Namespace string `json:"namespace" protobuf:"bytes,4,opt,name=namespace"`
	// Name specifies the name of the resource
	Name string `json:"name" protobuf:"bytes,5,opt,name=name"`
	// Status holds the final result of the sync. Will be empty if the resources is yet to be applied/pruned and is always zero-value for hooks
	Status synccommon.ResultCode `json:"status,omitempty" protobuf:"bytes,6,opt,name=status"`
	// Message contains an informational or error message for the last sync OR operation
	Message string `json:"message,omitempty" protobuf:"bytes,7,opt,name=message"`
	// HookType specifies the type of the hook. Empty for non-hook resources
	HookType synccommon.HookType `json:"hookType,omitempty" protobuf:"bytes,8,opt,name=hookType"`
	// HookPhase contains the state of any operation associated with this resource OR hook
	// This can also contain values for non-hook resources.
	HookPhase synccommon.OperationPhase `json:"hookPhase,omitempty" protobuf:"bytes,9,opt,name=hookPhase"`
	// SyncPhase indicates the particular phase of the sync that this result was acquired in
	SyncPhase synccommon.SyncPhase `json:"syncPhase,omitempty" protobuf:"bytes,10,opt,name=syncPhase"`
}

// GroupVersionKind returns the GVK schema information for a given resource within a sync result
func (r *ResourceResult) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

// ResourceResults defines a list of resource results for a given operation
type ResourceResults []*ResourceResult

// Find returns the operation result for a specified resource and the index in the list where it was found
func (r ResourceResults) Find(group string, kind string, namespace string, name string, phase synccommon.SyncPhase) (int, *ResourceResult) {
	for i, res := range r {
		if res.Group == group && res.Kind == kind && res.Namespace == namespace && res.Name == name && res.SyncPhase == phase {
			return i, res
		}
	}
	return 0, nil
}

// PruningRequired returns a positive integer containing the number of resources that require pruning after an operation has been completed
func (r ResourceResults) PruningRequired() (num int) {
	for _, res := range r {
		if res.Status == synccommon.ResultCodePruneSkipped {
			num++
		}
	}
	return num
}

// RevisionHistory contains history information about a previous sync
type RevisionHistory struct {
	// Revision holds the revision the sync was performed against
	Revision string `json:"revision" protobuf:"bytes,2,opt,name=revision"`
	// DeployedAt holds the time the sync operation completed
	DeployedAt metav1.Time `json:"deployedAt" protobuf:"bytes,4,opt,name=deployedAt"`
	// ID is an auto incrementing identifier of the RevisionHistory
	ID int64 `json:"id" protobuf:"bytes,5,opt,name=id"`
	// Source is a reference to the application source used for the sync operation
	Source ApplicationSource `json:"source,omitempty" protobuf:"bytes,6,opt,name=source"`
	// DeployStartedAt holds the time the sync operation started
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
	// SyncStatusCodeUnknown indicates that the status of a sync could not be reliably determined
	SyncStatusCodeUnknown SyncStatusCode = "Unknown"
	// SyncStatusCodeOutOfSync indicates that desired and live states match
	SyncStatusCodeSynced SyncStatusCode = "Synced"
	// SyncStatusCodeOutOfSync indicates that there is a drift beween desired and live states
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

// ApplicationCondition contains details about an application condition, which is usally an error or warning
type ApplicationCondition struct {
	// Type is an application condition type
	Type ApplicationConditionType `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Message contains human-readable message indicating details about condition
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
	// LastTransitionTime is the time the condition was last observed
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
}

// ComparedTo contains application source and target which was used for resources comparison
type ComparedTo struct {
	// Source is a reference to the application's source used for comparison
	Source ApplicationSource `json:"source" protobuf:"bytes,1,opt,name=source"`
	// Destination is a reference to the application's destination used for comparison
	Destination ApplicationDestination `json:"destination" protobuf:"bytes,2,opt,name=destination"`
}

// SyncStatus contains information about the currently observed live and desired states of an application
type SyncStatus struct {
	// Status is the sync state of the comparison
	Status SyncStatusCode `json:"status" protobuf:"bytes,1,opt,name=status,casttype=SyncStatusCode"`
	// ComparedTo contains information about what has been compared
	ComparedTo ComparedTo `json:"comparedTo,omitempty" protobuf:"bytes,2,opt,name=comparedTo"`
	// Revision contains information about the revision the comparison has been performed to
	Revision string `json:"revision,omitempty" protobuf:"bytes,3,opt,name=revision"`
}

// HealthStatus contains information about the currently observed health state of an application or resource
type HealthStatus struct {
	// Status holds the status code of the application or resource
	Status health.HealthStatusCode `json:"status,omitempty" protobuf:"bytes,1,opt,name=status"`
	// Message is a human-readable informational message describing the health status
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}

// InfoItem contains arbitrary, human readable information about an application
type InfoItem struct {
	// Name is a human readable title for this piece of information.
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Value is human readable content.
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

// ResourceNetworkingInfo holds networking resource related information
// TODO: describe members of this type
type ResourceNetworkingInfo struct {
	TargetLabels map[string]string        `json:"targetLabels,omitempty" protobuf:"bytes,1,opt,name=targetLabels"`
	TargetRefs   []ResourceRef            `json:"targetRefs,omitempty" protobuf:"bytes,2,opt,name=targetRefs"`
	Labels       map[string]string        `json:"labels,omitempty" protobuf:"bytes,3,opt,name=labels"`
	Ingress      []v1.LoadBalancerIngress `json:"ingress,omitempty" protobuf:"bytes,4,opt,name=ingress"`
	// ExternalURLs holds list of URLs which should be available externally. List is populated for ingress resources using rules hostnames.
	ExternalURLs []string `json:"externalURLs,omitempty" protobuf:"bytes,5,opt,name=externalURLs"`
}

// TODO: describe this type
type HostResourceInfo struct {
	ResourceName         v1.ResourceName `json:"resourceName,omitempty" protobuf:"bytes,1,name=resourceName"`
	RequestedByApp       int64           `json:"requestedByApp,omitempty" protobuf:"bytes,2,name=requestedByApp"`
	RequestedByNeighbors int64           `json:"requestedByNeighbors,omitempty" protobuf:"bytes,3,name=requestedByNeighbors"`
	Capacity             int64           `json:"capacity,omitempty" protobuf:"bytes,4,name=capacity"`
}

// HostInfo holds host name and resources metrics
// TODO: describe purpose of this type
// TODO: describe members of this type
type HostInfo struct {
	Name          string             `json:"name,omitempty" protobuf:"bytes,1,name=name"`
	ResourcesInfo []HostResourceInfo `json:"resourcesInfo,omitempty" protobuf:"bytes,2,name=resourcesInfo"`
	SystemInfo    v1.NodeSystemInfo  `json:"systemInfo,omitempty" protobuf:"bytes,3,opt,name=systemInfo"`
}

// ApplicationTree holds nodes which belongs to the application
// TODO: describe purpose of this type
type ApplicationTree struct {
	// Nodes contains list of nodes which either directly managed by the application and children of directly managed nodes.
	Nodes []ResourceNode `json:"nodes,omitempty" protobuf:"bytes,1,rep,name=nodes"`
	// OrphanedNodes contains if or orphaned nodes: nodes which are not managed by the app but in the same namespace. List is populated only if orphaned resources enabled in app project.
	OrphanedNodes []ResourceNode `json:"orphanedNodes,omitempty" protobuf:"bytes,2,rep,name=orphanedNodes"`
	// Hosts holds list of Kubernetes nodes that run application related pods
	Hosts []HostInfo `json:"hosts,omitempty" protobuf:"bytes,3,rep,name=hosts"`
}

// Normalize sorts application tree nodes and hosts. The persistent order allows to
// effectively compare previously cached app tree and allows to unnecessary Redis requests.
func (t *ApplicationTree) Normalize() {
	sort.Slice(t.Nodes, func(i, j int) bool {
		return t.Nodes[i].FullName() < t.Nodes[j].FullName()
	})
	sort.Slice(t.OrphanedNodes, func(i, j int) bool {
		return t.OrphanedNodes[i].FullName() < t.OrphanedNodes[j].FullName()
	})
	sort.Slice(t.Hosts, func(i, j int) bool {
		return t.Hosts[i].Name < t.Hosts[j].Name
	})
}

// ApplicationSummary contains information about URLs and container images used by an application
type ApplicationSummary struct {
	// ExternalURLs holds all external URLs of application child resources.
	ExternalURLs []string `json:"externalURLs,omitempty" protobuf:"bytes,1,opt,name=externalURLs"`
	// Images holds all images of application child resources.
	Images []string `json:"images,omitempty" protobuf:"bytes,2,opt,name=images"`
}

// TODO: Document purpose of this method
func (t *ApplicationTree) FindNode(group string, kind string, namespace string, name string) *ResourceNode {
	for _, n := range append(t.Nodes, t.OrphanedNodes...) {
		if n.Group == group && n.Kind == kind && n.Namespace == namespace && n.Name == name {
			return &n
		}
	}
	return nil
}

// TODO: Document purpose of this method
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

// ResourceRef includes fields which uniquely identify a resource
type ResourceRef struct {
	Group     string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Version   string `json:"version,omitempty" protobuf:"bytes,2,opt,name=version"`
	Kind      string `json:"kind,omitempty" protobuf:"bytes,3,opt,name=kind"`
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,4,opt,name=namespace"`
	Name      string `json:"name,omitempty" protobuf:"bytes,5,opt,name=name"`
	UID       string `json:"uid,omitempty" protobuf:"bytes,6,opt,name=uid"`
}

// ResourceNode contains information about live resource and its children
// TODO: describe members of this type
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

// FullName returns a resource node's full name in the format "group/kind/namespace/name"
// For cluster-scoped resources, namespace will be the empty string.
func (n *ResourceNode) FullName() string {
	return fmt.Sprintf("%s/%s/%s/%s", n.Group, n.Kind, n.Namespace, n.Name)
}

// GroupKindVersion returns the GVK schema type for given resource node
func (n *ResourceNode) GroupKindVersion() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   n.Group,
		Version: n.Version,
		Kind:    n.Kind,
	}
}

// ResourceStatus holds the current sync and health status of a resource
// TODO: describe members of this type
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

// GroupKindVersion returns the GVK schema type for given resource status
func (r *ResourceStatus) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: r.Group, Version: r.Version, Kind: r.Kind}
}

// ResourceDiff holds the diff of a live and target resource object
// TODO: describe members of this type
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
	ResourceVersion    string `json:"resourceVersion,omitempty" protobuf:"bytes,11,opt,name=resourceVersion"`
	Modified           bool   `json:"modified,omitempty" protobuf:"bytes,12,opt,name=modified"`
}

// FullName returns full name of a node that was used for diffing in the format "group/kind/namespace/name"
// For cluster-scoped resources, namespace will be the empty string.
func (r *ResourceDiff) FullName() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.Group, r.Kind, r.Namespace, r.Name)
}

// ConnectionStatus represents the status indicator for a connection to a remote resource
type ConnectionStatus = string

const (
	// ConnectionStatusSuccessful indicates that a connection has been successfully established
	ConnectionStatusSuccessful = "Successful"
	// ConnectionStatusFailed indicates that a connection attempt has failed
	ConnectionStatusFailed = "Failed"
	// ConnectionStatusUnknown indicates that the connection status could not be reliably determined
	ConnectionStatusUnknown = "Unknown"
)

// ConnectionState contains information about remote resource connection state, currently used for clusters and repositories
type ConnectionState struct {
	// Status contains the current status indicator for the connection
	Status ConnectionStatus `json:"status" protobuf:"bytes,1,opt,name=status"`
	// Message contains human readable information about the connection status
	Message string `json:"message" protobuf:"bytes,2,opt,name=message"`
	// ModifiedAt contains the timestamp when this connection status has been determined
	ModifiedAt *metav1.Time `json:"attemptedAt" protobuf:"bytes,3,opt,name=attemptedAt"`
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
	// Holds list of namespaces which are accessible in that cluster. Cluster level resources will be ignored if namespace list is not empty.
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,6,opt,name=namespaces"`
	// RefreshRequestedAt holds time when cluster cache refresh has been requested
	RefreshRequestedAt *metav1.Time `json:"refreshRequestedAt,omitempty" protobuf:"bytes,7,opt,name=refreshRequestedAt"`
	// Info holds information about cluster cache and state
	Info ClusterInfo `json:"info,omitempty" protobuf:"bytes,8,opt,name=info"`
	// Shard contains optional shard number. Calculated on the fly by the application controller if not specified.
	Shard *int64 `json:"shard,omitempty" protobuf:"bytes,9,opt,name=shard"`
	// Indicates if cluster level resources should be managed. This setting is used only if cluster is connected in a namespaced mode.
	ClusterResources bool `json:"clusterResources,omitempty" protobuf:"bytes,10,opt,name=clusterResources"`
	// Reference between project and cluster that allow you automatically to be added as item inside Destinations project entity
	Project string `json:"project,omitempty" protobuf:"bytes,11,opt,name=project"`
	// Labels for cluster secret metadata
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,12,opt,name=labels"`
	// Annotations for cluster secret metadata
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,13,opt,name=annotations"`
}

// Equals returns true if two cluster objects are considered to be equal
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

	if c.ClusterResources != other.ClusterResources {
		return false
	}

	if !collections.StringMapsEqual(c.Annotations, other.Annotations) {
		return false
	}

	if !collections.StringMapsEqual(c.Labels, other.Labels) {
		return false
	}

	return reflect.DeepEqual(c.Config, other.Config)
}

// ClusterInfo contains information about the cluster
type ClusterInfo struct {
	// ConnectionState contains information about the connection to the cluster
	ConnectionState ConnectionState `json:"connectionState,omitempty" protobuf:"bytes,1,opt,name=connectionState"`
	// ServerVersion contains information about the Kubernetes version of the cluster
	ServerVersion string `json:"serverVersion,omitempty" protobuf:"bytes,2,opt,name=serverVersion"`
	// CacheInfo contains information about the cluster cache
	CacheInfo ClusterCacheInfo `json:"cacheInfo,omitempty" protobuf:"bytes,3,opt,name=cacheInfo"`
	// ApplicationsCount is the number of applications managed by Argo CD on the cluster
	ApplicationsCount int64 `json:"applicationsCount" protobuf:"bytes,4,opt,name=applicationsCount"`
	// APIVersions contains list of API versions supported by the cluster
	APIVersions []string `json:"apiVersions,omitempty" protobuf:"bytes,5,opt,name=apiVersions"`
}

func (c *ClusterInfo) GetKubeVersion() string {
	return c.ServerVersion
}

func (c *ClusterInfo) GetApiVersions() []string {
	return c.APIVersions
}

// ClusterCacheInfo contains information about the cluster cache
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
	// Insecure specifies that the server should be accessed without verifying the TLS certificate. For testing only.
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

// KnownTypeField contains mapping between CRD field and known Kubernetes type.
// This is mainly used for unit conversion in unknown resources (e.g. 0.1 == 100mi)
// TODO: Describe the members of this type
type KnownTypeField struct {
	Field string `json:"field,omitempty" protobuf:"bytes,1,opt,name=field"`
	Type  string `json:"type,omitempty" protobuf:"bytes,2,opt,name=type"`
}

// OverrideIgnoreDiff contains configurations about how fields should be ignored during diffs between
// the desired state and live state
type OverrideIgnoreDiff struct {
	//JSONPointers is a JSON path list following the format defined in RFC4627 (https://datatracker.ietf.org/doc/html/rfc6902#section-3)
	JSONPointers []string `json:"jsonPointers" protobuf:"bytes,1,rep,name=jSONPointers"`
	//JQPathExpressions is a JQ path list that will be evaludated during the diff process
	JQPathExpressions []string `json:"jqPathExpressions" protobuf:"bytes,2,opt,name=jqPathExpressions"`
	// ManagedFieldsManagers is a list of trusted managers. Fields mutated by those managers will take precedence over the
	// desired state defined in the SCM and won't be displayed in diffs
	ManagedFieldsManagers []string `json:"managedFieldsManagers" protobuf:"bytes,3,opt,name=managedFieldsManagers"`
}

type rawResourceOverride struct {
	HealthLua         string           `json:"health.lua,omitempty"`
	UseOpenLibs       bool             `json:"health.lua.useOpenLibs,omitempty"`
	Actions           string           `json:"actions,omitempty"`
	IgnoreDifferences string           `json:"ignoreDifferences,omitempty"`
	KnownTypeFields   []KnownTypeField `json:"knownTypeFields,omitempty"`
}

// ResourceOverride holds configuration to customize resource diffing and health assessment
// TODO: describe the members of this type
type ResourceOverride struct {
	HealthLua         string             `protobuf:"bytes,1,opt,name=healthLua"`
	UseOpenLibs       bool               `protobuf:"bytes,5,opt,name=useOpenLibs"`
	Actions           string             `protobuf:"bytes,3,opt,name=actions"`
	IgnoreDifferences OverrideIgnoreDiff `protobuf:"bytes,2,opt,name=ignoreDifferences"`
	KnownTypeFields   []KnownTypeField   `protobuf:"bytes,4,opt,name=knownTypeFields"`
}

// TODO: describe this method
func (s *ResourceOverride) UnmarshalJSON(data []byte) error {
	raw := &rawResourceOverride{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	s.KnownTypeFields = raw.KnownTypeFields
	s.HealthLua = raw.HealthLua
	s.UseOpenLibs = raw.UseOpenLibs
	s.Actions = raw.Actions
	return yaml.Unmarshal([]byte(raw.IgnoreDifferences), &s.IgnoreDifferences)
}

// TODO: describe this method
func (s ResourceOverride) MarshalJSON() ([]byte, error) {
	ignoreDifferencesData, err := yaml.Marshal(s.IgnoreDifferences)
	if err != nil {
		return nil, err
	}
	raw := &rawResourceOverride{s.HealthLua, s.UseOpenLibs, s.Actions, string(ignoreDifferencesData), s.KnownTypeFields}
	return json.Marshal(raw)
}

// TODO: describe this method
func (o *ResourceOverride) GetActions() (ResourceActions, error) {
	var actions ResourceActions
	err := yaml.Unmarshal([]byte(o.Actions), &actions)
	if err != nil {
		return actions, err
	}
	return actions, nil
}

// TODO: describe this type
// TODO: describe members of this type
type ResourceActions struct {
	ActionDiscoveryLua string                     `json:"discovery.lua,omitempty" yaml:"discovery.lua,omitempty" protobuf:"bytes,1,opt,name=actionDiscoveryLua"`
	Definitions        []ResourceActionDefinition `json:"definitions,omitempty" protobuf:"bytes,2,rep,name=definitions"`
}

// TODO: describe this type
// TODO: describe members of this type
type ResourceActionDefinition struct {
	Name      string `json:"name" protobuf:"bytes,1,opt,name=name"`
	ActionLua string `json:"action.lua" yaml:"action.lua" protobuf:"bytes,2,opt,name=actionLua"`
}

// TODO: describe this type
// TODO: describe members of this type
type ResourceAction struct {
	Name     string                `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Params   []ResourceActionParam `json:"params,omitempty" protobuf:"bytes,2,rep,name=params"`
	Disabled bool                  `json:"disabled,omitempty" protobuf:"varint,3,opt,name=disabled"`
}

// TODO: describe this type
// TODO: describe members of this type
type ResourceActionParam struct {
	Name    string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Value   string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	Type    string `json:"type,omitempty" protobuf:"bytes,3,opt,name=type"`
	Default string `json:"default,omitempty" protobuf:"bytes,4,opt,name=default"`
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
	if resource != "applications" && resource != "repositories" && resource != "clusters" {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': project resource must be: 'applications', 'repositories' or 'clusters', not '%s'", policy, resource)
	}
	// action
	action := strings.Trim(policyComponents[3], " ")
	if !isValidAction(action) {
		return status.Errorf(codes.InvalidArgument, "invalid policy rule '%s': invalid action '%s'", policy, action)
	}
	// object
	object := strings.Trim(policyComponents[4], " ")
	objectRegexp, err := regexp.Compile(fmt.Sprintf(`^%s/[*\w-.]+$`, regexp.QuoteMeta(proj)))
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

var invalidChars = regexp.MustCompile("[\"\n\r\t]")

func validateGroupName(name string) error {
	name = strings.TrimSpace(name)
	if len(name) > 1 && strings.HasPrefix(name, "\"") && strings.HasSuffix(name, "\"") {
		// Remove surrounding quotes for further inspection of the group name
		name = name[1 : len(name)-1]
	} else if strings.Contains(name, ",") {
		return status.Errorf(codes.InvalidArgument, "group '%s' must be quoted", name)
	}

	if name == "" {
		return status.Errorf(codes.InvalidArgument, "group '%s' is empty", name)
	}
	if invalidChars.MatchString(name) {
		return status.Errorf(codes.InvalidArgument, "group '%s' contains invalid characters", name)
	}
	return nil
}

// OrphanedResourcesMonitorSettings holds settings of orphaned resources monitoring
type OrphanedResourcesMonitorSettings struct {
	// Warn indicates if warning condition should be created for apps which have orphaned resources
	Warn *bool `json:"warn,omitempty" protobuf:"bytes,1,name=warn"`
	// Ignore contains a list of resources that are to be excluded from orphaned resources monitoring
	Ignore []OrphanedResourceKey `json:"ignore,omitempty" protobuf:"bytes,2,opt,name=ignore"`
}

// OrphanedResourceKey is a reference to a resource to be ignored from
type OrphanedResourceKey struct {
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	Kind  string `json:"kind,omitempty" protobuf:"bytes,2,opt,name=kind"`
	Name  string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
}

// IsWarn returns true if warnings are enabled for orphan resources monitoring
func (s *OrphanedResourcesMonitorSettings) IsWarn() bool {
	return s.Warn != nil && *s.Warn
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
	// SignatureKeys contains a list of PGP key IDs that commits in Git must be signed with in order to be allowed for sync
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
	//TimeZone of the sync that will be applied to the schedule
	TimeZone string `json:"timeZone,omitempty" protobuf:"bytes,8,opt,name=timeZone"`
}

// HasWindows returns true if SyncWindows has one or more SyncWindow
func (s *SyncWindows) HasWindows() bool {
	return s != nil && len(*s) > 0
}

// Active returns a list of sync windows that are currently active
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

			// Offset the nextWindow time to consider the timeZone of the sync window
			timeZoneOffsetDuration := w.scheduleOffsetByTimeZone()
			nextWindow := schedule.Next(currentTime.Add(timeZoneOffsetDuration - duration))
			if nextWindow.Before(currentTime.Add(timeZoneOffsetDuration)) {
				active = append(active, w)
			}
		}
		if len(active) > 0 {
			return &active
		}
	}
	return nil
}

// InactiveAllows will iterate over the SyncWindows and return all inactive allow windows
// for the current time. If the current time is in an inactive allow window, syncs will
// be denied.
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
				// Offset the nextWindow time to consider the timeZone of the sync window
				timeZoneOffsetDuration := w.scheduleOffsetByTimeZone()
				nextWindow := schedule.Next(currentTime.Add(timeZoneOffsetDuration - duration))

				if !nextWindow.Before(currentTime.Add(timeZoneOffsetDuration)) && sErr == nil && dErr == nil {
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

func (w *SyncWindow) scheduleOffsetByTimeZone() time.Duration {
	loc, err := time.LoadLocation(w.TimeZone)
	if err != nil {
		log.Warnf("Invalid time zone %s specified. Using UTC as default time zone", w.TimeZone)
		loc = time.Now().UTC().Location()
	}
	_, tzOffset := time.Now().In(loc).Zone()
	return time.Duration(tzOffset) * time.Second
}

// AddWindow adds a sync window with the given parameters to the AppProject
func (s *AppProjectSpec) AddWindow(knd string, sch string, dur string, app []string, ns []string, cl []string, ms bool, timeZone string) error {
	if len(knd) == 0 || len(sch) == 0 || len(dur) == 0 {
		return fmt.Errorf("cannot create window: require kind, schedule, duration and one or more of applications, namespaces and clusters")

	}

	window := &SyncWindow{
		Kind:       knd,
		Schedule:   sch,
		Duration:   dur,
		ManualSync: ms,
		TimeZone:   timeZone,
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

// DeleteWindow deletes a sync window with the given id from the AppProject
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

// Matches returns a list of sync windows that are defined for a given application
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
					dst := app.Spec.Destination
					dstNameMatched := dst.Name != "" && globMatch(c, dst.Name)
					dstServerMatched := dst.Server != "" && globMatch(c, dst.Server)
					if dstNameMatched || dstServerMatched {
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

// CanSync returns true if a sync window currently allows a sync. isManual indicates whether the sync has been triggered manually.
func (w *SyncWindows) CanSync(isManual bool) bool {
	if !w.HasWindows() {
		return true
	}

	active := w.Active()
	hasActiveDeny, manualEnabled := active.hasDeny()

	if hasActiveDeny {
		if isManual && manualEnabled {
			return true
		} else {
			return false
		}
	}

	inactiveAllows := w.InactiveAllows()
	if inactiveAllows.HasWindows() {
		if isManual && inactiveAllows.manualEnabled() {
			return true
		} else {
			return false
		}
	}

	return true
}

// hasDeny will iterate over the SyncWindows and return if a deny window is found and if
// manual sync is enabled. It returns true in the first return boolean value if it finds
// any deny window. Will return true in the second return boolean value if all deny windows
// have manual sync enabled. If one deny window has manual sync disabled it returns false in
// the second return value.
func (w *SyncWindows) hasDeny() (bool, bool) {
	if !w.HasWindows() {
		return false, false
	}
	var denyFound, manualEnabled bool
	for _, a := range *w {
		if a.Kind == "deny" {
			if !denyFound {
				manualEnabled = a.ManualSync
			} else {
				if manualEnabled {
					manualEnabled = a.ManualSync
				}
			}
			denyFound = true
		}
	}
	return denyFound, manualEnabled
}

// hasAllow will iterate over the SyncWindows and returns true if it find any allow window.
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

// manualEnabled will iterate over the SyncWindows and return true if all windows have
// ManualSync set to true. Returns false if it finds at least one entry with ManualSync
// set to false
func (w *SyncWindows) manualEnabled() bool {
	if !w.HasWindows() {
		return false
	}
	for _, s := range *w {
		if !s.ManualSync {
			return false
		}
	}
	return true
}

// Active returns true if the sync window is currently active
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

	// Offset the nextWindow time to consider the timeZone of the sync window
	timeZoneOffsetDuration := w.scheduleOffsetByTimeZone()
	nextWindow := schedule.Next(currentTime.Add(timeZoneOffsetDuration - duration))

	return nextWindow.Before(currentTime.Add(timeZoneOffsetDuration))
}

// Update updates a sync window's settings with the given parameter
func (w *SyncWindow) Update(s string, d string, a []string, n []string, c []string, tz string) error {

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
	if tz == "" {
		tz = "UTC"
	}
	w.TimeZone = tz
	return nil
}

// Validate checks whether a sync window has valid configuration. The error returned indicates any problems that has been found.
func (w *SyncWindow) Validate() error {

	// Default timeZone to UTC if timeZone is not specified
	if w.TimeZone == "" {
		w.TimeZone = "UTC"
	}
	if _, err := time.LoadLocation(w.TimeZone); err != nil {
		log.Warnf("Invalid time zone %s specified. Using UTC as default time zone", w.TimeZone)
		w.TimeZone = "UTC"
	}

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

// DestinationClusters returns a list of cluster URLs allowed as destination in an AppProject
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
	// Policies Stores a list of casbin formatted strings that define access policies for the role in the project
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
	LockRepo bool     `json:"lockRepo,omitempty" protobuf:"bytes,4,name=lockRepo"`
}

// HelmOptions holds helm options
type HelmOptions struct {
	ValuesFileSchemes []string `protobuf:"bytes,1,opt,name=valuesFileSchemes"`
}

// KustomizeOptions are options for kustomize to use when building manifests
type KustomizeOptions struct {
	// BuildOptions is a string of build parameters to use when calling `kustomize build`
	BuildOptions string `protobuf:"bytes,1,opt,name=buildOptions"`
	// BinaryPath holds optional path to kustomize binary
	BinaryPath string `protobuf:"bytes,2,opt,name=binaryPath"`
}

// CascadedDeletion indicates if the deletion finalizer is set and controller should delete the application and it's cascaded resources
func (app *Application) CascadedDeletion() bool {
	for _, finalizer := range app.ObjectMeta.Finalizers {
		if isPropagationPolicyFinalizer(finalizer) {
			return true
		}
	}
	return false
}

// IsRefreshRequested returns whether a refresh has been requested for an application, and if yes, the type of refresh that should be executed.
func (app *Application) IsRefreshRequested() (RefreshType, bool) {
	refreshType := RefreshTypeNormal
	annotations := app.GetAnnotations()
	if annotations == nil {
		return refreshType, false
	}

	typeStr, ok := annotations[AnnotationKeyRefresh]
	if !ok {
		return refreshType, false
	}

	if typeStr == string(RefreshTypeHard) {
		refreshType = RefreshTypeHard
	}

	return refreshType, true
}

// SetCascadedDeletion will enable cascaded deletion by setting the propagation policy finalizer
func (app *Application) SetCascadedDeletion(finalizer string) {
	setFinalizer(&app.ObjectMeta, finalizer, true)
}

// Expired returns true if the application needs to be reconciled
func (status *ApplicationStatus) Expired(statusRefreshTimeout time.Duration) bool {
	return status.ReconciledAt == nil || status.ReconciledAt.Add(statusRefreshTimeout).Before(time.Now().UTC())
}

// UnSetCascadedDeletion will remove the propagation policy finalizers
func (app *Application) UnSetCascadedDeletion() {
	for _, f := range app.Finalizers {
		if isPropagationPolicyFinalizer(f) {
			setFinalizer(&app.ObjectMeta, f, false)
		}
	}
}

func isPropagationPolicyFinalizer(finalizer string) bool {
	switch finalizer {
	case ResourcesFinalizerName:
		return true
	case ForegroundPropagationPolicyFinalizer:
		return true
	case BackgroundPropagationPolicyFinalizer:
		return true
	default:
		return false
	}
}

// GetPropagationPolicy returns the value of propagation policy finalizer
func (app *Application) GetPropagationPolicy() string {
	for _, finalizer := range app.ObjectMeta.Finalizers {
		if isPropagationPolicyFinalizer(finalizer) {
			return finalizer
		}
	}
	return ""
}

// IsFinalizerPresent checks if the app has a given finalizer
func (app *Application) IsFinalizerPresent(finalizer string) bool {
	return getFinalizerIndex(app.ObjectMeta, finalizer) > -1
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

// IsError returns true if a condition indicates an error condition
func (condition *ApplicationCondition) IsError() bool {
	return strings.HasSuffix(condition.Type, "Error")
}

// Equals compares two instances of ApplicationSource and return true if instances are equal.
func (source *ApplicationSource) Equals(other ApplicationSource) bool {
	return reflect.DeepEqual(*source, other)
}

// ExplicitType returns the type (e.g. Helm, Kustomize, etc) of the application. If either none or multiple types are defined, returns an error.
func (source *ApplicationSource) ExplicitType() (*ApplicationSourceType, error) {
	var appTypes []ApplicationSourceType
	if source.Kustomize != nil {
		appTypes = append(appTypes, ApplicationSourceTypeKustomize)
	}
	if source.Helm != nil {
		appTypes = append(appTypes, ApplicationSourceTypeHelm)
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

// Equals compares two instances of ApplicationDestination and returns true if instances are equal.
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
		return DefaultAppProjectName
	}
	return spec.Project
}

// GetRevisionHistoryLimit returns the currently set revision history limit for an application
func (spec ApplicationSpec) GetRevisionHistoryLimit() int {
	if spec.RevisionHistoryLimit != nil {
		return int(*spec.RevisionHistoryLimit)
	}
	return RevisionHistoryLimit
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

// SetK8SConfigDefaults sets Kubernetes REST config default settings
func SetK8SConfigDefaults(config *rest.Config) error {
	config.QPS = K8sClientConfigQPS
	config.Burst = K8sClientConfigBurst
	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return err
	}

	dial := (&net.Dialer{
		Timeout:   K8sTCPTimeout,
		KeepAlive: K8sTCPKeepAlive,
	}).DialContext
	transport := utilnet.SetTransportDefaults(&http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: K8sTLSHandshakeTimeout,
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        K8sMaxIdleConnections,
		MaxIdleConnsPerHost: K8sMaxIdleConnections,
		MaxConnsPerHost:     K8sMaxIdleConnections,
		DialContext:         dial,
		DisableCompression:  config.DisableCompression,
		IdleConnTimeout:     K8sTCPIdleConnTimeout,
	})
	tr, err := rest.HTTPWrappersForConfig(config, transport)
	if err != nil {
		return err
	}

	// set default tls config and remove auth/exec provides since we use it in a custom transport
	config.TLSClientConfig = rest.TLSClientConfig{}
	config.AuthProvider = nil
	config.ExecProvider = nil

	// Set server-side timeout
	config.Timeout = K8sServerSideTimeout

	config.Transport = tr
	return nil
}

// RawRestConfig returns a go-client REST config from cluster that might be serialized into the file using kube.WriteKubeConfig method.
func (c *Cluster) RawRestConfig() *rest.Config {
	var config *rest.Config
	var err error
	if c.Server == KubernetesInternalAPIServerAddr && os.Getenv(EnvVarFakeInClusterConfig) == "true" {
		conf, exists := os.LookupEnv("KUBECONFIG")
		if exists {
			config, err = clientcmd.BuildConfigFromFlags("", conf)
		} else {
			config, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
		}
	} else if c.Server == KubernetesInternalAPIServerAddr && c.Config.Username == "" && c.Config.Password == "" && c.Config.BearerToken == "" {
		config, err = rest.InClusterConfig()
	} else if c.Server == KubernetesInternalAPIServerAddr {
		config, err = rest.InClusterConfig()
		if err == nil {
			config.Username = c.Config.Username
			config.Password = c.Config.Password
			config.BearerToken = c.Config.BearerToken
			config.BearerTokenFile = ""
		}
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
					APIVersion:      "client.authentication.k8s.io/v1alpha1",
					Command:         "aws",
					Args:            args,
					InteractiveMode: api.NeverExecInteractiveMode,
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
					APIVersion:      c.Config.ExecProviderConfig.APIVersion,
					Command:         c.Config.ExecProviderConfig.Command,
					Args:            c.Config.ExecProviderConfig.Args,
					Env:             env,
					InstallHint:     c.Config.ExecProviderConfig.InstallHint,
					InteractiveMode: api.NeverExecInteractiveMode,
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
	config.Timeout = K8sServerSideTimeout
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

// UnmarshalToUnstructured unmarshals a resource representation in JSON to unstructured data
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

// TODO: document this method
func (r ResourceDiff) LiveObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.LiveState)
}

// TODO: document this method
func (r ResourceDiff) TargetObject() (*unstructured.Unstructured, error) {
	return UnmarshalToUnstructured(r.TargetState)
}

// SetInferredServer sets the Server field of the destination. See IsServerInferred() for details.
func (d *ApplicationDestination) SetInferredServer(server string) {

	d.isServerInferred = true
	d.Server = server
}

// An ApplicationDestination has an 'inferred server' if the ApplicationDestination
// contains a Name, but not a Server URL. In this case it is necessary to retrieve
// the Server URL by looking up the cluster name.
//
// As of this writing, looking up the cluster name, and setting the URL, is
// performed by 'utils.ValidateDestination(...)', which then calls SetInferredServer.
func (d *ApplicationDestination) IsServerInferred() bool {
	return d.isServerInferred
}

// MarshalJSON marshals an application destination to JSON format
func (d *ApplicationDestination) MarshalJSON() ([]byte, error) {
	type Alias ApplicationDestination
	dest := d
	if d.isServerInferred {
		dest = dest.DeepCopy()
		dest.Server = ""
	}
	return json.Marshal(&struct{ *Alias }{Alias: (*Alias)(dest)})
}
