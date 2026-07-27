package configbus

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// CRDSource reads the singleton ArgoCDConfiguration when present.
// Absent CR (or CRD) → Config() returns nil and CRDProvider getters return
// ErrNotConfigured so HybridProvider falls back to LegacyProvider.
type CRDSource interface {
	Config() *v1alpha1.ArgoCDConfiguration
}

// CRDProvider resolves config only from the ArgoCDConfiguration CRD.
// A nil source (or nil Config) makes every getter return ErrNotConfigured.
// Unimplemented field getters fall through to the embedded empty ChainProvider.
type CRDProvider struct {
	ChainProvider
	source CRDSource
}

// NewCRDProvider constructs a CRDProvider. Pass nil when no CRD source is wired.
func NewCRDProvider(source CRDSource) *CRDProvider {
	return &CRDProvider{source: source}
}

// Ensure CRDProvider implements Provider.
var _ Provider = (*CRDProvider)(nil)

func (p *CRDProvider) Subscribe(_ chan<- *settings.ArgoCDSettings) {}

func (p *CRDProvider) Unsubscribe(_ chan<- *settings.ArgoCDSettings) {}

func (p *CRDProvider) SubscribeCRD(subCh chan<- struct{}) {
	if p == nil || p.source == nil {
		return
	}
	if n, ok := p.source.(CRDChangeNotifier); ok {
		n.Subscribe(subCh)
	}
}

func (p *CRDProvider) UnsubscribeCRD(subCh chan<- struct{}) {
	if p == nil || p.source == nil {
		return
	}
	if n, ok := p.source.(CRDChangeNotifier); ok {
		n.Unsubscribe(subCh)
	}
}

func (p *CRDProvider) Configuration(_ context.Context) (*v1alpha1.ArgoCDConfiguration, error) {
	if p == nil || p.source == nil {
		return nil, ErrNotConfigured
	}
	cfg := p.source.Config()
	if cfg == nil {
		return nil, ErrNotConfigured
	}
	return cfg, nil
}

func (p *CRDProvider) AllowedNodeLabels(_ context.Context) ([]string, error) {
	return requireCRDField(p, "AllowedNodeLabels", crdAllowedNodeLabels)
}

func (p *CRDProvider) AppInstanceLabelKey(_ context.Context) (string, error) {
	return requireCRDField(p, "AppInstanceLabelKey", crdAppInstanceLabelKey)
}

func (p *CRDProvider) CommitAuthorEmail(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitAuthorEmail", crdCommitAuthorEmail)
}

func (p *CRDProvider) CommitAuthorName(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitAuthorName", crdCommitAuthorName)
}

func (p *CRDProvider) EnabledSourceTypes(_ context.Context) (map[string]bool, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) GitRequestTimeout(_ context.Context) (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) HardReconciliationTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "HardReconciliationTimeout", crdHardReconciliationTimeout)
}

func (p *CRDProvider) HelmSettings(_ context.Context) (v1alpha1.HelmOptions, error) {
	v, err := requireCRDFieldErr(p, "HelmSettings", crdHelmSettings)
	if err != nil {
		return v1alpha1.HelmOptions{}, err
	}
	if v == nil {
		return v1alpha1.HelmOptions{}, nil
	}
	return *v, nil
}

func (p *CRDProvider) HydratorReadmeTemplate(_ context.Context) (string, error) {
	return requireCRDField(p, "HydratorReadmeTemplate", crdSourceHydratorReadmeMessageTemplate)
}

func (p *CRDProvider) IgnoreNormalizerJQTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "IgnoreNormalizerJQTimeout", crdControllerIgnoreNormalizerJqTimeout)
}


func (p *CRDProvider) IgnoreResourceUpdatesOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) InstallationID(_ context.Context) (string, error) {
	return requireCRDField(p, "InstallationID", crdInstallationID)
}

func (p *CRDProvider) IsIgnoreResourceUpdatesEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "IsIgnoreResourceUpdatesEnabled", crdIgnoreResourceUpdatesEnabled)
}

func (p *CRDProvider) IsImpersonationEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "IsImpersonationEnabled", crdCfgImpersonationEnabled)
}

func (p *CRDProvider) IsImpersonationEnforced(_ context.Context) (bool, error) {
	return requireCRDField(p, "IsImpersonationEnforced", crdCfgImpersonationEnforced)
}

