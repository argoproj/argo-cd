package clusterauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/common"
)

// ArgoCDManagerServiceAccount is the name of the service account for managing a cluster
const (
	ArgoCDManagerServiceAccount     = "argocd-manager"
	ArgoCDManagerClusterRole        = "argocd-manager-role"
	ArgoCDManagerClusterRoleBinding = "argocd-manager-role-binding"
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
	_, err := clientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), &serviceAccount, metav1.CreateOptions{})
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return fmt.Errorf("Failed to create service account %q in namespace %q: %w", serviceAccountName, namespace, err)
		}
		log.Infof("ServiceAccount %q already exists in namespace %q", serviceAccountName, namespace)
		return nil
	}
	log.Infof("ServiceAccount %q created in namespace %q", serviceAccountName, namespace)
	return nil
}

func upsert(kind string, name string, create func() (interface{}, error), update func() (interface{}, error)) error {
	_, err := create()
	if err != nil {
		if !apierr.IsAlreadyExists(err) {
			return fmt.Errorf("Failed to create %s %q: %w", kind, name, err)
		}
		_, err = update()
		if err != nil {
			return fmt.Errorf("Failed to update %s %q: %w", kind, name, err)
		}
		log.Infof("%s %q updated", kind, name)
	} else {
		log.Infof("%s %q created", kind, name)
	}
	return nil
}

func upsertClusterRole(clientset kubernetes.Interface, name string, rules []rbacv1.PolicyRule) error {
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
	return upsert("ClusterRole", name, func() (interface{}, error) {
		return clientset.RbacV1().ClusterRoles().Create(context.Background(), &clusterRole, metav1.CreateOptions{})
	}, func() (interface{}, error) {
		return clientset.RbacV1().ClusterRoles().Update(context.Background(), &clusterRole, metav1.UpdateOptions{})
	})
}

func upsertRole(clientset kubernetes.Interface, name string, namespace string, rules []rbacv1.PolicyRule) error {
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
	return upsert("Role", fmt.Sprintf("%s/%s", namespace, name), func() (interface{}, error) {
		return clientset.RbacV1().Roles(namespace).Create(context.Background(), &role, metav1.CreateOptions{})
	}, func() (interface{}, error) {
		return clientset.RbacV1().Roles(namespace).Update(context.Background(), &role, metav1.UpdateOptions{})
	})
}

func upsertClusterRoleBinding(clientset kubernetes.Interface, name string, clusterRoleName string, subject rbacv1.Subject) error {
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
	return upsert("ClusterRoleBinding", name, func() (interface{}, error) {
		return clientset.RbacV1().ClusterRoleBindings().Create(context.Background(), &roleBinding, metav1.CreateOptions{})
	}, func() (interface{}, error) {
		return clientset.RbacV1().ClusterRoleBindings().Update(context.Background(), &roleBinding, metav1.UpdateOptions{})
	})
}

func upsertRoleBinding(clientset kubernetes.Interface, name string, roleName string, namespace string, subject rbacv1.Subject) error {
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
	return upsert("RoleBinding", fmt.Sprintf("%s/%s", namespace, name), func() (interface{}, error) {
		return clientset.RbacV1().RoleBindings(namespace).Create(context.Background(), &roleBinding, metav1.CreateOptions{})
	}, func() (interface{}, error) {
		return clientset.RbacV1().RoleBindings(namespace).Update(context.Background(), &roleBinding, metav1.UpdateOptions{})
	})
}

// InstallClusterManagerRBAC installs RBAC resources for a cluster manager to operate a cluster. Returns a token
func InstallClusterManagerRBAC(clientset kubernetes.Interface, ns string, namespaces []string, bearerTokenTimeout time.Duration) (string, error) {
	err := CreateServiceAccount(clientset, ArgoCDManagerServiceAccount, ns)
	if err != nil {
		return "", err
	}

	if len(namespaces) == 0 {
		err = upsertClusterRole(clientset, ArgoCDManagerClusterRole, ArgoCDManagerClusterPolicyRules)
		if err != nil {
			return "", err
		}

		err = upsertClusterRoleBinding(clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, rbacv1.Subject{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      ArgoCDManagerServiceAccount,
			Namespace: ns,
		})
		if err != nil {
			return "", err
		}
	} else {
		for _, namespace := range namespaces {
			err = upsertRole(clientset, ArgoCDManagerClusterRole, namespace, ArgoCDManagerNamespacePolicyRules)
			if err != nil {
				return "", err
			}

			err = upsertRoleBinding(clientset, ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, namespace, rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      ArgoCDManagerServiceAccount,
				Namespace: ns,
			})
			if err != nil {
				return "", err
			}
		}
	}

	return GetServiceAccountBearerToken(clientset, ns, ArgoCDManagerServiceAccount, bearerTokenTimeout)
}

