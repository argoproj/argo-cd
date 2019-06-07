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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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
	// Repositories holds list of repo credentials
	RepositoryCredentials []RepoCredentials
	// Repositories holds list of configured helm repositories
	HelmRepositories []HelmRepoCredentials
	// AppInstanceLabelKey is the configured application instance label key used to label apps. May be empty
	AppInstanceLabelKey string
	// ConfigManagementPlugins hols list of configured config management plugins
	ConfigManagementPlugins []v1alpha1.ConfigManagementPlugin
	// ResourceOverrides holds the overrides for specific resources. The keys are in the format of `group/kind`
	// (e.g. argoproj.io/rollout) for the resource that is being overridden
	ResourceOverrides map[string]v1alpha1.ResourceOverride
	// ResourceExclusions holds the api groups, kinds per cluster to exclude from Argo CD's watch
	ResourceExclusions []FilteredResource
	// ResourceInclusions holds the only api groups, kinds per cluster that Argo CD will watch
	ResourceInclusions []FilteredResource
}

type OIDCConfig struct {
	Name            string   `json:"name,omitempty"`
	Issuer          string   `json:"issuer,omitempty"`
	ClientID        string   `json:"clientID,omitempty"`
	ClientSecret    string   `json:"clientSecret,omitempty"`
	CLIClientID     string   `json:"cliClientID,omitempty"`
	RequestedScopes []string `json:"requestedScopes,omitempty"`
}

type RepoCredentials struct {
	URL                   string                   `json:"url,omitempty"`
	UsernameSecret        *apiv1.SecretKeySelector `json:"usernameSecret,omitempty"`
	PasswordSecret        *apiv1.SecretKeySelector `json:"passwordSecret,omitempty"`
	SSHPrivateKeySecret   *apiv1.SecretKeySelector `json:"sshPrivateKeySecret,omitempty"`
	InsecureIgnoreHostKey bool                     `json:"insecureIgnoreHostKey,omitempty"`
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
	// repositoryCredentialsKey designates the key where ArgoCDs repositories credentials list is set
	repositoryCredentialsKey = "repository.credentials"
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
	// resourcesCustomizationsKey is the key to the map of resource overrides
	resourceCustomizationsKey = "resource.customizations"
	// resourceExclusions is the key to the list of excluded resources
	resourceExclusionsKey = "resource.exclusions"
	// resourceInclusions is the key to the list of explicitly watched resources
	resourceInclusionsKey = "resource.inclusions"
	// configManagementPluginsKey is the key to the list of config management plugins
	configManagementPluginsKey = "configManagementPlugins"
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

// GetResouceOverrides loads Resource Overrides from argocd-cm ConfigMap
func (mgr *SettingsManager) GetResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	argoCDCM, err := mgr.configmaps.ConfigMaps(mgr.namespace).Get(common.ArgoCDConfigMapName)
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := getResourceOverridesFromConfigMap(argoCDCM)
	if err != nil {
		return nil, err
	}

	return resourceOverrides, nil
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
	if err := updateSettingsFromConfigMap(&settings, argoCDCM); err != nil {
		errs = append(errs, err)
	}
	if err := updateSettingsFromSecret(&settings, argoCDSecret); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return &settings, errs[0]
	}
	return &settings, nil
}

// MigrateLegacyRepoSettings migrates legacy (v0.10 and below) repo secrets into the v0.11 configmap
func (mgr *SettingsManager) MigrateLegacyRepoSettings(settings *ArgoCDSettings) error {
	err := mgr.ensureSynced(false)
	if err != nil {
		return err
	}

	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{"repository"})
	if err != nil {
		return err
	}
	labelSelector = labelSelector.Add(*req)
	repoSecrets, err := mgr.secrets.Secrets(mgr.namespace).List(labelSelector)
	if err != nil {
		return err
	}
	settings.Repositories = make([]RepoCredentials, len(repoSecrets))
	for i, s := range repoSecrets {
		_, err = mgr.clientset.CoreV1().Secrets(mgr.namespace).Update(s)
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
			cred.SSHPrivateKeySecret = &apiv1.SecretKeySelector{
				LocalObjectReference: apiv1.LocalObjectReference{Name: s.Name},
				Key:                  "sshPrivateKey",
			}
		}
		settings.Repositories[i] = cred
	}
	return nil
}

