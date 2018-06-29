package settings

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/common"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// URL is the externally facing URL users will visit to reach ArgoCD.
	// The value here is used when configuring SSO. Omitting this value will disable SSO.
	URL string `json:"url,omitempty"`
	// DexConfig is contains portions of a dex config yaml
	DexConfig string `json:"dexConfig,omitempty"`
	// LocalUsers holds users local to (stored on) the server.  This is to be distinguished from any potential alternative future login providers (LDAP, SAML, etc.) that might ever be added.
	LocalUsers map[string]string `json:"localUsers,omitempty"`
	// ServerSignature holds the key used to generate JWT tokens.
	ServerSignature []byte `json:"serverSignature,omitempty"`
	// Certificate holds the certificate/private key for the ArgoCD API server.
	// If nil, will run insecure without TLS.
	Certificate *tls.Certificate `json:"-"`
	// WebhookGitLabSecret holds the shared secret for authenticating GitHub webhook events
	WebhookGitHubSecret string `json:"webhookGitHubSecret,omitempty"`
	// WebhookGitLabSecret holds the shared secret for authenticating GitLab webhook events
	WebhookGitLabSecret string `json:"webhookGitLabSecret,omitempty"`
	// WebhookBitbucketUUID holds the UUID for authenticating Bitbucket webhook events
	WebhookBitbucketUUID string `json:"webhookBitbucketUUID,omitempty"`
	// Secrets holds all secrets in argocd-secret as a map[string]string
	Secrets map[string]string `json:"secrets,omitempty"`
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
	// settingsWebhookGitHubSecret is the key for the GitHub shared webhook secret
	settingsWebhookGitHubSecretKey = "webhook.github.secret"
	// settingsWebhookGitLabSecret is the key for the GitLab shared webhook secret
	settingsWebhookGitLabSecretKey = "webhook.gitlab.secret"
	// settingsWebhookBitbucketUUID is the key for Bitbucket webhook UUID
	settingsWebhookBitbucketUUIDKey = "webhook.bitbucket.uuid"
)

// SettingsManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type SettingsManager struct {
	clientset kubernetes.Interface
	namespace string
	// subscribers is a list of subscribers to settings updates
	subscribers []chan<- struct{}
	// mutex protects the subscribers list from concurrent updates
	mutex *sync.Mutex
}

// GetSettings retrieves settings from the ArgoCD configmap and secret.
func (mgr *SettingsManager) GetSettings() (*ArgoCDSettings, error) {
	var settings ArgoCDSettings
	argoCDCM, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	updateSettingsFromConfigMap(&settings, argoCDCM)
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = updateSettingsFromSecret(&settings, argoCDSecret)
	if err != nil {
		return &settings, err
	}
	return &settings, nil
}

func updateSettingsFromConfigMap(settings *ArgoCDSettings, argoCDCM *apiv1.ConfigMap) {
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.URL = argoCDCM.Data[settingURLKey]
}

func updateSettingsFromSecret(settings *ArgoCDSettings, argoCDSecret *apiv1.Secret) error {
	adminPasswordHash, ok := argoCDSecret.Data[settingAdminPasswordKey]
	if !ok {
		return fmt.Errorf("admin user not found")
	}
	settings.LocalUsers = map[string]string{
		common.ArgoCDAdminUsername: string(adminPasswordHash),
	}
	secretKey, ok := argoCDSecret.Data[settingServerSignatureKey]
	if !ok {
		return fmt.Errorf("server secret key not found")
	}
	settings.ServerSignature = secretKey
	if githubWebhookSecret := argoCDSecret.Data[settingsWebhookGitHubSecretKey]; len(githubWebhookSecret) > 0 {
		settings.WebhookGitHubSecret = string(githubWebhookSecret)
	}
	if gitlabWebhookSecret := argoCDSecret.Data[settingsWebhookGitLabSecretKey]; len(gitlabWebhookSecret) > 0 {
		settings.WebhookGitLabSecret = string(gitlabWebhookSecret)
	}
	if bitbucketWebhookUUID := argoCDSecret.Data[settingsWebhookBitbucketUUIDKey]; len(bitbucketWebhookUUID) > 0 {
		settings.WebhookBitbucketUUID = string(bitbucketWebhookUUID)
	}

	serverCert, certOk := argoCDSecret.Data[settingServerCertificate]
	serverKey, keyOk := argoCDSecret.Data[settingServerPrivateKey]
	if certOk && keyOk {
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		if err != nil {
			return fmt.Errorf("invalid x509 key pair %s/%s in secret: %s", settingServerCertificate, settingServerPrivateKey, err)
		}
		settings.Certificate = &cert
	}
	secretValues := make(map[string]string, len(argoCDSecret.Data))
	for k, v := range argoCDSecret.Data {
		secretValues[k] = string(v)
	}
	settings.Secrets = secretValues
	return nil
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
		}
		createCM = true
	}
	if argoCDCM.Data == nil {
		argoCDCM.Data = make(map[string]string)
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
	if settings.WebhookGitHubSecret != "" {
		argoCDSecret.StringData[settingsWebhookGitHubSecretKey] = settings.WebhookGitHubSecret
	}
	if settings.WebhookGitLabSecret != "" {
		argoCDSecret.StringData[settingsWebhookGitLabSecretKey] = settings.WebhookGitLabSecret
	}
	if settings.WebhookBitbucketUUID != "" {
		argoCDSecret.StringData[settingsWebhookBitbucketUUIDKey] = settings.WebhookBitbucketUUID
	}
	if settings.Certificate != nil {
		cert, key := tlsutil.EncodeX509KeyPairString(*settings.Certificate)
		argoCDSecret.StringData[settingServerCertificate] = cert
		argoCDSecret.StringData[settingServerPrivateKey] = key
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
		mutex:     &sync.Mutex{},
	}
}

