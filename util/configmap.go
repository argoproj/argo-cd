package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConfigMapManager holds config info for a new server with which to access Kubernetes ConfigMaps.
type ConfigMapManager struct {
	Namespace string
	Clientset kubernetes.Interface
}

// NewConfigMapManager generates a new ConfigMapManager pointer and returns it
func NewConfigMapManager(namespace string, config *rest.Config) (server *ConfigMapManager, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		server = &ConfigMapManager{namespace, kubeclientset}
	}
	return
}

// Create stores a new config map in Kubernetes.
func (server *ConfigMapManager) Create(name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	newConfigMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: value,
	}
	configMap, err = server.Clientset.CoreV1().ConfigMaps(server.Namespace).Create(newConfigMap)
	return
}

// Read retrieves a config map from Kubernetes.
func (server *ConfigMapManager) Read(name string) (configMap *apiv1.ConfigMap, err error) {
	configMap, err = server.Clientset.CoreV1().ConfigMaps(server.Namespace).Get(name, metav1.GetOptions{})
	return
}

// Update overwrite-updates an existing config map in Kubernetes.  This overwrite is in contrast to the merge-update done for secrets.
func (server *ConfigMapManager) Update(name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	existingConfigMap, err := server.Read(name)
	if err == nil {
		existingConfigMap.Data = value
		configMap, err = server.Clientset.CoreV1().ConfigMaps(server.Namespace).Update(existingConfigMap)
	}
	return
}

// Delete removes a config map from Kubernetes.
func (server *ConfigMapManager) Delete(name string) (err error) {
	err = server.Clientset.CoreV1().ConfigMaps(server.Namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
