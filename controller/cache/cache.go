package cache

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/controller/metrics"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/env"
	argoerrors "github.com/argoproj/argo-cd/v3/util/errors"
	logutils "github.com/argoproj/argo-cd/v3/util/log"
	"github.com/argoproj/argo-cd/v3/util/lua"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	// EnvClusterCacheResyncDuration is the env variable that holds cluster cache re-sync duration
	EnvClusterCacheResyncDuration = "ARGOCD_CLUSTER_CACHE_RESYNC_DURATION"

	// EnvClusterCacheWatchResyncDuration is the env variable that holds cluster cache watch re-sync duration
	EnvClusterCacheWatchResyncDuration = "ARGOCD_CLUSTER_CACHE_WATCH_RESYNC_DURATION"

	// EnvClusterSyncRetryTimeoutDuration is the env variable that holds cluster retry duration when sync error happens
	EnvClusterSyncRetryTimeoutDuration = "ARGOCD_CLUSTER_SYNC_RETRY_TIMEOUT_DURATION"

	// EnvClusterCacheListPageSize is the env variable to control size of the list page size when making K8s queries
	EnvClusterCacheListPageSize = "ARGOCD_CLUSTER_CACHE_LIST_PAGE_SIZE"

	// EnvClusterCacheListPageBufferSize is the env variable to control the number of pages to buffer when making a K8s query to list resources
	EnvClusterCacheListPageBufferSize = "ARGOCD_CLUSTER_CACHE_LIST_PAGE_BUFFER_SIZE"

	// EnvClusterCacheListSemaphore is the env variable to control size of the list semaphore
	// This is used to limit the number of concurrent memory consuming operations on the
	// k8s list queries results across all clusters to avoid memory spikes during cache initialization.
	EnvClusterCacheListSemaphore = "ARGOCD_CLUSTER_CACHE_LIST_SEMAPHORE"

	// EnvClusterCacheAttemptLimit is the env variable to control the retry limit for listing resources during cluster cache sync
	EnvClusterCacheAttemptLimit = "ARGOCD_CLUSTER_CACHE_ATTEMPT_LIMIT"

	// EnvClusterCacheRetryUseBackoff is the env variable to control whether to use a backoff strategy with the retry during cluster cache sync
	EnvClusterCacheRetryUseBackoff = "ARGOCD_CLUSTER_CACHE_RETRY_USE_BACKOFF"

	// EnvClusterCacheBatchEventsProcessing is the env variable to control whether to enable batch events processing
	EnvClusterCacheBatchEventsProcessing = "ARGOCD_CLUSTER_CACHE_BATCH_EVENTS_PROCESSING"

	// EnvClusterCacheEventsProcessingInterval is the env variable to control the interval between processing events when BatchEventsProcessing is enabled
	EnvClusterCacheEventsProcessingInterval = "ARGOCD_CLUSTER_CACHE_EVENTS_PROCESSING_INTERVAL"

	// AnnotationIgnoreResourceUpdates when set to true on an untracked resource,
	// argo will apply `ignoreResourceUpdates` configuration on it.
	AnnotationIgnoreResourceUpdates = "argocd.argoproj.io/ignore-resource-updates"
)

// GitOps engine cluster cache tuning options
var (
	// clusterCacheResyncDuration controls the duration of cluster cache refresh.
	// NOTE: this differs from gitops-engine default of 24h
	clusterCacheResyncDuration = 12 * time.Hour

	// clusterCacheWatchResyncDuration controls the maximum duration that group/kind watches are allowed to run
	// for before relisting & restarting the watch
	clusterCacheWatchResyncDuration = 10 * time.Minute

	// clusterSyncRetryTimeoutDuration controls the sync retry duration when cluster sync error happens
	clusterSyncRetryTimeoutDuration = 10 * time.Second

	// The default limit of 50 is chosen based on experiments.
	clusterCacheListSemaphoreSize int64 = 50

	// clusterCacheListPageSize is the page size when performing K8s list requests.
	// 500 is equal to kubectl's size
	clusterCacheListPageSize int64 = 500

	// clusterCacheListPageBufferSize is the number of pages to buffer when performing K8s list requests
	clusterCacheListPageBufferSize int32 = 1

	// clusterCacheRetryLimit sets a retry limit for failed requests during cluster cache sync
	// If set to 1, retries are disabled.
	clusterCacheAttemptLimit int32 = 1

	// clusterCacheRetryUseBackoff specifies whether to use a backoff strategy on cluster cache sync, if retry is enabled
	clusterCacheRetryUseBackoff = false

	// clusterCacheBatchEventsProcessing specifies whether to enable batch events processing
	clusterCacheBatchEventsProcessing = false

	// clusterCacheEventsProcessingInterval specifies the interval between processing events when BatchEventsProcessing is enabled
	clusterCacheEventsProcessingInterval = 100 * time.Millisecond
)

func init() {
	clusterCacheResyncDuration = env.ParseDurationFromEnv(EnvClusterCacheResyncDuration, clusterCacheResyncDuration, 0, math.MaxInt64)
	clusterCacheWatchResyncDuration = env.ParseDurationFromEnv(EnvClusterCacheWatchResyncDuration, clusterCacheWatchResyncDuration, 0, math.MaxInt64)
	clusterSyncRetryTimeoutDuration = env.ParseDurationFromEnv(EnvClusterSyncRetryTimeoutDuration, clusterSyncRetryTimeoutDuration, 0, math.MaxInt64)
	clusterCacheListPageSize = env.ParseInt64FromEnv(EnvClusterCacheListPageSize, clusterCacheListPageSize, 0, math.MaxInt64)
	clusterCacheListPageBufferSize = int32(env.ParseNumFromEnv(EnvClusterCacheListPageBufferSize, int(clusterCacheListPageBufferSize), 1, math.MaxInt32))
	clusterCacheListSemaphoreSize = env.ParseInt64FromEnv(EnvClusterCacheListSemaphore, clusterCacheListSemaphoreSize, 0, math.MaxInt64)
	clusterCacheAttemptLimit = int32(env.ParseNumFromEnv(EnvClusterCacheAttemptLimit, int(clusterCacheAttemptLimit), 1, math.MaxInt32))
	clusterCacheRetryUseBackoff = env.ParseBoolFromEnv(EnvClusterCacheRetryUseBackoff, false)
	clusterCacheBatchEventsProcessing = env.ParseBoolFromEnv(EnvClusterCacheBatchEventsProcessing, true)
	clusterCacheEventsProcessingInterval = env.ParseDurationFromEnv(EnvClusterCacheEventsProcessingInterval, clusterCacheEventsProcessingInterval, 0, math.MaxInt64)
}

