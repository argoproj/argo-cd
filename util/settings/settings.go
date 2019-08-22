package settings

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	apiv1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/password"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
)

// ArgoCDSettings holds in-memory runtime configuration options.
type ArgoCDSettings struct {
	// URL is the externally facing URL users will visit to reach Argo CD.
	// The value here is used when configuring SSO. Omitting this value will disable SSO.
	URL string `json:"url,omitempty"`
	// Indicates if status badge is enabled or not.
	StatusBadgeEnabled bool `json:"statusBadgeEnable"`
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
	// WebhookBitbucketServerSecret holds the shared secret for authenticating BitbucketServer webhook events
	WebhookBitbucketServerSecret string `json:"webhookBitbucketServerSecret,omitempty"`
	// WebhookGogsSecret holds the shared secret for authenticating Gogs webhook events
	WebhookGogsSecret string `json:"webhookGogsSecret,omitempty"`
	// Secrets holds all secrets in argocd-secret as a map[string]string
	Secrets map[string]string `json:"secrets,omitempty"`
	// KustomizeBuildOptions is a string of kustomize build parameters
	KustomizeBuildOptions string
	// Indicates if anonymous user is enabled or not
	AnonymousUserEnabled bool
}

type GoogleAnalytics struct {
	TrackingID     string `json:"trackingID,omitempty"`
	AnonymizeUsers bool   `json:"anonymizeUsers,omitempty"`
}

// Help settings
type Help struct {
	// the URL for getting chat help, this will typically be your Slack channel for support
	ChatURL string `json:"chatUrl,omitempty"`
	// the text for getting chat help, defaults to "Chat now!"
	ChatText string `json:"chatText,omitempty"`
}

type OIDCConfig struct {
	Name            string   `json:"name,omitempty"`
	Issuer          string   `json:"issuer,omitempty"`
	ClientID        string   `json:"clientID,omitempty"`
	ClientSecret    string   `json:"clientSecret,omitempty"`
	CLIClientID     string   `json:"cliClientID,omitempty"`
	RequestedScopes []string `json:"requestedScopes,omitempty"`
}

// Credentials for accessing a Git repository
type RepoCredentials struct {
	// The URL to the repository
	URL string `json:"url,omitempty"`
	// Name of the secret storing the username used to access the repo
	UsernameSecret *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	// Name of the secret storing the password used to access the repo
	PasswordSecret *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	// Name of the secret storing the SSH private key used to access the repo
	SSHPrivateKeySecret *apiv1.SecretKeySelector `json:"sshPrivateKeySecret,omitempty"`
	// Whether to connect the repository in an insecure way (deprecated)
	InsecureIgnoreHostKey bool `json:"insecureIgnoreHostKey,omitempty"`
	// Whether to connect the repository in an insecure way
	Insecure bool `json:"insecure,omitempty"`
	// Whether the repo is git-lfs enabled
	EnableLFS bool `json:"enableLfs,omitempty"`
	// Name of the secret storing the TLS client cert data
	TLSClientCertDataSecret *apiv1.SecretKeySelector `json:"tlsClientCertDataSecret,omitempty"`
	// Name of the secret storing the TLS client cert's key data
	TLSClientCertKeySecret *apiv1.SecretKeySelector `json:"tlsClientCertKeySecret,omitempty"`
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
	// gaTrackingID holds Google Analytics tracking id
	gaTrackingID = "ga.trackingid"
	// the URL for getting chat help, this will typically be your Slack channel for support
	helpChatURL = "help.chatUrl"
	// the text for getting chat help, defaults to "Chat now!"
	helpChatText = "help.chatText"
	// gaAnonymizeUsers specifies if user ids should be anonymized (hashed) before sending to Google Analytics. True unless value is set to 'false'
	gaAnonymizeUsers = "ga.anonymizeusers"
	// settingServerCertificate designates the key for the public cert used in TLS
	settingServerCertificate = "tls.crt"
	// settingServerPrivateKey designates the key for the private key used in TLS
	settingServerPrivateKey = "tls.key"
	// settingURLKey designates the key where Argo CD's external URL is set
	settingURLKey = "url"
	// repositoriesKey designates the key where ArgoCDs repositories list is set
	repositoriesKey = "repositories"
	// repositoryCredentialsKey designates the key where ArgoCDs repositories credentials list is set
	repositoryCredentialsKey = "repository.credentials"
	// helmRepositoriesKey designates the key where list of helm repositories is set
	helmRepositoriesKey = "helm.repositories"
	// settingDexConfigKey designates the key for the dex config
	settingDexConfigKey = "dex.config"
	// settingsOIDCConfigKey designates the key for OIDC config
	settingsOIDCConfigKey = "oidc.config"
	// statusBadgeEnabledKey holds the key which enables of disables status badge feature
	statusBadgeEnabledKey = "statusbadge.enabled"
	// settingsWebhookGitHubSecret is the key for the GitHub shared webhook secret
	settingsWebhookGitHubSecretKey = "webhook.github.secret"
	// settingsWebhookGitLabSecret is the key for the GitLab shared webhook secret
	settingsWebhookGitLabSecretKey = "webhook.gitlab.secret"
	// settingsWebhookBitbucketUUID is the key for Bitbucket webhook UUID
	settingsWebhookBitbucketUUIDKey = "webhook.bitbucket.uuid"
	// settingsWebhookBitbucketServerSecret is the key for BitbucketServer webhook secret
	settingsWebhookBitbucketServerSecretKey = "webhook.bitbucketserver.secret"
	// settingsWebhookGogsSecret is the key for Gogs webhook secret
	settingsWebhookGogsSecretKey = "webhook.gogs.secret"
	// settingsApplicationInstanceLabelKey is the key to configure injected app instance label key
	settingsApplicationInstanceLabelKey = "application.instanceLabelKey"
	// resourcesCustomizationsKey is the key to the map of resource overrides
	resourceCustomizationsKey = "resource.customizations"
	// resourceExclusions is the key to the list of excluded resources
	resourceExclusionsKey = "resource.exclusions"
	// resourceInclusions is the key to the list of explicitly watched resources
	resourceInclusionsKey = "resource.inclusions"
	// configManagementPluginsKey is the key to the list of config management plugins
	configManagementPluginsKey = "configManagementPlugins"
	// kustomizeBuildOptions is a string of kustomize build parameters
	kustomizeBuildOptions = "kustomize.buildOptions"
	// anonymousUserEnabledKey is the key which enables or disables anonymous user
	anonymousUserEnabledKey = "users.anonymous.enabled"
)

