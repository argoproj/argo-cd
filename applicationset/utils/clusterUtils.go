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

// The contents of this file are from
// github.com/argoproj/argo-cd/util/db/cluster.go
//
// The main difference is that ListClusters(...) calls the kubeclient directly,
// via `g.clientset.CoreV1().Secrets`, rather than using the `db.listClusterSecrets()``
// which appears to have a race condition on when it is called.
//
// I was reminded of this issue that I opened, which might be related:
// https://github.com/argoproj/argo-cd/issues/4755
//
// I hope to upstream this change in some form, so that we do not need to worry about
// Argo CD changing the logic on us.

var (
	localCluster = appv1.Cluster{
		Name:            "in-cluster",
		Server:          appv1.KubernetesInternalAPIServerAddr,
		ConnectionState: appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful},
	}
	initLocalCluster sync.Once
)

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
)

// ValidateDestination checks:
// if we used destination name we infer the server url
// if we used both name and server then we return an invalid spec error
func ValidateDestination(ctx context.Context, dest *appv1.ApplicationDestination, clientset kubernetes.Interface, argoCDNamespace string) error {
	if dest.IsServerInferred() && dest.IsNameInferred() {
		return fmt.Errorf("application destination can't have both name and server inferred: %s %s", dest.Name, dest.Server)
	}
	if dest.Name != "" {
		if dest.Server == "" {
			server, err := getDestinationBy(ctx, dest.Name, clientset, argoCDNamespace, true)
			if err != nil {
				return fmt.Errorf("unable to find destination server: %w", err)
			}
			if server == "" {
				return fmt.Errorf("application references destination cluster %s which does not exist", dest.Name)
			}
			dest.SetInferredServer(server)
		} else if !dest.IsServerInferred() && !dest.IsNameInferred() {
			return fmt.Errorf("application destination can't have both name and server defined: %s %s", dest.Name, dest.Server)
		}
	} else if dest.Server != "" {
		if dest.Name == "" {
			serverName, err := getDestinationBy(ctx, dest.Server, clientset, argoCDNamespace, false)
			if err != nil {
				return fmt.Errorf("unable to find destination server: %w", err)
			}
			if serverName == "" {
				return fmt.Errorf("application references destination cluster %s which does not exist", dest.Server)
			}
			dest.SetInferredName(serverName)
		}
	}
	return nil
}

func getDestinationBy(ctx context.Context, cluster string, clientset kubernetes.Interface, argoCDNamespace string, byName bool) (string, error) {
	// settingsMgr := settings.NewSettingsManager(context.TODO(), clientset, namespace)
	// argoDB := db.NewDB(namespace, settingsMgr, clientset)
	// clusterList, err := argoDB.ListClusters(ctx)
	clusterList, err := ListClusters(ctx, clientset, argoCDNamespace)
	if err != nil {
		return "", err
	}
	var servers []string
	for _, c := range clusterList.Items {
		if byName && c.Name == cluster {
			servers = append(servers, c.Server)
		}
		if !byName && c.Server == cluster {
			servers = append(servers, c.Name)
		}
	}
	if len(servers) > 1 {
		return "", fmt.Errorf("there are %d clusters with the same name: %v", len(servers), servers)
	} else if len(servers) == 0 {
		return "", fmt.Errorf("there are no clusters with this name: %s", cluster)
	}
	return servers[0], nil
}

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
