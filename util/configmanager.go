package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	AdminPassword string
}

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset     kubernetes.Interface
	namespace     string
	configMapName string
}

// GetSettings retrieves settings from the ConfigManager.
func (mgr *ConfigManager) GetSettings() (settings ArgoCDSettings) {
	const (
		adminPasswordKeyName   = "adminPasswordSecretName"
		adminPasswordValueName = "admin.password"
	)
	configMap, err := mgr.readConfigMap(mgr.configMapName)
	if err == nil {
		adminPasswordSecretName, ok := configMap.Data[adminPasswordKeyName]
		if ok {
			adminPassword, err := mgr.readSecret(adminPasswordSecretName)
			if err == nil {
				settings.AdminPassword = string(adminPassword.Data[adminPasswordValueName])
			}
		}
	}
	return
}

// NewConfigManager generates a new ConfigManager pointer and returns it
func NewConfigManager(clientset kubernetes.Interface, namespace, configMapName string) (mgr *ConfigManager) {
	mgr = &ConfigManager{
		clientset:     clientset,
		namespace:     namespace,
		configMapName: configMapName,
	}
	return
}

// ReadConfigMap retrieves a config map from Kubernetes.
func (mgr *ConfigManager) readConfigMap(name string) (configMap *apiv1.ConfigMap, err error) {
	configMap, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(name, metav1.GetOptions{})
	return
}

// ReadSecret retrieves a secret from Kubernetes.
func (mgr *ConfigManager) readSecret(name string) (secret *apiv1.Secret, err error) {
	secret, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(name, metav1.GetOptions{})
	return
}
