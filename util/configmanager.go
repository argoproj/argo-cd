package util

import (
	"github.com/argoproj/argo-cd/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset kubernetes.Interface
}

// NewConfigManager generates a new ConfigManager pointer and returns it
func NewConfigManager(clientset kubernetes.Interface) (mgr *ConfigManager) {
	mgr = &ConfigManager{clientset}
	return
}

// ListConfigMaps returns a list of existing config maps.
func (mgr *ConfigManager) ListConfigMaps(namespace string) (configMaps *apiv1.ConfigMapList, err error) {
	configMaps, err = mgr.clientset.CoreV1().ConfigMaps(namespace).List(metav1.ListOptions{})
	return
}

// CreateConfigMap stores a new config map in Kubernetes.
func (mgr *ConfigManager) CreateConfigMap(namespace, name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	newConfigMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: value,
	}
	configMap, err = mgr.clientset.CoreV1().ConfigMaps(namespace).Create(newConfigMap)
	return
}

// ReadConfigMap retrieves a config map from Kubernetes.
func (mgr *ConfigManager) ReadConfigMap(namespace, name string) (configMap *apiv1.ConfigMap, err error) {
	configMap, err = mgr.clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateConfigMap overwrite-updates an existing config map in Kubernetes.  This overwrite is in contrast to the merge-update done for secrets.
func (mgr *ConfigManager) UpdateConfigMap(namespace, name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	existingConfigMap, err := mgr.ReadConfigMap(namespace, name)
	if err == nil {
		existingConfigMap.Data = value
		configMap, err = mgr.clientset.CoreV1().ConfigMaps(namespace).Update(existingConfigMap)
	}
	return
}

// DeleteConfigMap removes a config map from Kubernetes.
func (mgr *ConfigManager) DeleteConfigMap(namespace, name string) (err error) {
	err = mgr.clientset.CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{})
	return
}

// ListSecrets returns a list of existing config maps.
func (mgr *ConfigManager) ListSecrets(namespace string) (secrets *apiv1.SecretList, err error) {
	secrets, err = mgr.clientset.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	return
}

// CreateSecret stores a new secret in Kubernetes.  Set secretType to "" for no label.
func (mgr *ConfigManager) CreateSecret(namespace, name string, value map[string]string, secretType string) (secret *apiv1.Secret, err error) {
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
	secret, err = mgr.clientset.CoreV1().Secrets(namespace).Create(newSecret)
	return
}

// Read retrieves a secret from Kubernetes.
func (mgr *ConfigManager) ReadSecret(namespace, name string) (secret *apiv1.Secret, err error) {
	secret, err = mgr.clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateSecret merge-updates an existing secret in Kubernetes.  This merge-update is in contrast to the overwrite-update done for config maps.
func (mgr *ConfigManager) UpdateSecret(namespace, name string, value map[string]string) (secret *apiv1.Secret, err error) {
	existingSecret, err := mgr.ReadSecret(namespace, name)
	if err == nil {
		existingSecret.StringData = value
		secret, err = mgr.clientset.CoreV1().Secrets(namespace).Update(existingSecret)
	}
	return
}

// DeleteSecret removes a secret from Kubernetes.
func (mgr *ConfigManager) DeleteSecret(namespace, name string) (err error) {
	err = mgr.clientset.CoreV1().Secrets(namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
