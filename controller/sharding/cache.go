package sharding

import (
	"sync"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"
	log "github.com/sirupsen/logrus"
)

type ClusterShardingCache interface {
	Init(clusters *v1alpha1.ClusterList, apps *v1alpha1.ApplicationList)
	Add(c *v1alpha1.Cluster)
	Delete(clusterServer string)
	Update(c *v1alpha1.Cluster)
	AddApp(a *v1alpha1.Application)
	DeleteApp(a *v1alpha1.Application)
	UpdateApp(a *v1alpha1.Application)
	IsManagedCluster(c *v1alpha1.Cluster) bool
	GetDistribution() map[string]int
	GetAppDistribution() map[string]int
}

type ClusterSharding struct {
	Shard           int
	Replicas        int
	Shards          map[string]int
	Clusters        map[string]*v1alpha1.Cluster
	Apps            map[string]*v1alpha1.Application
	lock            sync.RWMutex
	getClusterShard DistributionFunction
}

func NewClusterSharding(db db.ArgoDB, shard, replicas int, shardingAlgorithm string) ClusterShardingCache {
	log.Debugf("Processing clusters from shard %d: Using filter function:  %s", shard, shardingAlgorithm)
	clusterSharding := &ClusterSharding{
		Shard:    shard,
		Replicas: replicas,
		Shards:   make(map[string]int),
		Clusters: make(map[string]*v1alpha1.Cluster),
		Apps:     make(map[string]*v1alpha1.Application),
	}
	distributionFunction := NoShardingDistributionFunction()
	if replicas > 1 {
		log.Debugf("Processing clusters from shard %d: Using filter function:  %s", shard, shardingAlgorithm)
		distributionFunction = GetDistributionFunction(clusterSharding.GetClusterAccessor(), clusterSharding.GetAppAccessor(), shardingAlgorithm, replicas)
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

func (sharding *ClusterSharding) Init(clusters *v1alpha1.ClusterList, apps *v1alpha1.ApplicationList) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	newClusters := make(map[string]*v1alpha1.Cluster, len(clusters.Items))
	for _, c := range clusters.Items {
		newClusters[c.Server] = &c
	}
	sharding.Clusters = newClusters
	newApps := make(map[string]*v1alpha1.Application, len(apps.Items))
	for i := range apps.Items {
		app := apps.Items[i]
		newApps[app.Name] = &app
	}
	sharding.Apps = newApps
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

func (sharding *ClusterSharding) Update(c *v1alpha1.Cluster) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	old, ok := sharding.Clusters[c.Server]
	sharding.Clusters[c.Server] = c
	if !ok || hasShardingUpdates(old, c) {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. No relevant changes")
	}
}

func (sharding *ClusterSharding) GetDistribution() map[string]int {
	sharding.lock.RLock()
	shards := sharding.Shards
	sharding.lock.RUnlock()

	distribution := make(map[string]int, len(shards))
	for k, v := range shards {
		distribution[k] = v
	}
	return distribution
}

func (sharding *ClusterSharding) updateDistribution() {
	log.Info("Updating cluster shards")

	for _, c := range sharding.Clusters {
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
		var shard64 int64 = int64(shard)
		c.Shard = &shard64
		sharding.Shards[c.Server] = shard
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

func (d *ClusterSharding) GetClusterAccessor() clusterAccessor {
	return func() []*v1alpha1.Cluster {
		clusters := make([]*v1alpha1.Cluster, 0, len(d.Clusters))
		for _, c := range d.Clusters {
			clusters = append(clusters, c)
		}
		return clusters
	}
}

func (d *ClusterSharding) GetAppAccessor() appAccessor {
	return func() []*v1alpha1.Application {
		apps := make([]*v1alpha1.Application, 0, len(d.Apps))
		for _, a := range d.Apps {
			apps = append(apps, a)
		}
		return apps
	}
}

func (sharding *ClusterSharding) AddApp(a *v1alpha1.Application) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	_, ok := sharding.Apps[a.Name]
	sharding.Apps[a.Name] = a
	if !ok {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. App already added")
	}
}

func (sharding *ClusterSharding) DeleteApp(a *v1alpha1.Application) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()
	if _, ok := sharding.Apps[a.Name]; ok {
		delete(sharding.Apps, a.Name)
		sharding.updateDistribution()
	}
}

func (sharding *ClusterSharding) UpdateApp(a *v1alpha1.Application) {
	sharding.lock.Lock()
	defer sharding.lock.Unlock()

	_, ok := sharding.Apps[a.Name]
	sharding.Apps[a.Name] = a
	if !ok {
		sharding.updateDistribution()
	} else {
		log.Debugf("Skipping sharding distribution update. No relevant changes")
	}
}

func (sharding *ClusterSharding) GetAppDistribution() map[string]int {
	appDistribution := make(map[string]int, len(sharding.Clusters))

	for _, a := range sharding.Apps {
		if _, ok := appDistribution[a.Spec.Destination.Server]; !ok {
			appDistribution[a.Spec.Destination.Server] = 0
		}
		appDistribution[a.Spec.Destination.Server]++
	}
	return appDistribution
}
