package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	descClusterDefaultLabels = []string{"server"}

	descClusterInfo = prometheus.NewDesc(
		"argocd_cluster_info",
		"Information about cluster.",
		append(descClusterDefaultLabels, "k8s_version"),
		nil,
	)
	descClusterCacheResourcesCount = prometheus.NewDesc(
		"argocd_cluster_resources_count",
		"Number of k8s resources in cache.",
		descClusterDefaultLabels,
		nil,
	)
	descClusterAPIsCount = prometheus.NewDesc(
		"argocd_cluster_apis_count",
		"Number of monitored kubernetes APIs.",
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

type ClusterInfo struct {
	Server            string
	K8SVersion        string
	ResourcesCount    int
	APIsCount         int
	LastCacheSyncTime *time.Time
}

type HasClustersInfo interface {
	GetClustersInfo() []ClusterInfo
}

type clusterCollector struct {
	infoSource HasClustersInfo
}

// Describe implements the prometheus.Collector interface
func (c *clusterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descClusterInfo
	ch <- descClusterCacheResourcesCount
	ch <- descClusterAPIsCount
	ch <- descClusterCacheAgeSeconds
}

func (c *clusterCollector) Collect(ch chan<- prometheus.Metric) {
	now := time.Now()
	for _, c := range c.infoSource.GetClustersInfo() {
		defaultValues := []string{c.Server}
		ch <- prometheus.MustNewConstMetric(descClusterInfo, prometheus.GaugeValue, 1, append(defaultValues, c.K8SVersion)...)
		ch <- prometheus.MustNewConstMetric(descClusterCacheResourcesCount, prometheus.GaugeValue, float64(c.ResourcesCount), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterAPIsCount, prometheus.GaugeValue, float64(c.APIsCount), defaultValues...)
		cacheAgeSeconds := -1
		if c.LastCacheSyncTime != nil {
			cacheAgeSeconds = int(now.Sub(*c.LastCacheSyncTime).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(descClusterCacheAgeSeconds, prometheus.GaugeValue, float64(cacheAgeSeconds), defaultValues...)
	}
}
