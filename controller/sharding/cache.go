package sharding

import (
	"sync"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	log "github.com/sirupsen/logrus"
)

type ClusterSharding interface {
	Init(clusters *v1alpha1.ClusterList)
	Add(c *v1alpha1.Cluster)
	Delete(clusterServer string)
	Update(c *v1alpha1.Cluster)
	IsManagedCluster(c *v1alpha1.Cluster) bool
	GetDistribution() map[string]int
}

type clusterSharding struct {
	shard           int
	replicas        int
	shards          map[string]int
	clusters        map[string]*v1alpha1.Cluster
	lock            sync.RWMutex
	getClusterShard DistributionFunction
}

func NewClusterSharding(db db.ArgoDB, shard, replicas int, shardingAlgorithm string) ClusterSharding {

	log.Infof("Processing clusters from shard %d", shard)
	log.Infof("Using filter function:  %s", shardingAlgorithm)
	clusterSharding := &clusterSharding{
		shard:    shard,
		replicas: replicas,
		shards:   make(map[string]int),
		clusters: make(map[string]*v1alpha1.Cluster),
	}

	distributionFunction := NoShardingDistributionFunction(0)
	if replicas > 1 {
		log.Infof("Processing clusters from shard %d", shard)
		log.Infof("Using filter function:  %s", shardingAlgorithm)
		distributionFunction = GetDistributionFunction(db, clusterSharding.getClusterAccessor(), shardingAlgorithm)
	} else {
		log.Info("Processing all cluster shards")
	}
	clusterSharding.getClusterShard = distributionFunction
	return clusterSharding
}

// IsManagedCluster returns wheter or not the cluster should be processed by a given shard.
func (s *clusterSharding) IsManagedCluster(c *v1alpha1.Cluster) bool {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if c == nil { // nil cluster (in-cluster) is always managed by current clusterShard
		return true
	}
	clusterShard := 0
	if shard, ok := s.shards[c.Server]; ok {
		clusterShard = shard
	} else {
		log.Warnf("The cluster %s has no assigned shard.", c.Server)
	}
	log.Debugf("Checking if cluster %s with clusterShard %d should be processed by shard %d", c.Server, clusterShard, s.shard)
	return clusterShard == s.shard
	// If the clusterShard is never equal to sharding.shard accross all shards, we should check that a shard infered
	// from hostname with value 0 exists
}

func (sharding *clusterSharding) Init(clusters *v1alpha1.ClusterList) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	newClusters := make(map[string]*v1alpha1.Cluster, len(clusters.Items))
	for _, c := range clusters.Items {
		newClusters[c.Server] = &c
	}
	sharding.clusters = newClusters
	sharding.updateDistribution()
}

func (sharding *clusterSharding) Add(c *v1alpha1.Cluster) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	old, ok := sharding.clusters[c.Server]
	sharding.clusters[c.Server] = c
	if !ok || hasShardingUpdates(old, c) {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. Cluster already added")
	}
}

func (sharding *clusterSharding) Delete(clusterServer string) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	if _, ok := sharding.clusters[clusterServer]; ok {
		delete(sharding.clusters, clusterServer)
		delete(sharding.shards, clusterServer)
		sharding.updateDistribution()
	}
}

func (sharding *clusterSharding) Update(c *v1alpha1.Cluster) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	old, ok := sharding.clusters[c.Server]
	sharding.clusters[c.Server] = c
	if !ok || hasShardingUpdates(old, c) {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. No relevant changes")
	}
}

func (sharding *clusterSharding) GetDistribution() map[string]int {
	sharding.lock.RLock()
	shards := sharding.shards
	sharding.lock.RUnlock()

	distribution := make(map[string]int, len(shards))
	for k, v := range shards {
		distribution[k] = v
	}
	return distribution
}

func (sharding *clusterSharding) updateDistribution() {
	log.Info("Updating cluster shards")

	for _, c := range sharding.clusters {
		shard := 0
		if c.Shard != nil {
			requestedShard := int(*c.Shard)
			if requestedShard < sharding.replicas {
				shard = requestedShard
			} else {
				log.Warnf("Specified cluster shard (%d) for cluster: %s is greater than the number of available shard (%d). Using shard 0.", requestedShard, c.Server, sharding.replicas)
			}
		} else {
			shard = sharding.getClusterShard(c)
		}
		var shard64 int64 = int64(shard)
		c.Shard = &shard64
		sharding.shards[c.Server] = shard
	}
}

// hasShardingUpdates returns true if the sharding distribution has been updated.
// nil checking is done for the corner case of the in-cluster cluster which may
// have a nil shard assigned
func hasShardingUpdates(old, new *v1alpha1.Cluster) bool {
	if old == nil || new == nil || (old.Shard == nil && new.Shard == nil) {
		return false
	}
	return old.Shard != new.Shard
}

func (d *clusterSharding) getClusterAccessor() clusterAccessor {
	return func() []*v1alpha1.Cluster {
		clusters := make([]*v1alpha1.Cluster, 0, len(d.clusters))
		for _, c := range d.clusters {
			clusters = append(clusters, c)
		}
		return clusters
	}
}
