package sharding

import (
	"fmt"
	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"hash/fnv"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/v2/util/env"
	log "github.com/sirupsen/logrus"
)

var osHostnameFunction = os.Hostname

type DistributionFunction func(c *v1alpha1.Application) int
type ApplicationFilterFunction func(c *v1alpha1.Application) (bool, int)

type Sharding interface {
	GetApplicationFilter(distributionFunction DistributionFunction, shard int) ApplicationFilterFunction
	GetDistributionFunction(shardingAlgorithm string) DistributionFunction
}

type sharding struct {
}

func NewSharding() Sharding {
	return &sharding{}
}

func (s *sharding) GetApplicationFilter(distributionFunction DistributionFunction, shard int) ApplicationFilterFunction {
	return func(app *v1alpha1.Application) (bool, int) {
		expectedShard := distributionFunction(app)
		// TODO: [reporter] provide ability define label with shard number
		return expectedShard == shard, expectedShard
	}
}

// GetDistributionFunction returns which DistributionFunction should be used based on the passed algorithm and
// the current datas.
func (s *sharding) GetDistributionFunction(shardingAlgorithm string) DistributionFunction {
	log.Infof("Using filter function:  %s", shardingAlgorithm)
	//TODO: implement switch case for multiple strategies
	return s.LegacyDistributionFunction()
}

func (s *sharding) LegacyDistributionFunction() DistributionFunction {
	replicas := env.ParseNumFromEnv(argocommon.EnvEventReporterReplicas, 0, 0, math.MaxInt32)
	return func(a *v1alpha1.Application) int {
		if replicas == 0 {
			return -1
		}
		if a == nil {
			return 0
		}
		id := a.Name
		if id == "" {
			return 0
		} else {
			h := fnv.New32a()
			_, _ = h.Write([]byte(id))
			shard := int32(h.Sum32() % uint32(replicas))
			log.Debugf("Application with id=%s will be processed by shard %d", id, shard)
			return int(shard)
		}
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

func GetShardNumber() int {
	shard := env.ParseNumFromEnv(argocommon.EnvEventReporterShard, -1, -math.MaxInt32, math.MaxInt32)

	if shard < 0 {
		var err error
		shard, err = InferShard()
		if err != nil {
			return -1
		}
	}

	return shard
}
