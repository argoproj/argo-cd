package sharding

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"encoding/json"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/settings"
	log "github.com/sirupsen/logrus"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
)

// Make it overridable for testing
var osHostnameFunction = os.Hostname

// Make it overridable for testing
var heartbeatCurrentTime = metav1.Now

var (
	HeartbeatDuration = env.ParseNumFromEnv(common.EnvControllerHeartbeatTime, 10, 10, 60)
	HeartbeatTimeout  = 3 * HeartbeatDuration
)

const ShardControllerMappingKey = "shardControllerMapping"

type DistributionFunction func(c *v1alpha1.Cluster) int
type ClusterFilterFunction func(c *v1alpha1.Cluster) bool
type clusterAccessor func() []*v1alpha1.Cluster

// shardApplicationControllerMapping stores the mapping of Shard Number to Application Controller in ConfigMap.
// It also stores the heartbeat of last synced time of the application controller.
type shardApplicationControllerMapping struct {
	ShardNumber    int
	ControllerName string
	HeartbeatTime  metav1.Time
}

// GetClusterFilter returns a ClusterFilterFunction which is a function taking a cluster as a parameter
// and returns wheter or not the cluster should be processed by a given shard. It calls the distributionFunction
// to determine which shard will process the cluster, and if the given shard is equal to the calculated shard
// the function will return true.
func GetClusterFilter(db db.ArgoDB, distributionFunction DistributionFunction, shard int) ClusterFilterFunction {
	replicas := db.GetApplicationControllerReplicas()
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
func GetDistributionFunction(db db.ArgoDB, clusters clusterAccessor, shardingAlgorithm string) DistributionFunction {
	log.Infof("Using filter function:  %s", shardingAlgorithm)
	distributionFunction := LegacyDistributionFunction(db)
	switch shardingAlgorithm {
	case common.NoShardingAlgorithm:
		distributionFunction = NoShardingDistributionFunction(0)
	case common.RoundRobinShardingAlgorithm:
		distributionFunction = RoundRobinDistributionFunction(db, clusters)
	case common.LegacyShardingAlgorithm:
		distributionFunction = LegacyDistributionFunction(db)
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
func LegacyDistributionFunction(db db.ArgoDB) DistributionFunction {
	replicas := db.GetApplicationControllerReplicas()
	return func(c *v1alpha1.Cluster) int {
		if replicas == 0 {
			log.Debugf("Replicas count is : %d, returning -1", replicas)
			return -1
		}
		if c == nil {
			log.Debug("In-cluster: returning 0")
			return 0
		}
		// if Shard is manually set and the assigned value is lower than the number of replicas,
		// then its value is returned otherwise it is the default calculated value
		if c.Shard != nil && int(*c.Shard) < replicas {
			return int(*c.Shard)
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

func RoundRobinDistributionFunction(db db.ArgoDB, clusters clusterAccessor) DistributionFunction {
	replicas := db.GetApplicationControllerReplicas()
	return func(c *v1alpha1.Cluster) int {
		if replicas > 0 {
			if c == nil { // in-cluster does not necessarly have a secret assigned. So we are receiving a nil cluster here.
				return 0
			}
			// if Shard is manually set and the assigned value is lower than the number of replicas,
			// then its value is returned otherwise it is the default calculated value
			if c.Shard != nil && int(*c.Shard) < replicas {
				return int(*c.Shard)
			} else {
				clusterIndexdByClusterIdMap := createClusterIndexByClusterIdMap(clusters)
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

// NoShardingDistributionFunction returns a DistributionFunction that will process all shards
func NoShardingDistributionFunction(shard int) DistributionFunction {
	return func(c *v1alpha1.Cluster) int { return shard }
}

// InferShard extracts the shard index based on its hostname.
func InferShard() (int, error) {
	hostname, err := osHostnameFunction()
	if err != nil {
		return -1, err
	}
	parts := strings.Split(hostname, "-")
	if len(parts) == 0 {
		log.Warnf("hostname should end with shard number separated by '-' but got: %s", hostname)
		return 0, nil
	}
	shard, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		log.Warnf("hostname should end with shard number separated by '-' but got: %s", hostname)
		return 0, nil
	}
	return int(shard), nil
}

func getSortedClustersList(getCluster clusterAccessor) []*v1alpha1.Cluster {
	clusters := getCluster()
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].ID < clusters[j].ID
	})
	return clusters
}

func createClusterIndexByClusterIdMap(getCluster clusterAccessor) map[string]int {
	clusters := getSortedClustersList(getCluster)
	log.Debugf("ClustersList has %d items", len(clusters))
	clusterById := make(map[string]*v1alpha1.Cluster)
	clusterIndexedByClusterId := make(map[string]int)
	for i, cluster := range clusters {
		log.Debugf("Adding cluster with id=%s and name=%s to cluster's map", cluster.ID, cluster.Name)
		clusterById[cluster.ID] = cluster
		clusterIndexedByClusterId[cluster.ID] = i
	}
	return clusterIndexedByClusterId
}

// GetOrUpdateShardFromConfigMap finds the shard number from the shard mapping configmap. If the shard mapping configmap does not exist,
// the function creates the shard mapping configmap.
// The function takes the shard number from the environment variable (default value -1, if not set) and passes it to this function.
// If the shard value passed to this function is -1, that is, the shard was not set as an environment variable,
// we default the shard number to 0 for computing the default config map.
func GetOrUpdateShardFromConfigMap(kubeClient *kubernetes.Clientset, settingsMgr *settings.SettingsManager, replicas, shard int) (int, error) {
	hostname, err := osHostnameFunction()
	if err != nil {
		return -1, err
	}

	// fetch the shard mapping configMap
	shardMappingCM, err := kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Get(context.Background(), common.ArgoCDAppControllerShardConfigMapName, metav1.GetOptions{})

	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return -1, fmt.Errorf("error getting sharding config map: %s", err)
		}
		log.Infof("shard mapping configmap %s not found. Creating default shard mapping configmap.", common.ArgoCDAppControllerShardConfigMapName)

		// if the shard is not set as an environment variable, set the default value of shard to 0 for generating default CM
		if shard == -1 {
			shard = 0
		}
		shardMappingCM, err = generateDefaultShardMappingCM(settingsMgr.GetNamespace(), hostname, replicas, shard)
		if err != nil {
			return -1, fmt.Errorf("error generating default shard mapping configmap %s", err)
		}
		if _, err = kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Create(context.Background(), shardMappingCM, metav1.CreateOptions{}); err != nil {
			return -1, fmt.Errorf("error creating shard mapping configmap %s", err)
		}
		// return 0 as the controller is assigned to shard 0 while generating default shard mapping ConfigMap
		return shard, nil
	} else {
		// Identify the available shard and update the ConfigMap
		data := shardMappingCM.Data[ShardControllerMappingKey]
		var shardMappingData []shardApplicationControllerMapping
		err := json.Unmarshal([]byte(data), &shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error unmarshalling shard config map data: %s", err)
		}

		shard, shardMappingData := getOrUpdateShardNumberForController(shardMappingData, hostname, replicas, shard)
		updatedShardMappingData, err := json.Marshal(shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error marshalling data of shard mapping ConfigMap: %s", err)
		}
		shardMappingCM.Data[ShardControllerMappingKey] = string(updatedShardMappingData)

		_, err = kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Update(context.Background(), shardMappingCM, metav1.UpdateOptions{})
		if err != nil {
			return -1, err
		}
		return shard, nil
	}
}

// getOrUpdateShardNumberForController takes list of shardApplicationControllerMapping and performs computation to find the matching or empty shard number
func getOrUpdateShardNumberForController(shardMappingData []shardApplicationControllerMapping, hostname string, replicas, shard int) (int, []shardApplicationControllerMapping) {

	// if current length of shardMappingData in shard mapping configMap is less than the number of replicas,
	// create additional empty entries for missing shard numbers in shardMappingDataconfigMap
	if len(shardMappingData) < replicas {
		// generate extra default mappings
		for currentShard := len(shardMappingData); currentShard < replicas; currentShard++ {
			shardMappingData = append(shardMappingData, shardApplicationControllerMapping{
				ShardNumber: currentShard,
			})
		}
	}

	// if current length of shardMappingData in shard mapping configMap is more than the number of replicas,
	// we replace the config map with default config map and let controllers self assign the new shard to itself
	if len(shardMappingData) > replicas {
		shardMappingData = getDefaultShardMappingData(replicas)
	}

	if shard != -1 && shard < replicas {
		log.Debugf("update heartbeat for shard %d", shard)
		for i := range shardMappingData {
			shardMapping := shardMappingData[i]
			if shardMapping.ShardNumber == shard {
				log.Debugf("Shard found. Updating heartbeat!!")
				shardMapping.ControllerName = hostname
				shardMapping.HeartbeatTime = heartbeatCurrentTime()
				shardMappingData[i] = shardMapping
				break
			}
		}
	} else {
		// find the matching shard with assigned controllerName
		for i := range shardMappingData {
			shardMapping := shardMappingData[i]
			if shardMapping.ControllerName == hostname {
				log.Debugf("Shard matched. Updating heartbeat!!")
				shard = int(shardMapping.ShardNumber)
				shardMapping.HeartbeatTime = heartbeatCurrentTime()
				shardMappingData[i] = shardMapping
				break
			}
		}
	}

	// at this point, we have still not found a shard with matching hostname.
	// So, find a shard with either no controller assigned or assigned controller
	// with heartbeat past threshold
	if shard == -1 {
		for i := range shardMappingData {
			shardMapping := shardMappingData[i]
			if (shardMapping.ControllerName == "") || (metav1.Now().After(shardMapping.HeartbeatTime.Add(time.Duration(HeartbeatTimeout) * time.Second))) {
				shard = int(shardMapping.ShardNumber)
				log.Debugf("Empty shard found %d", shard)
				shardMapping.ControllerName = hostname
				shardMapping.HeartbeatTime = heartbeatCurrentTime()
				shardMappingData[i] = shardMapping
				break
			}
		}
	}
	return shard, shardMappingData
}

// generateDefaultShardMappingCM creates a default shard mapping configMap. Assigns current controller to shard 0.
func generateDefaultShardMappingCM(namespace, hostname string, replicas, shard int) (*v1.ConfigMap, error) {

	shardingCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDAppControllerShardConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}

	shardMappingData := getDefaultShardMappingData(replicas)

	// if shard is not assigned to a controller, we use shard 0
	if shard == -1 || shard > replicas {
		shard = 0
	}
	shardMappingData[shard].ControllerName = hostname
	shardMappingData[shard].HeartbeatTime = heartbeatCurrentTime()

	data, err := json.Marshal(shardMappingData)
	if err != nil {
		return nil, fmt.Errorf("error generating default ConfigMap: %s", err)
	}
	shardingCM.Data[ShardControllerMappingKey] = string(data)

	return shardingCM, nil
}

func getDefaultShardMappingData(replicas int) []shardApplicationControllerMapping {
	shardMappingData := make([]shardApplicationControllerMapping, 0)

	for i := 0; i < replicas; i++ {
		mapping := shardApplicationControllerMapping{
			ShardNumber: i,
		}
		shardMappingData = append(shardMappingData, mapping)
	}
	return shardMappingData
}