type LiveStateCache interface {
	// Returns k8s server version
	GetVersionsInfo(server *appv1.Cluster) (string, []kube.APIResourceInfo, error)
	// Returns true of given group kind is a namespaced resource
	IsNamespaced(server *appv1.Cluster, gk schema.GroupKind) (bool, error)
	// Returns synced cluster cache
	GetClusterCache(server *appv1.Cluster) (clustercache.ClusterCache, error)
	// Executes give callback against resources specified by the keys and all its children
	IterateHierarchyV2(server *appv1.Cluster, keys []kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetManagedLiveObjs(destCluster *appv1.Cluster, a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// IterateResources iterates all resource stored in cache
	IterateResources(server *appv1.Cluster, callback func(res *clustercache.Resource, info *ResourceInfo)) error
	// Returns all top level resources (resources without owner references) of a specified namespace
	GetNamespaceTopLevelResources(server *appv1.Cluster, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context) error
	// Returns information about monitored clusters
	GetClustersInfo() []clustercache.ClusterInfo
	// Init must be executed before cache can be used
	Init() error
	// UpdateShard will update the shard of ClusterSharding when the shard has changed.
	UpdateShard(shard int) bool

	// Taint management methods (primarily for testing and internal use)
	MarkClusterTainted(server string, reason string, gvk string, errorType string)
	IsClusterTainted(server string) bool
	GetTaintedGVKs(server string) []string
	ClearClusterTaints(server string)
	ClearGVKTaint(server string, gvk string)
}

type ObjectUpdatedHandler = func(managedByApp map[string]bool, ref corev1.ObjectReference)

type PodInfo struct {
	NodeName         string
	ResourceRequests corev1.ResourceList
	Phase            corev1.PodPhase
}

type NodeInfo struct {
	Name       string
	Capacity   corev1.ResourceList
	SystemInfo corev1.NodeSystemInfo
	Labels     map[string]string
}

type ResourceInfo struct {
	Info    []appv1.InfoItem
	AppName string
	Images  []string
	Health  *health.HealthStatus
	// NetworkingInfo are available only for known types involved into networking: Ingress, Service, Pod
	NetworkingInfo *appv1.ResourceNetworkingInfo
	// PodInfo is available for pods only
	PodInfo *PodInfo
	// NodeInfo is available for nodes only
	NodeInfo *NodeInfo

	manifestHash string
}

func NewLiveStateCache(
	db db.ArgoDB,
	appInformer cache.SharedIndexInformer,
	settingsMgr *settings.SettingsManager,
	metricsServer *metrics.MetricsServer,
	onObjectUpdated ObjectUpdatedHandler,
	clusterSharding sharding.ClusterShardingCache,
	resourceTracking argo.ResourceTracking,
) LiveStateCache {
	return &liveStateCache{
		appInformer:      appInformer,
		db:               db,
		clusters:         make(map[string]clustercache.ClusterCache),
		onObjectUpdated:  onObjectUpdated,
		settingsMgr:      settingsMgr,
		metricsServer:    metricsServer,
		clusterSharding:  clusterSharding,
		resourceTracking: resourceTracking,
		taintManager:     newClusterTaintManager(),
	}
}

type cacheSettings struct {
	clusterSettings     clustercache.Settings
	appInstanceLabelKey string
	trackingMethod      appv1.TrackingMethod
	installationID      string
	// resourceOverrides provides a list of ignored differences to ignore watched resource updates
	resourceOverrides map[string]appv1.ResourceOverride

	// ignoreResourceUpdates is a flag to enable resource-ignore rules.
	ignoreResourceUpdatesEnabled bool
}

type liveStateCache struct {
	db                   db.ArgoDB
	appInformer          cache.SharedIndexInformer
	onObjectUpdated      ObjectUpdatedHandler
	settingsMgr          *settings.SettingsManager
	metricsServer        *metrics.MetricsServer
	clusterSharding      sharding.ClusterShardingCache
	resourceTracking     argo.ResourceTracking
	ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts

	clusters      map[string]clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
	taintManager  *clusterTaintManager
}

// cleanupExpiredFailedGVKs removes expired taint entries using the instance taint manager
func (c *liveStateCache) cleanupExpiredFailedGVKs() {
	c.taintManager.cleanupExpiredTaints()
}

// isResourceGVKFailed checks if a specific GVK is marked as failed for a cluster
func (c *liveStateCache) isResourceGVKFailed(server, gvk string) bool {
	return c.taintManager.isResourceGVKFailed(server, gvk)
}

func (c *liveStateCache) loadCacheSettings() (*cacheSettings, error) {
	appInstanceLabelKey, err := c.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, err
	}
	trackingMethod, err := c.settingsMgr.GetTrackingMethod()
	if err != nil {
		return nil, err
	}
	installationID, err := c.settingsMgr.GetInstallationID()
	if err != nil {
		return nil, err
	}
	resourceUpdatesOverrides, err := c.settingsMgr.GetIgnoreResourceUpdatesOverrides()
	if err != nil {
		return nil, err
	}
	ignoreResourceUpdatesEnabled, err := c.settingsMgr.GetIsIgnoreResourceUpdatesEnabled()
	if err != nil {
		return nil, err
	}
	resourcesFilter, err := c.settingsMgr.GetResourcesFilter()
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := c.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}
	clusterSettings := clustercache.Settings{
		ResourceHealthOverride: lua.ResourceHealthOverrides(resourceOverrides),
		ResourcesFilter:        resourcesFilter,
	}

	return &cacheSettings{clusterSettings, appInstanceLabelKey, appv1.TrackingMethod(trackingMethod), installationID, resourceUpdatesOverrides, ignoreResourceUpdatesEnabled}, nil
}