func (p *CRDProvider) KustomizeSettings(_ context.Context) (v1alpha1.KustomizeOptions, error) {
	v, err := requireCRDFieldErr(p, "KustomizeSettings", crdKustomizeBuildOptions)
	if err != nil {
		return v1alpha1.KustomizeOptions{}, err
	}
	if v == nil {
		return v1alpha1.KustomizeOptions{}, nil
	}
	return *v, nil
}

func (p *CRDProvider) MetricsClusterLabels(_ context.Context) ([]string, error) {
	return requireCRDField(p, "MetricsClusterLabels", crdControllerMetricsClusterLabels)
}

func (p *CRDProvider) PersistResourceHealth(_ context.Context) (bool, error) {
	return requireCRDField(p, "PersistResourceHealth", crdControllerResourceHealthPersist)
}

func (p *CRDProvider) ReconciliationJitter(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "ReconciliationJitter", crdReconciliationJitter)
}

func (p *CRDProvider) ReconciliationTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "ReconciliationTimeout", crdReconciliationTimeout)
}

func (p *CRDProvider) RepoErrorGracePeriod(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "RepoErrorGracePeriod", crdControllerRepoErrorGracePeriodSeconds)
}

func (p *CRDProvider) ResourceCompareOptions(_ context.Context) (settings.ArgoCDDiffOptions, error) {
	return requireCRDFieldErr(p, "ResourceCompareOptions", crdResourceCompareOptions)
}

func (p *CRDProvider) ResourceCustomLabels(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ResourceOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return requireCRDFieldErr(p, "ResourceOverrides", crdResourceOverrides)
}

func (p *CRDProvider) ResourcesFilter(_ context.Context) (settings.ResourcesFilter, error) {
	v, err := requireCRDField(p, "ResourcesFilter", crdCfgResourcesFilter)
	if err != nil {
		return settings.ResourcesFilter{}, err
	}
	if v == nil {
		return settings.ResourcesFilter{}, nil
	}
	return *v, nil
}

func (p *CRDProvider) RespectRBAC(_ context.Context) (int, error) {
	return requireCRDField(p, "RespectRBAC", crdCfgRespectRBAC)
}

func (p *CRDProvider) SelfHealRetry(_ context.Context) (SelfHealRetry, error) {
	return SelfHealRetry{}, ErrNotConfigured
}

func (p *CRDProvider) SelfHealTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "SelfHealTimeout", crdControllerSelfHealTimeoutSeconds)
}

func (p *CRDProvider) SensitiveAnnotations(_ context.Context) (map[string]bool, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ServerSideDiff(_ context.Context) (bool, error) {
	return requireCRDField(p, "ServerSideDiff", crdControllerDiffServerSide)
}

func (p *CRDProvider) SourceHydratorCommitMessageTemplate(_ context.Context) (string, error) {
	return requireCRDField(p, "SourceHydratorCommitMessageTemplate", crdSourceHydratorCommitMessageTemplate)
}

func (p *CRDProvider) SyncTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "SyncTimeout", crdControllerSyncTimeoutSeconds)
}

func (p *CRDProvider) TrackingMethod(_ context.Context) (string, error) {
	return requireCRDField(p, "TrackingMethod", crdResourceTrackingMethod)
}

// ---------------------------------------------------------------------------
// applicationset
// ---------------------------------------------------------------------------

func (p *CRDProvider) ApplicationsetAllowedScmProviders(_ context.Context) ([]string, error) {
	return requireCRDField(p, "ApplicationsetAllowedScmProviders", crdApplicationsetAllowedScmProviders)
}

func (p *CRDProvider) ApplicationsetConcurrentApplicationUpdates(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetEnableGitHubAPIMetrics(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetEnableGitHubAPIMetrics", crdApplicationsetEnableGithubApiMetrics)
}

func (p *CRDProvider) ApplicationsetEnableNewGitFileGlobbing(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetEnableNewGitFileGlobbing", crdApplicationsetEnableNewGitFileGlobbing)
}

func (p *CRDProvider) ApplicationsetEnablePolicyOverride(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetEnablePolicyOverride", crdApplicationsetEnablePolicyOverride)
}

func (p *CRDProvider) ApplicationsetEnableProgressiveSyncs(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetEnableProgressiveSyncs", crdApplicationsetEnableProgressiveSyncs)
}

func (p *CRDProvider) ApplicationsetEnableScmProviders(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetEnableScmProviders", crdApplicationsetEnableScmProviders)
}

func (p *CRDProvider) ApplicationsetGitSubmoduleEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetGitSubmoduleEnabled", crdApplicationsetEnableGitSubmodule)
}

