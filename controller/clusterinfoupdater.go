package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v2/common"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/v2/util/env"

	"github.com/argoproj/argo-cd/v2/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
)

const (
	defaultSecretUpdateInterval = 10 * time.Second

	EnvClusterInfoTimeout = "ARGO_CD_UPDATE_CLUSTER_INFO_TIMEOUT"
)

var clusterInfoTimeout = env.ParseDurationFromEnv(EnvClusterInfoTimeout, defaultSecretUpdateInterval, defaultSecretUpdateInterval, 1*time.Minute)

type clusterInfoUpdater struct {
	infoSource    metrics.HasClustersInfo
	db            db.ArgoDB
	appLister     v1alpha1.ApplicationNamespaceLister
	cache         *appstatecache.Cache
	clusterFilter func(cluster *appv1.Cluster) bool
	projGetter    func(app *appv1.Application) (*appv1.AppProject, error)
	namespace     string
	lastUpdated   time.Time
}

func NewClusterInfoUpdater(
	infoSource metrics.HasClustersInfo,
	db db.ArgoDB,
	appLister v1alpha1.ApplicationNamespaceLister,
	cache *appstatecache.Cache,
	clusterFilter func(cluster *appv1.Cluster) bool,
	projGetter func(app *appv1.Application) (*appv1.AppProject, error),
	namespace string,
) *clusterInfoUpdater {
	return &clusterInfoUpdater{infoSource, db, appLister, cache, clusterFilter, projGetter, namespace, time.Time{}}
}

func (c *clusterInfoUpdater) Run(ctx context.Context) {
	c.updateClusters()
	ticker := time.NewTicker(clusterInfoTimeout)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			c.updateClusters()
		}
	}
}

func (c *clusterInfoUpdater) updateClusters() {
	if time.Since(c.lastUpdated) < clusterInfoTimeout {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), clusterInfoTimeout)
	defer func() {
		cancel()
		c.lastUpdated = time.Now()
	}()

	infoByServer := make(map[string]*cache.ClusterInfo)
	clustersInfo := c.infoSource.GetClustersInfo()
	for i := range clustersInfo {
		info := clustersInfo[i]
		infoByServer[info.Server] = &info
	}
	clusters, err := c.db.ListClusters(ctx)
	if err != nil {
		log.Warnf("Failed to save clusters info: %v", err)
		return
	}
	var clustersFiltered []appv1.Cluster
	if c.clusterFilter == nil {
		clustersFiltered = clusters.Items
	} else {
		for i := range clusters.Items {
			if c.clusterFilter(&clusters.Items[i]) {
				clustersFiltered = append(clustersFiltered, clusters.Items[i])
			}
		}
	}
	_ = kube.RunAllAsync(len(clustersFiltered), func(i int) error {
		cluster := clustersFiltered[i]
		clusterInfo := infoByServer[cluster.Server]
		if err := c.updateClusterInfo(ctx, cluster, clusterInfo); err != nil {
			log.Warnf("Failed to save cluster info: %v", err)
		} else if err := updateClusterLabels(ctx, clusterInfo, cluster, c.db.UpdateCluster); err != nil {
			log.Warnf("Failed to update cluster labels: %v", err)
		}
		return nil
	})
	log.Debugf("Successfully saved info of %d clusters", len(clustersFiltered))
}

func (c *clusterInfoUpdater) updateClusterInfo(ctx context.Context, cluster appv1.Cluster, info *cache.ClusterInfo) error {
	apps, err := c.appLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error while fetching the apps list: %w", err)
	}

	updated := c.getUpdatedClusterInfo(ctx, apps, cluster, info, metav1.Now())
	return c.cache.SetClusterInfo(cluster.Server, &updated)
}

func (c *clusterInfoUpdater) getUpdatedClusterInfo(ctx context.Context, apps []*appv1.Application, cluster appv1.Cluster, info *cache.ClusterInfo, now metav1.Time) appv1.ClusterInfo {
	var appCount int64
	for _, a := range apps {
		if c.projGetter != nil {
			proj, err := c.projGetter(a)
			if err != nil || !proj.IsAppNamespacePermitted(a, c.namespace) {
				continue
			}
		}
		if err := argo.ValidateDestination(ctx, &a.Spec.Destination, c.db); err != nil {
			continue
		}
		if a.Spec.Destination.Server == cluster.Server {
			appCount += 1
		}
	}
	clusterInfo := appv1.ClusterInfo{
		ConnectionState:   appv1.ConnectionState{ModifiedAt: &now},
		ApplicationsCount: appCount,
	}
	if info != nil {
		clusterInfo.ServerVersion = info.K8SVersion
		clusterInfo.APIVersions = argo.APIResourcesToStrings(info.APIResources, true)
		if info.LastCacheSyncTime == nil {
			clusterInfo.ConnectionState.Status = appv1.ConnectionStatusUnknown
		} else if info.SyncError == nil {
			clusterInfo.ConnectionState.Status = appv1.ConnectionStatusSuccessful
			syncTime := metav1.NewTime(*info.LastCacheSyncTime)
			clusterInfo.CacheInfo.LastCacheSyncTime = &syncTime
			clusterInfo.CacheInfo.APIsCount = int64(info.APIsCount)
			clusterInfo.CacheInfo.ResourcesCount = int64(info.ResourcesCount)
		} else {
			clusterInfo.ConnectionState.Status = appv1.ConnectionStatusFailed
			clusterInfo.ConnectionState.Message = info.SyncError.Error()
		}
	} else {
		clusterInfo.ConnectionState.Status = appv1.ConnectionStatusUnknown
		if appCount == 0 {
			clusterInfo.ConnectionState.Message = "Cluster has no applications and is not being monitored."
		}
	}

	return clusterInfo
}

func updateClusterLabels(ctx context.Context, clusterInfo *cache.ClusterInfo, cluster appv1.Cluster, updateCluster func(context.Context, *appv1.Cluster) (*appv1.Cluster, error)) error {
	if clusterInfo != nil && cluster.Labels[common.LabelKeyAutoLabelClusterInfo] == "true" && cluster.Labels[common.LabelKeyClusterKubernetesVersion] != clusterInfo.K8SVersion {
		cluster.Labels[common.LabelKeyClusterKubernetesVersion] = clusterInfo.K8SVersion
		_, err := updateCluster(ctx, &cluster)
		return err
	}

	return nil
}