func asResourceNode(r *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) appv1.ResourceNode {
	gv, err := schema.ParseGroupVersion(r.Ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	parentRefs := make([]appv1.ResourceRef, len(r.OwnerRefs))
	for i, ownerRef := range r.OwnerRefs {
		ownerGvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
		parentRef := appv1.ResourceRef{
			Group:   ownerGvk.Group,
			Kind:    ownerGvk.Kind,
			Version: ownerGvk.Version,
			Name:    ownerRef.Name,
			UID:     string(ownerRef.UID),
		}

		// Look up the parent in namespace resources
		// If found, it's namespaced and we use its namespace
		// If not found, it must be cluster-scoped (namespace = "")
		parentKey := kube.NewResourceKey(ownerGvk.Group, ownerGvk.Kind, r.Ref.Namespace, ownerRef.Name)
		if parent, ok := namespaceResources[parentKey]; ok {
			parentRef.Namespace = parent.Ref.Namespace
		} else {
			// Not in namespace => must be cluster-scoped
			parentRef.Namespace = ""
			// Debug logging for cross-namespace relationships
			if r.Ref.Namespace != "" {
				log.Debugf("Cross-namespace ref: %s/%s in namespace %s has parent %s/%s (cluster-scoped)",
					r.Ref.Kind, r.Ref.Name, r.Ref.Namespace, ownerGvk.Kind, ownerRef.Name)
			}
		}
		parentRefs[i] = parentRef
	}
	var resHealth *appv1.HealthStatus
	resourceInfo := resInfo(r)
	if resourceInfo.Health != nil {
		resHealth = &appv1.HealthStatus{Status: resourceInfo.Health.Status, Message: resourceInfo.Health.Message}
	}
	return appv1.ResourceNode{
		ResourceRef: appv1.ResourceRef{
			UID:       string(r.Ref.UID),
			Name:      r.Ref.Name,
			Group:     gv.Group,
			Version:   gv.Version,
			Kind:      r.Ref.Kind,
			Namespace: r.Ref.Namespace,
		},
		ParentRefs:      parentRefs,
		Info:            resourceInfo.Info,
		ResourceVersion: r.ResourceVersion,
		NetworkingInfo:  resourceInfo.NetworkingInfo,
		Images:          resourceInfo.Images,
		Health:          resHealth,
		CreatedAt:       r.CreationTimestamp,
	}
}

func resInfo(r *clustercache.Resource) *ResourceInfo {
	info, ok := r.Info.(*ResourceInfo)
	if !ok || info == nil {
		info = &ResourceInfo{}
	}
	return info
}

func isRootAppNode(r *clustercache.Resource) bool {
	return resInfo(r).AppName != "" && len(r.OwnerRefs) == 0
}

func getApp(r *clustercache.Resource, ns map[kube.ResourceKey]*clustercache.Resource) string {
	name, _ := getAppRecursive(r, ns, map[kube.ResourceKey]bool{})
	return name
}

func ownerRefGV(ownerRef metav1.OwnerReference) schema.GroupVersion {
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return gv
}

func getAppRecursive(r *clustercache.Resource, ns map[kube.ResourceKey]*clustercache.Resource, visited map[kube.ResourceKey]bool) (string, bool) {
	if visited[r.ResourceKey()] {
		log.Warnf("Circular dependency detected: %v.", visited)
		return resInfo(r).AppName, false
	}
	visited[r.ResourceKey()] = true

	if resInfo(r).AppName != "" {
		return resInfo(r).AppName, true
	}
	for _, ownerRef := range r.OwnerRefs {
		gv := ownerRefGV(ownerRef)
		if parent, ok := ns[kube.NewResourceKey(gv.Group, ownerRef.Kind, r.Ref.Namespace, ownerRef.Name)]; ok {
			visitedBranch := make(map[kube.ResourceKey]bool, len(visited))
			for k, v := range visited {
				visitedBranch[k] = v
			}
			app, ok := getAppRecursive(parent, ns, visitedBranch)
			if app != "" || !ok {
				return app, ok
			}
		}
	}
	return "", true
}

var ignoredRefreshResources = map[string]bool{
	"/" + kube.EndpointsKind: true,
}

// skipAppRequeuing checks if the object is an API type which we want to skip requeuing against.
// We ignore API types which have a high churn rate, and/or whose updates are irrelevant to the app
func skipAppRequeuing(key kube.ResourceKey) bool {
	return ignoredRefreshResources[key.Group+"/"+key.Kind]
}

func skipResourceUpdate(oldInfo, newInfo *ResourceInfo) bool {
	if oldInfo == nil || newInfo == nil {
		return false
	}
	isSameHealthStatus := (oldInfo.Health == nil && newInfo.Health == nil) || oldInfo.Health != nil && newInfo.Health != nil && oldInfo.Health.Status == newInfo.Health.Status
	isSameManifest := oldInfo.manifestHash != "" && newInfo.manifestHash != "" && oldInfo.manifestHash == newInfo.manifestHash
	return isSameHealthStatus && isSameManifest
}

// shouldHashManifest validates if the API resource needs to be hashed.
// If there's an app name from resource tracking, or if this is itself an app, we should generate a hash.
// Otherwise, the hashing should be skipped to save CPU time.
func shouldHashManifest(appName string, gvk schema.GroupVersionKind, un *unstructured.Unstructured) bool {
	// Only hash if the resource belongs to an app OR argocd.argoproj.io/ignore-resource-updates is present and set to true
	// Best      - Only hash for resources that are part of an app or their dependencies
	// (current) - Only hash for resources that are part of an app + all apps that might be from an ApplicationSet
	// Orphan    - If orphan is enabled, hash should be made on all resource of that namespace and a config to disable it
	// Worst     - Hash all resources watched by Argo
	isTrackedResource := appName != "" || (gvk.Group == application.Group && gvk.Kind == application.ApplicationKind)

	// If the resource is not a tracked resource, we will look up argocd.argoproj.io/ignore-resource-updates and decide
	// whether we generate hash or not.
	// If argocd.argoproj.io/ignore-resource-updates is presented and is true, return true
	// Else return false
	if !isTrackedResource {
		if val, ok := un.GetAnnotations()[AnnotationIgnoreResourceUpdates]; ok {
			applyResourcesUpdate, err := strconv.ParseBool(val)
			if err != nil {
				applyResourcesUpdate = false
			}
			return applyResourcesUpdate
		}
		return false
	}

	return isTrackedResource
}

// isRetryableError is a helper method to see whether an error
// returned from the dynamic client is potentially retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	return apierrors.IsInternalError(err) ||
		apierrors.IsInvalid(err) ||
		apierrors.IsTooManyRequests(err) ||
		apierrors.IsServerTimeout(err) ||
		apierrors.IsServiceUnavailable(err) ||
		apierrors.IsTimeout(err) ||
		apierrors.IsUnexpectedObjectError(err) ||
		apierrors.IsUnexpectedServerError(err) ||
		isResourceQuotaConflictErr(err) ||
		isTransientNetworkErr(err) ||
		isExceededQuotaErr(err) ||
		isHTTP2GoawayErr(err) ||
		errors.Is(err, syscall.ECONNRESET)
}

