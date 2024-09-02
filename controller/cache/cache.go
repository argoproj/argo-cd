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
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/controller/sharding"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	"github.com/argoproj/argo-cd/v2/util/lua"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
	clusterCacheRetryUseBackoff bool = false
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
}

type LiveStateCache interface {
	// Returns k8s server version
	GetVersionsInfo(serverURL string) (string, []kube.APIResourceInfo, error)
	// Returns true of given group kind is a namespaced resource
	IsNamespaced(server string, gk schema.GroupKind) (bool, error)
	// Returns synced cluster cache
	GetClusterCache(server string) (clustercache.ClusterCache, error)
	// Executes give callback against resource specified by the key and all its children
	IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error
	// Executes give callback against resources specified by the keys and all its children
	IterateHierarchyV2(server string, keys []kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// IterateResources iterates all resource stored in cache
	IterateResources(server string, callback func(res *clustercache.Resource, info *ResourceInfo)) error
	// Returns all top level resources (resources without owner references) of a specified namespace
	GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context) error
	// Returns information about monitored clusters
	GetClustersInfo() []clustercache.ClusterInfo
	// Init must be executed before cache can be used
	Init() error
}

type ObjectUpdatedHandler = func(managedByApp map[string]bool, ref v1.ObjectReference)

type PodInfo struct {
	NodeName         string
	ResourceRequests v1.ResourceList
	Phase            v1.PodPhase
}

type NodeInfo struct {
	Name       string
	Capacity   v1.ResourceList
	SystemInfo v1.NodeSystemInfo
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
	kubectl kube.Kubectl,
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
		kubectl:          kubectl,
		settingsMgr:      settingsMgr,
		metricsServer:    metricsServer,
		clusterSharding:  clusterSharding,
		resourceTracking: resourceTracking,
	}
}

type cacheSettings struct {
	clusterSettings     clustercache.Settings
	appInstanceLabelKey string
	trackingMethod      appv1.TrackingMethod
	// resourceOverrides provides a list of ignored differences to ignore watched resource updates
	resourceOverrides map[string]appv1.ResourceOverride

	// ignoreResourceUpdates is a flag to enable resource-ignore rules.
	ignoreResourceUpdatesEnabled bool
}

type liveStateCache struct {
	db                   db.ArgoDB
	appInformer          cache.SharedIndexInformer
	onObjectUpdated      ObjectUpdatedHandler
	kubectl              kube.Kubectl
	settingsMgr          *settings.SettingsManager
	metricsServer        *metrics.MetricsServer
	clusterSharding      sharding.ClusterShardingCache
	resourceTracking     argo.ResourceTracking
	ignoreNormalizerOpts normalizers.IgnoreNormalizerOpts

	clusters      map[string]clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
}

func (c *liveStateCache) loadCacheSettings() (*cacheSettings, error) {
	appInstanceLabelKey, err := c.settingsMgr.GetAppInstanceLabelKey()
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

	return &cacheSettings{clusterSettings, appInstanceLabelKey, argo.GetTrackingMethod(c.settingsMgr), resourceUpdatesOverrides, ignoreResourceUpdatesEnabled}, nil
}

