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
	tick := time.Tick(secretUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			break
		case <-tick:
			for _, info := range c.infoSource.GetClustersInfo() {
				err := updateClusterFromClusterCache(c.db, &info)
				if err != nil {
					continue
				}
			}
		}
	}
}

func updateClusterFromClusterCache(db db.ArgoDB, info *cache.ClusterInfo) error {
	cluster, err := db.GetCluster(context.Background(), info.Server)
	if err != nil {
		return err
	}
	toUpdate := false
	if info.K8SVersion != cluster.ServerVersion {
		cluster.ServerVersion = info.K8SVersion
		toUpdate = true
	}
	if info.LastCacheSyncTime == nil {
		if cluster.ConnectionState.ModifiedAt != nil {
			cluster.ConnectionState.ModifiedAt = nil
			toUpdate = true
		}
	} else {
		if cluster.ConnectionState.ModifiedAt == nil {
			cluster.ConnectionState.ModifiedAt = &metav1.Time{Time: *info.LastCacheSyncTime}
			toUpdate = true
		} else {
			if cluster.ConnectionState.ModifiedAt.Format(time.RFC3339) != (*info.LastCacheSyncTime).Format(time.RFC3339) {
				cluster.ConnectionState.ModifiedAt = &metav1.Time{Time: *info.LastCacheSyncTime}
				toUpdate = true
			}
		}
	}
	if info.SyncError == nil {
		if cluster.ConnectionState.Message != "" {
			cluster.ConnectionState.Message = ""
			toUpdate = true
		}
	} else {
		if info.SyncError.Error() != cluster.ConnectionState.Message {
			cluster.ConnectionState.Message = info.SyncError.Error()
			toUpdate = true
		}
	}
	connectionStatus := getConnectionStatus(info)
	if connectionStatus != cluster.ConnectionState.Status {
		cluster.ConnectionState.Status = connectionStatus
		toUpdate = true
	}
	if toUpdate {
		_, err := db.UpdateCluster(context.Background(), cluster)
		if err != nil {
			log.Warnf("Unable to update Cluster %s: %v", cluster.Server, err)
		}
	}
	return err
}

func getConnectionStatus(info *cache.ClusterInfo) appv1.ConnectionStatus {
	var connectionStatus appv1.ConnectionStatus
	if info.LastCacheSyncTime == nil {
		connectionStatus = appv1.ConnectionStatusUnknown
	} else {
		if info.SyncError == nil {
			connectionStatus = appv1.ConnectionStatusSuccessful
		} else {
			connectionStatus = appv1.ConnectionStatusFailed
		}
	}
	return connectionStatus
}
