package db

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"reflect"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

var (
	localCluster = appv1.Cluster{
		Server:          common.KubernetesInternalAPIServerAddr,
		ConnectionState: appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful},
	}
)

func (db *db) listClusterSecrets() ([]*apiv1.Secret, error) {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.LabelValueSecretTypeCluster})
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)

	secretsLister, err := db.settingsMgr.GetSecretsLister()
	if err != nil {
		return nil, err
	}
	clusterSecrets, err := secretsLister.Secrets(db.ns).List(labelSelector)
	if err != nil {
		return nil, err
	}
	return clusterSecrets, nil
}

// ListClusters returns list of clusters
func (db *db) ListClusters(ctx context.Context) (*appv1.ClusterList, error) {
	clusterSecrets, err := db.listClusterSecrets()
	if err != nil {
		return nil, err
	}
	clusterList := appv1.ClusterList{
		Items: make([]appv1.Cluster, len(clusterSecrets)),
	}
	hasInClusterCredentials := false
	for i, clusterSecret := range clusterSecrets {
		cluster := *secretToCluster(clusterSecret)
		clusterList.Items[i] = cluster
		if cluster.Server == common.KubernetesInternalAPIServerAddr {
			hasInClusterCredentials = true
		}
	}
	if !hasInClusterCredentials {
		clusterList.Items = append(clusterList.Items, localCluster)
	}
	return &clusterList, nil
}

// CreateCluster creates a cluster
func (db *db) CreateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	secName, err := serverToSecretName(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
	}
	clusterSecret.Data = clusterToData(c)
	clusterSecret, err = db.kubeclientset.CoreV1().Secrets(db.ns).Create(clusterSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "cluster %q already exists", c.Server)
		}
		return nil, err
	}
	return secretToCluster(clusterSecret), db.settingsMgr.ResyncInformers()
}

// ClusterEvent contains information about cluster event
type ClusterEvent struct {
	Type    watch.EventType
	Cluster *appv1.Cluster
}

// WatchClusters allow watching for cluster events
func (db *db) WatchClusters(ctx context.Context, callback func(*ClusterEvent)) error {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.LabelValueSecretTypeCluster})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	w, err := db.kubeclientset.CoreV1().Secrets(db.ns).Watch(listOpts)
	if err != nil {
		return err
	}

	localCls, err := db.GetCluster(ctx, common.KubernetesInternalAPIServerAddr)
	if err != nil {
		return err
	}

	defer w.Stop()
	done := make(chan bool)

	// trigger callback with event for local cluster since it always considered added
	callback(&ClusterEvent{Type: watch.Added, Cluster: localCls})

	go func() {
		for next := range w.ResultChan() {
			secret := next.Object.(*apiv1.Secret)
			cluster := secretToCluster(secret)

			// change local cluster event to modified or deleted, since it cannot be re-added or deleted
			if cluster.Server == common.KubernetesInternalAPIServerAddr {
				if next.Type == watch.Deleted {
					next.Type = watch.Modified
					cluster = &localCluster
				} else if next.Type == watch.Added {
					if !reflect.DeepEqual(localCls.Config, cluster.Config) {
						localCls = cluster
						next.Type = watch.Modified
					} else {
						continue
					}
				} else {
					localCls = cluster
				}
			}

			callback(&ClusterEvent{
				Type:    next.Type,
				Cluster: cluster,
			})
		}
		done <- true
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}

func (db *db) getClusterSecret(server string) (*apiv1.Secret, error) {
	clusterSecrets, err := db.listClusterSecrets()
	if err != nil {
		return nil, err
	}
	for _, clusterSecret := range clusterSecrets {
		if secretToCluster(clusterSecret).Server == server {
			return clusterSecret, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "cluster %q not found", server)

}

// GetCluster returns a cluster from a query
func (db *db) GetCluster(ctx context.Context, server string) (*appv1.Cluster, error) {
	clusterSecret, err := db.getClusterSecret(server)
	if err != nil {
		if errorStatus, ok := status.FromError(err); ok && errorStatus.Code() == codes.NotFound && server == common.KubernetesInternalAPIServerAddr {
			return &localCluster, nil
		} else {
			return nil, err
		}
	}
	return secretToCluster(clusterSecret), nil
}

// UpdateCluster updates a cluster
func (db *db) UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	clusterSecret, err := db.getClusterSecret(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret.Data = clusterToData(c)
	clusterSecret, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(clusterSecret)
	if err != nil {
		return nil, err
	}
	return secretToCluster(clusterSecret), db.settingsMgr.ResyncInformers()
}

// Delete deletes a cluster by name
func (db *db) DeleteCluster(ctx context.Context, name string) error {
	secret, err := db.getClusterSecret(name)
	if err != nil {
		if errorStatus, ok := status.FromError(err); ok && errorStatus.Code() == codes.NotFound {
			return nil
		} else {
			return err
		}
	}

	canDelete := secret.Annotations != nil && secret.Annotations[common.AnnotationKeyManagedBy] == common.AnnotationValueManagedByArgoCD

	if canDelete {
		err = db.kubeclientset.CoreV1().Secrets(db.ns).Delete(secret.Name, &metav1.DeleteOptions{})
	} else {
		delete(secret.Labels, common.LabelKeySecretType)
		_, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(secret)
	}
	if err != nil {
		return err
	}
	return db.settingsMgr.ResyncInformers()
}

// serverToSecretName hashes server address to the secret name using a formula.
// Part of the server address is incorporated for debugging purposes
func serverToSecretName(server string) (string, error) {
	serverURL, err := url.ParseRequestURI(server)
	if err != nil {
		return "", err
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(server))
	host := strings.ToLower(strings.Split(serverURL.Host, ":")[0])
	return fmt.Sprintf("cluster-%s-%v", host, h.Sum32()), nil
}

// clusterToData converts a cluster object to string data for serialization to a secret
func clusterToData(c *appv1.Cluster) map[string][]byte {
	data := make(map[string][]byte)
	data["server"] = []byte(c.Server)
	if c.Name == "" {
		data["name"] = []byte(c.Server)
	} else {
		data["name"] = []byte(c.Name)
	}
	configBytes, err := json.Marshal(c.Config)
	if err != nil {
		panic(err)
	}
	data["config"] = configBytes
	return data
}

// secretToCluster converts a secret into a repository object
func secretToCluster(s *apiv1.Secret) *appv1.Cluster {
	var config appv1.ClusterConfig
	err := json.Unmarshal(s.Data["config"], &config)
	if err != nil {
		panic(err)
	}
	cluster := appv1.Cluster{
		Server: string(s.Data["server"]),
		Name:   string(s.Data["name"]),
		Config: config,
	}
	return &cluster
}