func (mgr *SettingsManager) initialize(ctx context.Context) error {
	tweakConfigMap := func(options *metav1.ListOptions) {
		cmFieldSelector := fields.ParseSelectorOrDie(fmt.Sprintf("metadata.name=%s", common.ArgoCDConfigMapName))
		options.FieldSelector = cmFieldSelector.String()
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
	if !forceResync && mgr.secrets != nil && mgr.configmaps != nil {
		return nil
	}
	mgr.mutex.Lock()
	defer mgr.mutex.Unlock()

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

func updateSettingsFromConfigMap(settings *ArgoCDSettings, argoCDCM *apiv1.ConfigMap) error {
	settings.DexConfig = argoCDCM.Data[settingDexConfigKey]
	settings.OIDCConfigRAW = argoCDCM.Data[settingsOIDCConfigKey]
	settings.URL = argoCDCM.Data[settingURLKey]
	settings.AppInstanceLabelKey = argoCDCM.Data[settingsApplicationInstanceLabelKey]
	repositoriesStr := argoCDCM.Data[repositoriesKey]
	repositoryCredentialsStr := argoCDCM.Data[repositoryCredentialsKey]
	var errors []error
	if repositoriesStr != "" {
		repositories := make([]RepoCredentials, 0)
		err := yaml.Unmarshal([]byte(repositoriesStr), &repositories)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.Repositories = repositories
		}
	}
	if repositoryCredentialsStr != "" {
		repositoryCredentials := make([]RepoCredentials, 0)
		err := yaml.Unmarshal([]byte(repositoryCredentialsStr), &repositoryCredentials)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.RepositoryCredentials = repositoryCredentials
		}
	}
	helmRepositoriesStr := argoCDCM.Data[helmRepositoriesKey]
	if helmRepositoriesStr != "" {
		helmRepositories := make([]HelmRepoCredentials, 0)
		err := yaml.Unmarshal([]byte(helmRepositoriesStr), &helmRepositories)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.HelmRepositories = helmRepositories
		}
	}

	resourceOverrides, err := getResourceOverridesFromConfigMap(argoCDCM)
	if err != nil {
		errors = append(errors, err)
	} else {
		settings.ResourceOverrides = resourceOverrides
	}

	if value, ok := argoCDCM.Data[resourceInclusionsKey]; ok {
		includedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &includedResources)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.ResourceInclusions = includedResources
		}
	}

	if value, ok := argoCDCM.Data[resourceExclusionsKey]; ok {
		excludedResources := make([]FilteredResource, 0)
		err := yaml.Unmarshal([]byte(value), &excludedResources)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.ResourceExclusions = excludedResources
		}
	}

	if value, ok := argoCDCM.Data[configManagementPluginsKey]; ok {
		tools := make([]v1alpha1.ConfigManagementPlugin, 0)
		err := yaml.Unmarshal([]byte(value), &tools)
		if err != nil {
			errors = append(errors, err)
		} else {
			settings.ConfigManagementPlugins = tools
		}
	}

	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}

