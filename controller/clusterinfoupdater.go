package controller

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/db"
)

const (
	secretUpdateInterval = 10 * time.Second
)

type clusterInfoUpdater struct {
	infoSource metrics.HasClustersInfo
	db         db.ArgoDB
	appLister  v1alpha1.ApplicationNamespaceLister
	cache      *appstatecache.Cache
}

func NewClusterInfoUpdater(
	infoSource metrics.HasClustersInfo,
	db db.ArgoDB,
	appLister v1alpha1.ApplicationNamespaceLister,
	cache *appstatecache.Cache) *clusterInfoUpdater {

	return &clusterInfoUpdater{infoSource, db, appLister, cache}
}

func (c *clusterInfoUpdater) Run(ctx context.Context) {
	c.updateClusters()
	ticker := time.NewTicker(secretUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			break
		case <-ticker.C:
			c.updateClusters()
		}
	}
}

func (c *clusterInfoUpdater) updateClusters() {
	infoByServer := make(map[string]*cache.ClusterInfo)
	clustersInfo := c.infoSource.GetClustersInfo()
	for i := range clustersInfo {
		info := clustersInfo[i]
		infoByServer[info.Server] = &info
	}
	clusters, err := c.db.ListClusters(context.Background())
	if err != nil {
		log.Warnf("Failed to save clusters info: %v", err)
	}
	_ = kube.RunAllAsync(len(clusters.Items), func(i int) error {
		cluster := clusters.Items[i]
		if err := c.updateClusterInfo(cluster, infoByServer[cluster.Server]); err != nil {
			log.Warnf("Failed to save clusters info: %v", err)
		}
		return nil
	})
}

func (c *clusterInfoUpdater) updateClusterInfo(cluster appv1.Cluster, info *cache.ClusterInfo) error {
	apps, err := c.appLister.List(labels.Everything())
	if err != nil {
		return err
	}
	var appCount int64
	for _, a := range apps {
		if err := argo.ValidateDestination(context.Background(), &a.Spec.Destination, c.db); err != nil {
			continue
		}
		if a.Spec.Destination.Server == cluster.Server {
			appCount += 1
		}
	}
	now := metav1.Now()
	clusterInfo := appv1.ClusterInfo{
		ConnectionState:   appv1.ConnectionState{ModifiedAt: &now},
		ApplicationsCount: appCount,
	}
	if info != nil {
		clusterInfo.ServerVersion = info.K8SVersion
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
			clusterInfo.ConnectionState.Message = "Cluster has no application and not being monitored."
		}
	}

	return c.cache.SetClusterInfo(cluster.Server, &clusterInfo)
}