// SettingsManager holds config info for a new manager with which to access Kubernetes ConfigMaps.
type SettingsManager struct {
	ctx        context.Context
	clientset  kubernetes.Interface
	secrets    v1listers.SecretLister
	configmaps v1listers.ConfigMapLister
	namespace  string
	// subscribers is a list of subscribers to settings updates
	subscribers []chan<- *ArgoCDSettings
	// mutex protects concurrency sensitive parts of settings manager: access to subscribers list and initialization flag
	mutex             *sync.Mutex
	initContextCancel func()
}

type incompleteSettingsError struct {
	message string
}

func (e *incompleteSettingsError) Error() string {
	return e.message
}

func (mgr *SettingsManager) GetSecretsLister() (v1listers.SecretLister, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	return mgr.secrets, nil
}

func (mgr *SettingsManager) getConfigMap() (*apiv1.ConfigMap, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, err
	}
	if argoCDCM.Data == nil {
		argoCDCM.Data = make(map[string]string)
	}
	return argoCDCM, err
}

// Returns the ConfigMap with the given name from the cluster.
// The ConfigMap must be labeled with "app.kubernetes.io/part-of: argocd" in
// order to be retrievable.
func (mgr *SettingsManager) GetConfigMapByName(configMapName string) (*apiv1.ConfigMap, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	configMap, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(configMapName)
	if err != nil {
		return nil, err
	}
	return configMap, err
}

func (mgr *SettingsManager) GetResourcesFilter() (*ResourcesFilter, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	rf := &ResourcesFilter{}
	if value, ok := argoCDCM.Data[resourceInclusionsKey]; ok {
		includedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &includedResources)
		if err != nil {
			return nil, err
		}
		rf.ResourceInclusions = includedResources
	}

	if value, ok := argoCDCM.Data[resourceExclusionsKey]; ok {
		excludedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &excludedResources)
		if err != nil {
			return nil, err
		}
		rf.ResourceExclusions = excludedResources
	}
	return rf, nil
}

func (mgr *SettingsManager) GetAppInstanceLabelKey() (string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return "", err
	}
	label := argoCDCM.Data[settingsApplicationInstanceLabelKey]
	if label == "" {
		return common.LabelKeyAppInstance, nil
	}
	return label, nil
}

