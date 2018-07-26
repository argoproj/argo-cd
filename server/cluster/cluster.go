package cluster

import (
	"reflect"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/rbac"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Server provides a Cluster service
type Server struct {
	ns            string
	kubeclientset kubernetes.Interface
	db            db.ArgoDB
	enf           *rbac.Enforcer
}

// NewServer returns a new instance of the Cluster service
func NewServer(db db.ArgoDB, enf *rbac.Enforcer) *Server {
	return &Server{
		db:  db,
		enf: enf,
	}
}

// List returns list of clusters
func (s *Server) List(ctx context.Context, q *ClusterQuery) (*appv1.ClusterList, error) {
	clusterList, err := s.db.ListClusters(ctx)
	if clusterList != nil {
		newItems := make([]appv1.Cluster, 0)
		for _, clust := range clusterList.Items {
			if s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", clust.Server) {
				newItems = append(newItems, *redact(&clust))
			}
		}
		clusterList.Items = newItems
	}
	return clusterList, err
}

// Create creates a cluster
func (s *Server) Create(ctx context.Context, q *ClusterCreateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "create", q.Cluster.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	c := q.Cluster
	err := kube.TestConfig(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}

	if q.Kubeconfig != "" {
		// Temporarily install RBAC resources for managing the cluster
		defer func() {
			err := s.UninstallClusterManagerRBAC(ctx)
			if err != nil {
				log.Errorf("Error occurred uninstalling cluster manager: %s", err)
			}
		}()
		bearerToken, err := s.InstallClusterManagerRBAC(ctx)
		if err != nil {
			return nil, err
		}
		c.Config.BearerToken = bearerToken
	}

	c.ConnectionState = appv1.ConnectionState{Status: appv1.ConnectionStatusSuccessful}
	clust, err := s.db.CreateCluster(ctx, c)
	if status.Convert(err).Code() == codes.AlreadyExists {
		// act idempotent if existing spec matches new spec
		existing, getErr := s.db.GetCluster(ctx, c.Server)
		if getErr != nil {
			return nil, status.Errorf(codes.Internal, "unable to check existing cluster details: %v", getErr)
		}

		// cluster ConnectionState may differ, so make consistent before testing
		existing.ConnectionState = c.ConnectionState
		if reflect.DeepEqual(existing, c) {
			clust, err = existing, nil
		} else if q.Upsert {
			return s.Update(ctx, &ClusterUpdateRequest{Cluster: c})
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "existing cluster spec is different; use upsert flag to force update")
		}
	}
	return redact(clust), err
}

// Get returns a cluster from a query
func (s *Server) Get(ctx context.Context, q *ClusterQuery) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "get", q.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	clust, err := s.db.GetCluster(ctx, q.Server)
	return redact(clust), err
}

// Update updates a cluster
func (s *Server) Update(ctx context.Context, q *ClusterUpdateRequest) (*appv1.Cluster, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "update", q.Cluster.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	err := kube.TestConfig(q.Cluster.RESTConfig())
	if err != nil {
		return nil, err
	}
	clust, err := s.db.UpdateCluster(ctx, q.Cluster)
	return redact(clust), err
}

// Delete deletes a cluster by name
func (s *Server) Delete(ctx context.Context, q *ClusterQuery) (*ClusterResponse, error) {
	if !s.enf.EnforceClaims(ctx.Value("claims"), "clusters", "delete", q.Server) {
		return nil, grpc.ErrPermissionDenied
	}
	err := s.db.DeleteCluster(ctx, q.Server)
	return &ClusterResponse{}, err
}

func redact(clust *appv1.Cluster) *appv1.Cluster {
	if clust == nil {
		return nil
	}
	clust.Config.Password = ""
	clust.Config.BearerToken = ""
	clust.Config.TLSClientConfig.KeyData = nil
	return clust
}

// CreateServiceAccount creates a service account
func (s *Server) CreateServiceAccount(serviceAccountName string, namespace string) error {
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
func (s *Server) DeleteServiceAccount(serviceAccountName string, namespace string) error {
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
func (s *Server) CreateClusterRole(clusterRoleName string, rules []rbacv1.PolicyRule) error {
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
func (s *Server) CreateClusterRoleBinding(clusterBindingRoleName, serviceAccountName, clusterRoleName string, namespace string) error {
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
func (s *Server) InstallClusterManagerRBAC(ctx context.Context) (string, error) {
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
		return "", status.Errorf(codes.InvalidArgument, "Secret %q for service account %q did not have a token", secretName, serviceAccount)
	}
	return string(token), nil
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func (s *Server) UninstallClusterManagerRBAC(ctx context.Context) error {
	return s.UninstallRBAC("kube-system", common.ArgoCDManagerClusterRoleBinding, common.ArgoCDManagerClusterRole, common.ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources for a binding, role, and service account
func (s *Server) UninstallRBAC(namespace, bindingName, roleName, serviceAccount string) error {
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
