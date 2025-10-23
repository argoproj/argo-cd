package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	metricsutil "github.com/argoproj/argo-cd/v3/util/metrics"
)

const (
	metricsCollectionInterval = 30 * time.Second
	metricsCollectionTimeout  = 10 * time.Second
)

var (
	descClusterDefaultLabels = []string{"server"}

	descClusterLabels *prometheus.Desc

	descClusterInfo = prometheus.NewDesc(
		"argocd_cluster_info",
		"Information about cluster.",
		append(descClusterDefaultLabels, "k8s_version", "name"),
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
	descClusterConnectionStatus = prometheus.NewDesc(
		"argocd_cluster_connection_status",
		"The k8s cluster current connection status.",
		append(descClusterDefaultLabels, "k8s_version"),
		nil,
	)
)

type HasClustersInfo interface {
	GetClustersInfo() []cache.ClusterInfo
}

type ClusterLister func(ctx context.Context) (*argoappv1.ClusterList, error)

type clusterCollector struct {
	infoSource    HasClustersInfo
	lock          sync.RWMutex
	clusterLabels []string
	clusterLister ClusterLister

	latestInfo []*clusterData
}

type clusterData struct {
	info    *cache.ClusterInfo
	cluster *argoappv1.Cluster
}

func NewClusterCollector(ctx context.Context, source HasClustersInfo, clusterLister ClusterLister, clusterLabels []string) prometheus.Collector {
	if len(clusterLabels) > 0 {
		normalizedClusterLabels := metricsutil.NormalizeLabels("label", clusterLabels)
		descClusterLabels = prometheus.NewDesc(
			"argocd_cluster_labels",
			"Argo Cluster labels converted to Prometheus labels",
			append(append(descClusterDefaultLabels, "name"), normalizedClusterLabels...),
			nil,
		)
	}

	collector := &clusterCollector{
		infoSource:    source,
		clusterLabels: clusterLabels,
		clusterLister: clusterLister,
		lock:          sync.RWMutex{},
	}

	collector.setClusterData()
	go collector.run(ctx)

	return collector
}

func (c *clusterCollector) run(ctx context.Context) {
	//nolint:staticcheck // FIXME: complains about SA1015
	tick := time.Tick(metricsCollectionInterval)
	for {
		select {
		case <-ctx.Done():
		case <-tick:
			c.setClusterData()
		}
	}
}

func (c *clusterCollector) setClusterData() {
	if clusterData, err := c.getClusterData(); err == nil {
		c.lock.Lock()
		c.latestInfo = clusterData
		c.lock.Unlock()
	} else {
		log.Warnf("error collecting cluster metrics: %v", err)
	}
}

func (c *clusterCollector) getClusterData() ([]*clusterData, error) {
	clusterDatas := []*clusterData{}
	clusterInfos := c.infoSource.GetClustersInfo()

	ctx, cancel := context.WithTimeout(context.Background(), metricsCollectionTimeout)
	defer cancel()
	clusters, err := c.clusterLister(ctx)
	if err != nil {
		return nil, err
	}

	clusterMap := map[string]*argoappv1.Cluster{}
	for i, cluster := range clusters.Items {
		clusterMap[cluster.Server] = &clusters.Items[i]
	}

	// Base the cluster data on the ClusterInfo because it only contains the
	// clusters managed by this controller instance
	for i, info := range clusterInfos {
		cluster, ok := clusterMap[info.Server]
		if !ok {
			// This should not happen, but we cannot emit incomplete metrics, so we skip this cluster
			log.WithField("server", info.Server).Warnf("could find cluster for metrics collection")
			continue
		}
		clusterDatas = append(clusterDatas, &clusterData{
			info:    &clusterInfos[i],
			cluster: cluster,
		})
	}
	return clusterDatas, nil
}

// Describe implements the prometheus.Collector interface
func (c *clusterCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descClusterInfo
	ch <- descClusterCacheResources
	ch <- descClusterAPIs
	ch <- descClusterCacheAgeSeconds
	ch <- descClusterConnectionStatus
	if len(c.clusterLabels) > 0 {
		ch <- descClusterLabels
	}
}

func (c *clusterCollector) Collect(ch chan<- prometheus.Metric) {
	c.lock.RLock()
	latestInfo := c.latestInfo
	c.lock.RUnlock()

	now := time.Now()
	for _, clusterData := range latestInfo {
		info := clusterData.info
		name := clusterData.cluster.Name
		labels := clusterData.cluster.Labels

		defaultValues := []string{info.Server}
		ch <- prometheus.MustNewConstMetric(descClusterInfo, prometheus.GaugeValue, 1, append(defaultValues, info.K8SVersion, name)...)
		ch <- prometheus.MustNewConstMetric(descClusterCacheResources, prometheus.GaugeValue, float64(info.ResourcesCount), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterAPIs, prometheus.GaugeValue, float64(info.APIsCount), defaultValues...)
		cacheAgeSeconds := -1
		if info.LastCacheSyncTime != nil {
			cacheAgeSeconds = int(now.Sub(*info.LastCacheSyncTime).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(descClusterCacheAgeSeconds, prometheus.GaugeValue, float64(cacheAgeSeconds), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterConnectionStatus, prometheus.GaugeValue, boolFloat64(info.SyncError == nil), append(defaultValues, info.K8SVersion)...)

		if len(c.clusterLabels) > 0 && labels != nil {
			labelValues := []string{}
			labelValues = append(labelValues, info.Server, name)
			for _, desiredLabel := range c.clusterLabels {
				value := labels[desiredLabel]
				labelValues = append(labelValues, value)
			}
			ch <- prometheus.MustNewConstMetric(descClusterLabels, prometheus.GaugeValue, 1, labelValues...)
		}
	}
}
