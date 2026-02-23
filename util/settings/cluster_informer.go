package settings

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/common"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	// ClusterCacheByURLIndexer indexes clusters by server URL
	ClusterCacheByURLIndexer = "byClusterURL"
	// ClusterCacheByNameIndexer indexes clusters by name
	ClusterCacheByNameIndexer = "byClusterName"
)

// ClusterInformer provides a cached view of cluster secrets as Cluster objects.
// It uses an informer with a transform function to convert Secret -> Cluster
// once during ingestion, avoiding repeated conversions.
//
// This eliminates the performance cost of calling SecretToCluster on every
// GetCluster/GetClusterServersByName call, which can be significant in hot paths.
type ClusterInformer struct {
	cache.SharedIndexInformer
}

// NewClusterInformer creates a new cluster cache that watches cluster secrets
// and stores them as pre-converted Cluster objects using informer transforms.
//
// The transform function runs once per secret during informer ingestion,
// converting Secret -> Cluster at that time. This means:
// - Zero conversion overhead on reads (already converted)
// - Same freshness guarantees as regular informers
// - Automatic updates when secrets change
func NewClusterInformer(clientset kubernetes.Interface, namespace string) (*ClusterInformer, error) {
	informer := informersv1.NewFilteredSecretInformer(clientset, namespace, 3*time.Minute, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		ClusterCacheByURLIndexer: func(obj any) ([]string, error) {
			cluster, ok := obj.(*appv1.Cluster)
			if !ok {
				return nil, nil
			}
			return []string{strings.TrimRight(cluster.Server, "/")}, nil
		},
		ClusterCacheByNameIndexer: func(obj any) ([]string, error) {
			cluster, ok := obj.(*appv1.Cluster)
			if !ok {
				return nil, nil
			}
			if cluster.Name != "" {
				return []string{cluster.Name}, nil
			}
			return nil, nil
		},
	}, func(options *metav1.ListOptions) {
		// Only watch secrets with the cluster label
		options.LabelSelector = fmt.Sprintf("%s=%s", common.LabelKeySecretType, common.LabelValueSecretTypeCluster)
	})

	err := informer.SetTransform(func(obj any) (any, error) {
		secret, ok := obj.(*corev1.Secret)
		if !ok {
			// Not a secret, pass through (shouldn't happen but be defensive)
			return obj, nil
		}

		// Skip non-cluster secrets (shouldn't happen with our label selector, but be safe)
		if secret.Labels == nil || secret.Labels[common.LabelKeySecretType] != common.LabelValueSecretTypeCluster {
			return obj, nil
		}

		// Convert to cluster - this happens once during ingestion
		cluster, err := secretToCluster(secret)
		if err != nil {
			log.Warnf("Failed to convert secret %s to cluster: %v", secret.Name, err)
			// Return the secret on error so we don't lose the data
			return obj, nil
		}

		log.Debugf("Transformed cluster secret %s -> cluster %s (%s)", secret.Name, cluster.Name, cluster.Server)
		return cluster, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set transform on cluster informer: %w", err)
	}

	cc := &ClusterInformer{
		informer,
	}

	return cc, nil
}

// GetClusterByURL retrieves a cluster by its server URL from the cache.
// Returns the pre-converted Cluster object with zero conversion overhead.
func (cc *ClusterInformer) GetClusterByURL(url string) (*appv1.Cluster, error) {
	url = strings.TrimRight(url, "/")
	items, err := cc.GetIndexer().ByIndex(ClusterCacheByURLIndexer, url)
	if err != nil {
		return nil, fmt.Errorf("failed to query cluster cache by URL: %w", err)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("cluster %q not found in cache", url)
	}

	cluster, ok := items[0].(*appv1.Cluster)
	if !ok {
		return nil, fmt.Errorf("expected *appv1.Cluster, got %T (transform may have failed)", items[0])
	}

	// Return a copy to prevent callers from modifying the cached object
	return cluster.DeepCopy(), nil
}