func (p *CRDProvider) ApplicationsetGlobalPreservedAnnotations(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetGlobalPreservedLabels(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetMaxResourcesStatusCount(_ context.Context) (int, error) {
	return requireCRDField(p, "ApplicationsetMaxResourcesStatusCount", crdApplicationsetStatusMaxResourcesCount)
}

func (p *CRDProvider) ApplicationsetMetricsAddr(_ context.Context) (string, error) {
	return requireCRDField(p, "ApplicationsetMetricsAddr", crdApplicationsetMetricsAddr)
}

func (p *CRDProvider) ApplicationsetMetricsApplicationsetLabels(_ context.Context) ([]string, error) {
	return requireCRDField(p, "ApplicationsetMetricsApplicationsetLabels", crdApplicationsetMetricsApplicationsetLabels)
}

func (p *CRDProvider) ApplicationsetNamespaces(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetPolicy(_ context.Context) (string, error) {
	return requireCRDField(p, "ApplicationsetPolicy", crdApplicationsetPolicy)
}

func (p *CRDProvider) ApplicationsetProbeAddr(_ context.Context) (string, error) {
	return requireCRDField(p, "ApplicationsetProbeAddr", crdApplicationsetProbeAddr)
}

func (p *CRDProvider) ApplicationsetRequeueAfter(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "ApplicationsetRequeueAfter", crdApplicationsetRequeueAfter)
}

func (p *CRDProvider) ApplicationsetScmNoProxy(_ context.Context) (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetScmProxyURL(_ context.Context) (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) ApplicationsetScmRootCAPath(_ context.Context) (string, error) {
	return requireCRDField(p, "ApplicationsetScmRootCAPath", crdApplicationsetScmRootCaPath)
}

func (p *CRDProvider) ApplicationsetTokenRefStrictMode(_ context.Context) (bool, error) {
	return requireCRDField(p, "ApplicationsetTokenRefStrictMode", crdApplicationsetEnableTokenrefStrictMode)
}

func (p *CRDProvider) ApplicationsetWebhookAddr(_ context.Context) (string, error) {
	return requireCRDField(p, "ApplicationsetWebhookAddr", crdApplicationsetWebhookAddr)
}

// ---------------------------------------------------------------------------
// commitserver
// ---------------------------------------------------------------------------

func (p *CRDProvider) CommitserverGrpcEnableTxtServiceConfig(_ context.Context) (bool, error) {
	return requireCRDField(p, "CommitserverGrpcEnableTxtServiceConfig", crdCommitserverGrpcEnableTxtServiceConfig)
}

func (p *CRDProvider) CommitserverListenAddress(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitserverListenAddress", crdCommitserverListenAddress)
}

func (p *CRDProvider) CommitserverListenPort(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) CommitserverLogFormat(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitserverLogFormat", crdCommitserverLogFormat)
}

func (p *CRDProvider) CommitserverLogLevel(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitserverLogLevel", crdCommitserverLogLevel)
}

func (p *CRDProvider) CommitserverMetricsListenAddress(_ context.Context) (string, error) {
	return requireCRDField(p, "CommitserverMetricsListenAddress", crdCommitserverMetricsListenAddress)
}

func (p *CRDProvider) CommitserverMetricsPort(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

// ---------------------------------------------------------------------------
// notifications
// ---------------------------------------------------------------------------

func (p *CRDProvider) NotificationsAppLabelSelector(_ context.Context) (string, error) {
	return requireCRDField(p, "NotificationsAppLabelSelector", crdNotificationsAppLabelSelector)
}

// NotificationsApplicationNamespaces is an alias for ApplicationNamespaces.
func (p *CRDProvider) NotificationsApplicationNamespaces(ctx context.Context) ([]string, error) {
	return p.ApplicationNamespaces(ctx)
}

func (p *CRDProvider) NotificationsConfigMapName(_ context.Context) (string, error) {
	return requireCRDField(p, "NotificationsConfigMapName", crdNotificationsConfigMapName)
}

func (p *CRDProvider) NotificationsSecretName(_ context.Context) (string, error) {
	return requireCRDField(p, "NotificationsSecretName", crdNotificationsSecretName)
}

func (p *CRDProvider) NotificationsSelfserviceEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "NotificationsSelfserviceEnabled", crdNotificationsSelfserviceEnabled)
}

// ---------------------------------------------------------------------------
// reposerver
// ---------------------------------------------------------------------------

func (p *CRDProvider) AllowOutOfBoundsSymlinks(_ context.Context) (bool, error) {
	return requireCRDField(p, "AllowOutOfBoundsSymlinks", crdReposerverAllowOobSymlinks)
}

func (p *CRDProvider) CMPTarExcludedGlobs(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) CMPUseManifestGeneratePaths(_ context.Context) (bool, error) {
	return requireCRDField(p, "CMPUseManifestGeneratePaths", crdReposerverPluginUseManifestGeneratePaths)
}

func (p *CRDProvider) DisableHelmManifestMaxExtractedSize(_ context.Context) (bool, error) {
	return requireCRDField(p, "DisableHelmManifestMaxExtractedSize", crdReposerverDisableHelmManifestMaxExtractedSize)
}

func (p *CRDProvider) DisableOCIManifestMaxExtractedSize(_ context.Context) (bool, error) {
	return requireCRDField(p, "DisableOCIManifestMaxExtractedSize", crdReposerverDisableOciManifestMaxExtractedSize)
}

func (p *CRDProvider) EnableBuiltinGitConfig(_ context.Context) (bool, error) {
	return requireCRDField(p, "EnableBuiltinGitConfig", crdReposerverEnableBuiltinGitConfig)
}

func (p *CRDProvider) HelmChartCacheExpiration(_ context.Context) (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) HelmManifestMaxExtractedSize(_ context.Context) (int64, error) {
	return requireCRDField(p, "HelmManifestMaxExtractedSize", crdReposerverHelmManifestMaxExtractedSize)
}

func (p *CRDProvider) HelmRegistryMaxIndexSize(_ context.Context) (int64, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) HelmUserAgent(_ context.Context) (string, error) {
	return requireCRDField(p, "HelmUserAgent", crdReposerverHelmUserAgent)
}

func (p *CRDProvider) IncludeHiddenDirectories(_ context.Context) (bool, error) {
	return requireCRDField(p, "IncludeHiddenDirectories", crdReposerverIncludeHiddenDirectories)
}

func (p *CRDProvider) MaxCombinedDirectoryManifestsSize(_ context.Context) (resource.Quantity, error) {
	return requireCRDField(p, "MaxCombinedDirectoryManifestsSize", crdReposerverMaxCombinedDirectoryManifestsSize)
}

func (p *CRDProvider) OCIManifestMaxExtractedSize(_ context.Context) (int64, error) {
	return requireCRDField(p, "OCIManifestMaxExtractedSize", crdReposerverOciManifestMaxExtractedSize)
}

func (p *CRDProvider) OCIMediaTypes(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ParallelismLimit(_ context.Context) (int64, error) {
	return requireCRDField(p, "ParallelismLimit", crdReposerverParallelismLimit)
}

func (p *CRDProvider) PauseGenerationAfterFailedGenerationAttempts(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) PauseGenerationOnFailureForMinutes(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) PauseGenerationOnFailureForRequests(_ context.Context) (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) RepoCacheExpiration(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "RepoCacheExpiration", crdReposerverRepoCacheExpiration)
}

func (p *CRDProvider) RevisionCacheExpiration(_ context.Context) (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) RevisionCacheLockTimeout(_ context.Context) (time.Duration, error) {
	return requireCRDField(p, "RevisionCacheLockTimeout", crdReposerverRevisionCacheLockTimeout)
}

func (p *CRDProvider) StreamedManifestMaxExtractedSize(_ context.Context) (int64, error) {
	return requireCRDField(p, "StreamedManifestMaxExtractedSize", crdReposerverStreamedManifestMaxExtractedSize)
}

func (p *CRDProvider) StreamedManifestMaxTarSize(_ context.Context) (int64, error) {
	return requireCRDField(p, "StreamedManifestMaxTarSize", crdReposerverStreamedManifestMaxTarSize)
}

func (p *CRDProvider) SubmoduleEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "SubmoduleEnabled", crdReposerverEnableGitSubmodule)
}

