package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SecretServer holds config info for a new server with which to access secrets
type SecretServer struct {
	namespace     string
	kubeclientset kubernetes.Interface
}

// NewSecretServer generates a new SecretServer pointer and returns it
func NewSecretServer(namespace string, config *rest.Config) (server *SecretServer, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return
	}
	server = &SecretServer{namespace, kubeclientset}
	return
}

// CreateSecret stores a new secret in Kubernetes.
func (server *SecretServer) CreateSecret(name string, value map[string]string, secretType string) (secret *apiv1.Secret, err error) {
	newSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.LabelKeySecretType: secretType,
			},
		},
		StringData: value,
	}
	secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Create(newSecret)
	return
}

// ReadSecret retrieves a secret from Kubernetes.
func (server *SecretServer) ReadSecret(name string) (secret *apiv1.Secret, err error) {
	secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateSecret updates an existing secret in Kubernetes.
func (server *SecretServer) UpdateSecret(name string, value map[string]string) (secret *apiv1.Secret, err error) {
	existingSecret, err := server.ReadSecret(name)
	if err != nil {
		existingSecret.StringData = value
		secret, err = server.kubeclientset.CoreV1().Secrets(server.namespace).Update(existingSecret)
	}
	return
}

// DeleteSecret removes a secret from Kubernetes.
func (server *SecretServer) DeleteSecret(name string) (err error) {
	err = server.kubeclientset.CoreV1().Secrets(server.namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
