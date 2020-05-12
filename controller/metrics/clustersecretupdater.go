package metrics

import (
	"context"
	"sync"
	"time"

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
				cluster.ServerVersion = info.K8SVersion
				cluster.LastCacheSyncTime = info.LastCacheSyncTime.String()
				c.db.UpdateCluster(context.Background(), cluster)
			}

		}
	}
}
