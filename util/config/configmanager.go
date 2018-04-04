package config

import (
	"fmt"

	"github.com/argoproj/argo-cd/common"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// LocalUsers holds users local to (stored on) the server.  This is to be distinguished from any potential alternative future login providers (LDAP, SAML, etc.) that might ever be added.
	LocalUsers map[string]string

	// ServerSignature holds the key used to generate JWT tokens.
	ServerSignature []byte
}

const (
	// configManagerAdminPasswordKey designates the key for a root password inside a Kubernetes secret.
	configManagerAdminPasswordKey = "admin.password"

	// configManagerServerSignatureKey designates the key for a server secret key inside a Kubernetes secret.
	configManagerServerSignatureKey = "server.secretkey"
)

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset kubernetes.Interface
	namespace string
}

// GetSettings retrieves settings from the ConfigManager.
func (mgr *ConfigManager) GetSettings() (*ArgoCDSettings, error) {
	// TODO: we currently do not store anything in configmaps, yet. We eventually will (e.g.
	// tuning parameters). Future settings/tunables should be stored here
	_, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var settings ArgoCDSettings
	adminPasswordHash, ok := argoCDSecret.Data[configManagerAdminPasswordKey]
	if !ok {
		return nil, fmt.Errorf("admin user not found")
	}
	settings.LocalUsers = map[string]string{
		common.ArgoCDAdminUsername: string(adminPasswordHash),
	}
	secretKey, ok := argoCDSecret.Data[configManagerServerSignatureKey]
	if !ok {
		return nil, fmt.Errorf("server secret key not found")
	}
	settings.ServerSignature = secretKey
	return &settings, nil
}

// SaveSettings serializes ArgoCD settings and upserts it into K8s secret/configmap
func (mgr *ConfigManager) SaveSettings(settings *ArgoCDSettings) error {
	configMapData := make(map[string]string)
	_, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		newConfigMap := &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDConfigMapName,
			},
			Data: configMapData,
		}
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Create(newConfigMap)
		if err != nil {
			return err
		}
	} else {
		// mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update()
	}

	secretStringData := map[string]string{
		configManagerServerSignatureKey: string(settings.ServerSignature),
		configManagerAdminPasswordKey:   settings.LocalUsers[common.ArgoCDAdminUsername],
	}
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		newSecret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDSecretName,
			},
			StringData: secretStringData,
		}
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Create(newSecret)
		if err != nil {
			return err
		}
	} else {
		argoCDSecret.Data = nil
		argoCDSecret.StringData = secretStringData
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(argoCDSecret)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewConfigManager generates a new ConfigManager pointer and returns it
func NewConfigManager(clientset kubernetes.Interface, namespace string) *ConfigManager {
	return &ConfigManager{
		clientset: clientset,
		namespace: namespace,
	}
}