// ---------------------------------------------------------------------------
// server
// ---------------------------------------------------------------------------

func (p *CRDProvider) AllowedScmProviders(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ApplicationNamespaces(_ context.Context) ([]string, error) {
	return requireCRDField(p, "ApplicationNamespaces", crdApplicationNamespaces)
}

func (p *CRDProvider) BaseHRef(_ context.Context) (string, error) {
	return requireCRDField(p, "BaseHRef", crdServerBasehref)
}

func (p *CRDProvider) ContentSecurityPolicy(_ context.Context) (string, error) {
	return requireCRDField(p, "ContentSecurityPolicy", crdServerContentSecurityPolicy)
}

func (p *CRDProvider) ContentTypes(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) DexServerAddr(_ context.Context) (string, error) {
	return requireCRDField(p, "DexServerAddr", crdServerDexServer)
}

func (p *CRDProvider) DexServerPlaintext(_ context.Context) (bool, error) {
	return requireCRDField(p, "DexServerPlaintext", crdServerDexServerPlaintext)
}

func (p *CRDProvider) DexServerStrictTLS(_ context.Context) (bool, error) {
	return requireCRDField(p, "DexServerStrictTLS", crdServerDexServerStrictTls)
}

func (p *CRDProvider) DisableAuth(_ context.Context) (bool, error) {
	return requireCRDField(p, "DisableAuth", crdServerDisableAuth)
}

func (p *CRDProvider) EnableGZip(_ context.Context) (bool, error) {
	return requireCRDField(p, "EnableGZip", crdServerEnableGzip)
}

func (p *CRDProvider) EnableGitHubAPIMetrics(_ context.Context) (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) EnableK8sEvent(_ context.Context) ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) EnableNewGitFileGlobbing(_ context.Context) (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) EnableProxyExtension(_ context.Context) (bool, error) {
	return requireCRDField(p, "EnableProxyExtension", crdServerEnableProxyExtension)
}