func (mgr *SettingsManager) GetConfigManagementPlugins() ([]v1alpha1.ConfigManagementPlugin, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	plugins := make([]v1alpha1.ConfigManagementPlugin, 0)
	if value, ok := argoCDCM.Data[configManagementPluginsKey]; ok {
		err := yaml.Unmarshal([]byte(value), &plugins)
		if err != nil {
			return nil, err
		}
	}
	return plugins, nil
}

// GetResourceOverrides loads Resource Overrides from argocd-cm ConfigMap
func (mgr *SettingsManager) GetResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	resourceOverrides := map[string]v1alpha1.ResourceOverride{}
	if value, ok := argoCDCM.Data[resourceCustomizationsKey]; ok {
		err := yaml.Unmarshal([]byte(value), &resourceOverrides)
		if err != nil {
			return nil, err
		}
	}

	return resourceOverrides, nil
}

// GetKustomizeBuildOptions loads the kustomize build options from argocd-cm ConfigMap
func (mgr *SettingsManager) GetKustomizeBuildOptions() (string, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return "", err
	}
	return argoCDCM.Data[kustomizeBuildOptions], nil
}

func (mgr *SettingsManager) GetHelmRepositories() ([]HelmRepoCredentials, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	helmRepositories := make([]HelmRepoCredentials, 0)
	helmRepositoriesStr := argoCDCM.Data[helmRepositoriesKey]
	if helmRepositoriesStr != "" {
		err := yaml.Unmarshal([]byte(helmRepositoriesStr), &helmRepositories)
		if err != nil {
			return nil, err
		}
	}
	return helmRepositories, nil
}

func (mgr *SettingsManager) GetRepositories() ([]RepoCredentials, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	repositories := make([]RepoCredentials, 0)
	repositoriesStr := argoCDCM.Data[repositoriesKey]
	if repositoriesStr != "" {
		err := yaml.Unmarshal([]byte(repositoriesStr), &repositories)
		if err != nil {
			return nil, err
		}
	}
	return repositories, nil
}

func (mgr *SettingsManager) SaveRepositories(repos []RepoCredentials) error {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return err
	}
	if len(repos) > 0 {
		yamlStr, err := yaml.Marshal(repos)
		if err != nil {
			return err
		}
		argoCDCM.Data[repositoriesKey] = string(yamlStr)
	} else {
		delete(argoCDCM.Data, repositoriesKey)
	}
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(argoCDCM)
	return err
}

func (mgr *SettingsManager) GetRepositoryCredentials() ([]RepoCredentials, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}

	repositoryCredentials := make([]RepoCredentials, 0)
	repositoryCredentialsStr := argoCDCM.Data[repositoryCredentialsKey]
	if repositoryCredentialsStr != "" {
		err := yaml.Unmarshal([]byte(repositoryCredentialsStr), &repositoryCredentials)
		if err != nil {
			return nil, err
		}
	}
	return repositoryCredentials, nil
}

func (mgr *SettingsManager) GetGoogleAnalytics() (*GoogleAnalytics, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	return &GoogleAnalytics{
		TrackingID:     argoCDCM.Data[gaTrackingID],
		AnonymizeUsers: argoCDCM.Data[gaAnonymizeUsers] != "false",
	}, nil
}

func (mgr *SettingsManager) GetHelp() (*Help, error) {
	argoCDCM, err := mgr.getConfigMap()
	if err != nil {
		return nil, err
	}
	chatText, ok := argoCDCM.Data[helpChatText]
	if !ok {
		chatText = "Chat now!"
	}
	return &Help{
		ChatURL:  argoCDCM.Data[helpChatURL],
		ChatText: chatText,
	}, nil
}

// GetSettings retrieves settings from the ArgoCDConfigMap and secret.
func (mgr *SettingsManager) GetSettings() (*ArgoCDSettings, error) {
	err := mgr.ensureSynced(false)
	if err != nil {
		return nil, err
	}
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, err
	}
	argoCDSecret, err := mgr.secrets.Secrets(mgr.namespace).Get(common.ArgoCDSecretName)
	if err != nil {
		return nil, err
	}
	var settings ArgoCDSettings
	var errs []error
	updateSettingsFromConfigMap(&settings, argoCDCM)
	if err := updateSettingsFromSecret(&settings, argoCDSecret); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &settings, errs[0]
	}
	return &settings, nil
}

