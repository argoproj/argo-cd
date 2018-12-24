package settings

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/password"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// URL is the externally facing URL users will visit to reach Argo CD.
	// The value here is used when configuring SSO. Omitting this value will disable SSO.
	URL string `json:"url,omitempty"`
	// Admin superuser password storage
	AdminPasswordHash  string    `json:"adminPasswordHash,omitempty"`
	AdminPasswordMtime time.Time `json:"adminPasswordMtime,omitempty"`
	// DexConfig contains portions of a dex config yaml
	DexConfig string `json:"dexConfig,omitempty"`
	// OIDCConfigRAW holds OIDC configuration as a raw string
	OIDCConfigRAW string `json:"oidcConfig,omitempty"`
	// ServerSignature holds the key used to generate JWT tokens.
	ServerSignature []byte `json:"serverSignature,omitempty"`
	// Certificate holds the certificate/private key for the Argo CD API server.
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
	// Repositories holds list of configured git repositories
	Repositories []RepoCredentials
	// Repositories holds list of configured helm repositories
	HelmRepositories []HelmRepoCredentials
	// AppInstanceLabelKey is the configured application instance label key used to label apps. May be empty
	AppInstanceLabelKey string
}

type OIDCConfig struct {
	Name         string `json:"name,omitempty"`
	Issuer       string `json:"issuer,omitempty"`
	ClientID     string `json:"clientID,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

type RepoCredentials struct {
	URL                 string                   `json:"url,omitempty"`
	UsernameSecret      *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	PasswordSecret      *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	SshPrivateKeySecret *apiv1.SecretKeySelector `json:"sshPrivateKeySecret,omitempty"`
}

type HelmRepoCredentials struct {
	URL            string                   `json:"url,omitempty"`
	Name           string                   `json:"name,omitempty"`
	UsernameSecret *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	PasswordSecret *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	CASecret       *apiv1.SecretKeySelector `json:"caSecret,omitempty"`
	CertSecret     *apiv1.SecretKeySelector `json:"certSecret,omitempty"`
	KeySecret      *apiv1.SecretKeySelector `json:"keySecret,omitempty"`
}

const (
	// settingAdminPasswordHashKey designates the key for a root password hash inside a Kubernetes secret.
	settingAdminPasswordHashKey = "admin.password"
	// settingAdminPasswordMtimeKey designates the key for a root password mtime inside a Kubernetes secret.
	settingAdminPasswordMtimeKey = "admin.passwordMtime"
	// settingServerSignatureKey designates the key for a server secret key inside a Kubernetes secret.
	settingServerSignatureKey = "server.secretkey"
	// settingServerCertificate designates the key for the public cert used in TLS
	settingServerCertificate = "tls.crt"
	// settingServerPrivateKey designates the key for the private key used in TLS
	settingServerPrivateKey = "tls.key"
	// settingURLKey designates the key where Argo CD's external URL is set
	settingURLKey = "url"
	// repositoriesKey designates the key where ArgoCDs repositories list is set
	repositoriesKey = "repositories"
	// helmRepositoriesKey designates the key where list of helm repositories is set
	helmRepositoriesKey = "helm.repositories"
	// settingDexConfigKey designates the key for the dex config
	settingDexConfigKey = "dex.config"
	// settingsOIDCConfigKey designates the key for OIDC config
	settingsOIDCConfigKey = "oidc.config"
	// settingsWebhookGitHubSecret is the key for the GitHub shared webhook secret
	settingsWebhookGitHubSecretKey = "webhook.github.secret"
	// settingsWebhookGitLabSecret is the key for the GitLab shared webhook secret
	settingsWebhookGitLabSecretKey = "webhook.gitlab.secret"
	// settingsWebhookBitbucketUUID is the key for Bitbucket webhook UUID
	settingsWebhookBitbucketUUIDKey = "webhook.bitbucket.uuid"
	// settingsApplicationInstanceLabelKey is the key to configure injected app instance label key
	settingsApplicationInstanceLabelKey = "application.instanceLabelKey"
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

type incompleteSettingsError struct {
	message string
}

func (e *incompleteSettingsError) Error() string {
	return e.message
}

// GetSettings retrieves settings from the ArgoCDConfigMap and secret.
func (mgr *SettingsManager) GetSettings() (*ArgoCDSettings, error) {
	var settings ArgoCDSettings
	argoCDCM, err := mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	err = mgr.updateSettingsFromConfigMap(&settings, argoCDCM)
	if err != nil {
		return nil, err
	}
	argoCDSecret, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).Get(common.ArgoCDSecretName, metav1.GetOptions{})
	if err != nil {
		return &settings, err
	}
	err = updateSettingsFromSecret(&settings, argoCDSecret)
	if err != nil {
		return &settings, err
	}
	return &settings, nil
}

// MigrateLegacyRepoSettings migrates legacy (v0.10 and below) repo secrets into the v0.11 configmap
func (mgr *SettingsManager) MigrateLegacyRepoSettings(settings *ArgoCDSettings) error {
	listOpts := metav1.ListOptions{}
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{"repository"})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*req)
	listOpts.LabelSelector = labelSelector.String()
	repoSecrets, err := mgr.clientset.CoreV1().Secrets(mgr.namespace).List(listOpts)
	if err != nil {
		return err
	}
	settings.Repositories = make([]RepoCredentials, len(repoSecrets.Items))
	for i, s := range repoSecrets.Items {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(&s)
		if err != nil {
			return err
		}
		cred := RepoCredentials{URL: string(s.Data["repository"])}
		if username, ok := s.Data["username"]; ok && string(username) != "" {
			cred.UsernameSecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "username",
			}
		}
		if password, ok := s.Data["password"]; ok && string(password) != "" {
			cred.PasswordSecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "password",
			}
		}
		if sshPrivateKey, ok := s.Data["sshPrivateKey"]; ok && string(sshPrivateKey) != "" {
			cred.SshPrivateKeySecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "sshPrivateKey",
			}
		}
		settings.Repositories[i] = cred
	}
	return nil
}

func (mgr *SettingsManager) updateSettingsFromConfigMap(settings *ArgoCDSettings, argoCDCM *apiv1.ConfigMap) error {
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.OIDCConfigRAW = argoCDCM.Data[settingsOIDCConfigKey]
	settings.URL = argoCDCM.Data[settingURLKey]
	repositoriesStr := argoCDCM.Data[repositoriesKey]
	if repositoriesStr != "" {
		settings.Repositories = make([]RepoCredentials, 0)
		err := yaml.Unmarshal([]byte(repositoriesStr), &settings.Repositories)
		if err != nil {
			return err
		}
	}
	helmRepositoriesStr := argoCDCM.Data[helmRepositoriesKey]
	if helmRepositoriesStr != "" {
		settings.HelmRepositories = make([]HelmRepoCredentials, 0)
		err := yaml.Unmarshal([]byte(helmRepositoriesStr), &settings.HelmRepositories)
		if err != nil {
			return err
		}
	}
	settings.AppInstanceLabelKey = argoCDCM.Data[settingsApplicationInstanceLabelKey]
	return nil
}

// updateSettingsFromSecret transfers settings from a Kubernetes secret into an ArgoCDSettings struct.
func updateSettingsFromSecret(settings *ArgoCDSettings, argoCDSecret *apiv1.Secret) error {
	adminPasswordHash, ok := argoCDSecret.Data[settingAdminPasswordHashKey]
	if !ok {
		return &incompleteSettingsError{message: "admin.password is missing"}
	}
	settings.AdminPasswordHash = string(adminPasswordHash)
	settings.AdminPasswordMtime = time.Now().UTC()
	if adminPasswordMtimeBytes, ok := argoCDSecret.Data[settingAdminPasswordMtimeKey]; ok {
		if adminPasswordMtime, err := time.Parse(time.RFC3339, string(adminPasswordMtimeBytes)); err == nil {
			settings.AdminPasswordMtime = adminPasswordMtime
		}
	}

	secretKey, ok := argoCDSecret.Data[settingServerSignatureKey]
	if !ok {
		return &incompleteSettingsError{message: "server.secretkey is missing"}
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
			return &incompleteSettingsError{message: fmt.Sprintf("invalid x509 key pair %s/%s in secret: %s", settingServerCertificate, settingServerPrivateKey, err)}
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

// SaveSettings serializes ArgoCDSettings and upserts it into K8s secret/configmap
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
	if settings.URL != "" {
		argoCDCM.Data[settingURLKey] = settings.URL
	} else {
		delete(argoCDCM.Data, settingURLKey)
	}
	if settings.DexConfig != "" {
		argoCDCM.Data[settingDexConfigKey] = settings.DexConfig
	} else {
		delete(argoCDCM.Data, settings.DexConfig)
	}
	if settings.OIDCConfigRAW != "" {
		argoCDCM.Data[settingsOIDCConfigKey] = settings.OIDCConfigRAW
	} else {
		delete(argoCDCM.Data, settingsOIDCConfigKey)
	}
	if len(settings.Repositories) > 0 {
		yamlStr, err := yaml.Marshal(settings.Repositories)
		if err != nil {
			return err
		}
		argoCDCM.Data[repositoriesKey] = string(yamlStr)
	} else {
		delete(argoCDCM.Data, repositoriesKey)
	}
	if settings.AppInstanceLabelKey != "" {
		argoCDCM.Data[settingsApplicationInstanceLabelKey] = settings.AppInstanceLabelKey
	} else {
		delete(argoCDCM.Data, settingsApplicationInstanceLabelKey)
	}
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
	argoCDSecret.StringData[settingAdminPasswordHashKey] = settings.AdminPasswordHash
	argoCDSecret.StringData[settingAdminPasswordMtimeKey] = settings.AdminPasswordMtime.Format(time.RFC3339)
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
	if a.IsDexConfigured() {
		return true
	}
	if a.OIDCConfig() != nil {
		return true
	}
	return false
}

func (a *ArgoCDSettings) IsDexConfigured() bool {
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

func (a *ArgoCDSettings) OIDCConfig() *OIDCConfig {
	if a.OIDCConfigRAW == "" {
		return nil
	}
	var oidcConfig OIDCConfig
	err := yaml.Unmarshal([]byte(a.OIDCConfigRAW), &oidcConfig)
	if err != nil {
		log.Warnf("invalid oidc config: %v", err)
		return nil
	}
	oidcConfig.ClientSecret = ReplaceStringSecret(oidcConfig.ClientSecret, a.Secrets)
	return &oidcConfig
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
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.Issuer
	}
	if a.DexConfig != "" {
		return a.URL + common.DexAPIEndpoint
	}
	return ""
}

func (a *ArgoCDSettings) OAuth2ClientID() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.ClientID
	}
	if a.DexConfig != "" {
		return common.ArgoCDClientAppID
	}
	return ""
}

func (a *ArgoCDSettings) OAuth2ClientSecret() string {
	if oidcConfig := a.OIDCConfig(); oidcConfig != nil {
		return oidcConfig.ClientSecret
	}
	if a.DexConfig != "" {
		return a.DexOAuth2ClientSecret()
	}
	return ""
}

func (a *ArgoCDSettings) RedirectURL() string {
	return a.URL + common.CallbackEndpoint
}

// DexOAuth2ClientSecret calculates an arbitrary, but predictable OAuth2 client secret string derived
// from the server secret. This is called by the dex startup wrapper (argocd-util rundex), as well
// as the API server, such that they both independently come to the same conclusion of what the
// OAuth2 shared client secret should be.
func (a *ArgoCDSettings) DexOAuth2ClientSecret() string {
	h := sha256.New()
	_, err := h.Write(a.ServerSignature)
	if err != nil {
		panic(err)
	}
	sha := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(sha)[:40]
}

// newInformers returns two new informers for Argo CD's settings (argocd-cm and argocd-secret)
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
					err := mgr.updateSettingsFromConfigMap(a, cm)
					if err == nil {
						mgr.notifySubscribers()
					} else {
						log.Warnf("Unable to parse settings from config map: %v", err)
					}
				}
			},
			UpdateFunc: func(old, new interface{}) {
				oldCM := old.(*apiv1.ConfigMap)
				newCM := new.(*apiv1.ConfigMap)
				if oldCM.ResourceVersion == newCM.ResourceVersion {
					return
				}
				log.Infof("%s updated", common.ArgoCDConfigMapName)
				err := mgr.updateSettingsFromConfigMap(a, newCM)
				if err == nil {
					mgr.notifySubscribers()
				} else {
					log.Warnf("Unable to parse settings from config map: %v", err)
				}
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

func isIncompleteSettingsError(err error) bool {
	_, ok := err.(*incompleteSettingsError)
	return ok
}

// UpdateSettings is used to update the admin password, signature, certificate etc
func UpdateSettings(defaultPassword string, settingsMgr *SettingsManager, updateSignature bool, updateSuperuser bool, Namespace string) (*ArgoCDSettings, error) {

	cdSettings, err := settingsMgr.GetSettings()
	if err != nil && !apierr.IsNotFound(err) && !isIncompleteSettingsError(err) {
		return nil, err
	}
	if cdSettings == nil {
		cdSettings = &ArgoCDSettings{}
	}
	if cdSettings.ServerSignature == nil || updateSignature {
		// set JWT signature
		signature, err := util.MakeSignature(32)
		if err != nil {
			return nil, err
		}
		cdSettings.ServerSignature = signature
	}
	if cdSettings.AdminPasswordHash == "" || updateSuperuser {
		passwordRaw := defaultPassword
		if passwordRaw == "" {
			passwordRaw, err = cli.ReadAndConfirmPassword()
			if err != nil {
				return nil, err
			}
		}
		hashedPassword, err := password.HashPassword(passwordRaw)
		if err != nil {
			return nil, err
		}
		cdSettings.AdminPasswordHash = hashedPassword
		cdSettings.AdminPasswordMtime = time.Now().UTC()
	}

	if cdSettings.Certificate == nil {
		// generate TLS cert
		hosts := []string{
			"localhost",
			"argocd-server",
			fmt.Sprintf("argocd-server.%s", Namespace),
			fmt.Sprintf("argocd-server.%s.svc", Namespace),
			fmt.Sprintf("argocd-server.%s.svc.cluster.local", Namespace),
		}
		certOpts := tlsutil.CertOptions{
			Hosts:        hosts,
			Organization: "Argo CD",
			IsCA:         true,
		}
		cert, err := tlsutil.GenerateX509KeyPair(certOpts)
		if err != nil {
			return nil, err
		}
		cdSettings.Certificate = cert
	}

	if len(cdSettings.Repositories) == 0 {
		err = settingsMgr.MigrateLegacyRepoSettings(cdSettings)
		if err != nil {
			return nil, err
		}
	}

	return cdSettings, settingsMgr.SaveSettings(cdSettings)
}

// ReplaceStringSecret checks if given string is a secret key reference ( starts with $ ) and returns corresponding value from provided map
func ReplaceStringSecret(val string, secretValues map[string]string) string {
	if val == "" || !strings.HasPrefix(val, "$") {
		return val
	}
	secretKey := val[1:]
	secretVal, ok := secretValues[secretKey]
	if !ok {
		log.Warnf("config referenced '%s', but key does not exist in secret", val)
		return val
	}
	return secretVal
}

func (a *ArgoCDSettings) GetAppInstanceLabelKey() string {
	if a.AppInstanceLabelKey == "" {
		return common.LabelKeyAppInstance
	}
	return a.AppInstanceLabelKey
}
