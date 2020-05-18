package controller

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/util/db"
)

const (
	secretUpdateInterval = 30 * time.Second
)

type clusterSecretUpdater struct {
	infoSource metrics.HasClustersInfo
	lock       sync.Mutex
	db         db.ArgoDB
}

func (c clusterSecretUpdater) Run(ctx context.Context) {
	tick := time.Tick(secretUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			break
		case <-tick:
			for _, info := range c.infoSource.GetClustersInfo() {
				cluster, err := c.db.GetCluster(context.Background(), info.Server)
				if err != nil {
					continue
				}
				toUpdate := false
				if info.K8SVersion != "" && info.K8SVersion != cluster.ServerVersion {
					cluster.ServerVersion = info.K8SVersion
					toUpdate = true
				}
				if info.LastCacheSyncTime != nil {
					if cluster.ConnectionState.ModifiedAt == nil ||
						(cluster.ConnectionState.ModifiedAt != nil && cluster.ConnectionState.ModifiedAt.Time != *info.LastCacheSyncTime) {
						cluster.ConnectionState.ModifiedAt = &metav1.Time{Time: *info.LastCacheSyncTime}
						toUpdate = true
					}
				}
				if info.SyncError != nil {
					cluster.ConnectionState.Message = info.SyncError.Error()
					toUpdate = true
				}
				if toUpdate {
					_, err := c.db.UpdateCluster(context.Background(), cluster)
					if err != nil {
						log.Warnf("Unable to pdate Cluster %s: %v", cluster.Server, err)
					}
				}
			}
		}
	}
}
