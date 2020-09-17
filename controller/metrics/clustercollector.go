package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	metricsCollectionInterval = 30 * time.Second
)

var (
	descClusterDefaultLabels = []string{"server"}

	descClusterInfo = prometheus.NewDesc(
		"argocd_cluster_info",
		"Information about cluster.",
		append(descClusterDefaultLabels, "k8s_version"),
		nil,
	)
	descClusterCacheResources = prometheus.NewDesc(
		"argocd_cluster_api_resource_objects",
		"Number of k8s resource objects in the cache.",
		descClusterDefaultLabels,
		nil,
	)
	descClusterAPIs = prometheus.NewDesc(
		"argocd_cluster_api_resources",
		"Number of monitored kubernetes API resources.",
		descClusterDefaultLabels,
		nil,
	)
	descClusterCacheAgeSeconds = prometheus.NewDesc(
		"argocd_cluster_cache_age_seconds",
		"Cluster cache age in seconds.",
		descClusterDefaultLabels,
		nil,
	)
)

type HasClustersInfo interface {
	GetClustersInfo() []cache.ClusterInfo
}

type clusterCollector struct {
	infoSource HasClustersInfo
	info       []cache.ClusterInfo
	lock       sync.Mutex
}

func (c *clusterCollector) Run(ctx context.Context) {
	// FIXME: complains about SA1015
	// nolint:staticcheck
	tick := time.Tick(metricsCollectionInterval)
	for {
		select {
		case <-ctx.Done():
			break
		case <-tick:
			info := c.infoSource.GetClustersInfo()

			c.lock.Lock()
			c.info = info
			c.lock.Unlock()
		}
	}
}

// Describe implements the prometheus.Collector interface
func (c *clusterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descClusterInfo
	ch <- descClusterCacheResources
	ch <- descClusterAPIs
	ch <- descClusterCacheAgeSeconds
}

func (c *clusterCollector) Collect(ch chan<- prometheus.Metric) {
	now := time.Now()
	for _, c := range c.info {
		defaultValues := []string{c.Server}
		ch <- prometheus.MustNewConstMetric(descClusterInfo, prometheus.GaugeValue, 1, append(defaultValues, c.K8SVersion)...)
		ch <- prometheus.MustNewConstMetric(descClusterCacheResources, prometheus.GaugeValue, float64(c.ResourcesCount), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterAPIs, prometheus.GaugeValue, float64(c.APIsCount), defaultValues...)
		cacheAgeSeconds := -1
		if c.LastCacheSyncTime != nil {
			cacheAgeSeconds = int(now.Sub(*c.LastCacheSyncTime).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(descClusterCacheAgeSeconds, prometheus.GaugeValue, float64(cacheAgeSeconds), defaultValues...)
	}
}
