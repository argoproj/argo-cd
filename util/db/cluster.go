package db

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
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
	for i, clusterSecret := range clusterSecrets.Items {
		clusterList.Items[i] = *SecretToCluster(&clusterSecret)
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
	clusterSecret.Annotations = AnnotationsFromConnectionState(&c.ConnectionState)
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
		return nil, err
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

// CreateServiceAccount creates a service account
func (s *db) CreateServiceAccount(serviceAccountName string, namespace string) error {
	serviceAccount := apiv1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	_, err := s.kubeclientset.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to create service account %q: %v", serviceAccountName, err)
		}
		return status.Errorf(codes.AlreadyExists, "ServiceAccount %q already exists", serviceAccountName)
	}
	log.Infof("ServiceAccount %q created", serviceAccountName)
	return nil
}

// DeleteServiceAccount deletes a service account
func (s *db) DeleteServiceAccount(serviceAccountName string, namespace string) error {
	err := s.kubeclientset.CoreV1().ServiceAccounts(namespace).Delete(serviceAccountName, &metav1.DeleteOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to delete service account %q: %v", serviceAccountName, err)
		}
		return status.Errorf(codes.NotFound, "ServiceAccount %q not found", serviceAccountName)
	}
	log.Infof("ServiceAccount %q deleted", serviceAccountName)
	return nil
}

// CreateClusterRole creates a cluster role
func (s *db) CreateClusterRole(clusterRoleName string, rules []rbacv1.PolicyRule) error {
	clusterRole := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		Rules: rules,
	}
	crclient := s.kubeclientset.RbacV1().ClusterRoles()
	_, err := crclient.Create(&clusterRole)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to create ClusterRole %q: %v", clusterRoleName, err)
		}
		_, err = crclient.Update(&clusterRole)
		if err != nil {
			return status.Errorf(codes.FailedPrecondition, "Failed to update ClusterRole %q: %v", clusterRoleName, err)
		}
		log.Infof("ClusterRole %q updated", clusterRoleName)
	} else {
		log.Infof("ClusterRole %q created", clusterRoleName)
	}
	return nil
}

// CreateClusterRoleBinding create a ClusterRoleBinding
func (s *db) CreateClusterRoleBinding(clusterBindingRoleName, serviceAccountName, clusterRoleName string, namespace string) error {
	roleBinding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterBindingRoleName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
	}
	_, err := s.kubeclientset.RbacV1().ClusterRoleBindings().Create(&roleBinding)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to create ClusterRoleBinding %q: %v", clusterBindingRoleName, err)
		}
		return status.Errorf(codes.AlreadyExists, "ClusterRoleBinding %q already exists", clusterBindingRoleName)
	}
	log.Infof("ClusterRoleBinding %q created, bound %q to %q", clusterBindingRoleName, serviceAccountName, clusterRoleName)
	return nil
}

// InstallClusterManagerRBAC installs RBAC resources for a cluster manager to operate a cluster. Returns a token
func (s *db) InstallClusterManagerRBAC(ctx context.Context) (string, error) {
	const ns = "kube-system"
	if err := s.CreateServiceAccount(common.ArgoCDManagerServiceAccount, ns); err != nil {
		return "", err
	}
	if err := s.CreateClusterRole(common.ArgoCDManagerClusterRole, common.ArgoCDManagerPolicyRules); err != nil {
		return "", err
	}
	if err := s.CreateClusterRoleBinding(common.ArgoCDManagerClusterRoleBinding, common.ArgoCDManagerServiceAccount, common.ArgoCDManagerClusterRole, ns); err != nil {
		return "", err
	}

	var serviceAccount *apiv1.ServiceAccount
	var secretName string
	err := wait.Poll(500*time.Millisecond, 30*time.Second, func() (bool, error) {
		serviceAccount, err := s.kubeclientset.CoreV1().ServiceAccounts(ns).Get(common.ArgoCDManagerServiceAccount, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(serviceAccount.Secrets) == 0 {
			return false, nil
		}
		secretName = serviceAccount.Secrets[0].Name
		return true, nil
	})
	if err != nil {
		return "", status.Errorf(codes.DeadlineExceeded, "Failed to wait for service account secret: %v", err)
	}
	secret, err := s.kubeclientset.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", status.Errorf(codes.FailedPrecondition, "Failed to retrieve secret %q: %v", secretName, err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", status.Errorf(codes.FailedPrecondition, "Secret %q for service account %q did not have a token", secretName, serviceAccount)
	}
	return string(token), nil
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func (s *db) UninstallClusterManagerRBAC(ctx context.Context) error {
	return s.UninstallRBAC("kube-system", common.ArgoCDManagerClusterRoleBinding, common.ArgoCDManagerClusterRole, common.ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources for a binding, role, and service account
func (s *db) UninstallRBAC(namespace, bindingName, roleName, serviceAccount string) error {
	if err := s.kubeclientset.RbacV1().ClusterRoleBindings().Delete(bindingName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to delete ClusterRoleBinding: %v", err)
		}
		return status.Errorf(codes.NotFound, "ClusterRoleBinding %q not found", bindingName)
	} else {
		log.Infof("ClusterRoleBinding %q deleted", bindingName)
	}

	if err := s.kubeclientset.RbacV1().ClusterRoles().Delete(roleName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to delete ClusterRole: %v", err)
		}
		return status.Errorf(codes.NotFound, "ClusterRole %q not found", roleName)
	} else {
		log.Infof("ClusterRole %q deleted", roleName)
	}

	if err := s.kubeclientset.CoreV1().ServiceAccounts(namespace).Delete(serviceAccount, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return status.Errorf(codes.FailedPrecondition, "Failed to delete ServiceAccount: %v", err)
		}
		return status.Errorf(codes.NotFound, "ServiceAccount %q in namespace %q not found", serviceAccount, namespace)
	} else {
		log.Infof("ServiceAccount %q deleted", serviceAccount)
	}
	return nil
}