func (mgr *SettingsManager) initialize(ctx context.Context) error {
	tweakConfigMap := func(options *metav1.ListOptions) {
		//cmFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", common.ArgoCDConfigMapName))
		cmLabelSelector := fields.ParseSelectorOrDie("app.kubernetes.io/part-of=argocd")
		options.LabelSelector = cmLabelSelector.String()
	}

	cmInformer := v1.NewFilteredConfigMapInformer(mgr.clientset, mgr.namespace, 3*time.Minute, cache.Indexers{}, tweakConfigMap)
	secretsInformer := v1.NewSecretInformer(mgr.clientset, mgr.namespace, 3*time.Minute, cache.Indexers{})

	log.Info("Starting configmap/secret informers")
	go func() {
		cmInformer.Run(ctx.Done())
		log.Info("configmap informer cancelled")
	}()
	go func() {
		secretsInformer.Run(ctx.Done())
		log.Info("secrets informer cancelled")
	}()

	if !cache.WaitForCacheSync(ctx.Done(), cmInformer.HasSynced, secretsInformer.HasSynced) {
		return fmt.Errorf("Timed out waiting for settings cache to sync")
	}
	log.Info("Configmap/secret informer synced")

	tryNotify := func() {
		newSettings, err := mgr.GetSettings()
		if err != nil {
			log.Warnf("Unable to parse updated settings: %v", err)
		} else {
			mgr.notifySubscribers(newSettings)
		}
	}
	now := time.Now()
	handler := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if metaObj, ok := obj.(metav1.Object); ok {
				if metaObj.GetCreationTimestamp().After(now) {
					tryNotify()
				}
			}

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldMeta, oldOk := oldObj.(metav1.Common)
			newMeta, newOk := newObj.(metav1.Common)
			if oldOk && newOk && oldMeta.GetResourceVersion() != newMeta.GetResourceVersion() {
				tryNotify()
			}
		},
	}
	secretsInformer.AddEventHandler(handler)
	cmInformer.AddEventHandler(handler)
	mgr.secrets = v1listers.NewSecretLister(secretsInformer.GetIndexer())
	mgr.configmaps = v1listers.NewConfigMapLister(cmInformer.GetIndexer())
	return nil
}

func (mgr *SettingsManager) ensureSynced(forceResync bool) error {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	if !forceResync && mgr.secrets != nil && mgr.configmaps != nil {
		return nil
	}

	if !forceResync && mgr.secrets != nil && mgr.configmaps != nil {
		return nil
	}
	if mgr.initContextCancel != nil {
		mgr.initContextCancel()
	}
	ctx, cancel := context.WithCancel(mgr.ctx)
	mgr.initContextCancel = cancel
	return mgr.initialize(ctx)
}

func updateSettingsFromConfigMap(settings *ArgoCDSettings, argoCDCM *apiv1.ConfigMap) {
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.OIDCConfigRAW = argoCDCM.Data[settingsOIDCConfigKey]
	settings.URL = argoCDCM.Data[settingURLKey]
	settings.StatusBadgeEnabled = argoCDCM.Data[statusBadgeEnabledKey] == "true"
	settings.AnonymousUserEnabled = argoCDCM.Data[anonymousUserEnabledKey] == "true"
}