func asResourceNode(r *clustercache.Resource) appv1.ResourceNode {
	gv, err := schema.ParseGroupVersion(r.Ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	parentRefs := make([]appv1.ResourceRef, len(r.OwnerRefs))
	for i, ownerRef := range r.OwnerRefs {
		ownerGvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
		ownerKey := kube.NewResourceKey(ownerGvk.Group, ownerRef.Kind, r.Ref.Namespace, ownerRef.Name)
		parentRefs[i] = appv1.ResourceRef{Name: ownerRef.Name, Kind: ownerKey.Kind, Namespace: r.Ref.Namespace, Group: ownerKey.Group, UID: string(ownerRef.UID)}
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
	if !visited[r.ResourceKey()] {
		visited[r.ResourceKey()] = true
	} else {
		log.Warnf("Circular dependency detected: %v.", visited)
		return resInfo(r).AppName, false
	}

	if resInfo(r).AppName != "" {
		return resInfo(r).AppName, true
	}
	for _, ownerRef := range r.OwnerRefs {
		gv := ownerRefGV(ownerRef)
		if parent, ok := ns[kube.NewResourceKey(gv.Group, ownerRef.Kind, r.Ref.Namespace, ownerRef.Name)]; ok {
			visited_branch := make(map[kube.ResourceKey]bool, len(visited))
			for k, v := range visited {
				visited_branch[k] = v
			}
			app, ok := getAppRecursive(parent, ns, visited_branch)
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
	return kerrors.IsInternalError(err) ||
		kerrors.IsInvalid(err) ||
		kerrors.IsTooManyRequests(err) ||
		kerrors.IsServerTimeout(err) ||
		kerrors.IsServiceUnavailable(err) ||
		kerrors.IsTimeout(err) ||
		kerrors.IsUnexpectedObjectError(err) ||
		kerrors.IsUnexpectedServerError(err) ||
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
	return kerrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func isResourceQuotaConflictErr(err error) bool {
	return kerrors.IsConflict(err) && strings.Contains(err.Error(), "Operation cannot be fulfilled on resourcequota")
}

func isTransientNetworkErr(err error) bool {
	var netErr net.Error
	switch {
	case errors.As(err, &netErr):
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

func (c *liveStateCache) getCluster(server string) (clustercache.ClusterCache, error) {
	c.lock.RLock()
	clusterCache, ok := c.clusters[server]
	cacheSettings := c.cacheSettings
	c.lock.RUnlock()

	if ok {
		return clusterCache, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	clusterCache, ok = c.clusters[server]
	if ok {
		return clusterCache, nil
	}

	cluster, err := c.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %w", err)
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

	clusterCacheConfig := cluster.RESTConfig()
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
		clustercache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (interface{}, bool) {
			res := &ResourceInfo{}
			populateNodeInfo(un, res, resourceCustomLabels)
			c.lock.RLock()
			cacheSettings := c.cacheSettings
			c.lock.RUnlock()

			res.Health, _ = health.GetResourceHealth(un, cacheSettings.clusterSettings.ResourceHealthOverride)

			appName := c.resourceTracking.GetAppName(un, cacheSettings.appInstanceLabelKey, cacheSettings.trackingMethod)
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
	}

	clusterCache = clustercache.NewClusterCache(clusterCacheConfig, clusterCacheOpts...)

	_ = clusterCache.OnResourceUpdated(func(newRes *clustercache.Resource, oldRes *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
		toNotify := make(map[string]bool)
		var ref v1.ObjectReference
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

	_ = clusterCache.OnEvent(func(event watch.EventType, un *unstructured.Unstructured) {
		gvk := un.GroupVersionKind()
		c.metricsServer.IncClusterEventsCount(cluster.Server, gvk.Group, gvk.Kind)
	})

	c.clusters[server] = clusterCache

	return clusterCache, nil
}

func (c *liveStateCache) getSyncedCluster(server string) (clustercache.ClusterCache, error) {
	clusterCache, err := c.getCluster(server)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster: %w", err)
	}
	err = clusterCache.EnsureSynced()
	if err != nil {
		return nil, fmt.Errorf("error synchronizing cache state : %w", err)
	}
	return clusterCache, nil
}

func (c *liveStateCache) invalidate(cacheSettings cacheSettings) {
	log.Info("invalidating live state cache")
	c.lock.Lock()
	c.cacheSettings = cacheSettings
	clusters := c.clusters
	c.lock.Unlock()

	for _, clust := range clusters {
		clust.Invalidate(clustercache.SetSettings(cacheSettings.clusterSettings))
	}
	log.Info("live state cache invalidated")
}

func (c *liveStateCache) IsNamespaced(server string, gk schema.GroupKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.IsNamespaced(gk)
}

func (c *liveStateCache) IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	clusterInfo.IterateHierarchy(key, func(resource *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) bool {
		return action(asResourceNode(resource), getApp(resource, namespaceResources))
	})
	return nil
}

func (c *liveStateCache) IterateHierarchyV2(server string, keys []kube.ResourceKey, action func(child appv1.ResourceNode, appName string) bool) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	clusterInfo.IterateHierarchyV2(keys, func(resource *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) bool {
		return action(asResourceNode(resource), getApp(resource, namespaceResources))
	})
	return nil
}

func (c *liveStateCache) IterateResources(server string, callback func(res *clustercache.Resource, info *ResourceInfo)) error {
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

func (c *liveStateCache) GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}
	resources := clusterInfo.FindResources(namespace, clustercache.TopLevelResource)
	res := make(map[kube.ResourceKey]appv1.ResourceNode)
	for k, r := range resources {
		res[k] = asResourceNode(r)
	}
	return res, nil
}

func (c *liveStateCache) GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster(a.Spec.Destination.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info for %q: %w", a.Spec.Destination.Server, err)
	}
	return clusterInfo.GetManagedLiveObjs(targetObjs, func(r *clustercache.Resource) bool {
		return resInfo(r).AppName == a.InstanceName(c.settingsMgr.GetNamespace())
	})
}

func (c *liveStateCache) GetVersionsInfo(serverURL string) (string, []kube.APIResourceInfo, error) {
	clusterInfo, err := c.getSyncedCluster(serverURL)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get cluster info for %q: %w", serverURL, err)
	}
	return clusterInfo.GetServerVersion(), clusterInfo.GetAPIResources(), nil
}

func (c *liveStateCache) isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		app, ok := obj.(*appv1.Application)
		if !ok {
			continue
		}
		err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, c.db)
		if err != nil {
			continue
		}
		if app.Spec.Destination.Server == cluster.Server {
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
				_, _ = c.getSyncedCluster(cluster.Server)
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
			updateSettings = append(updateSettings, clustercache.SetConfig(newCluster.RESTConfig()))
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
			cluster.Invalidate(updateSettings...)
			go func() {
				// warm up cluster cache
				_ = cluster.EnsureSynced()
			}()
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
	for server, c := range clusters {
		info := c.GetClusterInfo()
		info.Server = server
		res = append(res, info)
	}
	return res
}

func (c *liveStateCache) GetClusterCache(server string) (clustercache.ClusterCache, error) {
	return c.getSyncedCluster(server)
}
