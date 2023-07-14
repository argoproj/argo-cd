package sharding

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	log "github.com/sirupsen/logrus"
)

// Make it overridable for testing
var osHostnameFunction = os.Hostname

type DistributionFunction func(c *v1alpha1.Cluster) int
type ClusterFilterFunction func(c *v1alpha1.Cluster) bool

// GetClusterFilter returns a ClusterFilterFunction which is a function taking a cluster as a parameter
// and returns wheter or not the cluster should be processed by a given shard. It calls the distributionFunction
// to determine which shard will process the cluster, and if the given shard is equal to the calculated shard
// the function will return true.
func GetClusterFilter(distributionFunction DistributionFunction, shard int) ClusterFilterFunction {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	return func(c *v1alpha1.Cluster) bool {
		clusterShard := 0
		if c != nil && c.Shard != nil {
			requestedShard := int(*c.Shard)
			if requestedShard < replicas {
				clusterShard = requestedShard
			} else {
				log.Warnf("Specified cluster shard (%d) for cluster: %s is greater than the number of available shard. Assigning automatically.", requestedShard, c.Name)
			}
		} else {
			clusterShard = distributionFunction(c)
		}
		return clusterShard == shard
	}
}

// GetDistributionFunction returns which DistributionFunction should be used based on the passed algorithm and
// the current datas.
func GetDistributionFunction(db db.ArgoDB, shardingAlgorithm string) DistributionFunction {
	log.Infof("Using filter function:  %s", shardingAlgorithm)
	distributionFunction := LegacyDistributionFunction()
	switch shardingAlgorithm {
	case common.RoundRobinShardingAlgorithm:
		distributionFunction = RoundRobinDistributionFunction(db)
	case common.LegacyShardingAlgorithm:
		distributionFunction = LegacyDistributionFunction()
	default:
		log.Warnf("distribution type %s is not supported, defaulting to %s", shardingAlgorithm, common.DefaultShardingAlgorithm)
	}
	return distributionFunction
}

// LegacyDistributionFunction returns a DistributionFunction using a stable distribution algorithm:
// for a given cluster the function will return the shard number based on the cluster id. This function
// is lightweight and can be distributed easily, however, it does not ensure an homogenous distribution as
// some shards may get assigned more clusters than others. It is the legacy function distribution that is
// kept for compatibility reasons
func LegacyDistributionFunction() DistributionFunction {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	return func(c *v1alpha1.Cluster) int {
		if replicas == 0 {
			return -1
		}
		if c == nil {
			return 0
		}
		id := c.ID
		log.Debugf("Calculating cluster shard for cluster id: %s", id)
		if id == "" {
			return 0
		} else {
			h := fnv.New32a()
			_, _ = h.Write([]byte(id))
			shard := int32(h.Sum32() % uint32(replicas))
			log.Debugf("Cluster with id=%s will be processed by shard %d", id, shard)
			return int(shard)
		}
	}
}

// RoundRobinDistributionFunction returns a DistributionFunction using an homogeneous distribution algorithm:
// for a given cluster the function will return the shard number based on the modulo of the cluster rank in
// the cluster's list sorted by uid on the shard number.
// This function ensures an homogenous distribution: each shards got assigned the same number of
// clusters +/-1 , but with the drawback of a reshuffling of clusters accross shards in case of some changes
// in the cluster list
func RoundRobinDistributionFunction(db db.ArgoDB) DistributionFunction {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	return func(c *v1alpha1.Cluster) int {
		if replicas > 0 {
			if c == nil { // in-cluster does not necessarly have a secret assigned. So we are receiving a nil cluster here.
				return 0
			} else {
				clusterIndexdByClusterIdMap := createClusterIndexByClusterIdMap(db)
				clusterIndex, ok := clusterIndexdByClusterIdMap[c.ID]
				if !ok {
					log.Warnf("Cluster with id=%s not found in cluster map.", c.ID)
					return -1
				}
				shard := int(clusterIndex % replicas)
				log.Debugf("Cluster with id=%s will be processed by shard %d", c.ID, shard)
				return shard
			}
		}
		log.Warnf("The number of replicas (%d) is lower than 1", replicas)
		return -1
	}
}

// InferShard extracts the shard index based on its hostname.
func InferShard() (int, error) {
	hostname, err := osHostnameFunction()
	if err != nil {
		return 0, err
	}
	parts := strings.Split(hostname, "-")
	if len(parts) == 0 {
		return 0, fmt.Errorf("hostname should ends with shard number separated by '-' but got: %s", hostname)
	}
	shard, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("hostname should ends with shard number separated by '-' but got: %s", hostname)
	}
	return int(shard), nil
}

func getSortedClustersList(db db.ArgoDB) []v1alpha1.Cluster {
	ctx := context.Background()
	clustersList, dbErr := db.ListClusters(ctx)
	if dbErr != nil {
		log.Warnf("Error while querying clusters list from database: %v", dbErr)
		return []v1alpha1.Cluster{}
	}
	clusters := clustersList.Items
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})
	return clusters
}

func createClusterIndexByClusterIdMap(db db.ArgoDB) map[string]int {
	clusters := getSortedClustersList(db)
	log.Debugf("ClustersList has %d items", len(clusters))
	clusterById := make(map[string]v1alpha1.Cluster)
	clusterIndexedByClusterId := make(map[string]int)
	for i, cluster := range clusters {
		log.Debugf("Adding cluster with id=%s and name=%s to cluster's map", cluster.ID, cluster.Name)
		clusterById[cluster.ID] = cluster
		clusterIndexedByClusterId[cluster.ID] = i
	}
	return clusterIndexedByClusterId
}
