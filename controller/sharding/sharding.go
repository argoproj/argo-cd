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
	"time"

	"encoding/json"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/settings"
	log "github.com/sirupsen/logrus"
)

// Make it overridable for testing
var osHostnameFunction = os.Hostname

var (
	HeartbeatDuration = env.ParseNumFromEnv(common.EnvControllerHeartbeatTime, 10, 10, 60)
	HeartbeatTimeout  = 3 * HeartbeatDuration
)

const ShardControllerMappingKey = "shardControllerMapping"

type DistributionFunction func(c *v1alpha1.Cluster) int
type ClusterFilterFunction func(c *v1alpha1.Cluster) bool

// shardApplicationControllerMapping stores the mapping of Shard Number to Application Controller in ConfigMap.
// It also stores the heartbeat of last synced time of the application controller.
type shardApplicationControllerMapping struct {
	ShardNumber    int32
	ControllerName string
	HeartbeatTime  metav1.Time
}

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
func InferShard(kubeClient *kubernetes.Clientset, settingsMgr *settings.SettingsManager) (int, error) {
	hostname, err := osHostnameFunction()
	if err != nil {
		return -1, err
	}
	appControllerDeployment, _ := kubeClient.AppsV1().Deployments(settingsMgr.GetNamespace()).Get(context.Background(), common.ApplicationController, metav1.GetOptions{})

	appControllerStatefulset, _ := kubeClient.AppsV1().StatefulSets(settingsMgr.GetNamespace()).Get(context.Background(), common.ApplicationController, metav1.GetOptions{})

	// check for shard mapping using configmap if application-controller is a deployment
	// else use existing logic to infer shard from pod name if application-controller is a statefulset
	if appControllerDeployment != nil {
		retryCount := 3
		shard := -1

		// retry 3 times if we find a conflict while updating shard mapping configMap.
		// If we still see conflicts after the retries, wait for next iteration of heartbeat process.
		for i := 0; i <= retryCount; i++ {
			shard, err := getShardFromConfigMap(kubeClient, settingsMgr, appControllerDeployment, hostname)
			if !errors.IsConflict(err) {
				return shard, err
			}
			log.Warnf("conflict when getting shard from shard mapping configMap. Retrying (%d/3)", i)
		}
		return shard, err
	} else if appControllerStatefulset != nil {
		return inferShardFromStatefulSet(hostname)
	} else {
		return -1, fmt.Errorf("could not find application controller deployment or statefulset")
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

/*
 */
func getShardFromConfigMap(kubeClient *kubernetes.Clientset, settingsMgr *settings.SettingsManager, appControllerDeployment *appv1.Deployment, hostname string) (int, error) {

	// Fetch replicas from application-controller deployment
	replicas, err := getAppControllerDeploymentReplicas(appControllerDeployment)
	if err != nil {
		return -1, fmt.Errorf("error getting replicas from application controller deployment: %s", err)
	}

	// fetch the shard mapping configMap
	shardMappingCM, err := kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Get(context.Background(), common.ArgoCDAppControllerShardConfigMapName, metav1.GetOptions{})

	if err != nil {
		log.Infof("shard mapping configmap %s not found. Creating default shard mapping configmap.", common.ArgoCDAppControllerShardConfigMapName)

		shardMappingCM, err = generateDefaultShardMappingCM(settingsMgr.GetNamespace(), hostname, *replicas)
		if err != nil {
			return -1, fmt.Errorf("error generating default shard mapping configmap %s", err)
		}
		if _, err = kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Create(context.Background(), shardMappingCM, metav1.CreateOptions{}); err != nil {
			return -1, fmt.Errorf("error creating shard mapping configmap %s", err)
		}
		// return 0 as the controller is assigned to shard 0 while generating default shard mapping ConfigMap
		return 0, nil
	} else {
		// Identify the available shard and update the ConfigMap
		data := shardMappingCM.Data[ShardControllerMappingKey]
		var shardMappingData []shardApplicationControllerMapping
		err := json.Unmarshal([]byte(data), &shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error unmarshalling shard config map data: %s", err)
		}

		shard, shardMappingData := getShardNumberForController(shardMappingData, hostname, replicas)
		updatedShardMappingData, err := json.Marshal(shardMappingData)
		if err != nil {
			return -1, fmt.Errorf("error marshalling data of shard mapping ConfigMap: %s", err)
		}
		shardMappingCM.Data[ShardControllerMappingKey] = string(updatedShardMappingData)

		_, err = kubeClient.CoreV1().ConfigMaps(settingsMgr.GetNamespace()).Update(context.Background(), shardMappingCM, metav1.UpdateOptions{})
		if err != nil {
			return -1, err
		}
		return shard, err
	}
}

// getShardNumberForController takes list of shardApplicationControllerMapping and performs computation to find the matching or empty shard number
func getShardNumberForController(shardMappingData []shardApplicationControllerMapping, hostname string, replicas *int32) (int, []shardApplicationControllerMapping) {

	now := metav1.Now()
	shard := -1

	// if current length of shardMappingData inconfigMap is less than the number of replicas,
	// create additional empty entries for missing shard numbers in shardMappingDataconfigMap
	if len(shardMappingData) > (int)(*replicas) {
		// generate extra default mappings
		for currentShard := len(shardMappingData); currentShard < (int)(*replicas); currentShard++ {
			shardMappingData = append(shardMappingData, shardApplicationControllerMapping{
				ShardNumber: int32(currentShard),
			})
		}
	}

	// find the matching shard with assigned controllerName
	for i := range shardMappingData {
		shardMapping := shardMappingData[i]
		if shardMapping.ControllerName == hostname {
			log.Debugf("Shard matched. Updating heartbeat!!")
			shard = int(shardMapping.ShardNumber)
			shardMapping.HeartbeatTime = now
			break
		}
	}
	if shard == -1 {
		// find a shard with either no controller assigned or assigned controller with heartbeat past threshold
		for i := range shardMappingData {
			shardMapping := shardMappingData[i]
			if (shardMapping.ControllerName == "") || (metav1.Now().After(shardMapping.HeartbeatTime.Add(time.Duration(HeartbeatTimeout) * time.Second))) {
				shard = int(shardMapping.ShardNumber)
				log.Debugf("Empty shard found %d", shard)
				shardMapping.ControllerName = hostname
				shardMapping.HeartbeatTime = now
				shardMappingData[i] = shardMapping
				break
			}
		}
	}

	return shard, shardMappingData
}

// generateDefaultShardMappingCM creates a default shard mapping configMap. Assigns current controller to shard 0.
func generateDefaultShardMappingCM(namespace, hostname string, replicas int32) (*v1.ConfigMap, error) {

	shardingCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDAppControllerShardConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}

	shardMappingData := make([]shardApplicationControllerMapping, 0)

	for i := int32(0); i < replicas; i++ {
		mapping := shardApplicationControllerMapping{
			ShardNumber: i,
		}
		shardMappingData = append(shardMappingData, mapping)
	}
	hostname, err := osHostnameFunction()
	if err != nil {
		return nil, fmt.Errorf("error getting hostname of the pod %s", err)
	}

	shardMappingData[0].ControllerName = hostname
	shardMappingData[0].HeartbeatTime = metav1.Now()

	data, err := json.Marshal(shardMappingData)
	if err != nil {
		return nil, fmt.Errorf("error generating default ConfigMap: %s", err)
	}
	shardingCM.Data[ShardControllerMappingKey] = string(data)

	return shardingCM, nil
}

func getAppControllerDeploymentReplicas(appControllerDeployment *appv1.Deployment) (*int32, error) {

	replicas := appControllerDeployment.Spec.Replicas

	if replicas == nil || *replicas < int32(1) {
		log.Errorf("Application Controller replicas can not be less than 1")
		return nil, fmt.Errorf("Application Controller replicas can not be less than 1")
	}

	return replicas, nil
}

func inferShardFromStatefulSet(hostname string) (int, error) {
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
