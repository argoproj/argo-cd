package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SecretManager holds config info for a new server with which to access Kubernetes Secrets.
type SecretManager struct {
	namespace     string
	kubeclientset kubernetes.Interface
}

// NewSecretManager generates a new SecretManager pointer and returns it
func NewSecretManager(namespace string, config *rest.Config) (server *SecretManager, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}
	server = &SecretManager{namespace, kubeclientset}
	return
}

// CreateSecret stores a new secret in Kubernetes.  Set secretType to "" for no label.
func (server *SecretManager) CreateSecret(name string, value map[string]string, secretType string) (secret *apiv1.Secret, err error) {
	labels := make(map[string]string)
	if secretType != "" {
		labels[common.LabelKeySecretType] = secretType
	}
	newSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		StringData: value,
	}
	secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Create(newSecret)
	return
}

// ReadSecret retrieves a secret from Kubernetes.
func (server *SecretManager) ReadSecret(name string) (secret *apiv1.Secret, err error) {
	secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateSecret updates an existing secret in Kubernetes.
func (server *SecretManager) UpdateSecret(name string, value map[string]string) (secret *apiv1.Secret, err error) {
	existingSecret, err := server.ReadSecret(name)
	if err == nil {
		existingSecret.StringData = value
		secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Update(existingSecret)
	}
	return
}

// DeleteSecret removes a secret from Kubernetes.
func (server *SecretManager) DeleteSecret(name string) (err error) {
	err = server.kubeclientset.CoreV1().Secrets(server.namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
