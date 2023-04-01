package sharding

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	log "github.com/sirupsen/logrus"
)

func InferShard() (int, error) {
	hostname, err := os.Hostname()
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

func GetClusterFilter(distributionFunction DistributionFunction, shard int) ClusterFilterFunction {
	return func(c *v1alpha1.Cluster) bool {
		return distributionFunction(c) == shard
	}
}

func GetDistributionFunction(db db.ArgoDB) DistributionFunction {
	filterFunctionName := env.StringFromEnv(common.EnvControllerShardingAlgorithm, "legacy")
	log.Infof("Using filter function:  %s", filterFunctionName)
	distributionFunction := GetShardByIdUsingHashDistributionFunction()
	switch {
	case filterFunctionName == "hash":
		distributionFunction = GetShardByIndexModuloReplicasCountDistributionFunction(db)
	case filterFunctionName == "legacy":
		distributionFunction = GetShardByIdUsingHashDistributionFunction()
	default:
		distributionFunctionName := runtime.FuncForPC(reflect.ValueOf(distributionFunction).Pointer())
		log.Warnf("distribution type %s is not supported, defaulting to %s", filterFunctionName, distributionFunctionName)
	}
	return distributionFunction
}

func GetShardByIndexModuloReplicasCountDistributionFunction(db db.ArgoDB) DistributionFunction {
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
				log.Infof("Cluster with id=%s will be processed by shard %d", c.ID, shard)
				return shard
			}
		}
		log.Warnf("The number of replicas (%d) is lower than 1", replicas)
		return -1
	}
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

func GetShardByIdUsingHashDistributionFunction() DistributionFunction {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	return func(c *v1alpha1.Cluster) int {
		if replicas == 0 {
			return -1
		}
		if c == nil {
			log.Infof("Cannot calculate cluster shard when cluster not specified")
			return -1
		}
		id := c.ID
		log.Debugf("Calculating cluster shard for cluster id: %s", id)
		if id == "" {
			return 0
		} else {
			h := fnv.New32a()
			_, _ = h.Write([]byte(id))
			shard := int32(h.Sum32() % uint32(replicas))
			log.Infof("Cluster with id=%s will be processed by shard %d", c.ID, shard)
			return int(shard)
		}
	}
}

type DistributionFunction func(c *v1alpha1.Cluster) int
type ClusterFilterFunction func(c *v1alpha1.Cluster) bool
