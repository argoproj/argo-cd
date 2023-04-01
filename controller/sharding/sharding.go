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
	"github.com/argoproj/argo-cd/v2/util/settings"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

func InferShard() (int32, error) {
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
	return int32(shard), nil
}

func GetClusterFilter(distributionFunction DistributionFunction, shard int32) func(c *v1alpha1.Cluster) bool {
	return func(c *v1alpha1.Cluster) bool {
		return distributionFunction(c) == shard
	}
}

func GetDistributionFunction(kubernetesClient *kubernetes.Clientset, settingsMgr *settings.SettingsManager) func(c *v1alpha1.Cluster) int32 {
	filterFunctionName := env.StringFromEnv(common.EnvControllerShardingAlgorithm, "legacy")
	distributionFunction := GetShardByIdUsingHashDistributionFunction()
	log.Infof("Using filter function:  %s", filterFunctionName)
	switch {
	case filterFunctionName == "hash":
		distributionFunction = GetShardByIndexModuloReplicasCountDistributionFunction(settingsMgr, kubernetesClient)
	case filterFunctionName == "legacy":
		distributionFunction = GetShardByIdUsingHashDistributionFunction()
	}
	return distributionFunction
}

func GetShardByIndexModuloReplicasCountDistributionFunction(settingsMgr *settings.SettingsManager, kubeClientset *kubernetes.Clientset) DistributionFunction {
	namespace := settingsMgr.GetNamespace()
	db := db.NewDB(namespace, settingsMgr, kubeClientset)
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	ctx := context.Background()
	clustersList, dbErr := db.ListClusters(ctx)
	if dbErr != nil {
		log.Warnf("Error while querying clusters list from database: %v", dbErr)
	}
	clusters := clustersList.Items
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})
	log.Debugf("ClustersList has %d items", len(clusters))
	clusterById := make(map[string]v1alpha1.Cluster)
	clusterIndexdByClusterId := make(map[string]int)
	for i, cluster := range clusters {
		log.Debugf("Adding cluster with id=%s and name=%s to clusterIndexdByClusterId map", cluster.ID, cluster.Name)
		clusterById[cluster.ID] = cluster
		clusterIndexdByClusterId[cluster.ID] = i
	}

	return func(c *v1alpha1.Cluster) int32 {
		if c != nil {
			clusterIndex, ok := clusterIndexdByClusterId[c.ID]
			if !ok {
				log.Infof("Cluster with id=%s not found in clusterIndexdByClusterId map", c.ID)
			}
			shard := int32(clusterIndex % replicas)
			log.Infof("Cluster with id=%s will be processed by shard %d", c.ID, shard)
			return shard
		}
		return 0
	}
}

func GetShardByIdUsingHashDistributionFunction() DistributionFunction {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	return func(c *v1alpha1.Cluster) int32 {
		if c == nil {
			log.Infof("Calculating cluster shard for cluster id: %v", c)
			return 0
		}
		id := c.ID
		log.Infof("Calculating cluster shard for cluster id: %s", id)
		if id == "" {
			return 0
		} else {
			h := fnv.New32a()
			_, _ = h.Write([]byte(id))
			shard := int32(h.Sum32() % uint32(replicas))
			log.Infof("Cluster with id=%s will be processed by shard %d", c.ID, shard)
			return shard
		}
	}
}

type DistributionFunction func(c *v1alpha1.Cluster) int32
