package metrics

import (
	"context"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/util/db"
)

type clusterSecretUpdater struct {
	infoSource HasClustersInfo
	lock       sync.Mutex
	db         db.ArgoDB
}

func (c clusterSecretUpdater) Run(ctx context.Context) {
	tick := time.Tick(metricsCollectionInterval)
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
				if toUpdate {
					c.db.UpdateCluster(context.Background(), cluster)
				}
			}
		}
	}
}
