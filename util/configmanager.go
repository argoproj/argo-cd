package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {

	// LocalUsers holds users local to (stored on) the server.  This is to be distinguished from any potential alternative future login providers (LDAP, SAML, etc.) that might ever be added.
	LocalUsers map[string]string
}

const (
	// RootCredentialsSecretNameKey designates the name of the config map field holding the name of a Kubernetes secret.
	RootCredentialsSecretNameKey = "rootCredentialsSecretName"

	// ConfigManagerDefaultRootCredentialsSecretName holds the default secret name for root credentials.
	ConfigManagerDefaultRootCredentialsSecretName = "argocd-root-credentials-secret"

	// ConfigManagerRootUsernameKey designates the root username inside a Kubernetes secret.
	ConfigManagerRootUsernameKey = "root.username"

	// ConfigManagerRootPasswordKey designates the root password inside a Kubernetes secret.
	ConfigManagerRootPasswordKey = "root.password"
)

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset     kubernetes.Interface
	namespace     string
	configMapName string
}

// GetSettings retrieves settings from the ConfigManager.
func (mgr *ConfigManager) GetSettings() (ArgoCDSettings, error) {
	settings := ArgoCDSettings{}
	settings.LocalUsers = make(map[string]string)
	configMap, err := mgr.readConfigMap(mgr.configMapName)
	if err != nil {
		if errors.IsNotFound(err) {
			return settings, nil
		} else {
			return settings, err
		}
	}

	// Try to retrieve the name of a Kubernetes secret holding root credentials
	rootCredentialsSecretName, ok := configMap.Data[RootCredentialsSecretNameKey]

	if !ok {
		return settings, nil
	}

	// Try to retrieve the secret
	rootCredentials, err := mgr.readSecret(rootCredentialsSecretName)

	if err != nil {
		return settings, err
	}
	// Retrieve credential info from the secret
	rootUsername, okUsername := rootCredentials.Data[ConfigManagerRootUsernameKey]
	rootPassword, okPassword := rootCredentials.Data[ConfigManagerRootPasswordKey]

	if okUsername && okPassword {
		// Store credential info inside LocalUsers
		settings.LocalUsers[string(rootUsername)] = string(rootPassword)
	}
	return settings, nil
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
