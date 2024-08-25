package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
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
	if dest.Name != "" {
		if dest.Server == "" {
			server, err := getDestinationServer(ctx, dest.Name, clientset, argoCDNamespace)
			if err != nil {
				return fmt.Errorf("unable to find destination server: %w", err)
			}
			if server == "" {
				return fmt.Errorf("application references destination cluster %s which does not exist", dest.Name)
			}
			dest.SetInferredServer(server)
		} else if !dest.IsServerInferred() {
			return fmt.Errorf("application destination can't have both name and server defined: %s %s", dest.Name, dest.Server)
		}
	}
	return nil
}

func getDestinationServer(ctx context.Context, clusterName string, clientset kubernetes.Interface, argoCDNamespace string) (string, error) {
	// settingsMgr := settings.NewSettingsManager(context.TODO(), clientset, namespace)
	// argoDB := db.NewDB(namespace, settingsMgr, clientset)
	// clusterList, err := argoDB.ListClusters(ctx)
	clusterList, err := ListClusters(ctx, clientset, argoCDNamespace)
	if err != nil {
		return "", err
	}
	var servers []string
	for _, c := range clusterList.Items {
		if c.Name == clusterName {
			servers = append(servers, c.Server)
		}
	}
	if len(servers) > 1 {
		return "", fmt.Errorf("there are %d clusters with the same name: %v", len(servers), servers)
	} else if len(servers) == 0 {
		return "", fmt.Errorf("there are no clusters with this name: %s", clusterName)
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
		cluster, err := secretToCluster(&clusterSecret)
		if err != nil || cluster == nil {
			return nil, fmt.Errorf("unable to convert cluster secret to cluster object '%s': %w", clusterSecret.Name, err)
		}

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

// secretToCluster converts a secret into a Cluster object
func secretToCluster(s *corev1.Secret) (*appv1.Cluster, error) {
	var config appv1.ClusterConfig
	if len(s.Data["config"]) > 0 {
		if err := json.Unmarshal(s.Data["config"], &config); err != nil {
			// This line has changed from the original Argo CD: now returns an error rather than panicing.
			return nil, err
		}
	}

	var namespaces []string
	for _, ns := range strings.Split(string(s.Data["namespaces"]), ",") {
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
	cluster := appv1.Cluster{
		ID:                 string(s.UID),
		Server:             strings.TrimRight(string(s.Data["server"]), "/"),
		Name:               string(s.Data["name"]),
		Namespaces:         namespaces,
		Config:             config,
		RefreshRequestedAt: refreshRequestedAt,
		Shard:              shard,
	}
	return &cluster, nil
}
