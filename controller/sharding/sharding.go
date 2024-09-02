package sharding

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	slices "golang.org/x/exp/slices"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/sharding/consistent"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/settings"
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

type (
	DistributionFunction  func(c *v1alpha1.Cluster) int
	ClusterFilterFunction func(c *v1alpha1.Cluster) bool
	clusterAccessor       func() []*v1alpha1.Cluster
	appAccessor           func() []*v1alpha1.Application
)

// shardApplicationControllerMapping stores the mapping of Shard Number to Application Controller in ConfigMap.
// It also stores the heartbeat of last synced time of the application controller.
type shardApplicationControllerMapping struct {
	ShardNumber    int
	ControllerName string
	HeartbeatTime  metav1.Time
}

// GetClusterFilter returns a ClusterFilterFunction which is a function taking a cluster as a parameter
// and returns whether or not the cluster should be processed by a given shard. It calls the distributionFunction
// to determine which shard will process the cluster, and if the given shard is equal to the calculated shard
// the function will return true.
func GetClusterFilter(db db.ArgoDB, distributionFunction DistributionFunction, replicas, shard int) ClusterFilterFunction {
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
func GetDistributionFunction(clusters clusterAccessor, apps appAccessor, shardingAlgorithm string, replicasCount int) DistributionFunction {
	log.Debugf("Using filter function:  %s", shardingAlgorithm)
	distributionFunction := LegacyDistributionFunction(replicasCount)
	switch shardingAlgorithm {
	case common.RoundRobinShardingAlgorithm:
		distributionFunction = RoundRobinDistributionFunction(clusters, replicasCount)
	case common.LegacyShardingAlgorithm:
		distributionFunction = LegacyDistributionFunction(replicasCount)
	case common.ConsistentHashingWithBoundedLoadsAlgorithm:
		distributionFunction = ConsistentHashingWithBoundedLoadsDistributionFunction(clusters, apps, replicasCount)
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
func LegacyDistributionFunction(replicas int) DistributionFunction {
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
// clusters +/-1 , but with the drawback of a reshuffling of clusters across shards in case of some changes
// in the cluster list

func RoundRobinDistributionFunction(clusters clusterAccessor, replicas int) DistributionFunction {
	return func(c *v1alpha1.Cluster) int {
		if replicas > 0 {
			if c == nil { // in-cluster does not necessarily have a secret assigned. So we are receiving a nil cluster here.
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

// ConsistentHashingWithBoundedLoadsDistributionFunction returns a DistributionFunction using an almost homogeneous distribution algorithm:
// for a given cluster the function will return the shard number based on a consistent hashing with bounded loads algorithm.
// This function ensures an almost homogenous distribution: each shards got assigned the fairly similar number of
// clusters +/-10% , but with it is resilient to sharding and/or number of clusters changes.
func ConsistentHashingWithBoundedLoadsDistributionFunction(clusters clusterAccessor, apps appAccessor, replicas int) DistributionFunction {
	return func(c *v1alpha1.Cluster) int {
		if replicas > 0 {
			if c == nil { // in-cluster does not necessarily have a secret assigned. So we are receiving a nil cluster here.
				return 0
			}

			// if Shard is manually set and the assigned value is lower than the number of replicas,
			// then its value is returned otherwise it is the default calculated value
			if c.Shard != nil && int(*c.Shard) < replicas {
				return int(*c.Shard)
			} else {
				// if the cluster is not in the clusters list anymore, we should unassign it from any shard, so we
				// return the reserved value of -1
				if !slices.Contains(clusters(), c) {
					log.Warnf("Cluster with id=%s not found in cluster map.", c.ID)
					return -1
				}
				shardIndexedByCluster := createConsistentHashingWithBoundLoads(replicas, clusters, apps)
				shard, ok := shardIndexedByCluster[c.ID]
				if !ok {
					log.Warnf("Cluster with id=%s not found in cluster map.", c.ID)
					return -1
				}
				log.Debugf("Cluster with id=%s will be processed by shard %d", c.ID, shard)
				return shard
			}
		}
		log.Warnf("The number of replicas (%d) is lower than 1", replicas)
		return -1
	}
}

func createConsistentHashingWithBoundLoads(replicas int, getCluster clusterAccessor, getApp appAccessor) map[string]int {
	clusters := getSortedClustersList(getCluster)
	appDistribution := getAppDistribution(getCluster, getApp)
	shardIndexedByCluster := make(map[string]int)
	appsIndexedByShard := make(map[string]int64)
	consistentHashing := consistent.New()
	// Adding a shard with id "-1" as a reserved value for clusters that does not have an assigned shard
	// this happens for clusters that are removed for the clusters list
	// consistentHashing.Add("-1")
	for i := 0; i < replicas; i++ {
		shard := strconv.Itoa(i)
		consistentHashing.Add(shard)
		appsIndexedByShard[shard] = 0
	}

	for _, c := range clusters {
		clusterIndex, err := consistentHashing.GetLeast(c.ID)
		if err != nil {
			log.Warnf("Cluster with id=%s not found in cluster map.", c.ID)
		}
		shardIndexedByCluster[c.ID], err = strconv.Atoi(clusterIndex)
		if err != nil {
			log.Errorf("Consistent Hashing was supposed to return a shard index but it returned %d", err)
		}
		numApps, ok := appDistribution[c.Server]
		if !ok {
			numApps = 0
		}
		appsIndexedByShard[clusterIndex] += numApps
		consistentHashing.UpdateLoad(clusterIndex, appsIndexedByShard[clusterIndex])
	}

	return shardIndexedByCluster
}

func getAppDistribution(getCluster clusterAccessor, getApps appAccessor) map[string]int64 {
	apps := getApps()
	clusters := getCluster()
	appDistribution := make(map[string]int64, len(clusters))

	for _, a := range apps {
		if _, ok := appDistribution[a.Spec.Destination.Server]; !ok {
			appDistribution[a.Spec.Destination.Server] = 0
		}
		appDistribution[a.Spec.Destination.Server]++
	}
	return appDistribution
}

// NoShardingDistributionFunction returns a DistributionFunction that will process all cluster by shard 0
// the function is created for API compatibility purposes and is not supposed to be activated.
func NoShardingDistributionFunction() DistributionFunction {
	return func(c *v1alpha1.Cluster) int { return 0 }
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
func GetOrUpdateShardFromConfigMap(kubeClient kubernetes.Interface, settingsMgr *settings.SettingsManager, replicas, shard int) (int, error) {
	hostname, err := osHostnameFunction()
	if err != nil {
		return -1, err
	}

	// fetch the shard mapping configMap
	shardMappingCM, err := kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Get(context.Background(), common.ArgoCDAppControllerShardConfigMapName, metav1.GetOptions{})

	if err != nil {
		if !kubeerrors.IsNotFound(err) {
			return -1, fmt.Errorf("error getting sharding config map: %w", err)
		}
		log.Infof("shard mapping configmap %s not found. Creating default shard mapping configmap.", common.ArgoCDAppControllerShardConfigMapName)

		// if the shard is not set as an environment variable, set the default value of shard to 0 for generating default CM
		if shard == -1 {
			shard = 0
		}
		shardMappingCM, err = generateDefaultShardMappingCM(settingsMgr.GetNamespace(), hostname, replicas, shard)
		if err != nil {
			return -1, fmt.Errorf("error generating default shard mapping configmap %w", err)
		}
		if _, err = kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Create(context.Background(), shardMappingCM, metav1.CreateOptions{}); err != nil {
			return -1, fmt.Errorf("error creating shard mapping configmap %w", err)
		}
		// return 0 as the controller is assigned to shard 0 while generating default shard mapping ConfigMap
		return shard, nil
	} else {
		// Identify the available shard and update the ConfigMap
		data := shardMappingCM.Data[ShardControllerMappingKey]
		var shardMappingData []shardApplicationControllerMapping
		err := json.Unmarshal([]byte(data), &shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error unmarshalling shard config map data: %w", err)
		}

		shard, shardMappingData := getOrUpdateShardNumberForController(shardMappingData, hostname, replicas, shard)
		updatedShardMappingData, err := json.Marshal(shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error marshalling data of shard mapping ConfigMap: %w", err)
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
		return nil, fmt.Errorf("error generating default ConfigMap: %w", err)
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

func GetClusterSharding(kubeClient kubernetes.Interface, settingsMgr *settings.SettingsManager, shardingAlgorithm string, enableDynamicClusterDistribution bool) (ClusterShardingCache, error) {
	var replicasCount int
	if enableDynamicClusterDistribution {
		applicationControllerName := env.StringFromEnv(common.EnvAppControllerName, common.DefaultApplicationControllerName)
		appControllerDeployment, err := kubeClient.AppsV1().Deployments(settingsMgr.GetNamespace()).Get(context.Background(), applicationControllerName, metav1.GetOptions{})
		// if app controller deployment is not found when dynamic cluster distribution is enabled error out
		if err != nil {
			return nil, fmt.Errorf("(dynamic cluster distribution) failed to get app controller deployment: %w", err)
		}

		if appControllerDeployment != nil && appControllerDeployment.Spec.Replicas != nil {
			replicasCount = int(*appControllerDeployment.Spec.Replicas)
		} else {
			return nil, fmt.Errorf("(dynamic cluster distribution) failed to get app controller deployment replica count")
		}
	} else {
		replicasCount = env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	}
	shardNumber := env.ParseNumFromEnv(common.EnvControllerShard, -1, -math.MaxInt32, math.MaxInt32)
	if replicasCount > 1 {
		// check for shard mapping using configmap if application-controller is a deployment
		// else use existing logic to infer shard from pod name if application-controller is a statefulset
		if enableDynamicClusterDistribution {
			var err error
			// retry 3 times if we find a conflict while updating shard mapping configMap.
			// If we still see conflicts after the retries, wait for next iteration of heartbeat process.
			for i := 0; i <= common.AppControllerHeartbeatUpdateRetryCount; i++ {
				shardNumber, err = GetOrUpdateShardFromConfigMap(kubeClient, settingsMgr, replicasCount, shardNumber)
				if err != nil && !kubeerrors.IsConflict(err) {
					err = fmt.Errorf("unable to get shard due to error updating the sharding config map: %w", err)
					break
				}
				log.Warnf("conflict when getting shard from shard mapping configMap. Retrying (%d/3)", i)
			}
			errors.CheckError(err)
		} else {
			if shardNumber < 0 {
				var err error
				shardNumber, err = InferShard()
				errors.CheckError(err)
			}
			if shardNumber > replicasCount {
				log.Warnf("Calculated shard number %d is greated than the number of replicas count. Defaulting to 0", shardNumber)
				shardNumber = 0
			}
		}
	} else {
		log.Info("Processing all cluster shards")
		shardNumber = 0
	}
	db := db.NewDB(settingsMgr.GetNamespace(), settingsMgr, kubeClient)
	return NewClusterSharding(db, shardNumber, replicasCount, shardingAlgorithm), nil
}
