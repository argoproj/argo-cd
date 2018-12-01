package common

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDManagerServiceAccount is the name of the service account for managing a cluster
const (
	ArgoCDManagerServiceAccount     = "argocd-manager"
	ArgoCDManagerClusterRole        = "argocd-manager-role"
	ArgoCDManagerClusterRoleBinding = "argocd-manager-role-binding"
)

// ArgoCDManagerPolicyRules are the policies to give argocd-manager
var ArgoCDManagerPolicyRules = []rbacv1.PolicyRule{
	{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	},
	{
		NonResourceURLs: []string{"*"},
		Verbs:           []string{"*"},
	},
}

// CreateServiceAccount creates a service account
func CreateServiceAccount(
	clientset kubernetes.Interface,
	serviceAccountName string,
	namespace string,
) error {
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
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(&serviceAccount)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return fmt.Errorf("Failed to create service account %q: %v", serviceAccountName, err)
		}
		log.Infof("ServiceAccount %q already exists", serviceAccountName)
		return nil
	}
	log.Infof("ServiceAccount %q created", serviceAccountName)
	return nil
}

// CreateClusterRole creates a cluster role
func CreateClusterRole(
	clientset kubernetes.Interface,
	clusterRoleName string,
	rules []rbacv1.PolicyRule,
) error {
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
	crclient := clientset.RbacV1().ClusterRoles()
	_, err := crclient.Create(&clusterRole)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return fmt.Errorf("Failed to create ClusterRole %q: %v", clusterRoleName, err)
		}
		_, err = crclient.Update(&clusterRole)
		if err != nil {
			return fmt.Errorf("Failed to update ClusterRole %q: %v", clusterRoleName, err)
		}
		log.Infof("ClusterRole %q updated", clusterRoleName)
	} else {
		log.Infof("ClusterRole %q created", clusterRoleName)
	}
	return nil
}

// CreateClusterRoleBinding create a ClusterRoleBinding
func CreateClusterRoleBinding(
	clientset kubernetes.Interface,
	clusterBindingRoleName,
	serviceAccountName,
	clusterRoleName string,
	namespace string,
) error {
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
	_, err := clientset.RbacV1().ClusterRoleBindings().Create(&roleBinding)
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return fmt.Errorf("Failed to create ClusterRoleBinding %s: %v", clusterBindingRoleName, err)
		}
		log.Infof("ClusterRoleBinding %q already exists", clusterBindingRoleName)
		return nil
	}
	log.Infof("ClusterRoleBinding %q created, bound %q to %q", clusterBindingRoleName, serviceAccountName, clusterRoleName)
	return nil
}

// InstallClusterManagerRBAC installs RBAC resources for a cluster manager to operate a cluster. Returns a token
func InstallClusterManagerRBAC(clientset kubernetes.Interface) (string, error) {
	const ns = "kube-system"
	var err error

	err = CreateServiceAccount(clientset, ArgoCDManagerServiceAccount, ns)
	if err != nil {
		return "", err
	}

	err = CreateClusterRole(clientset, ArgoCDManagerClusterRole, ArgoCDManagerPolicyRules)
	if err != nil {
		return "", err
	}

	err = CreateClusterRoleBinding(clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerServiceAccount, ArgoCDManagerClusterRole, ns)
	if err != nil {
		return "", err
	}

	var serviceAccount *apiv1.ServiceAccount
	var secretName string
	err = wait.Poll(500*time.Millisecond, 30*time.Second, func() (bool, error) {
		serviceAccount, err = clientset.CoreV1().ServiceAccounts(ns).Get(ArgoCDManagerServiceAccount, metav1.GetOptions{})
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
		return "", fmt.Errorf("Failed to wait for service account secret: %v", err)
	}
	secret, err := clientset.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve secret %q: %v", secretName, err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("Secret %q for service account %q did not have a token", secretName, serviceAccount)
	}
	return string(token), nil
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func UninstallClusterManagerRBAC(clientset kubernetes.Interface) error {
	return UninstallRBAC(clientset, "kube-system", ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources  for a binding, role, and service account
func UninstallRBAC(clientset kubernetes.Interface, namespace, bindingName, roleName, serviceAccount string) error {
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(bindingName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ClusterRoleBinding: %v", err)
		}
		log.Infof("ClusterRoleBinding %q not found", bindingName)
	} else {
		log.Infof("ClusterRoleBinding %q deleted", bindingName)
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(roleName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ClusterRole: %v", err)
		}
		log.Infof("ClusterRole %q not found", roleName)
	} else {
		log.Infof("ClusterRole %q deleted", roleName)
	}

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(serviceAccount, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ServiceAccount: %v", err)
		}
		log.Infof("ServiceAccount %q in namespace %q not found", serviceAccount, namespace)
	} else {
		log.Infof("ServiceAccount %q deleted", serviceAccount)
	}
	return nil
}