func isHTTP2GoawayErr(err error) bool {
	return strings.Contains(err.Error(), "http2: server sent GOAWAY and closed the connection")
}

func isExceededQuotaErr(err error) bool {
	return apierrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func isResourceQuotaConflictErr(err error) bool {
	return apierrors.IsConflict(err) && strings.Contains(err.Error(), "Operation cannot be fulfilled on resourcequota")
}

func isTransientNetworkErr(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		var dnsErr *net.DNSError
		var opErr *net.OpError
		var unknownNetworkErr net.UnknownNetworkError
		var urlErr *url.Error
		switch {
		case errors.As(err, &dnsErr), errors.As(err, &opErr), errors.As(err, &unknownNetworkErr):
			return true
		case errors.As(err, &urlErr):
			// For a URL error, where it replies "connection closed"
			// retry again.
			return strings.Contains(err.Error(), "Connection closed by foreign host")
		}
	}

	errorString := err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		errorString = fmt.Sprintf("%s %s", errorString, exitErr.Stderr)
	}
	if strings.Contains(errorString, "net/http: TLS handshake timeout") ||
		strings.Contains(errorString, "i/o timeout") ||
		strings.Contains(errorString, "connection timed out") ||
		strings.Contains(errorString, "connection reset by peer") {
		return true
	}
	return false
}

func (c *liveStateCache) getCluster(cluster *appv1.Cluster) (clustercache.ClusterCache, error) {
	c.lock.RLock()
	clusterCache, ok := c.clusters[cluster.Server]
	cacheSettings := c.cacheSettings
	c.lock.RUnlock()

	if ok {
		return clusterCache, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	clusterCache, ok = c.clusters[cluster.Server]
	if ok {
		return clusterCache, nil
	}

	if c.clusterSharding == nil {
		return nil, fmt.Errorf("unable to handle cluster %s: cluster sharding is not configured", cluster.Server)
	}

	if !c.canHandleCluster(cluster) {
		return nil, fmt.Errorf("controller is configured to ignore cluster %s", cluster.Server)
	}

	resourceCustomLabels, err := c.settingsMgr.GetResourceCustomLabels()
	if err != nil {
		return nil, fmt.Errorf("error getting custom label: %w", err)
	}

	respectRBAC, err := c.settingsMgr.RespectRBAC()
	if err != nil {
		return nil, fmt.Errorf("error getting value for %v: %w", settings.RespectRBAC, err)
	}

	clusterCacheConfig, err := cluster.RESTConfig()
	if err != nil {
		return nil, fmt.Errorf("error getting cluster RESTConfig: %w", err)
	}
	// Controller dynamically fetches all resource types available on the cluster
	// using a discovery API that may contain deprecated APIs.
	// This causes log flooding when managing a large number of clusters.
	// https://github.com/argoproj/argo-cd/issues/11973
	// However, we can safely suppress deprecation warnings
	// because we do not rely on resources with a particular API group or version.
	// https://kubernetes.io/blog/2020/09/03/warnings/#customize-client-handling
	//
	// Completely suppress warning logs only for log levels that are less than Debug.
	if log.GetLevel() < log.DebugLevel {
		clusterCacheConfig.WarningHandler = rest.NoWarnings{}
	}

	clusterCacheOpts := []clustercache.UpdateSettingsFunc{
		clustercache.SetListSemaphore(semaphore.NewWeighted(clusterCacheListSemaphoreSize)),
		clustercache.SetListPageSize(clusterCacheListPageSize),
		clustercache.SetListPageBufferSize(clusterCacheListPageBufferSize),
		clustercache.SetWatchResyncTimeout(clusterCacheWatchResyncDuration),
		clustercache.SetClusterSyncRetryTimeout(clusterSyncRetryTimeoutDuration),
		clustercache.SetResyncTimeout(clusterCacheResyncDuration),
		clustercache.SetSettings(cacheSettings.clusterSettings),
		clustercache.SetNamespaces(cluster.Namespaces),
		clustercache.SetClusterResources(cluster.ClusterResources),
		clustercache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (any, bool) {
			res := &ResourceInfo{}
			populateNodeInfo(un, res, resourceCustomLabels)
			c.lock.RLock()
			cacheSettings := c.cacheSettings
			c.lock.RUnlock()

			res.Health, _ = health.GetResourceHealth(un, cacheSettings.clusterSettings.ResourceHealthOverride)

			appName := c.resourceTracking.GetAppName(un, cacheSettings.appInstanceLabelKey, cacheSettings.trackingMethod, cacheSettings.installationID)
			if isRoot && appName != "" {
				res.AppName = appName
			}

			gvk := un.GroupVersionKind()

			if cacheSettings.ignoreResourceUpdatesEnabled && shouldHashManifest(appName, gvk, un) {
				hash, err := generateManifestHash(un, nil, cacheSettings.resourceOverrides, c.ignoreNormalizerOpts)
				if err != nil {
					log.Errorf("Failed to generate manifest hash: %v", err)
				} else {
					res.manifestHash = hash
				}
			}

			// edge case. we do not label CRDs, so they miss the tracking label we inject. But we still
			// want the full resource to be available in our cache (to diff), so we store all CRDs
			return res, res.AppName != "" || gvk.Kind == kube.CustomResourceDefinitionKind
		}),
		clustercache.SetLogr(logutils.NewLogrusLogger(log.WithField("server", cluster.Server))),
		clustercache.SetRetryOptions(clusterCacheAttemptLimit, clusterCacheRetryUseBackoff, isRetryableError),
		clustercache.SetRespectRBAC(respectRBAC),
		clustercache.SetBatchEventsProcessing(clusterCacheBatchEventsProcessing),
		clustercache.SetEventProcessingInterval(clusterCacheEventsProcessingInterval),
	}

	clusterCache = clustercache.NewClusterCache(clusterCacheConfig, clusterCacheOpts...)

	_ = clusterCache.OnResourceUpdated(func(newRes *clustercache.Resource, oldRes *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
		toNotify := make(map[string]bool)
		var ref corev1.ObjectReference
		if newRes != nil {
			ref = newRes.Ref
		} else {
			ref = oldRes.Ref
		}

		c.lock.RLock()
		cacheSettings := c.cacheSettings
		c.lock.RUnlock()

		if cacheSettings.ignoreResourceUpdatesEnabled && oldRes != nil && newRes != nil && skipResourceUpdate(resInfo(oldRes), resInfo(newRes)) {
			// Additional check for debug level so we don't need to evaluate the
			// format string in case of non-debug scenarios
			if log.GetLevel() >= log.DebugLevel {
				namespace := ref.Namespace
				if ref.Namespace == "" {
					namespace = "(cluster-scoped)"
				}
				log.WithFields(log.Fields{
					"server":      cluster.Server,
					"namespace":   namespace,
					"name":        ref.Name,
					"api-version": ref.APIVersion,
					"kind":        ref.Kind,
				}).Debug("Ignoring change of object because none of the watched resource fields have changed")
			}
			return
		}

		for _, r := range []*clustercache.Resource{newRes, oldRes} {
			if r == nil {
				continue
			}
			app := getApp(r, namespaceResources)
			if app == "" || skipAppRequeuing(r.ResourceKey()) {
				continue
			}
			toNotify[app] = isRootAppNode(r) || toNotify[app]
		}
		c.onObjectUpdated(toNotify, ref)
	})

	_ = clusterCache.OnEvent(func(_ watch.EventType, un *unstructured.Unstructured) {
		gvk := un.GroupVersionKind()
		c.metricsServer.IncClusterEventsCount(cluster.Server, gvk.Group, gvk.Kind)
	})

	_ = clusterCache.OnProcessEventsHandler(func(duration time.Duration, processedEventsNumber int) {
		c.metricsServer.ObserveResourceEventsProcessingDuration(cluster.Server, duration, processedEventsNumber)
	})

	c.clusters[cluster.Server] = clusterCache

	return clusterCache, nil
}

