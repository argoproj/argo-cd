package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConfigMapClientsetWrapper holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigMapClientsetWrapper struct {
	Clientset kubernetes.Interface
}

// NewConfigMapClientsetWrapper generates a new ConfigMapClientsetWrapper pointer and returns it
func NewConfigMapClientsetWrapper(config *rest.Config) (manager *ConfigMapClientsetWrapper, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		manager = &ConfigMapClientsetWrapper{kubeclientset}
	}
	return
}

// Create stores a new config map in Kubernetes.
func (manager *ConfigMapClientsetWrapper) Create(namespace, name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	newConfigMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: value,
	}
	configMap, err = manager.Clientset.CoreV1().ConfigMaps(namespace).Create(newConfigMap)
	return
}

// Read retrieves a config map from Kubernetes.
func (manager *ConfigMapClientsetWrapper) Read(namespace, name string) (configMap *apiv1.ConfigMap, err error) {
	configMap, err = manager.Clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	return
}

// Update overwrite-updates an existing config map in Kubernetes.  This overwrite is in contrast to the merge-update done for secrets.
func (manager *ConfigMapClientsetWrapper) Update(namespace, name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	existingConfigMap, err := manager.Read(namespace, name)
	if err == nil {
		existingConfigMap.Data = value
		configMap, err = manager.Clientset.CoreV1().ConfigMaps(namespace).Update(existingConfigMap)
	}
	return
}

// Delete removes a config map from Kubernetes.
func (manager *ConfigMapClientsetWrapper) Delete(namespace, name string) (err error) {
	err = manager.Clientset.CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
