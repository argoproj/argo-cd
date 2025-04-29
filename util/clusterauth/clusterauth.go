package clusterauth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v3/common"
)

// ArgoCDManagerServiceAccount is the name of the service account for managing a cluster
const (
	ArgoCDManagerServiceAccount     = "argocd-manager"
	ArgoCDManagerClusterRole        = "argocd-manager-role"
	ArgoCDManagerClusterRoleBinding = "argocd-manager-role-binding"
	SATokenSecretSuffix             = "-long-lived-token"
)

// ArgoCDManagerPolicyRules are the policies to give argocd-manager
var ArgoCDManagerClusterPolicyRules = []rbacv1.PolicyRule{
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

// ArgoCDManagerNamespacePolicyRules are the namespace level policies to give argocd-manager
var ArgoCDManagerNamespacePolicyRules = []rbacv1.PolicyRule{
	{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	},
}

// CreateServiceAccount creates a service account in a given namespace
func CreateServiceAccount(
	ctx context.Context,
	clientset kubernetes.Interface,
	serviceAccountName string,
	namespace string,
) error {
	serviceAccount := corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: namespace,
		},
	}
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(ctx, &serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create service account %q in namespace %q: %w", serviceAccountName, namespace, err)
		}
		log.Infof("ServiceAccount %q already exists in namespace %q", serviceAccountName, namespace)
		return nil
	}
	log.Infof("ServiceAccount %q created in namespace %q", serviceAccountName, namespace)
	return nil
}

func upsert(kind string, name string, create func() (any, error), update func() (any, error)) error {
	_, err := create()
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %s %q: %w", kind, name, err)
		}
		_, err = update()
		if err != nil {
			return fmt.Errorf("failed to update %s %q: %w", kind, name, err)
		}
		log.Infof("%s %q updated", kind, name)
	} else {
		log.Infof("%s %q created", kind, name)
	}
	return nil
}

func upsertClusterRole(ctx context.Context, clientset kubernetes.Interface, name string, rules []rbacv1.PolicyRule) error {
	clusterRole := rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
	return upsert("ClusterRole", name, func() (any, error) {
		return clientset.RbacV1().ClusterRoles().Create(ctx, &clusterRole, metav1.CreateOptions{})
	}, func() (any, error) {
		return clientset.RbacV1().ClusterRoles().Update(ctx, &clusterRole, metav1.UpdateOptions{})
	})
}

func upsertRole(ctx context.Context, clientset kubernetes.Interface, name string, namespace string, rules []rbacv1.PolicyRule) error {
	role := rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Rules: rules,
	}
	return upsert("Role", fmt.Sprintf("%s/%s", namespace, name), func() (any, error) {
		return clientset.RbacV1().Roles(namespace).Create(ctx, &role, metav1.CreateOptions{})
	}, func() (any, error) {
		return clientset.RbacV1().Roles(namespace).Update(ctx, &role, metav1.UpdateOptions{})
	})
}

func upsertClusterRoleBinding(ctx context.Context, clientset kubernetes.Interface, name string, clusterRoleName string, subject rbacv1.Subject) error {
	roleBinding := rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{subject},
	}
	return upsert("ClusterRoleBinding", name, func() (any, error) {
		return clientset.RbacV1().ClusterRoleBindings().Create(ctx, &roleBinding, metav1.CreateOptions{})
	}, func() (any, error) {
		return clientset.RbacV1().ClusterRoleBindings().Update(ctx, &roleBinding, metav1.UpdateOptions{})
	})
}

func upsertRoleBinding(ctx context.Context, clientset kubernetes.Interface, name string, roleName string, namespace string, subject rbacv1.Subject) error {
	roleBinding := rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
		Subjects: []rbacv1.Subject{subject},
	}
	return upsert("RoleBinding", fmt.Sprintf("%s/%s", namespace, name), func() (any, error) {
		return clientset.RbacV1().RoleBindings(namespace).Create(ctx, &roleBinding, metav1.CreateOptions{})
	}, func() (any, error) {
		return clientset.RbacV1().RoleBindings(namespace).Update(ctx, &roleBinding, metav1.UpdateOptions{})
	})
}

