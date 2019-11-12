package pkg

import (
	"context"

	"github.com/argoproj/argo-cd/engine/util/kube"

	"github.com/argoproj/argo-cd/engine/resource"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
)

// The GitOps engine API is represented by two interfaces ReconciliationSettings, CredentialsStore and settings if the Application object.

type Engine interface {
	Run(ctx context.Context, statusProcessors int, operationProcessors int)
}

type SyncTaskInfo struct {
	Phase     v1alpha1.SyncPhase
	LiveObj   *unstructured.Unstructured
	TargetObj *unstructured.Unstructured
	IsHook    bool
}

// Consider callback registration instead
type Callbacks interface {
	OnBeforeSync(appName string, tasks []SyncTaskInfo) ([]SyncTaskInfo, error)
	OnSyncCompleted(appName string, state appv1.OperationState) error

	OnResourceUpdated(un *unstructured.Unstructured)
	OnResourceRemoved(key kube.ResourceKey)
	OnClusterInitialized(server string)
}

// ReconciliationSettings provides set of methods which expose manifest generation and diffing settings.
type ReconciliationSettings interface {
	// TODO: merge into one method
	GetAppInstanceLabelKey() (string, error)
	GetResourcesFilter() (*resource.ResourcesFilter, error)
	GetResourceOverrides() (map[string]appv1.ResourceOverride, error)

	Subscribe(subCh chan<- bool)
	Unsubscribe(subCh chan<- bool)

	GetConfigManagementPlugins() ([]appv1.ConfigManagementPlugin, error) // TODO: remove
	GetKustomizeBuildOptions() (string, error)                           // TODO: remove
}

// ClusterEvent contains information about cluster event
type ClusterEvent struct {
	Type    watch.EventType
	Cluster *appv1.Cluster
}

// CredentialsStore allows to get repository and cluster credentials
type CredentialsStore interface {
	GetCluster(ctx context.Context, name string) (*appv1.Cluster, error)
	WatchClusters(ctx context.Context, callback func(event *ClusterEvent)) error
	ListHelmRepositories(ctx context.Context) ([]*appv1.Repository, error)    // TODO: remove
	GetRepository(ctx context.Context, url string) (*appv1.Repository, error) // TODO: remove
}

type EventInfo struct {
	Type   string
	Reason string
}

// In addition to main API consumer have to provide infrastructure interfaces:

const (
	EventReasonStatusRefreshed    = "StatusRefreshed"
	EventReasonResourceCreated    = "ResourceCreated"
	EventReasonResourceUpdated    = "ResourceUpdated"
	EventReasonResourceDeleted    = "ResourceDeleted"
	EventReasonOperationStarted   = "OperationStarted"
	EventReasonOperationCompleted = "OperationCompleted"
)

// AuditLogger allows to react to application events such as sync started, sync failed/completed etc.
type AuditLogger interface {
	LogAppEvent(app *appv1.Application, info EventInfo, message string)
}

// AppStateCache is used to cache intermediate reconciliation results. Engine is tolerant to cache errors, so implementation can just return 'not implemented' error.
type AppStateCache interface {
	SetAppResourcesTree(appName string, resourcesTree *appv1.ApplicationTree) error
	SetAppManagedResources(appName string, managedResources []*appv1.ResourceDiff) error
	GetAppManagedResources(appName string, res *[]*appv1.ResourceDiff) error
}

type ManifestResponse struct {
	Manifests  []string
	Namespace  string
	Server     string
	Revision   string
	SourceType string
}

type ManifestGenerationSettings struct {
	AppLabelKey      string
	AppLabelValue    string
	Namespace        string
	Repos            []*appv1.Repository
	Plugins          []*appv1.ConfigManagementPlugin
	KustomizeOptions *appv1.KustomizeOptions
	KubeVersion      string
	NoCache          bool
}

// ManifestGenerator allows to move manifest generation into separate process if in-process manifest generation does not work for performance reasons.
type ManifestGenerator interface {
	// TODO: remove manifest generation related settings
	Generate(ctx context.Context, repo *appv1.Repository, revision string, source *appv1.ApplicationSource, setting *ManifestGenerationSettings) (*ManifestResponse, error)
}