func (c *liveStateCache) getSyncedCluster(server *appv1.Cluster) (clustercache.ClusterCache, error) {
	clusterCache, err := c.getCluster(server)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %w", err)
	}

	err = clusterCache.EnsureSynced()
	if err != nil {
		if c.shouldReturnPartialCache(server.Server, err) {
			log.WithField("cluster", server.Server).Warnf("Cluster cache sync error detected for specific resource types, proceeding with partial cache: %v", err)
			return clusterCache, nil
		}
		return nil, fmt.Errorf("error synchronizing cache state: %w", err)
	}

	return clusterCache, nil
}

func (c *liveStateCache) invalidate(cacheSettings cacheSettings) {
	log.Info("invalidating live state cache")
	c.lock.Lock()
	c.cacheSettings = cacheSettings
	clusters := c.clusters
	c.lock.Unlock()

	for server, clust := range clusters {
		clust.Invalidate(clustercache.SetSettings(cacheSettings.clusterSettings))

		// Clear any cluster taints when invalidating a cluster cache
		c.taintManager.clearTaints(server)
	}
	log.Info("live state cache invalidated")
}

func (c *liveStateCache) IsNamespaced(server *appv1.Cluster, gk schema.GroupKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.IsNamespaced(gk)
}

func (c *liveStateCache) IterateHierarchyV2(server *appv1.Cluster, keys []kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}

	// Wrap the cluster iteration with error recovery
	defer func() {
		if r := recover(); r != nil {
			log.WithField("cluster", server.Server).Errorf("Recovered from panic during IterateHierarchyV2: %v", r)
		}
	}()

	clusterInfo.IterateHierarchyV2(keys, func(resource *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) bool {
		return action(asResourceNode(resource, namespaceResources), getApp(resource, namespaceResources))
	})
	return nil
}

func (c *liveStateCache) IterateResources(server *appv1.Cluster, callback func(res *clustercache.Resource, info *ResourceInfo)) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	_ = clusterInfo.FindResources("", func(r *clustercache.Resource) bool {
		if info, ok := r.Info.(*ResourceInfo); ok {
			callback(r, info)
		}
		return false
	})
	return nil
}

func (c *liveStateCache) GetNamespaceTopLevelResources(server *appv1.Cluster, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}

	resources := clusterInfo.FindResources(namespace, clustercache.TopLevelResource)

	// Get all namespace resources for parent lookups
	namespaceResources := clusterInfo.FindResources(namespace, func(_ *clustercache.Resource) bool {
		return true
	})

	res := make(map[kube.ResourceKey]appv1.ResourceNode)

	for k, r := range resources {
		res[k] = asResourceNode(r, namespaceResources)
	}

	return res, nil
}

func (c *liveStateCache) GetManagedLiveObjs(destCluster *appv1.Cluster, a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster(destCluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info for %q: %w", destCluster.Server, err)
	}

	return clusterInfo.GetManagedLiveObjs(targetObjs, func(r *clustercache.Resource) bool {
		return resInfo(r).AppName == a.InstanceName(c.settingsMgr.GetNamespace())
	})
}

func (c *liveStateCache) GetVersionsInfo(server *appv1.Cluster) (string, []kube.APIResourceInfo, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get cluster info for %q: %w", server.Server, err)
	}
	return clusterInfo.GetServerVersion(), clusterInfo.GetAPIResources(), nil
}