// updateSettingsFromSecret transfers settings from a Kubernetes secret into an ArgoCDSettings struct.
func updateSettingsFromSecret(settings *ArgoCDSettings, argoCDSecret *apiv1.Secret) error {
	var errs []error
	adminPasswordHash, ok := argoCDSecret.Data[settingAdminPasswordHashKey]
	if ok {
		settings.AdminPasswordHash = string(adminPasswordHash)
	} else {
		errs = append(errs, &incompleteSettingsError{message: "admin.password is missing"})
	}
	adminPasswordMtimeBytes, ok := argoCDSecret.Data[settingAdminPasswordMtimeKey]
	if ok {
		if adminPasswordMtime, err := time.Parse(time.RFC3339, string(adminPasswordMtimeBytes)); err == nil {
			settings.AdminPasswordMtime = adminPasswordMtime
		}
	}
	secretKey, ok := argoCDSecret.Data[settingServerSignatureKey]
	if ok {
		settings.ServerSignature = secretKey
	} else {
		errs = append(errs, &incompleteSettingsError{message: "server.secretkey is missing"})
	}
	if githubWebhookSecret := argoCDSecret.Data[settingsWebhookGitHubSecretKey]; len(githubWebhookSecret) > 0 {
		settings.WebhookGitHubSecret = string(githubWebhookSecret)
	}
	if gitlabWebhookSecret := argoCDSecret.Data[settingsWebhookGitLabSecretKey]; len(gitlabWebhookSecret) > 0 {
		settings.WebhookGitLabSecret = string(gitlabWebhookSecret)
	}
	if bitbucketWebhookUUID := argoCDSecret.Data[settingsWebhookBitbucketUUIDKey]; len(bitbucketWebhookUUID) > 0 {
		settings.WebhookBitbucketUUID = string(bitbucketWebhookUUID)
	}
	if bitbucketserverWebhookSecret := argoCDSecret.Data[settingsWebhookBitbucketServerSecretKey]; len(bitbucketserverWebhookSecret) > 0 {
		settings.WebhookBitbucketServerSecret = string(bitbucketserverWebhookSecret)
	}
	if gogsWebhookSecret := argoCDSecret.Data[settingsWebhookGogsSecretKey]; len(gogsWebhookSecret) > 0 {
		settings.WebhookGogsSecret = string(gogsWebhookSecret)
	}

	serverCert, certOk := argoCDSecret.Data[settingServerCertificate]
	serverKey, keyOk := argoCDSecret.Data[settingServerPrivateKey]
	if certOk && keyOk {
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		if err != nil {
			errs = append(errs, &incompleteSettingsError{message: fmt.Sprintf("invalid x509 key pair %s/%s in secret: %s", settingServerCertificate, settingServerPrivateKey, err)})
		} else {
			settings.Certificate = &cert
		}
	}
	secretValues := make(map[string]string, len(argoCDSecret.Data))
	for k, v := range argoCDSecret.Data {
		secretValues[k] = string(v)
	}
	settings.Secrets = secretValues
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// SaveSettings serializes ArgoCDSettings and upserts it into K8s secret/configmap
func (mgr *SettingsManager) SaveSettings(settings *ArgoCDSettings) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	// Upsert the config data
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
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

	if createCM {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Create(argoCDCM)
	} else {
		_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(argoCDCM)
	}
	if err != nil {
		return err
	}

	// Upsert the secret data. Ensure we do not delete any extra keys which user may have added
	argoCDSecret, err := mgr.secrets.Secrets(mgr.namespace).Get(common.ArgoCDSecretName)
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
	if argoCDSecret.Data == nil {
		argoCDSecret.Data = make(map[string][]byte)
	}

	argoCDSecret.Data[settingServerSignatureKey] = settings.ServerSignature
	argoCDSecret.Data[settingAdminPasswordHashKey] = []byte(settings.AdminPasswordHash)
	argoCDSecret.Data[settingAdminPasswordMtimeKey] = []byte(settings.AdminPasswordMtime.Format(time.RFC3339))
	if settings.WebhookGitHubSecret != "" {
		argoCDSecret.Data[settingsWebhookGitHubSecretKey] = []byte(settings.WebhookGitHubSecret)
	}
	if settings.WebhookGitLabSecret != "" {
		argoCDSecret.Data[settingsWebhookGitLabSecretKey] = []byte(settings.WebhookGitLabSecret)
	}
	if settings.WebhookBitbucketUUID != "" {
		argoCDSecret.Data[settingsWebhookBitbucketUUIDKey] = []byte(settings.WebhookBitbucketUUID)
	}
	if settings.WebhookBitbucketServerSecret != "" {
		argoCDSecret.Data[settingsWebhookBitbucketServerSecretKey] = []byte(settings.WebhookBitbucketServerSecret)
	}
	if settings.WebhookGogsSecret != "" {
		argoCDSecret.Data[settingsWebhookGogsSecretKey] = []byte(settings.WebhookGogsSecret)
	}
	if settings.Certificate != nil {
		cert, key := tlsutil.EncodeX509KeyPair(*settings.Certificate)
		argoCDSecret.Data[settingServerCertificate] = cert
		argoCDSecret.Data[settingServerPrivateKey] = key
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
	return mgr.ResyncInformers()
}

// Save the SSH known host data into the corresponding ConfigMap
func (mgr *SettingsManager) SaveSSHKnownHostsData(ctx context.Context, knownHostsList []string) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	certCM, err := mgr.GetConfigMapByName(common.ArgoCDKnownHostsConfigMapName)
	if err != nil {
		return err
	}

	if certCM.Data == nil {
		certCM.Data = make(map[string]string)
	}

	sshKnownHostsData := strings.Join(knownHostsList, "\n") + "\n"
	certCM.Data["ssh_known_hosts"] = sshKnownHostsData
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(certCM)
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

func (mgr *SettingsManager) SaveTLSCertificateData(ctx context.Context, tlsCertificates map[string]string) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	certCM, err := mgr.GetConfigMapByName(common.ArgoCDTLSCertsConfigMapName)
	if err != nil {
		return err
	}

	certCM.Data = tlsCertificates
	_, err = mgr.clientset.CoreV1().ConfigMaps(mgr.namespace).Update(certCM)
	if err != nil {
		return err
	}

	return mgr.ResyncInformers()
}

