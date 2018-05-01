package cluster

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/kube"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// Server provides a Cluster service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	appclientset  appclientset.Interface
}

// NewServer returns a new instance of the Cluster service
func NewServer(namespace string, kubeclientset kubernetes.Interface, appclientset appclientset.Interface) *Server {
	return &Server{
		ns:            namespace,
		appclientset:  appclientset,
		kubeclientset: kubeclientset,
	}
}

// ListPods returns application related pods in a cluster
func (s *Server) ListPods(ctx context.Context, q *ClusterQuery) (*apiv1.PodList, error) {
	// TODO: filter by the app label
	return s.kubeclientset.CoreV1().Pods(s.ns).List(metav1.ListOptions{})
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
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
	for i, clusterSecret := range clusterSecrets.Items {
		clusterList.Items[i] = *secretToCluster(&clusterSecret, true)
	}
	return &clusterList, nil
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	err := kube.TestConfig(c.RESTConfig())
	if err != nil {
		return nil, err
	}
	secName := serverToSecretName(c.Server)
	clusterSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.SecretTypeCluster,
			},
		},
	}
	clusterSecret.StringData = clusterToStringData(c)
	clusterSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Create(clusterSecret)
	if err != nil {
		if apierr.IsAlreadyExists(err) {
			return nil, status.Errorf(codes.AlreadyExists, "cluster '%s' already exists", c.Server)
		}
		return nil, err
	}
	return secretToCluster(clusterSecret, true), nil
}

// ClusterEvent contains information about cluster event
type ClusterEvent struct {
	Type    watch.EventType
	Cluster *appv1.Cluster
}

// WatchClusters allow watching for cluster events
func (s *Server) WatchClusters(ctx context.Context, callback func(*ClusterEvent)) error {
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
	go func() {
		<-ctx.Done()
		w.Stop()
	}()
	for next := range w.ResultChan() {
		secret := next.Object.(*apiv1.Secret)
		cluster := secretToCluster(secret, false)
		callback(&ClusterEvent{
			Type:    next.Type,
			Cluster: cluster,
		})
	}
	return nil
}

func (s *Server) getClusterSecret(server string) (*apiv1.Secret, error) {
	secName := serverToSecretName(server)
	clusterSecret, err := s.kubeclientset.CoreV1().Secrets(s.ns).Get(secName, metav1.GetOptions{})
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "cluster '%s' not found", server)
		}
		return nil, err
	}
	return clusterSecret, nil
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	clusterSecret, err := s.getClusterSecret(q.Server)
	if err != nil {
		return nil, err
	}
	return secretToCluster(clusterSecret, true), nil
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, c *appv1.Cluster) (*appv1.Cluster, error) {
	err := kube.TestConfig(c.RESTConfig())
	if err != nil {
		return nil, err
	}
	clusterSecret, err := s.getClusterSecret(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret.StringData = clusterToStringData(c)
	clusterSecret, err = s.kubeclientset.CoreV1().Secrets(s.ns).Update(clusterSecret)
	if err != nil {
		return nil, err
	}
	return secretToCluster(clusterSecret, true), nil
}

// UpdateREST updates a cluster (special handler intended to be used only by the gRPC gateway)
func (s *Server) UpdateREST(ctx context.Context, r *ClusterUpdateRequest) (*appv1.Cluster, error) {
	return s.Update(ctx, r.Cluster)
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	secName := serverToSecretName(q.Server)
	err := s.kubeclientset.CoreV1().Secrets(s.ns).Delete(secName, &metav1.DeleteOptions{})
	return &ClusterResponse{}, err
}

// serverToSecretName hashes server address to the secret name using a formula.
// Part of the server address is incorporated for debugging purposes
func serverToSecretName(server string) string {
	serverURL, err := url.ParseRequestURI(server)
	if err != nil {
		panic(err)
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(server))
	host := strings.ToLower(strings.Split(serverURL.Host, ":")[0])
	return fmt.Sprintf("cluster-%s-%v", host, h.Sum32())
}

// clusterToStringData converts a cluster object to string data for serialization to a secret
func clusterToStringData(c *appv1.Cluster) map[string]string {
	stringData := make(map[string]string)
	stringData["server"] = c.Server
	if c.Name == "" {
		stringData["name"] = c.Server
	} else {
		stringData["name"] = c.Name
	}
	configBytes, err := json.Marshal(c.Config)
	if err != nil {
		panic(err)
	}
	stringData["config"] = string(configBytes)
	return stringData
}

// secretToCluster converts a secret into a repository object, optionally redacting sensitive information
func secretToCluster(s *apiv1.Secret, redact bool) *appv1.Cluster {
	var config appv1.ClusterConfig
	err := json.Unmarshal(s.Data["config"], &config)
	if err != nil {
		panic(err)
	}
	cluster := appv1.Cluster{
		Server: string(s.Data["server"]),
		Name:   string(s.Data["name"]),
	}
	if !redact {
		cluster.Config = config
	}
	return &cluster
}