func (p *CRDProvider) EnableScmProviders(_ context.Context) (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) GitSubmoduleEnabled(_ context.Context) (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) HydratorEnabled(_ context.Context) (bool, error) {
	return requireCRDField(p, "HydratorEnabled", crdHydratorEnabled)
}

func (p *CRDProvider) Insecure(_ context.Context) (bool, error) {
	return requireCRDField(p, "Insecure", crdServerInsecure)
}

func (p *CRDProvider) ListenHost(_ context.Context) (string, error) {
	return requireCRDField(p, "ListenHost", crdServerListenAddress)
}

func (p *CRDProvider) ListenPort(_ context.Context) (int, error) {
	return requireCRDField(p, "ListenPort", crdServerListenPort)
}

func (p *CRDProvider) MetricsHost(_ context.Context) (string, error) {
	return requireCRDField(p, "MetricsHost", crdServerMetricsListenAddress)
}

func (p *CRDProvider) MetricsPort(_ context.Context) (int, error) {
	return requireCRDField(p, "MetricsPort", crdServerMetricsPort)
}

func (p *CRDProvider) RootPath(_ context.Context) (string, error) {
	return requireCRDField(p, "RootPath", crdServerRootpath)
}

func (p *CRDProvider) ScmRootCAPath(_ context.Context) (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) StaticAssetsDir(_ context.Context) (string, error) {
	return requireCRDField(p, "StaticAssetsDir", crdServerStaticassets)
}

func (p *CRDProvider) SyncWithReplaceAllowed(_ context.Context) (bool, error) {
	return requireCRDField(p, "SyncWithReplaceAllowed", crdServerSyncReplaceAllowed)
}

func (p *CRDProvider) WebhookParallelism(_ context.Context) (int, error) {
	return requireCRDField(p, "WebhookParallelism", crdServerWebhookParallelismLimit)
}

func (p *CRDProvider) WebhookRefreshWorkers(_ context.Context) (int, error) {
	return requireCRDField(p, "WebhookRefreshWorkers", crdServerWebhookRefreshWorkers)
}

func (p *CRDProvider) XFrameOptions(_ context.Context) (string, error) {
	return requireCRDField(p, "XFrameOptions", crdServerXFrameOptions)
}
