package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConfigMapManager holds config info for a new server with which to access Kubernetes ConfigMaps.
type ConfigMapManager struct {
	namespace     string
	kubeclientset kubernetes.Interface
}

// NewConfigMapManager generates a new ConfigMapManager pointer and returns it
func NewConfigMapManager(namespace string, config *rest.Config) (server *ConfigMapManager, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		server = &ConfigMapManager{namespace, kubeclientset}
	}
	return
}

// CreateConfigMap stores a new config map in Kubernetes.
func (server *ConfigMapManager) CreateConfigMap(name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	newConfigMap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: value,
	}
	configMap, err = server.kubeclientset.CoreV1().ConfigMaps(server.namespace).Create(newConfigMap)
	return
}

// ReadConfigMap retrieves a config map from Kubernetes.
func (server *ConfigMapManager) ReadConfigMap(name string) (configMap *apiv1.ConfigMap, err error) {
	configMap, err = server.kubeclientset.CoreV1().ConfigMaps(server.namespace).Get(name, metav1.GetOptions{})
	return
}

// UpdateConfigMap updates an existing config map in Kubernetes.
func (server *ConfigMapManager) UpdateConfigMap(name string, value map[string]string) (configMap *apiv1.ConfigMap, err error) {
	existingConfigMap, err := server.ReadConfigMap(name)
	if err == nil {
		existingConfigMap.Data = value
		configMap, err = server.kubeclientset.CoreV1().ConfigMaps(server.namespace).Update(existingConfigMap)
	}
	return
}

// DeleteConfigMap removes a config map from Kubernetes.
func (server *ConfigMapManager) DeleteConfigMap(name string) (err error) {
	err = server.kubeclientset.CoreV1().ConfigMaps(server.namespace).Delete(name, &metav1.DeleteOptions{})
	return
}
