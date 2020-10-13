package sharding

import (
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
	return shard, nil
}

// getShardByID calculates cluster shard as `clusterSecret.UID % replicas count`
func getShardByID(id string, replicas int) int {
	if id == "" {
		return 0
	} else {
		h := fnv.New32a()
		_, _ = h.Write([]byte(id))
		return int(h.Sum32() % uint32(replicas))
	}
}

func GetClusterFilter(replicas int, shard int) func(c *v1alpha1.Cluster) bool {
	return func(c *v1alpha1.Cluster) bool {
		clusterShard := 0
		//  cluster might be nil if app is using invalid cluster URL, assume shard 0 in this case.
		if c != nil {
			if c.Shard != nil {
				clusterShard = int(*c.Shard)
			} else {
				clusterShard = getShardByID(c.ID, replicas)
			}
		}
		return clusterShard == shard
	}
}