// InstallClusterManagerRBAC installs RBAC resources for a cluster manager to operate a cluster. Returns a token
func InstallClusterManagerRBAC(ctx context.Context, clientset kubernetes.Interface, ns string, namespaces []string, bearerTokenTimeout time.Duration) (string, error) {
	err := CreateServiceAccount(ctx, clientset, ArgoCDManagerServiceAccount, ns)
	if err != nil {
		return "", err
	}

	if len(namespaces) == 0 {
		err = upsertClusterRole(ctx, clientset, ArgoCDManagerClusterRole, ArgoCDManagerClusterPolicyRules)
		if err != nil {
			return "", err
		}

		err = upsertClusterRoleBinding(ctx, clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      ArgoCDManagerServiceAccount,
			Namespace: ns,
		})
		if err != nil {
			return "", err
		}
	} else {
		for _, namespace := range namespaces {
			err = upsertRole(ctx, clientset, ArgoCDManagerClusterRole, namespace, ArgoCDManagerNamespacePolicyRules)
			if err != nil {
				return "", err
			}

			err = upsertRoleBinding(ctx, clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, namespace, rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      ArgoCDManagerServiceAccount,
				Namespace: ns,
			})
			if err != nil {
				return "", err
			}
		}
	}

	return GetServiceAccountBearerToken(ctx, clientset, ns, ArgoCDManagerServiceAccount, bearerTokenTimeout)
}

// GetServiceAccountBearerToken determines if a ServiceAccount has a
// bearer token secret to use or if a secret should be created. It then
// waits for the secret to have a bearer token if a secret needs to
// be created and returns the token in encoded base64.
func GetServiceAccountBearerToken(ctx context.Context, clientset kubernetes.Interface, ns string, sa string, timeout time.Duration) (string, error) {
	secretName, err := getOrCreateServiceAccountTokenSecret(ctx, clientset, sa, ns)
	if err != nil {
		return "", err
	}

	var secret *corev1.Secret
	err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, common.ClusterAuthRequestTimeout)
		defer cancel()
		secret, err = clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get secret %q for serviceaccount %q: %w", secretName, sa, err)
		}

		_, ok := secret.Data[corev1.ServiceAccountTokenKey]
		if !ok {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to get token for serviceaccount %q: %w", sa, err)
	}

	return string(secret.Data[corev1.ServiceAccountTokenKey]), nil
}

// getOrCreateServiceAccountTokenSecret will create a
// kubernetes.io/service-account-token secret associated with a
// ServiceAccount named '<service account name>-long-lived-token', or
// use the existing one with that name.
// This was added to help add k8s v1.24+ clusters.
func getOrCreateServiceAccountTokenSecret(ctx context.Context, clientset kubernetes.Interface, serviceaccount, namespace string) (string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceaccount + SATokenSecretSuffix,
			Namespace: namespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: serviceaccount,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	ctx, cancel := context.WithTimeout(ctx, common.ClusterAuthRequestTimeout)
	defer cancel()
	_, err := clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})

	switch {
	case apierrors.IsAlreadyExists(err):
		log.Infof("Using existing bearer token secret %q for ServiceAccount %q", secret.Name, serviceaccount)
	case err != nil:
		return "", fmt.Errorf("failed to create secret %q for serviceaccount %q: %w", secret.Name, serviceaccount, err)
	default:
		log.Infof("Created bearer token secret %q for ServiceAccount %q", secret.Name, serviceaccount)
	}

	return secret.Name, nil
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func UninstallClusterManagerRBAC(ctx context.Context, clientset kubernetes.Interface) error {
	return UninstallRBAC(ctx, clientset, "kube-system", ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources  for a binding, role, and service account
func UninstallRBAC(ctx context.Context, clientset kubernetes.Interface, namespace, bindingName, roleName, serviceAccount string) error {
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(ctx, bindingName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClusterRoleBinding: %w", err)
		}
		log.Infof("ClusterRoleBinding %q not found", bindingName)
	} else {
		log.Infof("ClusterRoleBinding %q deleted", bindingName)
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(ctx, roleName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ClusterRole: %w", err)
		}
		log.Infof("ClusterRole %q not found", roleName)
	} else {
		log.Infof("ClusterRole %q deleted", roleName)
	}

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, serviceAccount, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ServiceAccount: %w", err)
		}
		log.Infof("ServiceAccount %q in namespace %q not found", serviceAccount, namespace)
	} else {
		log.Infof("ServiceAccount %q deleted", serviceAccount)
	}
	return nil
}

