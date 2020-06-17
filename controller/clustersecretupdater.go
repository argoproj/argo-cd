package controller

import (
	"context"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
)

const (
	secretUpdateInterval = 30 * time.Second
)

type clusterSecretUpdater struct {
	infoSource metrics.HasClustersInfo
	db         db.ArgoDB
}

func (c *clusterSecretUpdater) Run(ctx context.Context) {
	ticker := time.NewTicker(secretUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			break
		case <-ticker.C:
			for _, info := range c.infoSource.GetClustersInfo() {
				err := updateClusterConnectionState(c.db, &info)
				if err != nil {
					continue
				}
			}
		}
	}
}

func updateClusterConnectionState(db db.ArgoDB, info *cache.ClusterInfo) error {
	cluster, err := db.GetCluster(context.Background(), info.Server)
	if err != nil {
		return err
	}
	now := metav1.Now()
	cluster.ServerVersion = info.K8SVersion
	cluster.ConnectionState.ModifiedAt = &now
	cluster.ConnectionState.Message = ""
	if info.LastCacheSyncTime == nil {
		cluster.ConnectionState.Status = appv1.ConnectionStatusUnknown
	} else if info.SyncError == nil {
		cluster.ConnectionState.Status = appv1.ConnectionStatusSuccessful
		syncTime := metav1.NewTime(*info.LastCacheSyncTime)
		cluster.CacheInfo.LastCacheSyncTime = &syncTime
		cluster.CacheInfo.APIsCount = int64(info.APIsCount)
		cluster.CacheInfo.ResourcesCount = int64(info.ResourcesCount)
	} else {
		cluster.ConnectionState.Status = appv1.ConnectionStatusFailed
		cluster.ConnectionState.Message = info.SyncError.Error()
	}
	if _, err := db.UpdateCluster(context.Background(), cluster); err != nil {
		log.Warnf("Unable to update Cluster %s: %v", cluster.Server, err)
	}

	return err
}
