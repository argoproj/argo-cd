package util

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/common"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func makeKubeclientset() (kubeclientset *kubernetes.Clientset) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("REST config invalid: %s", err)
	}
	_, err = kubeclientset.ServerVersion()
	if err != nil {
		return fmt.Errorf("REST config invalid: %s", err)
	}
	return nil
}

// CreateSecret stores a new secret in Kubernetes.
func CreateSecret(namespace, name string, value map[string]string, secretType string) (secret *apiv1.Secret, err error) {
	newSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.LabelKeySecretType: secretType,
			},
		},
		StringData: value,
	}
	secret, err = makeKubeclientset().CoreV1().Secrets(namespace).Create(newSecret)
	return
}

// ReadSecret retrieves a secret from Kubernetes.
func ReadSecret(namespace, name string) (secret *apiv1.Secret, err error) {
	secret, err = makeKubeclientset().CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateSecret updates an existing secret in Kubernetes.
func UpdateSecret(namespace, name string, value map[string]string) (secret *apiv1.Secret, err error) {
	existingSecret, err := ReadSecret(namespace, name)
	if err != nil {
		existingSecret.StringData = value
		secret, err = makeKubeclientset().CoreV1().Secrets(namespace).Update(existingSecret)
	}
	return
}

// DeleteSecret removes a secret from Kubernetes.
func DeleteSecret(namespace, name string) (err error) {
	err = makeKubeclientset().CoreV1().Secrets(namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
