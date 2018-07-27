package common

import (
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/errors"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// CreateServiceAccount creates a service account
func CreateServiceAccount(
	clientset kubernetes.Interface,
	serviceAccountName string,
	namespace string,
) {
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
			log.Fatalf("Failed to create service account '%s': %v\n", serviceAccountName, err)
		}
		fmt.Printf("ServiceAccount '%s' already exists\n", serviceAccountName)
		return
	}
	fmt.Printf("ServiceAccount '%s' created\n", serviceAccountName)
}

// CreateClusterRole creates a cluster role
func CreateClusterRole(
	clientset kubernetes.Interface,
	clusterRoleName string,
	rules []rbacv1.PolicyRule,
) {
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
			log.Fatalf("Failed to create ClusterRole '%s': %v\n", clusterRoleName, err)
		}
		_, err = crclient.Update(&clusterRole)
		if err != nil {
			log.Fatalf("Failed to update ClusterRole '%s': %v\n", clusterRoleName, err)
		}
		fmt.Printf("ClusterRole '%s' updated\n", clusterRoleName)
	} else {
		fmt.Printf("ClusterRole '%s' created\n", clusterRoleName)
	}
}

// CreateClusterRoleBinding create a ClusterRoleBinding
func CreateClusterRoleBinding(
	clientset kubernetes.Interface,
	clusterBindingRoleName,
	serviceAccountName,
	clusterRoleName string,
	namespace string,
) {
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
			log.Fatalf("Failed to create ClusterRoleBinding %s: %v\n", clusterBindingRoleName, err)
		}
		fmt.Printf("ClusterRoleBinding '%s' already exists\n", clusterBindingRoleName)
		return
	}
	fmt.Printf("ClusterRoleBinding '%s' created, bound '%s' to '%s'\n", clusterBindingRoleName, serviceAccountName, clusterRoleName)
}

// InstallClusterManagerRBAC installs RBAC resources for a cluster manager to operate a cluster. Returns a token
func InstallClusterManagerRBAC(conf *rest.Config) string {
	const ns = "kube-system"
	clientset, err := kubernetes.NewForConfig(conf)
	errors.CheckError(err)
	CreateServiceAccount(clientset, ArgoCDManagerServiceAccount, ns)
	CreateClusterRole(clientset, ArgoCDManagerClusterRole, ArgoCDManagerPolicyRules)
	CreateClusterRoleBinding(clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerServiceAccount, ArgoCDManagerClusterRole, ns)

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
		log.Fatalf("Failed to wait for service account secret: %v", err)
	}
	secret, err := clientset.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to retrieve secret '%s': %v", secretName, err)
	}
	token, ok := secret.Data["token"]
	if !ok {
		log.Fatalf("Secret '%s' for service account '%s' did not have a token", secretName, serviceAccount)
	}
	return string(token)
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func UninstallClusterManagerRBAC(conf *rest.Config) {
	clientset, err := kubernetes.NewForConfig(conf)
	errors.CheckError(err)
	UninstallRBAC(clientset, "kube-system", ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources  for a binding, role, and service account
func UninstallRBAC(clientset kubernetes.Interface, namespace, bindingName, roleName, serviceAccount string) {
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(bindingName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalf("Failed to delete ClusterRoleBinding: %v\n", err)
		}
		fmt.Printf("ClusterRoleBinding '%s' not found\n", bindingName)
	} else {
		fmt.Printf("ClusterRoleBinding '%s' deleted\n", bindingName)
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(roleName, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalf("Failed to delete ClusterRole: %v\n", err)
		}
		fmt.Printf("ClusterRole '%s' not found\n", roleName)
	} else {
		fmt.Printf("ClusterRole '%s' deleted\n", roleName)
	}

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(serviceAccount, &metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			log.Fatalf("Failed to delete ServiceAccount: %v\n", err)
		}
		fmt.Printf("ServiceAccount '%s' in namespace '%s' not found\n", serviceAccount, namespace)
	} else {
		fmt.Printf("ServiceAccount '%s' deleted\n", serviceAccount)
	}
}
