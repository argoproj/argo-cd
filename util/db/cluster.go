package db

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/common"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var (
	localCluster = appv1.Cluster{
		Name:            "in-cluster",
		Server:          appv1.KubernetesInternalAPIServerAddr,
		ConnectionState: appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful},
	}
	initLocalCluster sync.Once
)

func (db *db) getLocalCluster() *appv1.Cluster {
	initLocalCluster.Do(func() {
		info, err := db.kubeclientset.Discovery().ServerVersion()
		if err == nil {
			localCluster.ServerVersion = fmt.Sprintf("%s.%s", info.Major, info.Minor)
			localCluster.ConnectionState = appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful}
		} else {
			localCluster.ConnectionState = appv1.ConnectionState{
				Status:  appv1.ConnectionStatusFailed,
				Message: err.Error(),
			}
		}
	})
	cluster := localCluster.DeepCopy()
	now := metav1.Now()
	cluster.ConnectionState.ModifiedAt = &now
	return cluster
}

// ListClusters returns list of clusters
func (db *db) ListClusters(ctx context.Context) (*appv1.ClusterList, error) {
	clusterSecrets, err := db.listSecretsByType(common.LabelValueSecretTypeCluster)
	if err != nil {
		return nil, err
	}
	clusterList := appv1.ClusterList{
		Items: make([]appv1.Cluster, 0),
	}
	hasInClusterCredentials := false
	for _, clusterSecret := range clusterSecrets {
		cluster, err := secretToCluster(clusterSecret)
		if err != nil {
			log.Errorf("could not unmarshal cluster secret %s", clusterSecret.Name)
			continue
		}
		clusterList.Items = append(clusterList.Items, *cluster)
		if cluster.Server == appv1.KubernetesInternalAPIServerAddr {
			hasInClusterCredentials = true
		}
	}
	if !hasInClusterCredentials {
		clusterList.Items = append(clusterList.Items, *db.getLocalCluster())
	}
	return &clusterList, nil
}

// CreateCluster creates a cluster
func (db *db) CreateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	secName, err := URIToSecretName("cluster", c.Server)
	if err != nil {
		return nil, err
	}

	clusterSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
		},
	}

	if err = clusterToSecret(c, clusterSecret); err != nil {
		return nil, err
	}

	clusterSecret, err = db.createSecret(ctx, common.LabelValueSecretTypeCluster, clusterSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "cluster %q already exists", c.Server)
		}
		return nil, err
	}

	cluster, err := secretToCluster(clusterSecret)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "could not unmarshal cluster secret %s", clusterSecret.Name)
	}
	return cluster, db.settingsMgr.ResyncInformers()
}

// ClusterEvent contains information about cluster event
type ClusterEvent struct {
	Type    watch.EventType
	Cluster *appv1.Cluster
}

func (db *db) WatchClusters(ctx context.Context,
	handleAddEvent func(cluster *appv1.Cluster),
	handleModEvent func(oldCluster *appv1.Cluster, newCluster *appv1.Cluster),
	handleDeleteEvent func(clusterServer string)) error {
	localCls, err := db.GetCluster(ctx, appv1.KubernetesInternalAPIServerAddr)
	if err != nil {
		return err
	}
	handleAddEvent(localCls)

	db.watchSecrets(
		ctx,
		common.LabelValueSecretTypeCluster,

		func(secret *apiv1.Secret) {
			cluster, err := secretToCluster(secret)
			if err != nil {
				log.Errorf("could not unmarshal cluster secret %s", secret.Name)
				return
			}
			if cluster.Server == appv1.KubernetesInternalAPIServerAddr {
				// change local cluster event to modified or deleted, since it cannot be re-added or deleted
				handleModEvent(localCls, cluster)
				localCls = cluster
				return
			}
			handleAddEvent(cluster)
		},

		func(oldSecret *apiv1.Secret, newSecret *apiv1.Secret) {
			oldCluster, err := secretToCluster(oldSecret)
			if err != nil {
				log.Errorf("could not unmarshal cluster secret %s", oldSecret.Name)
				return
			}
			newCluster, err := secretToCluster(newSecret)
			if err != nil {
				log.Errorf("could not unmarshal cluster secret %s", newSecret.Name)
				return
			}
			if newCluster.Server == appv1.KubernetesInternalAPIServerAddr {
				localCls = newCluster
			}
			handleModEvent(oldCluster, newCluster)
		},

		func(secret *apiv1.Secret) {
			if string(secret.Data["server"]) == appv1.KubernetesInternalAPIServerAddr {
				// change local cluster event to modified or deleted, since it cannot be re-added or deleted
				handleModEvent(localCls, db.getLocalCluster())
				localCls = db.getLocalCluster()
			} else {
				handleDeleteEvent(string(secret.Data["server"]))
			}
		},
	)

	return err
}

