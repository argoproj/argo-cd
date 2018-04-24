package config

import (
	"crypto/tls"
	"fmt"

	"github.com/argoproj/argo-cd/common"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// URL is the externally facing URL users will visit to reach ArgoCD.
	// The value here is used when configuring SSO. Omitting this value will disable SSO.
	URL string

	// DexConfig is contains portions of a dex config yaml
	DexConfig string

	// LocalUsers holds users local to (stored on) the server.  This is to be distinguished from any potential alternative future login providers (LDAP, SAML, etc.) that might ever be added.
	LocalUsers map[string]string

	// ServerSignature holds the key used to generate JWT tokens.
	ServerSignature []byte

	// Certificate holds the certificate/private key for the ArgoCD API server.
	// If nil, will run insecure without TLS.
	Certificate *tls.Certificate
}

const (
	// configManagerAdminPasswordKey designates the key for a root password inside a Kubernetes secret.
	configManagerAdminPasswordKey = "admin.password"

	// configManagerServerSignatureKey designates the key for a server secret key inside a Kubernetes secret.
	configManagerServerSignatureKey = "server.secretkey"

	// configManagerServerCertificate designates the key for the public cert used in TLS
	configManagerServerCertificate = "server.crt"

	// configManagerServerPrivateKey designates the key for the private key used in TLS
	configManagerServerPrivateKey = "server.key"

	// configManagerURL designates the key where ArgoCDs external URL is set
	configManagerURLKey = "url"

	// configManagerDexConfig designates the key for the dex config
	configManagerDexConfigKey = "dex.config"
)

// ConfigManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type ConfigManager struct {
	clientset kubernetes.Interface
	namespace string
}

// GetSettings retrieves settings from the ConfigManager.
func (mgr *ConfigManager) GetSettings() (*ArgoCDSettings, error) {
	argoCDCM, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var settings ArgoCDSettings
	settings.DexConfig = argoCDCM.Data[configManagerDexConfigKey]
	settings.URL = argoCDCM.Data[configManagerURLKey]

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

	serverCert, certOk := argoCDSecret.Data[configManagerServerCertificate]
	serverKey, keyOk := argoCDSecret.Data[configManagerServerPrivateKey]
	if certOk && keyOk {
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		if err != nil {
			return nil, fmt.Errorf("invalid x509 key pair %s/%s in secret: %s", configManagerServerCertificate, configManagerServerPrivateKey, err)
		}
		settings.Certificate = &cert
	}
	return &settings, nil
}

// SaveSettings serializes ArgoCD settings and upserts it into K8s secret/configmap
func (mgr *ConfigManager) SaveSettings(settings *ArgoCDSettings) error {
	// Upsert the config data
	argoCDCM, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	createCM := false
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		argoCDCM = &apiv1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDConfigMapName,
			},
			Data: make(map[string]string),
		}
		createCM = true
	}
	argoCDCM.Data[configManagerURLKey] = settings.URL
	argoCDCM.Data[configManagerDexConfigKey] = settings.DexConfig
	if createCM {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Create(argoCDCM)
	} else {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(argoCDCM)
	}
	if err != nil {
		return err
	}

	// Upsert the secret data. Ensure we do not delete any extra keys which user may have added
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	createSecret := false
	if err != nil {
		if !apierr.IsNotFound(err) {
			return err
		}
		argoCDSecret = &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.ArgoCDSecretName,
			},
			Data: make(map[string][]byte),
		}
		createSecret = true
	}
	argoCDSecret.StringData = make(map[string]string)
	argoCDSecret.StringData[configManagerServerSignatureKey] = string(settings.ServerSignature)
	argoCDSecret.StringData[configManagerAdminPasswordKey] = settings.LocalUsers[common.ArgoCDAdminUsername]
	if settings.Certificate != nil {
		certBytes, keyBytes := tlsutil.EncodeX509KeyPair(*settings.Certificate)
		argoCDSecret.StringData[configManagerServerCertificate] = string(certBytes)
		argoCDSecret.StringData[configManagerServerPrivateKey] = string(keyBytes)
	} else {
		delete(argoCDSecret.Data, configManagerServerCertificate)
		delete(argoCDSecret.Data, configManagerServerPrivateKey)
	}
	if createSecret {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Create(argoCDSecret)
	} else {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(argoCDSecret)
	}
	if err != nil {
		return err
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

func (a *ArgoCDSettings) IsSSOConfigured() bool {
	if a.URL == "" {
		return false
	}
	var dexCfg map[string]interface{}
	err := yaml.Unmarshal([]byte(a.DexConfig), &dexCfg)
	if err != nil {
		log.Warn("invalid dex yaml config")
		return false
	}
	return len(dexCfg) > 0
}
