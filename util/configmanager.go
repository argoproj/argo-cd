package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset kubernetes.Interface
}

// NewConfigManager generates a new ConfigManager pointer and returns it
func NewConfigManager(config *rest.Config) (mgr *ConfigManager, err error) {
	kubeclientset, err := kubernetes.NewForConfig(config)
	if err == nil {
		mgr = &ConfigManager{kubeclientset}
	}
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