func (c *liveStateCache) isClusterHasApps(apps []any, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		app, ok := obj.(*appv1.Application)
		if !ok {
			continue
		}
		destCluster, err := argo.GetDestinationCluster(context.Background(), app.Spec.Destination, c.db)
		if err != nil {
			log.Warnf("Failed to get destination cluster: %v", err)
			continue
		}
		if destCluster.Server == cluster.Server {
			return true
		}
	}
	return false
}

func (c *liveStateCache) watchSettings(ctx context.Context) {
	updateCh := make(chan *settings.ArgoCDSettings, 1)
	c.settingsMgr.Subscribe(updateCh)

	done := false
	for !done {
		select {
		case <-updateCh:
			nextCacheSettings, err := c.loadCacheSettings()
			if err != nil {
				log.Warnf("Failed to read updated settings: %v", err)
				continue
			}

			c.lock.Lock()
			needInvalidate := false
			if !reflect.DeepEqual(c.cacheSettings, *nextCacheSettings) {
				c.cacheSettings = *nextCacheSettings
				needInvalidate = true
			}
			c.lock.Unlock()
			if needInvalidate {
				c.invalidate(*nextCacheSettings)
			}
		case <-ctx.Done():
			done = true
		}
	}
	log.Info("shutting down settings watch")
	c.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
}

func (c *liveStateCache) Init() error {
	cacheSettings, err := c.loadCacheSettings()
	if err != nil {
		return fmt.Errorf("error loading cache settings: %w", err)
	}
	c.cacheSettings = *cacheSettings
	return nil
}

// Run watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (c *liveStateCache) Run(ctx context.Context) error {
	go c.watchSettings(ctx)

	kube.RetryUntilSucceed(ctx, clustercache.ClusterRetryTimeout, "watch clusters", logutils.NewLogrusLogger(logutils.NewWithCurrentConfig()), func() error {
		return c.db.WatchClusters(ctx, c.handleAddEvent, c.handleModEvent, c.handleDeleteEvent)
	})

	<-ctx.Done()
	c.invalidate(c.cacheSettings)
	return nil
}

func (c *liveStateCache) canHandleCluster(cluster *appv1.Cluster) bool {
	return c.clusterSharding.IsManagedCluster(cluster)
}

func (c *liveStateCache) handleAddEvent(cluster *appv1.Cluster) {
	c.clusterSharding.Add(cluster)
	if !c.canHandleCluster(cluster) {
		log.Infof("Ignoring cluster %s", cluster.Server)
		return
	}
	c.lock.Lock()
	_, ok := c.clusters[cluster.Server]
	c.lock.Unlock()
	if !ok {
		log.Debugf("Checking if cache %v / cluster %v has appInformer %v", c, cluster, c.appInformer)
		if c.appInformer == nil {
			log.Warn("Cannot get a cluster appInformer. Cache may not be started this time")
			return
		}
		if c.isClusterHasApps(c.appInformer.GetStore().List(), cluster) {
			go func() {
				// warm up cache for cluster with apps
				_, _ = c.getSyncedCluster(cluster)
			}()
		}
	}
}

func (c *liveStateCache) handleModEvent(oldCluster *appv1.Cluster, newCluster *appv1.Cluster) {
	c.clusterSharding.Update(oldCluster, newCluster)
	c.lock.Lock()
	cluster, ok := c.clusters[newCluster.Server]
	c.lock.Unlock()
	if ok {
		if !c.canHandleCluster(newCluster) {
			cluster.Invalidate()
			c.lock.Lock()
			delete(c.clusters, newCluster.Server)
			c.lock.Unlock()
			return
		}

		var updateSettings []clustercache.UpdateSettingsFunc
		if !reflect.DeepEqual(oldCluster.Config, newCluster.Config) {
			newClusterRESTConfig, err := newCluster.RESTConfig()
			if err == nil {
				updateSettings = append(updateSettings, clustercache.SetConfig(newClusterRESTConfig))
			} else {
				log.Errorf("error getting cluster REST config: %v", err)
			}
		}
		if !reflect.DeepEqual(oldCluster.Namespaces, newCluster.Namespaces) {
			updateSettings = append(updateSettings, clustercache.SetNamespaces(newCluster.Namespaces))
		}
		if !reflect.DeepEqual(oldCluster.ClusterResources, newCluster.ClusterResources) {
			updateSettings = append(updateSettings, clustercache.SetClusterResources(newCluster.ClusterResources))
		}
		forceInvalidate := false
		if newCluster.RefreshRequestedAt != nil &&
			cluster.GetClusterInfo().LastCacheSyncTime != nil &&
			cluster.GetClusterInfo().LastCacheSyncTime.Before(newCluster.RefreshRequestedAt.Time) {
			forceInvalidate = true
		}

		if len(updateSettings) > 0 || forceInvalidate {
			// When explicitly refreshed, we'll validate tainted GVKs AFTER sync
			anyGVKsRecovered := false

			cluster.Invalidate(updateSettings...)

			// When we have tainted GVKs and are doing a hard refresh, sync synchronously
			// to test if the GVKs have recovered and repopulate resources if they have
			switch {
			case forceInvalidate:
				log.WithField("cluster", newCluster.Server).Info("Performing synchronous cache sync for hard refresh to ensure errors are discovered")

				// Try to sync - this will fail for broken GVKs but succeed for healthy ones
				syncErr := cluster.EnsureSynced()

				// After sync, re-validate tainted GVKs to see which ones actually recovered
				anyGVKsRecovered = c.validateAndClearRecoveredTaints(newCluster, cluster)

				if anyGVKsRecovered {
					stillHasTaints := c.IsClusterTainted(newCluster.Server)
					if stillHasTaints {
						log.WithField("cluster", newCluster.Server).Info("Some GVKs recovered during refresh, resources have been restored")
					} else {
						log.WithField("cluster", newCluster.Server).Info("All GVKs recovered from degraded state, all resources restored")
					}
				} else if syncErr != nil {
					log.WithField("cluster", newCluster.Server).WithError(syncErr).Debug("Cache sync completed with errors, no GVKs recovered")
				}
			default:
				// For other cases, warm up cache asynchronously as before
				go func() {
					// warm up cluster cache
					_ = cluster.EnsureSynced()
				}()
			}
		}
	}
}