type ServiceAccountClaims struct {
	Namespace          string `json:"kubernetes.io/serviceaccount/namespace"`
	SecretName         string `json:"kubernetes.io/serviceaccount/secret.name"`
	ServiceAccountName string `json:"kubernetes.io/serviceaccount/service-account.name"`
	ServiceAccountUID  string `json:"kubernetes.io/serviceaccount/service-account.uid"`
	jwt.RegisteredClaims
}

// ParseServiceAccountToken parses a Kubernetes service account token
func ParseServiceAccountToken(token string) (*ServiceAccountClaims, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims ServiceAccountClaims
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	return &claims, nil
}

// GenerateNewClusterManagerSecret creates a new secret derived with same metadata as existing one
// and waits until the secret is populated with a bearer token
func GenerateNewClusterManagerSecret(ctx context.Context, clientset kubernetes.Interface, claims *ServiceAccountClaims) (*corev1.Secret, error) {
	secretsClient := clientset.CoreV1().Secrets(claims.Namespace)
	existingSecret, err := secretsClient.Get(ctx, claims.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var newSecret corev1.Secret
	secretNameSplit := strings.Split(claims.SecretName, "-")
	if len(secretNameSplit) > 0 {
		secretNameSplit = secretNameSplit[:len(secretNameSplit)-1]
	}
	newSecret.Type = corev1.SecretTypeServiceAccountToken
	newSecret.GenerateName = strings.Join(secretNameSplit, "-") + "-"
	newSecret.Annotations = existingSecret.Annotations
	// We will create an empty secret and let kubernetes populate the data
	newSecret.Data = nil

	created, err := secretsClient.Create(ctx, &newSecret, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 30*time.Second, false, func(ctx context.Context) (bool, error) {
		created, err = secretsClient.Get(ctx, created.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if len(created.Data) == 0 {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("timed out waiting for secret to generate new token: %w", err)
	}
	return created, nil
}

// RotateServiceAccountSecrets rotates the entries in the service accounts secrets list
func RotateServiceAccountSecrets(ctx context.Context, clientset kubernetes.Interface, claims *ServiceAccountClaims, newSecret *corev1.Secret) error {
	// 1. update service account secrets list with new secret name while also removing the old name
	saClient := clientset.CoreV1().ServiceAccounts(claims.Namespace)
	sa, err := saClient.Get(ctx, claims.ServiceAccountName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	var newSecretsList []corev1.ObjectReference
	alreadyPresent := false
	for _, objRef := range sa.Secrets {
		if objRef.Name == claims.SecretName {
			continue
		}
		if objRef.Name == newSecret.Name {
			alreadyPresent = true
		}
		newSecretsList = append(newSecretsList, objRef)
	}
	if !alreadyPresent {
		sa.Secrets = append(newSecretsList, corev1.ObjectReference{Name: newSecret.Name})
	}
	_, err = saClient.Update(ctx, sa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// 2. delete existing secret object
	secretsClient := clientset.CoreV1().Secrets(claims.Namespace)
	err = secretsClient.Delete(ctx, claims.SecretName, metav1.DeleteOptions{})
	if !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
