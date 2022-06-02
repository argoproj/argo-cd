package sharding

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/common"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/env"
	log "github.com/sirupsen/logrus"
	"hash/fnv"
	"math"
	"strconv"
	"strings"
)

// ApplicationSetFilter the function used by the controller to filter ApplicationSets that belongs to its shard
type ApplicationSetFilter func(appset *argoprojiov1alpha1.ApplicationSet) bool

var noFilter ApplicationSetFilter = func(appset *argoprojiov1alpha1.ApplicationSet) bool {
	return true
}

// InferShardFromHostname tries to detect the shard which controller instance manages by its hostname
// For instance, applicationset-controller-0 manages the shard 0
// For instance, applicationset-controller-1 manages the shard 1
func InferShardFromHostname(hostnameDetector func() (string, error)) (int, error) {
	hostname, err := hostnameDetector()
	if err != nil {
		return 0, err
	}
	parts := strings.Split(hostname, "-")
	if len(parts) == 1 {
		return 0, fmt.Errorf("hostname should ends with shard number separated by '-' but got: %s", hostname)
	}
	shard, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0, fmt.Errorf("hostname should ends with shard number separated by '-' but got: %s", hostname)
	}
	return shard, nil
}

// InferShard initially tries to detect the shard which controller instance manages by environment variable
// If not specified, it fallbacks to InferShardFromHostname
func InferShard(hostnameDetector func() (string, error)) (int, error) {
	shard := env.ParseNumFromEnv(common.EnvApplicationSetControllerShard, -1, -math.MaxInt32, math.MaxInt32)
	if shard < 0 {
		return InferShardFromHostname(hostnameDetector)
	}
	return shard, nil
}

func GenerateApplicationSetFilterForStatefulSet(hostnameDetector func() (string, error)) (ApplicationSetFilter, error) {
	replicas := env.ParseNumFromEnv(common.EnvApplicationSetControllerReplicas, 0, 0, math.MaxInt32)
	if replicas <= 1 {
		return noFilter, nil
	}

	shard, err := InferShard(hostnameDetector)
	if err != nil {
		return nil, err
	}
	if shard >= replicas {
		return nil, fmt.Errorf("illegal status detected while generating applicastionset filter we have %d replicas but controller assigned to %d shard", replicas, shard)
	}
	log.Debugf("Generating applicationset filter with replicas: %d, shard:%d", replicas, shard)

	return func(appset *argoprojiov1alpha1.ApplicationSet) bool {
		shardOfAppset := 0
		if appset != nil {
			shardOfAppset = getShardByID(string(appset.UID), replicas)
		}
		return shardOfAppset == shard
	}, nil
}

// getShardByID calculates the shard as `id % replicas count`
func getShardByID(id string, replicas int) int {
	if id == "" {
		return 0
	} else {
		h := fnv.New32a()
		_, _ = h.Write([]byte(id))
		return int(h.Sum32() % uint32(replicas))
	}
}