func getResourceOverridesFromConfigMap(argoCDCM *apiv1.ConfigMap) (map[string]v1alpha1.ResourceOverride, error) {
	resourceOverrides := map[string]v1alpha1.ResourceOverride{}
	if value, ok := argoCDCM.Data[resourceCustomizationsKey]; ok {
		err := yaml.Unmarshal([]byte(value), &resourceOverrides)
		if err != nil {
			return nil, err
		}
	}
	return resourceOverrides, nil
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
	if len(settings.Repositories) > 0 {
		yamlStr, err := yaml.Marshal(settings.Repositories)
		if err != nil {
			return err
		}
		argoCDCM.Data[repositoriesKey] = string(yamlStr)
	} else {
		delete(argoCDCM.Data, repositoriesKey)
	}
	if len(settings.RepositoryCredentials) > 0 {
		yamlStr, err := yaml.Marshal(settings.RepositoryCredentials)
		if err != nil {
			return err
		}
		argoCDCM.Data[repositoryCredentialsKey] = string(yamlStr)
	} else {
		delete(argoCDCM.Data, repositoryCredentialsKey)
	}
	if settings.AppInstanceLabelKey != "" {
		argoCDCM.Data[settingsApplicationInstanceLabelKey] = settings.AppInstanceLabelKey
	} else {
		delete(argoCDCM.Data, settingsApplicationInstanceLabelKey)
	}

	if len(settings.ResourceOverrides) > 0 {
		yamlBytes, err := yaml.Marshal(settings.ResourceOverrides)
		if err != nil {
			return err
		}
		argoCDCM.Data[resourceCustomizationsKey] = string(yamlBytes)
	} else {
		delete(argoCDCM.Data, resourceCustomizationsKey)
	}

	if len(settings.ResourceInclusions) > 0 {
		yamlBytes, err := yaml.Marshal(settings.ResourceInclusions)
		if err != nil {
			return err
		}
		argoCDCM.Data[resourceInclusionsKey] = string(yamlBytes)
	} else {
		delete(argoCDCM.Data, resourceInclusionsKey)
	}

	if len(settings.ResourceExclusions) > 0 {
		yamlBytes, err := yaml.Marshal(settings.ResourceExclusions)
		if err != nil {
			return err
		}
		argoCDCM.Data[resourceExclusionsKey] = string(yamlBytes)
	} else {
		delete(argoCDCM.Data, resourceExclusionsKey)
	}

	if len(settings.ConfigManagementPlugins) > 0 {
		yamlBytes, err := yaml.Marshal(settings.ConfigManagementPlugins)
		if err != nil {
			return err
		}
		argoCDCM.Data[configManagementPluginsKey] = string(yamlBytes)
	} else {
		delete(argoCDCM.Data, configManagementPluginsKey)
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
func (mgr *SettingsManager) InitializeSettings() (*ArgoCDSettings, error) {
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

	if cdSettings.Certificate == nil {
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

	if len(cdSettings.Repositories) == 0 {
		err = mgr.MigrateLegacyRepoSettings(cdSettings)
		if err != nil {
			return nil, err
		}
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

func (a *ArgoCDSettings) GetAppInstanceLabelKey() string {
	if a.AppInstanceLabelKey == "" {
		return common.LabelKeyAppInstance
	}
	return a.AppInstanceLabelKey
}

func (a *ArgoCDSettings) getExcludedResources() []FilteredResource {
	coreExcludedResources := []FilteredResource{
		{APIGroups: []string{"events.k8s.io", "metrics.k8s.io"}},
		{APIGroups: []string{""}, Kinds: []string{"Event"}},
	}
	return append(coreExcludedResources, a.ResourceExclusions...)
}

func (a *ArgoCDSettings) checkResourcePresence(apiGroup, kind, cluster string, filteredResources []FilteredResource) bool {

	for _, includedResource := range filteredResources {
		if includedResource.Match(apiGroup, kind, cluster) {
			return true
		}
	}

	return false
}

func (a *ArgoCDSettings) isIncludedResource(apiGroup, kind, cluster string) bool {
	return a.checkResourcePresence(apiGroup, kind, cluster, a.ResourceInclusions)
}

func (a *ArgoCDSettings) isExcludedResource(apiGroup, kind, cluster string) bool {
	return a.checkResourcePresence(apiGroup, kind, cluster, a.getExcludedResources())
}

// Behavior of this function is as follows:
// +-------------+-------------+-------------+
// |  Inclusions |  Exclusions |    Result   |
// +-------------+-------------+-------------+
// |    Empty    |    Empty    |   Allowed   |
// +-------------+-------------+-------------+
// |   Present   |    Empty    |   Allowed   |
// +-------------+-------------+-------------+
// | Not Present |    Empty    | Not Allowed |
// +-------------+-------------+-------------+
// |    Empty    |   Present   | Not Allowed |
// +-------------+-------------+-------------+
// |    Empty    | Not Present |   Allowed   |
// +-------------+-------------+-------------+
// |   Present   | Not Present |   Allowed   |
// +-------------+-------------+-------------+
// | Not Present |   Present   | Not Allowed |
// +-------------+-------------+-------------+
// | Not Present | Not Present | Not Allowed |
// +-------------+-------------+-------------+
// |   Present   |   Present   | Not Allowed |
// +-------------+-------------+-------------+
//
func (a *ArgoCDSettings) IsExcludedResource(apiGroup, kind, cluster string) bool {
	if len(a.ResourceInclusions) > 0 {
		if a.isIncludedResource(apiGroup, kind, cluster) {
			return a.isExcludedResource(apiGroup, kind, cluster)
		} else {
			return true
		}
	} else {
		return a.isExcludedResource(apiGroup, kind, cluster)
	}
}
