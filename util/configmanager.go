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

type configMapData struct {
	rootCredentialsSecretName string
}

const (
	// defaultConfigMapName default name of config map with argo-cd settings
	defaultConfigMapName = "argo-cd-cm"

	// defaultRootCredentialsSecretName contains default name of secret with root user credentials
	defaultRootCredentialsSecretName = "argo-cd-root-credentials"

	// configManagerRootUsernameKey designates the root username inside a Kubernetes secret.
	configManagerRootUsernameKey = "root.username"

	// configManagerRootPasswordKey designates the root password inside a Kubernetes secret.
	configManagerRootPasswordKey = "root.password"
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
	data, err := mgr.getConfigMapData()
	if err != nil {
		return settings, err
	}

	// Try to retrieve the secret
	rootCredentials, err := mgr.readSecret(data.rootCredentialsSecretName)
	if err != nil {
		if errors.IsNotFound(err) {
			return settings, nil
		} else {
			return settings, err
		}
	}
	// Retrieve credential info from the secret
	rootUsername, okUsername := rootCredentials.Data[configManagerRootUsernameKey]
	rootPassword, okPassword := rootCredentials.Data[configManagerRootPasswordKey]

	if okUsername && okPassword {
		// Store credential info inside LocalUsers
		settings.LocalUsers[string(rootUsername)] = string(rootPassword)
	}
	return settings, nil
}

func (mgr *ConfigManager) SetRootUserCredentials(username string, password string) error {
	data, err := mgr.getConfigMapData()
	if err != nil {
		return err
	}

	// Don't commit plaintext passwords
	passwordHash, err := HashPassword(password)
	if err != nil {
		return err
	}

	credentials := map[string]string{
		configManagerRootUsernameKey: username,
		configManagerRootPasswordKey: passwordHash,
	}

	// See if we've already written this secret
	secret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(data.rootCredentialsSecretName, metav1.GetOptions{})
	if err != nil {
		newSecret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: data.rootCredentialsSecretName,
			},
		}
		newSecret.StringData = credentials
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Create(newSecret)

	} else {
		secret.StringData = credentials
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(secret)
	}
	return err
}

// NewConfigManager generates a new ConfigManager pointer and returns it
func NewConfigManager(clientset kubernetes.Interface, namespace, configMapName string) (mgr *ConfigManager) {
	if configMapName == "" {
		configMapName = defaultConfigMapName
	}
	mgr = &ConfigManager{
		clientset:     clientset,
		namespace:     namespace,
		configMapName: configMapName,
	}
	return
}

func (mgr *ConfigManager) getConfigMapData() (configMapData, error) {
	data := configMapData{}
	configMap, err := mgr.readConfigMap(mgr.configMapName)
	if err != nil {
		if errors.IsNotFound(err) {
			data.rootCredentialsSecretName = defaultRootCredentialsSecretName
			return data, nil
		} else {
			return data, err
		}
	}
	rootCredentialsSecretName, ok := configMap.Data[defaultRootCredentialsSecretName]
	if !ok {
		rootCredentialsSecretName = defaultRootCredentialsSecretName
	}
	data.rootCredentialsSecretName = rootCredentialsSecretName
	return data, nil
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