// GetClusterServersByName retrieves all server URLs for clusters with the given name.
func (cc *ClusterInformer) GetClusterServersByName(name string) ([]string, error) {
	items, err := cc.GetIndexer().ByIndex(ClusterCacheByNameIndexer, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query cluster cache by name: %w", err)
	}

	servers := make([]string, 0, len(items))
	for _, item := range items {
		cluster, ok := item.(*appv1.Cluster)
		if !ok {
			log.Warnf("Expected *appv1.Cluster in cache, got %T (skipping)", item)
			continue
		}
		servers = append(servers, cluster.Server)
	}

	return servers, nil
}

// ListClusters returns all clusters in the cache.
// Returns an error if any item in the cache is not a *Cluster (indicates transform failure).
func (cc *ClusterInformer) ListClusters() ([]*appv1.Cluster, error) {
	items := cc.GetIndexer().List()
	clusters := make([]*appv1.Cluster, 0, len(items))

	for _, item := range items {
		cluster, ok := item.(*appv1.Cluster)
		if !ok {
			// Return an error to prevent partial data from causing incorrect applicationset deletions
			return nil, fmt.Errorf("cluster cache contains unexpected type %T instead of *Cluster, secret conversion failure", item)
		}
		// Return copies to prevent modification of cached objects
		clusters = append(clusters, cluster.DeepCopy())
	}

	return clusters, nil
}

// secretToCluster converts a secret into a Cluster object.
// This is a copy of db.SecretToCluster to avoid circular dependency.
func secretToCluster(s *corev1.Secret) (*appv1.Cluster, error) {
	var config appv1.ClusterConfig
	if len(s.Data["config"]) > 0 {
		err := json.Unmarshal(s.Data["config"], &config)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cluster config: %w", err)
		}
	}

	var namespaces []string
	for ns := range strings.SplitSeq(string(s.Data["namespaces"]), ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	var refreshRequestedAt *metav1.Time
	if v, found := s.Annotations[appv1.AnnotationKeyRefresh]; found {
		requestedAt, err := time.Parse(time.RFC3339, v)
		if err != nil {
			log.Warnf("Error while parsing date in cluster secret '%s': %v", s.Name, err)
		} else {
			refreshRequestedAt = &metav1.Time{Time: requestedAt}
		}
	}
	var shard *int64
	if shardStr := s.Data["shard"]; shardStr != nil {
		if val, err := strconv.Atoi(string(shardStr)); err != nil {
			log.Warnf("Error while parsing shard in cluster secret '%s': %v", s.Name, err)
		} else {
			shard = ptr.To(int64(val))
		}
	}

	// copy labels and annotations excluding system ones
	labels := map[string]string{}
	if s.Labels != nil {
		labels = maps.Clone(s.Labels)
		delete(labels, common.LabelKeySecretType)
	}
	annotations := map[string]string{}
	if s.Annotations != nil {
		annotations = maps.Clone(s.Annotations)
		// delete system annotations
		delete(annotations, corev1.LastAppliedConfigAnnotation)
		delete(annotations, common.AnnotationKeyManagedBy)
	}

	cluster := appv1.Cluster{
		ID:                 string(s.UID),
		Server:             strings.TrimRight(string(s.Data["server"]), "/"),
		Name:               string(s.Data["name"]),
		Namespaces:         namespaces,
		ClusterResources:   string(s.Data["clusterResources"]) == "true",
		Config:             config,
		RefreshRequestedAt: refreshRequestedAt,
		Shard:              shard,
		Project:            string(s.Data["project"]),
		Labels:             labels,
		Annotations:        annotations,
	}
	// To ensure the informer cache is properly populated, use the secret's name/namespace as the cache key
	cluster.ObjectMeta.Name = s.Name
	cluster.Namespace = s.Namespace

	return &cluster, nil
}