// GetServiceAccountBearerToken determines if a ServiceAccount has a
// bearer token secret to use or if a secret should be created. It then
// waits for the secret to have a bearer token if a secret needs to
// be created and returns the token in encoded base64.
func GetServiceAccountBearerToken(clientset kubernetes.Interface, ns string, sa string, timeout time.Duration) (string, error) {
	secretName, err := getOrCreateServiceAccountTokenSecret(clientset, sa, ns)
	if err != nil {
		return "", err
	}

	var secret *corev1.Secret
	err = wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, common.ClusterAuthRequestTimeout)
		defer cancel()
		secret, err = clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get secret %q for serviceaccount %q: %w", secretName, sa, err)
		}

		_, ok := secret.Data["token"]
		if !ok {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to get token for serviceaccount %q: %w", sa, err)
	}

	return string(secret.Data["token"]), nil
}

// getOrCreateServiceAccountTokenSecret will check if a ServiceAccount
// already has a kubernetes.io/service-account-token secret associated
// with it or creates one if the ServiceAccount doesn't have one. This
// was added to help add k8s v1.24+ clusters.
func getOrCreateServiceAccountTokenSecret(clientset kubernetes.Interface, sa, ns string) (string, error) {
	// Wait for sa to have secret, but don't wait too
	// long for 1.24+ clusters
	var serviceAccount *corev1.ServiceAccount
	err := wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		ctx, cancel := context.WithTimeout(ctx, common.ClusterAuthRequestTimeout)
		defer cancel()
		var getErr error
		serviceAccount, getErr = clientset.CoreV1().ServiceAccounts(ns).Get(ctx, sa, metav1.GetOptions{})
		if getErr != nil {
			return false, fmt.Errorf("failed to get serviceaccount %q: %w", sa, getErr)
		}
		return true, nil
	})
	if err != nil && !wait.Interrupted(err) {
		return "", fmt.Errorf("failed to get serviceaccount token secret: %w", err)
	}
	if serviceAccount == nil {
		log.Errorf("Unexpected nil serviceaccount '%s/%s' with no error returned", ns, sa)
		return "", fmt.Errorf("failed to create serviceaccount token secret: nil serviceaccount returned for '%s/%s' with no error", ns, sa)
	}

	outerCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, s := range serviceAccount.Secrets {
		innerCtx, cancel := context.WithTimeout(outerCtx, common.ClusterAuthRequestTimeout)
		defer cancel()
		existingSecret, err := clientset.CoreV1().Secrets(ns).Get(innerCtx, s.Name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to retrieve secret %q: %w", s.Name, err)
		}
		if existingSecret.Type == corev1.SecretTypeServiceAccountToken {
			return existingSecret.Name, nil
		}
	}

	return createServiceAccountToken(clientset, serviceAccount)
}

func createServiceAccountToken(clientset kubernetes.Interface, serviceAccount *corev1.ServiceAccount) (string, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: serviceAccount.Name + "-token-",
			Namespace:    serviceAccount.Namespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: serviceAccount.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	ctx, cancel := context.WithTimeout(context.Background(), common.ClusterAuthRequestTimeout)
	defer cancel()
	secret, err := clientset.CoreV1().Secrets(serviceAccount.Namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create secret for serviceaccount %q: %w", serviceAccount.Name, err)
	}

	log.Infof("Created bearer token secret for ServiceAccount %q", serviceAccount.Name)
	serviceAccount.Secrets = []corev1.ObjectReference{{
		Name:      secret.Name,
		Namespace: secret.Namespace,
	}}
	patch, err := json.Marshal(serviceAccount)
	if err != nil {
		return "", fmt.Errorf("failed marshaling patch for serviceaccount %q: %w", serviceAccount.Name, err)
	}

	_, err = clientset.CoreV1().ServiceAccounts(serviceAccount.Namespace).Patch(ctx, serviceAccount.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to patch serviceaccount %q with bearer token secret: %w", serviceAccount.Name, err)
	}

	return secret.Name, nil
}

