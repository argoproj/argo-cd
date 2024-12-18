package utils

import (
	"context"
	"fmt"
	"sync"

	"github.com/argoproj/argo-cd/v2/common"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/db"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	localCluster = appv1.Cluster{
		Name:            "in-cluster",
		Server:          appv1.KubernetesInternalAPIServerAddr,
		ConnectionState: appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful},
	}
	initLocalCluster sync.Once
)

func ListClusters(ctx context.Context, clientset kubernetes.Interface, namespace string) (*appv1.ClusterList, error) {
	clusterSecretsList, err := clientset.CoreV1().Secrets(namespace).List(ctx,
		metav1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster})
	if err != nil {
		return nil, err
	}

	if clusterSecretsList == nil {
		return nil, nil
	}

	clusterSecrets := clusterSecretsList.Items

	clusterList := appv1.ClusterList{
		Items: make([]appv1.Cluster, len(clusterSecrets)),
	}
	hasInClusterCredentials := false
	for i, clusterSecret := range clusterSecrets {
		// This line has changed from the original Argo CD code: now receives an error, and handles it
		cluster, err := db.SecretToCluster(&clusterSecret)
		if err != nil || cluster == nil {
			return nil, fmt.Errorf("unable to convert cluster secret to cluster object '%s': %w", clusterSecret.Name, err)
		}

		// db.SecretToCluster populates these, but they're not meant to be available to the caller.
		cluster.Labels = nil
		cluster.Annotations = nil

		clusterList.Items[i] = *cluster
		if cluster.Server == appv1.KubernetesInternalAPIServerAddr {
			hasInClusterCredentials = true
		}
	}
	if !hasInClusterCredentials {
		localCluster := getLocalCluster(clientset)
		if localCluster != nil {
			clusterList.Items = append(clusterList.Items, *localCluster)
		}
	}
	return &clusterList, nil
}

func getLocalCluster(clientset kubernetes.Interface) *appv1.Cluster {
	initLocalCluster.Do(func() {
		info, err := clientset.Discovery().ServerVersion()
		if err == nil {
			// nolint:staticcheck
			localCluster.ServerVersion = fmt.Sprintf("%s.%s", info.Major, info.Minor)
			// nolint:staticcheck
			localCluster.ConnectionState = appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful}
		} else {
			// nolint:staticcheck
			localCluster.ConnectionState = appv1.ConnectionState{
				Status:  appv1.ConnectionStatusFailed,
				Message: err.Error(),
			}
		}
	})
	cluster := localCluster.DeepCopy()
	now := metav1.Now()
	// nolint:staticcheck
	cluster.ConnectionState.ModifiedAt = &now
	return cluster
}
