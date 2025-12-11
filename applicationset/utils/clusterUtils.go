package utils

import (
	corev1 "k8s.io/api/core/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// ClusterSpecifier contains only the name and server URL of a cluster. We use this struct to avoid partially-populating
// the full Cluster struct, which would be misleading.
type ClusterSpecifier struct {
	Name   string
	Server string
}

// SecretsContainInClusterCredentials checks if any of the provided secrets represent the in-cluster configuration.
func SecretsContainInClusterCredentials(secrets []corev1.Secret) bool {
	for _, secret := range secrets {
		if string(secret.Data["server"]) == appv1.KubernetesInternalAPIServerAddr {
			return true
		}
	}
	return false
}

// ListClusters returns a list of cluster specifiers using the ClusterInformer.
func ListClusters(clusterInformer *settings.ClusterInformer) ([]ClusterSpecifier, error) {
	clusters, err := clusterInformer.ListClusters()
	if err != nil {
		return nil, err
	}
	// len of clusters +1 for the in cluster secret
	clusterList := make([]ClusterSpecifier, 0, len(clusters)+1)
	hasInCluster := false

	for _, cluster := range clusters {
		clusterList = append(clusterList, ClusterSpecifier{
			Name:   cluster.Name,
			Server: cluster.Server,
		})
		if cluster.Server == appv1.KubernetesInternalAPIServerAddr {
			hasInCluster = true
		}
	}

	if !hasInCluster {
		// There was no secret for the in-cluster config, so we add it here. We don't fully-populate the Cluster struct,
		// since only the name and server fields are used by the generator.
		clusterList = append(clusterList, ClusterSpecifier{
			Name:   appv1.KubernetesInClusterName,
			Server: appv1.KubernetesInternalAPIServerAddr,
		})
	}

	return clusterList, nil
}
