package db

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
)

var (
	localCluster = appv1.Cluster{
		Server:          common.KubernetesInternalAPIServerAddr,
		ConnectionState: appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful},
	}
)

// ListClusters returns list of clusters
func (s *db) ListClusters(ctx context.Context) (*appv1.ClusterList, error) {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.SecretTypeCluster})
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	clusterSecrets, err := s.kubeclientset.CoreV1().Secrets(s.ns).List(listOpts)
	if err != nil {
		return nil, err
	}
	clusterList := appv1.ClusterList{
		Items: make([]appv1.Cluster, len(clusterSecrets.Items)),
	}
	hasInClusterCredentials := false
	for i, clusterSecret := range clusterSecrets.Items {
		cluster := *SecretToCluster(&clusterSecret)
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
func (s *db) CreateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	secName, err := serverToSecretName(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.SecretTypeCluster,
			},
		},
	}
	clusterSecret.Data = clusterToData(c)
	clusterSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Create(clusterSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "cluster %q already exists", c.Server)
		}
		return nil, err
	}
	return SecretToCluster(clusterSecret), nil
}

// ClusterEvent contains information about cluster event
type ClusterEvent struct {
	Type    watch.EventType
	Cluster *appv1.Cluster
}

// WatchClusters allow watching for cluster events
func (s *db) WatchClusters(ctx context.Context, callback func(*ClusterEvent)) error {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.SecretTypeCluster})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	w, err := s.kubeclientset.CoreV1().Secrets(s.ns).Watch(listOpts)
	if err != nil {
		return err
	}

	defer w.Stop()
	done := make(chan bool)
	go func() {
		for next := range w.ResultChan() {
			secret := next.Object.(*apiv1.Secret)
			cluster := SecretToCluster(secret)
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

func (s *db) getClusterSecret(server string) (*apiv1.Secret, error) {
	secName, err := serverToSecretName(server)
	if err != nil {
		return nil, err
	}
	clusterSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(secName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "cluster %q not found", server)
		}
		return nil, err
	}
	return clusterSecret, nil
}

// GetCluster returns a cluster from a query
func (s *db) GetCluster(ctx context.Context, server string) (*appv1.Cluster, error) {
	clusterSecret, err := s.getClusterSecret(server)
	if err != nil {
		if grpc.Code(err) == codes.NotFound && server == common.KubernetesInternalAPIServerAddr {
			return &localCluster, nil
		} else {
			return nil, err
		}
	}
	return SecretToCluster(clusterSecret), nil
}

// UpdateCluster updates a cluster
func (s *db) UpdateCluster(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	clusterSecret, err := s.getClusterSecret(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret.Data = clusterToData(c)
	clusterSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(clusterSecret)
	if err != nil {
		return nil, err
	}
	return SecretToCluster(clusterSecret), nil
}

// Delete deletes a cluster by name
func (s *db) DeleteCluster(ctx context.Context, name string) error {
	secName, err := serverToSecretName(name)
	if err != nil {
		return err
	}
	return s.kubeclientset.CoreV1().Secrets(s.ns).Delete(secName, &metav1.DeleteOptions{})
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

// SecretToCluster converts a secret into a repository object
func SecretToCluster(s *apiv1.Secret) *appv1.Cluster {
	var config appv1.ClusterConfig
	err := json.Unmarshal(s.Data["config"], &config)
	if err != nil {
		panic(err)
	}
	cluster := appv1.Cluster{
		Server:          string(s.Data["server"]),
		Name:            string(s.Data["name"]),
		Config:          config,
		ConnectionState: ConnectionStateFromAnnotations(s.Annotations),
	}
	return &cluster
}
