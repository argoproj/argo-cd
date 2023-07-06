package sharding

import (
	"sync"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	log "github.com/sirupsen/logrus"
)

type ClusterSharding interface {
	// Init() error
	Add(c *v1alpha1.Cluster)
	Delete(clusterServer string)
	Update(c *v1alpha1.Cluster)
	IsManagedCluster(c *v1alpha1.Cluster) bool
}

type clusterSharding struct {
	shard           int
	replicas        int
	shards          map[string]int
	clusters        map[string]*v1alpha1.Cluster
	lock            sync.RWMutex
	getClusterShard DistributionFunction
}

type noClusterSharding struct{}

func (d *noClusterSharding) Add(c *v1alpha1.Cluster)     {}
func (d *noClusterSharding) Delete(clusterServer string) {}
func (d *noClusterSharding) Update(c *v1alpha1.Cluster)  {}
func (d *noClusterSharding) IsManagedCluster(c *v1alpha1.Cluster) bool {
	return true
}

func NewClusterSharding(db db.ArgoDB, shard, replicas int, shardingAlgorithm string) ClusterSharding {
	if replicas <= 1 {
		log.Info("Processing all cluster shards")
		return &noClusterSharding{}
	}

	log.Infof("Processing clusters from shard %d", shard)
	log.Infof("Using filter function:  %s", shardingAlgorithm)
	clusterSharding := &clusterSharding{
		shard:    shard,
		shards:   make(map[string]int),
		clusters: make(map[string]*v1alpha1.Cluster),
	}
	distributionFunc := GetDistributionFunction(db, clusterSharding.getClusterAccessor(), shardingAlgorithm)
	clusterSharding.getClusterShard = distributionFunc
	return clusterSharding
}

// IsManagedCluster returns wheter or not the cluster should be processed by a given shard.
func (d *clusterSharding) IsManagedCluster(c *v1alpha1.Cluster) bool {
	d.lock.RLock()
	defer d.lock.RUnlock()

	clusterShard := 0
	if shard, ok := d.shards[c.Server]; ok {
		clusterShard = shard
	} else {
		log.Warnf("The cluster %s has no assigned shard.", c.Server)
	}
	return clusterShard == d.shard
}

func (d *clusterSharding) Add(c *v1alpha1.Cluster) {
	d.lock.Lock()
	defer d.lock.Unlock()

	d.clusters[c.Server] = c
	d.updateDistribution()
}

func (d *clusterSharding) Delete(clusterServer string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if _, ok := d.clusters[clusterServer]; ok {
		delete(d.clusters, clusterServer)
		delete(d.shards, clusterServer)
		d.updateDistribution()
	}
}

func (d *clusterSharding) Update(c *v1alpha1.Cluster) {
	d.lock.Lock()
	defer d.lock.Unlock()

	old, ok := d.clusters[c.Server]
	d.clusters[c.Server] = c
	if !ok || hasShardingUpdates(old, c) {
		d.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. No relevant changes")
	}
}

func (d *clusterSharding) updateDistribution() {
	log.Info("Updating cluster shards")

	for _, c := range d.clusters {
		shard := 0
		if c.Shard != nil {
			requestedShard := int(*c.Shard)
			if requestedShard < d.replicas {
				shard = requestedShard
			} else {
				log.Warnf("Specified cluster shard (%d) for cluster: %s is greater than the number of available shard. Using shard 0.", requestedShard, c.Server)
			}
		} else {
			shard = d.getClusterShard(c)
		}
		d.shards[c.Server] = shard
	}
}

func hasShardingUpdates(old, new *v1alpha1.Cluster) bool {
	return *old.Shard != *new.Shard
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
