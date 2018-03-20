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
	Namespace string
	Clientset kubernetes.Interface
}

// NewSecretManager generates a new SecretManager pointer and returns it
func NewSecretManager(namespace string, config *rest.Config) (server *SecretManager, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		server = &SecretManager{namespace, kubeclientset}
	}
	return
}

// Create stores a new secret in Kubernetes.  Set secretType to "" for no label.
func (server *SecretManager) Create(name string, value map[string]string, secretType string) (secret *apiv1.Secret, err error) {
	labels := make(map[string]string)
	if secretType != "" {
		labels[common.LabelKeySecretType] = secretType
	}
	newSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	newSecret.StringData = value
	secret, err = server.Clientset.CoreV1().Secrets(server.Namespace).Create(newSecret)
	return
}

// Read retrieves a secret from Kubernetes.
func (server *SecretManager) Read(name string) (secret *apiv1.Secret, err error) {
	secret, err = server.Clientset.CoreV1().Secrets(server.Namespace).Get(name, metav1.GetOptions{})
	return
}

// Update merge-updates an existing secret in Kubernetes.  This merge-update is in contrast to the overwrite-update done for config maps.
func (server *SecretManager) Update(name string, value map[string]string) (secret *apiv1.Secret, err error) {
	existingSecret, err := server.Read(name)
	if err == nil {
		existingSecret.StringData = value
		secret, err = server.Clientset.CoreV1().Secrets(server.Namespace).Update(existingSecret)
	}
	return
}

// Delete removes a secret from Kubernetes.
func (server *SecretManager) Delete(name string) (err error) {
	err = server.Clientset.CoreV1().Secrets(server.Namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
