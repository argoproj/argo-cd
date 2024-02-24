package sharding

import (
	"sync"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	log "github.com/sirupsen/logrus"
)

type ClusterShardingCache interface {
	Init(clusters *v1alpha1.ClusterList)
	Add(c *v1alpha1.Cluster)
	Delete(clusterServer string)
	Update(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster)
	IsManagedCluster(c *v1alpha1.Cluster) bool
	GetDistribution() map[string]int
}

type ClusterSharding struct {
	Shard           int
	Replicas        int
	Shards          map[string]int
	Clusters        map[string]*v1alpha1.Cluster
	lock            sync.RWMutex
	getClusterShard DistributionFunction
}

func NewClusterSharding(_ db.ArgoDB, shard, replicas int, shardingAlgorithm string) ClusterShardingCache {
	log.Debugf("Processing clusters from shard %d: Using filter function:  %s", shard, shardingAlgorithm)
	clusterSharding := &ClusterSharding{
		Shard:    shard,
		Replicas: replicas,
		Shards:   make(map[string]int),
		Clusters: make(map[string]*v1alpha1.Cluster),
	}
	distributionFunction := NoShardingDistributionFunction()
	if replicas > 1 {
		log.Debugf("Processing clusters from shard %d: Using filter function:  %s", shard, shardingAlgorithm)
		distributionFunction = GetDistributionFunction(clusterSharding.GetClusterAccessor(), shardingAlgorithm, replicas)
	} else {
		log.Info("Processing all cluster shards")
	}
	clusterSharding.getClusterShard = distributionFunction
	return clusterSharding
}

// IsManagedCluster returns wheter or not the cluster should be processed by a given shard.
func (s *ClusterSharding) IsManagedCluster(c *v1alpha1.Cluster) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if c == nil { // nil cluster (in-cluster) is always managed by current clusterShard
		return true
	}
	clusterShard := 0
	if shard, ok := s.Shards[c.Server]; ok {
		clusterShard = shard
	} else {
		log.Warnf("The cluster %s has no assigned shard.", c.Server)
	}
	log.Debugf("Checking if cluster %s with clusterShard %d should be processed by shard %d", c.Server, clusterShard, s.Shard)
	return clusterShard == s.Shard
}

func (sharding *ClusterSharding) Init(clusters *v1alpha1.ClusterList) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	newClusters := make(map[string]*v1alpha1.Cluster, len(clusters.Items))
	for _, c := range clusters.Items {
		cluster := c
		newClusters[c.Server] = &cluster
	}
	sharding.Clusters = newClusters
	sharding.updateDistribution()
}

func (sharding *ClusterSharding) Add(c *v1alpha1.Cluster) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	old, ok := sharding.Clusters[c.Server]
	sharding.Clusters[c.Server] = c
	if !ok || hasShardingUpdates(old, c) {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. Cluster already added")
	}
}

func (sharding *ClusterSharding) Delete(clusterServer string) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	if _, ok := sharding.Clusters[clusterServer]; ok {
		delete(sharding.Clusters, clusterServer)
		delete(sharding.Shards, clusterServer)
		sharding.updateDistribution()
	}
}

func (sharding *ClusterSharding) Update(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	if _, ok := sharding.Clusters[oldCluster.Server]; ok && oldCluster.Server != newCluster.Server {
		delete(sharding.Clusters, oldCluster.Server)
		delete(sharding.Shards, oldCluster.Server)
	}
	sharding.Clusters[newCluster.Server] = newCluster
	if hasShardingUpdates(oldCluster, newCluster) {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. No relevant changes")
	}
}

func (sharding *ClusterSharding) GetDistribution() map[string]int {
	sharding.lock.RLock()
	defer sharding.lock.RUnlock()
	shards := sharding.Shards

	distribution := make(map[string]int, len(shards))
	for k, v := range shards {
		distribution[k] = v
	}
	return distribution
}

func (sharding *ClusterSharding) updateDistribution() {
	for k, c := range sharding.Clusters {
		shard := 0
		if c.Shard != nil {
			requestedShard := int(*c.Shard)
			if requestedShard < sharding.Replicas {
				shard = requestedShard
			} else {
				log.Warnf("Specified cluster shard (%d) for cluster: %s is greater than the number of available shard (%d). Using shard 0.", requestedShard, c.Server, sharding.Replicas)
			}
		} else {
			shard = sharding.getClusterShard(c)
		}

		existingShard, ok := sharding.Shards[k]
		if ok && existingShard != shard {
			log.Infof("Cluster %s has changed shard from %d to %d", k, existingShard, shard)
		} else if !ok {
			log.Infof("Cluster %s has been assigned to shard %d", k, shard)
		} else {
			log.Debugf("Cluster %s has not changed shard", k)
		}
		sharding.Shards[k] = shard
	}
}

// hasShardingUpdates returns true if the sharding distribution has explicitly changed
func hasShardingUpdates(old, new *v1alpha1.Cluster) bool {
	if old == nil || new == nil {
		return false
	}

	// returns true if the cluster id has changed because some sharding algorithms depend on it.
	if old.ID != new.ID {
		return true
	}

	if old.Server != new.Server {
		return true
	}

	// return false if the shard field has not been modified
	if old.Shard == nil && new.Shard == nil {
		return false
	}
	return old.Shard == nil || new.Shard == nil || int64(*old.Shard) != int64(*new.Shard)
}

func (d *ClusterSharding) GetClusterAccessor() clusterAccessor {
	return func() []*v1alpha1.Cluster {
		// no need to lock, as this is only called from the updateDistribution function
		clusters := make([]*v1alpha1.Cluster, 0, len(d.Clusters))
		for _, c := range d.Clusters {
			clusters = append(clusters, c)
		}
		return clusters
	}
}