func (c *liveStateCache) handleDeleteEvent(clusterServer string) {
	c.lock.RLock()
	c.clusterSharding.Delete(clusterServer)
	cluster, ok := c.clusters[clusterServer]
	c.lock.RUnlock()
	if ok {
		cluster.Invalidate()
		c.lock.Lock()
		delete(c.clusters, clusterServer)
		c.lock.Unlock()

		// Also clear any cluster taints for the deleted/invalidated cluster
		c.taintManager.clearTaints(clusterServer)
	}
}

func (c *liveStateCache) GetClustersInfo() []clustercache.ClusterInfo {
	clusters := make(map[string]clustercache.ClusterCache)
	c.lock.RLock()
	for k := range c.clusters {
		clusters[k] = c.clusters[k]
	}
	c.lock.RUnlock()

	res := make([]clustercache.ClusterInfo, 0)
	for server, clusterCache := range clusters {
		info := clusterCache.GetClusterInfo()
		info.Server = server

		res = append(res, info)
	}
	return res
}

func (c *liveStateCache) GetClusterCache(server *appv1.Cluster) (clustercache.ClusterCache, error) {
	return c.getSyncedCluster(server)
}

// UpdateShard will update the shard of ClusterSharding when the shard has changed.
func (c *liveStateCache) UpdateShard(shard int) bool {
	return c.clusterSharding.UpdateShard(shard)
}

// MarkClusterTainted marks a cluster as having tainted cache state
func (c *liveStateCache) MarkClusterTainted(server string, reason string, gvk string, errorType string) {
	c.taintManager.markTainted(server, gvk, errorType, reason)
}

// IsClusterTainted checks if a cluster is in tainted state
func (c *liveStateCache) IsClusterTainted(server string) bool {
	// Check ArgoCD's taint manager
	if c.taintManager.isTainted(server) {
		return true
	}

	// Also check gitops-engine's failed GVKs
	c.lock.RLock()
	clusterCache, ok := c.clusters[server]
	c.lock.RUnlock()

	if ok && clusterCache != nil {
		clusterInfo := clusterCache.GetClusterInfo()
		return len(clusterInfo.FailedResourceGVKs) > 0
	}

	return false
}

// GetTaintedGVKs returns a list of tainted GVKs for a cluster
func (c *liveStateCache) GetTaintedGVKs(server string) []string {
	// First get any taints tracked by ArgoCD's taint manager
	taintedGVKs := c.taintManager.getTaintedGVKs(server)

	// Also get failed GVKs tracked by gitops-engine
	c.lock.RLock()
	clusterCache, ok := c.clusters[server]
	c.lock.RUnlock()

	if ok && clusterCache != nil {
		clusterInfo := clusterCache.GetClusterInfo()
		// Merge the failed GVKs from gitops-engine with ArgoCD's tracked taints
		// Use a map to deduplicate
		gvkMap := make(map[string]bool)
		for _, gvk := range taintedGVKs {
			gvkMap[gvk] = true
		}
		for _, gvk := range clusterInfo.FailedResourceGVKs {
			gvkMap[gvk] = true
		}
		// Convert back to slice
		result := make([]string, 0, len(gvkMap))
		for gvk := range gvkMap {
			result = append(result, gvk)
		}
		return result
	}

	return taintedGVKs
}

// ClearClusterTaints removes all taints for a cluster
func (c *liveStateCache) ClearClusterTaints(server string) {
	c.taintManager.clearTaints(server)
}

// ClearGVKTaint removes a specific GVK taint from a cluster
func (c *liveStateCache) ClearGVKTaint(server string, gvk string) {
	c.taintManager.clearGVKTaint(server, gvk)
}

// shouldReturnPartialCache determines whether to return partial cache based on
// tainted GVKs and error type. Only returns partial cache when:
// 1. Specific GVKs are tainted (indicating partial, not total failure)
// 2. The error is a recoverable cache error (not a connection failure)
func (c *liveStateCache) shouldReturnPartialCache(server string, err error) bool {
	taintedGVKs := c.taintManager.getTaintedGVKs(server)

	// Only return partial cache if we have specific GVK failures AND it's a recoverable error
	if len(taintedGVKs) == 0 || !argoerrors.IsPartialCacheError(err) {
		return false
	}

	log.WithField("cluster", server).WithField("taintedGVKs", taintedGVKs).Debug("Partial cache acceptable for tainted GVKs")
	return true
}

// clusterTaintManager manages cluster taint state in a thread-safe manner
type clusterTaintManager struct {
	mu         sync.RWMutex
	taints     map[string]map[string]string    // server -> gvk -> error type
	taintTimes map[string]map[string]time.Time // server -> gvk -> timestamp
}

// Default expiration time for failed GVKs (30 minutes)
const failedGVKExpirationTime = 30 * time.Minute

// newClusterTaintManager creates a new instance of cluster taint manager
func newClusterTaintManager() *clusterTaintManager {
	return &clusterTaintManager{
		taints:     make(map[string]map[string]string),
		taintTimes: make(map[string]map[string]time.Time),
	}
}

// markTainted marks a specific GVK as tainted for a cluster
func (ctm *clusterTaintManager) markTainted(server, gvk, errorType, reason string) {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	// Initialize if not exists
	if ctm.taints[server] == nil {
		ctm.taints[server] = make(map[string]string)
		ctm.taintTimes[server] = make(map[string]time.Time)
	}

	// Store the GVK, error type, and timestamp
	if gvk != "" {
		ctm.taints[server][gvk] = errorType
		ctm.taintTimes[server][gvk] = time.Now()

		log.WithFields(log.Fields{
			"server":    server,
			"gvk":       gvk,
			"errorType": errorType,
			"reason":    reason,
		}).Warnf("Marked cluster GVK as tainted")
	}
}

// isTainted checks if a cluster is in tainted state
func (ctm *clusterTaintManager) isTainted(server string) bool {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	taints, exists := ctm.taints[server]
	return exists && len(taints) > 0
}