// NewSettingsManager generates a new SettingsManager pointer and returns it
func NewSettingsManager(ctx context.Context, clientset kubernetes.Interface, namespace string) *SettingsManager {

	mgr := &SettingsManager{
		ctx:       ctx,
		clientset: clientset,
		namespace: namespace,
		mutex:     &sync.Mutex{},
	}

	return mgr
}

func (mgr *SettingsManager) ResyncInformers() error {
	return mgr.ensureSynced(true)
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

// Subscribe registers a channel in which to subscribe to settings updates
func (mgr *SettingsManager) Subscribe(subCh chan<- *ArgoCDSettings) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	mgr.subscribers = append(mgr.subscribers, subCh)
	log.Infof("%v subscribed to settings updates", subCh)
}

// Unsubscribe unregisters a channel from receiving of settings updates
func (mgr *SettingsManager) Unsubscribe(subCh chan<- *ArgoCDSettings) {
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

func (mgr *SettingsManager) notifySubscribers(newSettings *ArgoCDSettings) {
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()
	if len(mgr.subscribers) > 0 {
		log.Infof("Notifying %d settings subscribers: %v", len(mgr.subscribers), mgr.subscribers)
		for _, sub := range mgr.subscribers {
			sub <- newSettings
		}
	}
}

func isIncompleteSettingsError(err error) bool {
	_, ok := err.(*incompleteSettingsError)
	return ok
}

// InitializeSettings is used to initialize empty admin password, signature, certificate etc if missing
func (mgr *SettingsManager) InitializeSettings(insecureModeEnabled bool) (*ArgoCDSettings, error) {
	cdSettings, err := mgr.GetSettings()
	if err != nil && !isIncompleteSettingsError(err) {
		return nil, err
	}
	if cdSettings == nil {
		cdSettings = &ArgoCDSettings{}
	}
	if cdSettings.ServerSignature == nil {
		// set JWT signature
		signature, err := util.MakeSignature(32)
		if err != nil {
			return nil, err
		}
		cdSettings.ServerSignature = signature
		log.Info("Initialized server signature")
	}
	if cdSettings.AdminPasswordHash == "" {
		defaultPassword, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		hashedPassword, err := password.HashPassword(defaultPassword)
		if err != nil {
			return nil, err
		}
		cdSettings.AdminPasswordHash = hashedPassword
		cdSettings.AdminPasswordMtime = time.Now().UTC()
		log.Info("Initialized admin password")
	}
	if cdSettings.AdminPasswordMtime.IsZero() {
		cdSettings.AdminPasswordMtime = time.Now().UTC()
		log.Info("Initialized admin mtime")
	}

	if cdSettings.Certificate == nil && !insecureModeEnabled {
		// generate TLS cert
		hosts := []string{
			"localhost",
			"argocd-server",
			fmt.Sprintf("argocd-server.%s", mgr.namespace),
			fmt.Sprintf("argocd-server.%s.svc", mgr.namespace),
			fmt.Sprintf("argocd-server.%s.svc.cluster.local", mgr.namespace),
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
		log.Info("Initialized TLS certificate")
	}

	err = mgr.SaveSettings(cdSettings)
	if apierrors.IsConflict(err) {
		// assume settings are initialized by another instance of api server
		log.Warnf("conflict when initializing settings. assuming updated by another replica")
		return mgr.GetSettings()
	}
	return cdSettings, nil
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
