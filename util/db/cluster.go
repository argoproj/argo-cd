package db

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"reflect"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/watch"
	informerv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

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
	if c.ConnectionState.ModifiedAt != nil {
		clusterSecret.Annotations[common.AnnotationKeyModifiedAt] = c.ConnectionState.ModifiedAt.Format(time.RFC3339)
	}
	if c.ConnectionState.Message != "" {
		clusterSecret.Annotations[common.AnnotationKeyMessage] = c.ConnectionState.Message
	}
	if c.ConnectionState.Status != "" {
		clusterSecret.Annotations[common.AnnotationKeyStatus] = c.ConnectionState.Status
	}
	if c.ServerVersion != "" {
		clusterSecret.Annotations[common.AnnotationKeyServerVersion] = c.ServerVersion
	}
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

func (db *db) WatchClusters(ctx context.Context,
	handleAddEvent func(cluster *appv1.Cluster),
	handleModEvent func(oldCluster *appv1.Cluster, newCluster *appv1.Cluster),
	handleDeleteEvent func(clusterServer string)) error {
	localCls, err := db.GetCluster(ctx, common.KubernetesInternalAPIServerAddr)
	if err != nil {
		return err
	}
	handleAddEvent(localCls)

	clusterSecretListOptions := func(options *metav1.ListOptions) {
		clusterLabelSelector := fields.ParseSelectorOrDie(common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster)
		options.LabelSelector = clusterLabelSelector.String()
	}
	clusterEventHandler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if secretObj, ok := obj.(*apiv1.Secret); ok {
				cluster := secretToCluster(secretObj)
				if cluster.Server == common.KubernetesInternalAPIServerAddr {
					if reflect.DeepEqual(localCls.Config, cluster.Config) {
						return
					}
					// change local cluster event to modified or deleted, since it cannot be re-added or deleted
					handleModEvent(localCls, cluster)
					localCls = cluster
					return
				}
				handleAddEvent(cluster)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if secretObj, ok := obj.(*apiv1.Secret); ok {
				if string(secretObj.Data["server"]) == common.KubernetesInternalAPIServerAddr {
					// change local cluster event to modified or deleted, since it cannot be re-added or deleted
					handleModEvent(localCls, &localCluster)
					localCls = &localCluster
				} else {
					handleDeleteEvent(string(secretObj.Data["server"]))
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if oldSecretObj, ok := oldObj.(*apiv1.Secret); ok {
				if newSecretObj, ok := newObj.(*apiv1.Secret); ok {
					oldCluster := secretToCluster(oldSecretObj)
					newCluster := secretToCluster(newSecretObj)
					if newCluster.Server == common.KubernetesInternalAPIServerAddr {
						localCls = newCluster
					}
					handleModEvent(oldCluster, newCluster)
				}
			}
		},
	}
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	clusterSecretInformer := informerv1.NewFilteredSecretInformer(db.kubeclientset, db.ns, 3*time.Minute, indexers, clusterSecretListOptions)
	clusterSecretInformer.AddEventHandler(clusterEventHandler)
	log.Info("Starting clusterSecretInformer informers")
	go func() {
		clusterSecretInformer.Run(ctx.Done())
		log.Info("clusterSecretInformer cancelled")
	}()

	<-ctx.Done()
	return err
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
	if c.ConnectionState.ModifiedAt != nil {
		clusterSecret.Annotations[common.AnnotationKeyModifiedAt] = c.ConnectionState.ModifiedAt.Format(time.RFC3339)
	} else {
		delete(clusterSecret.Annotations, common.AnnotationKeyModifiedAt)
	}
	if c.ServerVersion != "" {
		clusterSecret.Annotations[common.AnnotationKeyServerVersion] = c.ServerVersion
	} else {
		delete(clusterSecret.Annotations, common.AnnotationKeyServerVersion)
	}
	if c.ConnectionState.Message != "" {
		clusterSecret.Annotations[common.AnnotationKeyMessage] = c.ConnectionState.Message
	} else {
		delete(clusterSecret.Annotations, common.AnnotationKeyMessage)
	}
	if c.ConnectionState.Status != "" {
		clusterSecret.Annotations[common.AnnotationKeyStatus] = c.ConnectionState.Status
	} else {
		delete(clusterSecret.Annotations, common.AnnotationKeyStatus)
	}
	clusterSecret, err = db.kubeclientset.CoreV1().Secrets(db.ns).Update(clusterSecret)
	if err != nil {
		return nil, err
	}
	return secretToCluster(clusterSecret), db.settingsMgr.ResyncInformers()
}

// Delete deletes a cluster by name
func (db *db) DeleteCluster(ctx context.Context, server string) error {
	secret, err := db.getClusterSecret(server)
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
	if len(c.Namespaces) != 0 {
		data["namespaces"] = []byte(strings.Join(c.Namespaces, ","))
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
	var namespaces []string
	for _, ns := range strings.Split(string(s.Data["namespaces"]), ",") {
		if ns = strings.TrimSpace(ns); ns != "" {
			namespaces = append(namespaces, ns)
		}
	}
	connectionState := appv1.ConnectionState{}
	if v, found := s.Annotations[common.AnnotationKeyModifiedAt]; found {
		time, err := time.Parse(time.RFC3339, v)
		if err != nil {
			log.Warnf("Error while parsing date : %v", err)
		} else {
			connectionState.ModifiedAt = &metav1.Time{Time: time}
		}
	}
	connectionState.Status = s.Annotations[common.AnnotationKeyStatus]
	connectionState.Message = s.Annotations[common.AnnotationKeyMessage]
	cluster := appv1.Cluster{
		Server:          string(s.Data["server"]),
		Name:            string(s.Data["name"]),
		Namespaces:      namespaces,
		Config:          config,
		ServerVersion:   s.Annotations[common.AnnotationKeyServerVersion],
		ConnectionState: connectionState,
	}
	return &cluster
}