// getTaintedGVKs returns a copy of tainted GVKs for a cluster
func (ctm *clusterTaintManager) getTaintedGVKs(server string) []string {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	taints, exists := ctm.taints[server]
	if !exists {
		return nil
	}

	// Return a copy to avoid concurrent modification issues
	gvks := make([]string, 0, len(taints))
	for gvk := range taints {
		gvks = append(gvks, gvk)
	}
	return gvks
}

// getAllTaints returns a copy of all cluster taints
func (ctm *clusterTaintManager) getAllTaints() map[string]map[string]string {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	// Return a deep copy to prevent external modifications
	result := make(map[string]map[string]string)
	for server, taints := range ctm.taints {
		result[server] = make(map[string]string)
		for gvk, errorType := range taints {
			result[server][gvk] = errorType
		}
	}
	return result
}

// ClearClusterTaints removes all taints for a specific cluster
// clearTaints removes all taints for a cluster
func (ctm *clusterTaintManager) clearTaints(server string) {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	delete(ctm.taints, server)
	delete(ctm.taintTimes, server)
	log.WithFields(log.Fields{
		"server": server,
	}).Info("Cleared cluster taints")
}

// clearGVKTaint removes a specific GVK taint for a cluster
func (ctm *clusterTaintManager) clearGVKTaint(server string, gvk string) {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	if serverTaints, exists := ctm.taints[server]; exists {
		delete(serverTaints, gvk)
		if len(serverTaints) == 0 {
			// If no taints left, remove the server entry
			delete(ctm.taints, server)
			delete(ctm.taintTimes, server)
		}
	}

	log.WithFields(log.Fields{
		"server": server,
		"gvk":    gvk,
	}).Info("Cleared specific GVK taint")
}

// cleanupExpiredTaints removes expired taint entries for all clusters
func (ctm *clusterTaintManager) cleanupExpiredTaints() {
	ctm.mu.Lock()
	defer ctm.mu.Unlock()

	now := time.Now()
	for server, taintTimes := range ctm.taintTimes {
		for gvk, taintTime := range taintTimes {
			if now.Sub(taintTime) > failedGVKExpirationTime {
				// Remove expired taint
				delete(ctm.taints[server], gvk)
				delete(ctm.taintTimes[server], gvk)

				log.WithFields(log.Fields{
					"server": server,
					"gvk":    gvk,
					"age":    now.Sub(taintTime),
				}).Debug("Cleaned up expired cluster taint")
			}
		}

		// Clean up empty server entries
		if len(ctm.taints[server]) == 0 {
			delete(ctm.taints, server)
			delete(ctm.taintTimes, server)
		}
	}
}

// isResourceGVKFailed checks if a specific GVK is marked as failed for a cluster
func (ctm *clusterTaintManager) isResourceGVKFailed(server, gvk string) bool {
	ctm.mu.RLock()
	defer ctm.mu.RUnlock()

	serverTaints, exists := ctm.taints[server]
	if !exists {
		return false
	}

	_, failed := serverTaints[gvk]
	return failed
}

// validateAndClearRecoveredTaints validates tainted GVKs AFTER a cache sync
// and only clears those that successfully synced (have accessible resources).
// Returns true if any GVKs were cleared (recovered).
func (c *liveStateCache) validateAndClearRecoveredTaints(cluster *appv1.Cluster, clusterCache clustercache.ClusterCache) bool {
	taintedGVKs := c.taintManager.getTaintedGVKs(cluster.Server)

	if len(taintedGVKs) == 0 {
		return false
	}

	log.WithField("server", cluster.Server).WithField("taintedGVKs", taintedGVKs).Info("Validating tainted GVKs after sync")

	clearedCount := 0
	for _, gvkString := range taintedGVKs {
		// After a successful sync, resources should be in the cache
		// Try to find resources of this GVK (this may panic if the GVK is still broken)
		var resources map[kube.ResourceKey]*clustercache.Resource
		var findErr error

		func() {
			defer func() {
				if r := recover(); r != nil {
					findErr = fmt.Errorf("GVK %s is still broken: %v", gvkString, r)
				}
			}()
			resources = clusterCache.FindResources("", func(res *clustercache.Resource) bool {
				return res.Ref.GroupVersionKind().String() == gvkString
			})
		}()

		// If FindResources panicked, the GVK is still broken
		if findErr != nil {
			log.WithField("server", cluster.Server).WithField("gvk", gvkString).WithError(findErr).Debug("GVK still tainted - FindResources failed")
			continue
		}

		// If we found resources with valid ResourceVersions, the GVK has recovered
		resourceFound := false
		for _, res := range resources {
			if res.ResourceVersion != "" {
				resourceFound = true
				break
			}
		}

		if resourceFound {
			// GVK has recovered - we found accessible resources after sync
			c.taintManager.clearGVKTaint(cluster.Server, gvkString)
			log.WithField("server", cluster.Server).WithField("gvk", gvkString).Info("GVK recovered - found accessible resources after sync")

			// Note: Stale resources will be automatically refreshed in GetManagedLiveObjs when applications request them

			clearedCount++
		} else {
			log.WithField("server", cluster.Server).WithField("gvk", gvkString).Debug("GVK still tainted - no accessible resources found after sync")
		}
	}

	// Report final taint status
	remainingTaints := c.taintManager.getTaintedGVKs(cluster.Server)
	if len(remainingTaints) == 0 {
		log.WithField("server", cluster.Server).Info("All tainted GVKs validated successfully, cluster cache is now clean")
	} else {
		log.WithField("server", cluster.Server).WithField("remainingTaints", remainingTaints).Info("Some GVKs remain tainted after validation")
	}

	return clearedCount > 0
}

// validateAndClearHealthyTaints is deprecated and no longer used.
// Validation now happens after EnsureSynced via validateAndClearRecoveredTaints.
func (c *liveStateCache) validateAndClearHealthyTaints(_ *appv1.Cluster, _ clustercache.ClusterCache) bool {
	// This function is no longer used but kept for compatibility
	// All validation now happens after sync in validateAndClearRecoveredTaints
	return false
}