// IsSSOConfigured returns whether or not single-sign-on is configured
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

// TLSConfig returns a tls.Config with the configured certificates
func (a *ArgoCDSettings) TLSConfig() *tls.Config {
	if a.Certificate == nil {
		return nil
	}
	certPool := x509.NewCertPool()
	pemCertBytes, _ := tlsutil.EncodeX509KeyPair(*a.Certificate)
	ok := certPool.AppendCertsFromPEM(pemCertBytes)
	if !ok {
		panic("bad certs")
	}
	return &tls.Config{
		RootCAs: certPool,
	}
}

func (a *ArgoCDSettings) IssuerURL() string {
	return a.URL + common.DexAPIEndpoint
}

func (a *ArgoCDSettings) RedirectURL() string {
	return a.URL + common.CallbackEndpoint
}

// OAuth2ClientSecret calculates an arbitrary, but predictable OAuth2 client secret string derived
// from the server secret. This is called by the dex startup wrapper (argocd-util rundex), as well
// as the API server, such that they both independently come to the same conclusion of what the
// OAuth2 shared client secret should be.
func (a *ArgoCDSettings) OAuth2ClientSecret() string {
	h := sha256.New()
	_, err := h.Write(a.ServerSignature)
	if err != nil {
		panic(err)
	}
	sha := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(sha)[:40]
}

// newInformers returns two new informers on the ArgoCD
func (mgr *SettingsManager) newInformers() (cache.SharedIndexInformer, cache.SharedIndexInformer) {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", common.ArgoCDConfigMapName))
		options.FieldSelector = cmFieldSelector.String()
	}
	cmInformer := v1.NewFilteredConfigMapInformer(mgr.clientset, mgr.namespace, 3*time.Minute, cache.Indexers{}, tweakConfigMap)
	tweakSecret := func(options *metav1.ListOptions) {
		secFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", common.ArgoCDSecretName))
		options.FieldSelector = secFieldSelector.String()
	}
	secInformer := v1.NewFilteredSecretInformer(mgr.clientset, mgr.namespace, 3*time.Minute, cache.Indexers{}, tweakSecret)
	return cmInformer, secInformer
}

// StartNotifier starts background goroutines to update the supplied settings instance with new updates
func (mgr *SettingsManager) StartNotifier(ctx context.Context, a *ArgoCDSettings) {
	log.Info("Starting settings notifier")
	cmInformer, secInformer := mgr.newInformers()
	cmInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if cm, ok := obj.(*apiv1.ConfigMap); ok {
					updateSettingsFromConfigMap(a, cm)
					mgr.notifySubscribers()
				}
			},
			UpdateFunc: func(old, new interface{}) {
				oldCM := old.(*apiv1.ConfigMap)
				newCM := new.(*apiv1.ConfigMap)
				if oldCM.ResourceVersion == newCM.ResourceVersion {
					return
				}
				log.Infof("%s updated", common.ArgoCDConfigMapName)
				updateSettingsFromConfigMap(a, newCM)
				mgr.notifySubscribers()
			},
		},
	)
	secInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if sec, ok := obj.(*apiv1.Secret); ok {
					if err := updateSettingsFromSecret(a, sec); err != nil {
						log.Errorf("new settings had error: %v", err)
					}
					mgr.notifySubscribers()
				}
			},
			UpdateFunc: func(old, new interface{}) {
				oldSec := old.(*apiv1.Secret)
				newSec := new.(*apiv1.Secret)
				if oldSec.ResourceVersion == newSec.ResourceVersion {
					return
				}
				log.Infof("%s updated", common.ArgoCDSecretName)
				if err := updateSettingsFromSecret(a, newSec); err != nil {
					log.Errorf("new settings had error: %v", err)
				}
				mgr.notifySubscribers()
			},
		},
	)
	log.Info("Starting configmap/secret informers")
	go func() {
		cmInformer.Run(ctx.Done())
		log.Info("configmap informer cancelled")
	}()
	go func() {
		secInformer.Run(ctx.Done())
		log.Info("secret informer cancelled")
	}()
}

// Subscribe registers a channel in which to subscribe to settings updates
func (mgr *SettingsManager) Subscribe(subCh chan<- struct{}) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	mgr.subscribers = append(mgr.subscribers, subCh)
	log.Infof("%v subscribed to settings updates", subCh)
}

// Unsubscribe unregisters a channel from receiving of settings updates
func (mgr *SettingsManager) Unsubscribe(subCh chan<- struct{}) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	for i, ch := range mgr.subscribers {
		if ch == subCh {
			mgr.subscribers = append(mgr.subscribers[:i], mgr.subscribers[i+1:]...)
			log.Infof("%v unsubscribed from settings updates", subCh)
			return
		}
	}
}

func (mgr *SettingsManager) notifySubscribers() {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	log.Infof("Notifying %d settings subscribers: %v", len(mgr.subscribers), mgr.subscribers)
	for _, sub := range mgr.subscribers {
		sub <- struct{}{}
	}
}