// UninstallClusterManagerRBAC removes RBAC resources for a cluster manager to operate a cluster
func UninstallClusterManagerRBAC(clientset kubernetes.Interface) error {
	return UninstallRBAC(clientset, "kube-system", ArgoCDManagerClusterRoleBinding, ArgoCDManagerClusterRole, ArgoCDManagerServiceAccount)
}

// UninstallRBAC uninstalls RBAC related resources  for a binding, role, and service account
func UninstallRBAC(clientset kubernetes.Interface, namespace, bindingName, roleName, serviceAccount string) error {
	if err := clientset.RbacV1().ClusterRoleBindings().Delete(context.Background(), bindingName, metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ClusterRoleBinding: %w", err)
		}
		log.Infof("ClusterRoleBinding %q not found", bindingName)
	} else {
		log.Infof("ClusterRoleBinding %q deleted", bindingName)
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(context.Background(), roleName, metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ClusterRole: %w", err)
		}
		log.Infof("ClusterRole %q not found", roleName)
	} else {
		log.Infof("ClusterRole %q deleted", roleName)
	}

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(context.Background(), serviceAccount, metav1.DeleteOptions{}); err != nil {
		if !apierr.IsNotFound(err) {
			return fmt.Errorf("Failed to delete ServiceAccount: %w", err)
		}
		log.Infof("ServiceAccount %q in namespace %q not found", serviceAccount, namespace)
	} else {
		log.Infof("ServiceAccount %q deleted", serviceAccount)
	}
	return nil
}

type ServiceAccountClaims struct {
	Sub                string `json:"sub"`
	Iss                string `json:"iss"`
	Namespace          string `json:"kubernetes.io/serviceaccount/namespace"`
	SecretName         string `json:"kubernetes.io/serviceaccount/secret.name"`
	ServiceAccountName string `json:"kubernetes.io/serviceaccount/service-account.name"`
	ServiceAccountUID  string `json:"kubernetes.io/serviceaccount/service-account.uid"`
}

// Valid satisfies the jwt.Claims interface to enable JWT parsing
func (sac *ServiceAccountClaims) Valid() error {
	return nil
}

// ParseServiceAccountToken parses a Kubernetes service account token
func ParseServiceAccountToken(token string) (*ServiceAccountClaims, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims ServiceAccountClaims
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse service account token: %w", err)
	}
	return &claims, nil
}

// GenerateNewClusterManagerSecret creates a new secret derived with same metadata as existing one
// and waits until the secret is populated with a bearer token
func GenerateNewClusterManagerSecret(clientset kubernetes.Interface, claims *ServiceAccountClaims) (*corev1.Secret, error) {
	secretsClient := clientset.CoreV1().Secrets(claims.Namespace)
	existingSecret, err := secretsClient.Get(context.Background(), claims.SecretName, metav1.GetOptions{})
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

	created, err := secretsClient.Create(context.Background(), &newSecret, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	err = wait.PollUntilContextTimeout(context.Background(), 500*time.Millisecond, 30*time.Second, false, func(ctx context.Context) (bool, error) {
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
		return nil, fmt.Errorf("Timed out waiting for secret to generate new token: %w", err)
	}
	return created, nil
}

// RotateServiceAccountSecrets rotates the entries in the service accounts secrets list
func RotateServiceAccountSecrets(clientset kubernetes.Interface, claims *ServiceAccountClaims, newSecret *corev1.Secret) error {
	// 1. update service account secrets list with new secret name while also removing the old name
	saClient := clientset.CoreV1().ServiceAccounts(claims.Namespace)
	sa, err := saClient.Get(context.Background(), claims.ServiceAccountName, metav1.GetOptions{})
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
	_, err = saClient.Update(context.Background(), sa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// 2. delete existing secret object
	secretsClient := clientset.CoreV1().Secrets(claims.Namespace)
	err = secretsClient.Delete(context.Background(), claims.SecretName, metav1.DeleteOptions{})
	if !apierr.IsNotFound(err) {
		return err
	}
	return nil
}
