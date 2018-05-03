package settings

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
	// settingAdminPasswordKey designates the key for a root password inside a Kubernetes secret.
	settingAdminPasswordKey = "admin.password"
	// settingServerSignatureKey designates the key for a server secret key inside a Kubernetes secret.
	settingServerSignatureKey = "server.secretkey"
	// settingServerCertificate designates the key for the public cert used in TLS
	settingServerCertificate = "server.crt"
	// settingServerPrivateKey designates the key for the private key used in TLS
	settingServerPrivateKey = "server.key"
	// settingURLKey designates the key where ArgoCDs external URL is set
	settingURLKey = "url"
	// settingDexConfigKey designates the key for the dex config
	settingDexConfigKey = "dex.config"
)

// SettingsManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type SettingsManager struct {
	clientset kubernetes.Interface
	namespace string
}

// GetSettings retrieves settings from the ConfigManager.
func (mgr *SettingsManager) GetSettings() (*ArgoCDSettings, error) {
	argoCDCM, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var settings ArgoCDSettings
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.URL = argoCDCM.Data[settingURLKey]

	adminPasswordHash, ok := argoCDSecret.Data[settingAdminPasswordKey]
	if !ok {
		return nil, fmt.Errorf("admin user not found")
	}
	settings.LocalUsers = map[string]string{
		common.ArgoCDAdminUsername: string(adminPasswordHash),
	}
	secretKey, ok := argoCDSecret.Data[settingServerSignatureKey]
	if !ok {
		return nil, fmt.Errorf("server secret key not found")
	}
	settings.ServerSignature = secretKey

	serverCert, certOk := argoCDSecret.Data[settingServerCertificate]
	serverKey, keyOk := argoCDSecret.Data[settingServerPrivateKey]
	if certOk && keyOk {
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		if err != nil {
			return nil, fmt.Errorf("invalid x509 key pair %s/%s in secret: %s", settingServerCertificate, settingServerPrivateKey, err)
		}
		settings.Certificate = &cert
	}
	return &settings, nil
}

// SaveSettings serializes ArgoCD settings and upserts it into K8s secret/configmap
func (mgr *SettingsManager) SaveSettings(settings *ArgoCDSettings) error {
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
	argoCDCM.Data[settingURLKey] = settings.URL
	argoCDCM.Data[settingDexConfigKey] = settings.DexConfig
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
	argoCDSecret.StringData[settingServerSignatureKey] = string(settings.ServerSignature)
	argoCDSecret.StringData[settingAdminPasswordKey] = settings.LocalUsers[common.ArgoCDAdminUsername]
	if settings.Certificate != nil {
		certBytes, keyBytes := tlsutil.EncodeX509KeyPair(*settings.Certificate)
		argoCDSecret.StringData[settingServerCertificate] = string(certBytes)
		argoCDSecret.StringData[settingServerPrivateKey] = string(keyBytes)
	} else {
		delete(argoCDSecret.Data, settingServerCertificate)
		delete(argoCDSecret.Data, settingServerPrivateKey)
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

// NewSettingsManager generates a new SettingsManager pointer and returns it
func NewSettingsManager(clientset kubernetes.Interface, namespace string) *SettingsManager {
	return &SettingsManager{
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
