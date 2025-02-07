package utils

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/common"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Cluster struct {
	Name   string
	Server string
}

func ListClusters(ctx context.Context, clientset kubernetes.Interface, namespace string) ([]Cluster, error) {
	clusterSecretsList, err := clientset.CoreV1().Secrets(namespace).List(ctx,
		metav1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster})
	if err != nil {
		return nil, err
	}

	if clusterSecretsList == nil {
		return nil, nil
	}

	clusterSecrets := clusterSecretsList.Items

	clusterList := make([]Cluster, len(clusterSecrets))

	hasInClusterCredentials := false
	for i, clusterSecret := range clusterSecrets {
		cluster, err := db.SecretToCluster(&clusterSecret)
		if err != nil || cluster == nil {
			return nil, fmt.Errorf("unable to convert cluster secret to cluster object '%s': %w", clusterSecret.Name, err)
		}
		clusterList[i] = Cluster{
			Name:   cluster.Name,
			Server: cluster.Server,
		}
		if cluster.Server == appv1.KubernetesInternalAPIServerAddr {
			hasInClusterCredentials = true
		}
	}
	if !hasInClusterCredentials {
		// There was no secret for the in-cluster config, so we add it here. We don't fully-populate the Cluster struct,
		// since only the name and server fields are used by the generator.
		clusterList = append(clusterList, Cluster{
			Name:   "in-cluster",
			Server: appv1.KubernetesInternalAPIServerAddr,
		})
	}
	return clusterList, nil
}
