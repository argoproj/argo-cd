package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"

	argoappv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	metricsutil "github.com/argoproj/argo-cd/v3/util/metrics"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	metricsCollectionInterval = 30 * time.Second
)

var (
	descClusterDefaultLabels = []string{"server"}

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

type clusterCollector struct {
	infoSource      HasClustersInfo
	info            []cache.ClusterInfo
	lock            sync.Mutex
	clusterLabels   []string
	kubeClientset   kubernetes.Interface
	argoCDNamespace string
}

func (c *clusterCollector) Run(ctx context.Context) {
	//nolint:staticcheck // FIXME: complains about SA1015
	tick := time.Tick(metricsCollectionInterval)
	for {
		select {
		case <-ctx.Done():
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
	if len(c.clusterLabels) > 0 {
		clusterLabels := append(descClusterDefaultLabels, "name")
		normalizedClusterLabels := metricsutil.NormalizeLabels("label", c.clusterLabels)

		descClusterLabels = prometheus.NewDesc(
			"argocd_cluster_labels",
			"Argo Cluster labels converted to Prometheus labels",
			append(clusterLabels, normalizedClusterLabels...),
			nil,
		)
	}

	if len(c.clusterLabels) > 0 {
		ch <- descClusterLabels
	}

	ch <- descClusterInfo
	ch <- descClusterCacheResources
	ch <- descClusterAPIs
	ch <- descClusterCacheAgeSeconds
	ch <- descClusterConnectionStatus
}

func (c *clusterCollector) Collect(ch chan<- prometheus.Metric) {
	now := time.Now()
	// list clusters from Argo CD
	settingsMgr := settings.NewSettingsManager(context.TODO(), c.kubeClientset, c.argoCDNamespace)
	argoDB := db.NewDB(c.argoCDNamespace, settingsMgr, c.kubeClientset)
	clustersList, err := argoDB.ListClusters(context.TODO())
	if err != nil {
		return
	}

	clusters := *clustersList
	for _, cluster := range c.info {
		metadata := findClusterMetadata(cluster.Server, clusters)

		defaultValues := []string{cluster.Server}
		ch <- prometheus.MustNewConstMetric(descClusterInfo, prometheus.GaugeValue, 1, append(defaultValues, cluster.K8SVersion, metadata.Name)...)
		ch <- prometheus.MustNewConstMetric(descClusterCacheResources, prometheus.GaugeValue, float64(cluster.ResourcesCount), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterAPIs, prometheus.GaugeValue, float64(cluster.APIsCount), defaultValues...)
		cacheAgeSeconds := -1
		if cluster.LastCacheSyncTime != nil {
			cacheAgeSeconds = int(now.Sub(*cluster.LastCacheSyncTime).Seconds())
		}
		ch <- prometheus.MustNewConstMetric(descClusterCacheAgeSeconds, prometheus.GaugeValue, float64(cacheAgeSeconds), defaultValues...)
		ch <- prometheus.MustNewConstMetric(descClusterConnectionStatus, prometheus.GaugeValue, boolFloat64(cluster.SyncError == nil), append(defaultValues, cluster.K8SVersion)...)

		if len(c.clusterLabels) > 0 {
			labelValues := []string{}
			labelValues = append(labelValues, cluster.Server, metadata.Name)
			for _, desiredLabel := range c.clusterLabels {
				value := metadata.Labels[desiredLabel]
				labelValues = append(labelValues, value)
			}
			ch <- prometheus.MustNewConstMetric(descClusterLabels, prometheus.GaugeValue, 1, labelValues...)
		}
	}
}

func findClusterMetadata(server string, clusters argoappv1.ClusterList) argoappv1.Cluster {
	for _, cluster := range clusters.Items {
		if cluster.Server == server {
			return cluster
		}
	}
	return argoappv1.Cluster{}
}