func (db *db) getClusterSecret(server string) (*apiv1.Secret, error) {
	clusterSecrets, err := db.listSecretsByType(common.LabelValueSecretTypeCluster)
	if err != nil {
		return nil, err
	}
	for _, clusterSecret := range clusterSecrets {
		if cluster, err := secretToCluster(clusterSecret); err == nil && cluster.Server == strings.TrimRight(server, "/") {
			return clusterSecret, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "cluster %q not found", server)
}

// GetCluster returns a cluster from a query
func (db *db) GetCluster(ctx context.Context, server string) (*appv1.Cluster, error) {
	clusterSecret, err := db.getClusterSecret(server)
	if err != nil {
		if errorStatus, ok := status.FromError(err); ok && errorStatus.Code() == codes.NotFound && server == appv1.KubernetesInternalAPIServerAddr {
			return db.getLocalCluster(), nil
		} else {
			return nil, err
		}
	}
	return secretToCluster(clusterSecret)
}

// UpdateCluster updates a cluster
func (db *db) UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	clusterSecret, err := db.getClusterSecret(c.Server)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return db.CreateCluster(ctx, c)
		}
		return nil, err
	}
	if err := clusterToSecret(c, clusterSecret); err != nil {
		return nil, err
	}

	clusterSecret, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(ctx, clusterSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	cluster, err := secretToCluster(clusterSecret)
	if err != nil {
		log.Errorf("could not unmarshal cluster secret %s", clusterSecret.Name)
		return nil, err
	}
	return cluster, db.settingsMgr.ResyncInformers()
}

// DeleteCluster deletes a cluster by name
func (db *db) DeleteCluster(ctx context.Context, server string) error {
	secret, err := db.getClusterSecret(server)
	if err != nil {
		return err
	}

	err = db.deleteSecret(ctx, secret)
	if err != nil {
		return err
	}

	return db.settingsMgr.ResyncInformers()
}

// clusterToData converts a cluster object to string data for serialization to a secret
func clusterToSecret(c *appv1.Cluster, secret *apiv1.Secret) error {
	data := make(map[string][]byte)
	data["server"] = []byte(strings.TrimRight(c.Server, "/"))
	if c.Name == "" {
		data["name"] = []byte(c.Server)
	} else {
		data["name"] = []byte(c.Name)
	}
	if len(c.Namespaces) != 0 {
		data["namespaces"] = []byte(strings.Join(c.Namespaces, ","))
	}
	configBytes, err := json.Marshal(c.Config)
	if err != nil {
		return err
	}
	data["config"] = configBytes
	if c.Shard != nil {
		data["shard"] = []byte(strconv.Itoa(int(*c.Shard)))
	}
	if c.ClusterResources {
		data["clusterResources"] = []byte("true")
	}
	if c.Project != "" {
		data["project"] = []byte(c.Project)
	}
	secret.Data = data

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	if c.RefreshRequestedAt != nil {
		secret.Annotations[appv1.AnnotationKeyRefresh] = c.RefreshRequestedAt.Format(time.RFC3339)
	} else {
		delete(secret.Annotations, appv1.AnnotationKeyRefresh)
	}
	return nil
}

// secretToCluster converts a secret into a Cluster object
func secretToCluster(s *apiv1.Secret) (*appv1.Cluster, error) {
	var config appv1.ClusterConfig
	if len(s.Data["config"]) > 0 {
		err := json.Unmarshal(s.Data["config"], &config)
		if err != nil {
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
			shard = pointer.Int64Ptr(int64(val))
		}
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
	}
	return &cluster, nil
}
