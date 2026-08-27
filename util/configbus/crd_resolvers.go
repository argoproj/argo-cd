package configbus

import (
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	enginecache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func crdDur(p *metav1.Duration) (time.Duration, bool) {
	if p == nil {
		return 0, false
	}
	return p.Duration, true
}

func crdDurStr(p *metav1.Duration) (string, bool) {
	if p == nil {
		return "", false
	}
	return p.Duration.String(), true
}

func crdInt(p *int32) (int, bool) {
	if p == nil {
		return 0, false
	}
	return int(*p), true
}

func crdInt64(p *int32) (int64, bool) {
	if p == nil {
		return 0, false
	}
	return int64(*p), true
}

func crdInt64FromInt64(p *int64) (int64, bool) {
	if p == nil {
		return 0, false
	}
	return *p, true
}

func crdInt64FromQty(p *resource.Quantity) (int64, bool) {
	if p == nil {
		return 0, false
	}
	return p.Value(), true
}

func crdQty(p *resource.Quantity) (resource.Quantity, bool) {
	if p == nil {
		return resource.Quantity{}, false
	}
	return *p, true
}

func crdBool(p *bool) (bool, bool) {
	if p == nil {
		return false, false
	}
	return *p, true
}

func crdBoolNot(p *bool) (bool, bool) {
	if p == nil {
		return false, false
	}
	return !*p, true
}

func crdStr(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	return s, true
}

func crdStrSlice(ss []string) ([]string, bool) {
	if len(ss) == 0 {
		return nil, false
	}
	return ss, true
}

func crdLogFormat(l *appv1.LogConfig) (string, bool) {
	if l == nil {
		return "", false
	}
	return crdStr(l.Format)
}

func crdLogLevel(l *appv1.LogConfig) (string, bool) {
	if l == nil {
		return "", false
	}
	return crdStr(l.Level)
}

func crdMapCommaKV(m map[string]string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ","), true
}

func crdMapColonKV(m map[string]string) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+":"+v)
	}
	return strings.Join(parts, ","), true
}

func crdRespectRBAC(s string) (int, bool) {
	if s == "" {
		return int(enginecache.RespectRbacDisabled), true
	}
	switch s {
	case "normal":
		return int(enginecache.RespectRbacNormal), true
	case "strict":
		return int(enginecache.RespectRbacStrict), true
	default:
		return 0, false
	}
}

func crdSensitiveMaskMap(keys []string) (map[string]bool, bool) {
	if len(keys) == 0 {
		return nil, false
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out, true
}

func crdCompareOptions(co *appv1.CompareOptions) (settings.ArgoCDDiffOptions, bool) {
	if co == nil {
		return settings.ArgoCDDiffOptions{}, false
	}
	return settings.ArgoCDDiffOptions{
		IgnoreAggregatedRoles:              co.IgnoreAggregatedRoles,
		IgnoreResourceStatusField:          settings.IgnoreStatus(co.IgnoreResourceStatusField),
		IgnoreDifferencesOnResourceUpdates: co.IgnoreDifferencesOnResourceUpdates,
	}, true
}

func crdHelmOptions(h *appv1.HelmConfig) (*appv1.HelmOptions, bool) {
	if h == nil || len(h.ValuesFileSchemes) == 0 {
		return nil, false
	}
	return &appv1.HelmOptions{ValuesFileSchemes: h.ValuesFileSchemes}, true
}

func crdKustomizeOptions(k *appv1.KustomizeConfig) (*appv1.KustomizeOptions, bool) {
	if k == nil {
		return nil, false
	}
	if k.BuildOptions == "" && len(k.Versions) == 0 {
		return nil, false
	}
	return &appv1.KustomizeOptions{BuildOptions: k.BuildOptions, Versions: k.Versions}, true
}

func crdImpersonationEnabled(mode string) (bool, bool) {
	if mode == "" {
		return false, false
	}
	return mode != "disabled", true
}

func crdImpersonationEnforced(mode string) (bool, bool) {
	if mode == "" {
		return false, false
	}
	return mode == "required", true
}

func crdRBACOverlayCSV(overlays []appv1.RBACPolicyOverlay) (string, bool) {
	if len(overlays) == 0 {
		return "", false
	}
	var b strings.Builder
	for i, o := range overlays {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(o.CSV)
	}
	return b.String(), true
}

func crdServerURLs(cfg *appv1.ArgoCDConfiguration) []string {
	if cfg == nil || cfg.Spec.Server == nil {
		return nil
	}
	return cfg.Spec.Server.URLs
}

func crdRepoClient(cfg *appv1.ArgoCDConfiguration) *appv1.RepoServerClientConfig {
	if cfg == nil || cfg.Spec.RepoServer == nil {
		return nil
	}
	return cfg.Spec.RepoServer.Client
}

func crdControllerK8s(cfg *appv1.ArgoCDConfiguration) *appv1.K8sClientConfig {
	if cfg == nil || cfg.Spec.Controller == nil {
		return nil
	}
	return cfg.Spec.Controller.K8sClient
}

func crdServerK8s(cfg *appv1.ArgoCDConfiguration) *appv1.K8sClientConfig {
	if cfg == nil || cfg.Spec.Server == nil {
		return nil
	}
	return cfg.Spec.Server.K8sClient
}

func crdAppSetK8s(cfg *appv1.ArgoCDConfiguration) *appv1.K8sClientConfig {
	if cfg == nil || cfg.Spec.ApplicationSet == nil {
		return nil
	}
	return cfg.Spec.ApplicationSet.K8sClient
}

func crdK8sQPS(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil {
		return "", false
	}
	return crdStr(k.QPS)
}

func crdK8sBurst(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.Burst == nil {
		return "", false
	}
	return strconv.Itoa(int(*k.Burst)), true
}

func crdK8sMaxIdle(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.MaxIdleConnections == nil {
		return "", false
	}
	return strconv.Itoa(int(*k.MaxIdleConnections)), true
}

func crdK8sTCPTimeout(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.TCP == nil {
		return "", false
	}
	return crdDurStr(k.TCP.Timeout)
}

func crdK8sTCPKeepalive(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.TCP == nil {
		return "", false
	}
	return crdDurStr(k.TCP.KeepAlive)
}

func crdK8sTCPIdle(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.TCP == nil {
		return "", false
	}
	return crdDurStr(k.TCP.IdleTimeout)
}

func crdK8sTLSHandshake(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil {
		return "", false
	}
	return crdDurStr(k.TLSHandshakeTimeout)
}

func crdK8sRetryMax(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.Retry == nil || k.Retry.Max == nil {
		return "", false
	}
	return strconv.Itoa(int(*k.Retry.Max)), true
}

func crdK8sRetryBackoff(k *appv1.K8sClientConfig) (string, bool) {
	if k == nil || k.Retry == nil || k.Retry.Backoff == nil {
		return "", false
	}
	return crdDurStr(k.Retry.Backoff.Duration)
}

func crdTLSMin(t *appv1.TLSVersionConfig) (string, bool) {
	if t == nil {
		return "", false
	}
	return crdStr(t.MinVersion)
}

func crdTLSMax(t *appv1.TLSVersionConfig) (string, bool) {
	if t == nil {
		return "", false
	}
	return crdStr(t.MaxVersion)
}

func crdTLSCiphers(t *appv1.TLSVersionConfig) (string, bool) {
	if t == nil || len(t.Ciphers) == 0 {
		return "", false
	}
	return strings.Join(t.Ciphers, ","), true
}

func crdMTLSCA(m *appv1.MTLSCertConfig) (string, bool) {
	if m == nil {
		return "", false
	}
	return crdStr(m.CACertPath)
}

func crdMTLSCert(m *appv1.MTLSCertConfig) (string, bool) {
	if m == nil {
		return "", false
	}
	return crdStr(m.ClientCertPath)
}

func crdMTLSCertKey(m *appv1.MTLSCertConfig) (string, bool) {
	if m == nil {
		return "", false
	}
	return crdStr(m.ClientCertKeyPath)
}

func crdAllowedNodeLabels(cfg *appv1.ArgoCDConfiguration) ([]string, bool) {
	if cfg.Spec.Controller != nil {
		return crdStrSlice(cfg.Spec.Controller.AllowedNodeLabelKeys)
	}
	return nil, false
}

func crdAppInstanceLabelKey(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Controller != nil {
		return crdStr(cfg.Spec.Controller.InstanceLabelKey)
	}
	return "", false
}

func crdApplicationNamespaces(cfg *appv1.ArgoCDConfiguration) ([]string, bool) {
	// Empty / omitted means control-plane namespace only — still a valid resolved value.
	if cfg.Spec.ApplicationNamespaceGlobs == nil {
		return []string{}, true
	}
	return append([]string(nil), cfg.Spec.ApplicationNamespaceGlobs...), true
}

func crdApplicationsetAllowedScmProviders(cfg *appv1.ArgoCDConfiguration) ([]string, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdStrSlice(cfg.Spec.ApplicationSet.AllowedSCMProviderURLs)
	}
	return nil, false
}

func crdApplicationsetEnableGitSubmodule(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.GitSubmoduleEnabled)
	}
	return false, false
}

func crdApplicationsetEnableGithubApiMetrics(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.GitHubAPIMetricsEnabled)
	}
	return false, false
}

func crdApplicationsetEnableNewGitFileGlobbing(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.NewGitFileGlobbingEnabled)
	}
	return false, false
}

func crdApplicationsetEnablePolicyOverride(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.PolicyOverrideEnabled)
	}
	return false, false
}

func crdApplicationsetEnableProgressiveSyncs(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil && cfg.Spec.ApplicationSet.ProgressiveSyncs != nil {
		return crdBool(cfg.Spec.ApplicationSet.ProgressiveSyncs.Enabled)
	}
	return false, false
}

func crdApplicationsetEnableScmProviders(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.SCMProvidersEnabled)
	}
	return false, false
}

func crdApplicationsetEnableTokenrefStrictMode(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdBool(cfg.Spec.ApplicationSet.TokenRefStrictModeEnabled)
	}
	return false, false
}

func crdApplicationsetMetricsAddr(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.ApplicationSet != nil && cfg.Spec.ApplicationSet.Metrics != nil {
		return crdStr(cfg.Spec.ApplicationSet.Metrics.Address)
	}
	return "", false
}

func crdApplicationsetMetricsApplicationsetLabels(cfg *appv1.ArgoCDConfiguration) ([]string, bool) {
	if cfg.Spec.ApplicationSet != nil && cfg.Spec.ApplicationSet.Metrics != nil {
		return crdStrSlice(cfg.Spec.ApplicationSet.Metrics.ApplicationSetLabels)
	}
	return nil, false
}

func crdApplicationsetPolicy(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdStr(cfg.Spec.ApplicationSet.Policy)
	}
	return "", false
}

func crdApplicationsetProbeAddr(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdStr(cfg.Spec.ApplicationSet.ProbeAddr)
	}
	return "", false
}

func crdApplicationsetRequeueAfter(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdDur(cfg.Spec.ApplicationSet.RequeueAfter)
	}
	return 0, false
}

func crdApplicationsetScmRootCaPath(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdStr(cfg.Spec.ApplicationSet.SCMRootCAPath)
	}
	return "", false
}

func crdApplicationsetStatusMaxResourcesCount(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdInt(cfg.Spec.ApplicationSet.StatusMaxResourcesCount)
	}
	return 0, false
}

func crdApplicationsetWebhookAddr(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.ApplicationSet != nil {
		return crdStr(cfg.Spec.ApplicationSet.WebhookAddr)
	}
	return "", false
}

func crdCommitAuthorEmail(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil && cfg.Spec.CommitServer.Commit != nil && cfg.Spec.CommitServer.Commit.Author != nil {
		return crdStr(cfg.Spec.CommitServer.Commit.Author.Email)
	}
	return "", false
}

func crdCommitAuthorName(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil && cfg.Spec.CommitServer.Commit != nil && cfg.Spec.CommitServer.Commit.Author != nil {
		return crdStr(cfg.Spec.CommitServer.Commit.Author.Name)
	}
	return "", false
}

func crdCommitserverGrpcEnableTxtServiceConfig(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.CommitServer != nil {
		return crdBool(cfg.Spec.CommitServer.GRPCTXTServiceConfigEnabled)
	}
	return false, false
}

func crdCommitserverListenAddress(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil && cfg.Spec.CommitServer.Listen != nil {
		return crdStr(cfg.Spec.CommitServer.Listen.Address)
	}
	return "", false
}

func crdCommitserverLogFormat(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil {
		return crdLogFormat(cfg.Spec.CommitServer.Log)
	}
	return "", false
}

func crdCommitserverLogLevel(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil {
		return crdLogLevel(cfg.Spec.CommitServer.Log)
	}
	return "", false
}

func crdCommitserverMetricsListenAddress(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.CommitServer != nil && cfg.Spec.CommitServer.Listen != nil {
		return crdStr(cfg.Spec.CommitServer.Listen.MetricsAddress)
	}
	return "", false
}

func crdControllerDiffServerSide(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Diff != nil && cfg.Spec.Controller.Diff.ServerSide != nil {
		return crdBool(cfg.Spec.Controller.Diff.ServerSide.Enabled)
	}
	return false, false
}

func crdControllerHydrationProcessors(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Processors != nil && cfg.Spec.Controller.Processors.Hydration != nil {
		return int(*cfg.Spec.Controller.Processors.Hydration), true
	}
	return 0, false
}

func crdControllerIgnoreNormalizerJqTimeout(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Diff != nil {
		return crdDur(cfg.Spec.Controller.Diff.IgnoreNormalizerJQTimeout)
	}
	return 0, false
}

func crdControllerOperationProcessors(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Processors != nil && cfg.Spec.Controller.Processors.Operation != nil {
		return int(*cfg.Spec.Controller.Processors.Operation), true
	}
	return 0, false
}

func crdControllerRepoErrorGracePeriodSeconds(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil {
		return crdDur(cfg.Spec.Controller.RepoErrorGracePeriod)
	}
	return 0, false
}

func crdControllerResourceHealthPersist(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil {
		return crdBool(cfg.Spec.Controller.ResourceHealthPersist)
	}
	return false, false
}

func crdControllerSelfHealTimeoutSeconds(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.SelfHeal != nil {
		return crdDur(cfg.Spec.Controller.SelfHeal.Timeout)
	}
	return 0, false
}

func crdControllerStatusProcessors(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Processors != nil && cfg.Spec.Controller.Processors.Status != nil {
		return int(*cfg.Spec.Controller.Processors.Status), true
	}
	return 0, false
}

func crdControllerSyncTimeoutSeconds(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Sync != nil {
		return crdDur(cfg.Spec.Controller.Sync.Timeout)
	}
	return 0, false
}

func crdHelmSettings(cfg *appv1.ArgoCDConfiguration) (*appv1.HelmOptions, bool, error) {
	if cfg.Spec.RepoServer != nil {
		v, ok := crdHelmOptions(cfg.Spec.RepoServer.Helm)
		return v, ok, nil
	}
	return nil, false, nil
}

func crdHydratorEnabled(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.SourceHydrator != nil && cfg.Spec.Controller.SourceHydrator.Enabled != nil {
		return crdBool(cfg.Spec.Controller.SourceHydrator.Enabled)
	}
	return false, false
}

func crdIgnoreResourceUpdatesEnabled(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Diff != nil {
		return crdBool(cfg.Spec.Controller.Diff.IgnoreResourceUpdatesEnabled)
	}
	return false, false
}

func crdCfgImpersonationEnabled(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Sync != nil && cfg.Spec.Controller.Sync.Impersonation != nil {
		return crdImpersonationEnabled(cfg.Spec.Controller.Sync.Impersonation.Mode)
	}
	return false, false
}

func crdCfgImpersonationEnforced(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Sync != nil && cfg.Spec.Controller.Sync.Impersonation != nil {
		return crdImpersonationEnforced(cfg.Spec.Controller.Sync.Impersonation.Mode)
	}
	return false, false
}

func crdInstallationID(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	return crdStr(cfg.Spec.InstallationID)
}

func crdKustomizeBuildOptions(cfg *appv1.ArgoCDConfiguration) (*appv1.KustomizeOptions, bool, error) {
	if cfg.Spec.RepoServer != nil {
		v, ok := crdKustomizeOptions(cfg.Spec.RepoServer.Kustomize)
		return v, ok, nil
	}
	return nil, false, nil
}

func crdNotificationsAppLabelSelector(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Notifications != nil {
		return crdStr(cfg.Spec.Notifications.AppLabelSelector)
	}
	return "", false
}

func crdNotificationsConfigMapName(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Notifications != nil {
		return crdStr(cfg.Spec.Notifications.ConfigMapName)
	}
	return "", false
}

func crdNotificationsSecretName(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Notifications != nil {
		return crdStr(cfg.Spec.Notifications.SecretName)
	}
	return "", false
}

func crdNotificationsSelfserviceEnabled(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Notifications != nil {
		return crdBool(cfg.Spec.Notifications.SelfServiceEnabled)
	}
	return false, false
}

func crdNotificationscontrollerProcessorsCount(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Notifications != nil && cfg.Spec.Notifications.ProcessorsCount != nil {
		return int(*cfg.Spec.Notifications.ProcessorsCount), true
	}
	return 0, false
}

func crdReposerverAllowOobSymlinks(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil {
		return crdBool(cfg.Spec.RepoServer.AllowOOBSymlinks)
	}
	return false, false
}

func crdReposerverDisableHelmManifestMaxExtractedSize(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Helm != nil && cfg.Spec.RepoServer.Helm.Manifest != nil {
		return crdBoolNot(cfg.Spec.RepoServer.Helm.Manifest.MaxExtractedSizeEnabled)
	}
	return false, false
}

func crdReposerverDisableOciManifestMaxExtractedSize(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.OCI != nil && cfg.Spec.RepoServer.OCI.Manifest != nil {
		return crdBoolNot(cfg.Spec.RepoServer.OCI.Manifest.MaxExtractedSizeEnabled)
	}
	return false, false
}

func crdReposerverEnableBuiltinGitConfig(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Git != nil {
		return crdBool(cfg.Spec.RepoServer.Git.BuiltinConfigEnabled)
	}
	return false, false
}

func crdReposerverEnableGitSubmodule(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Git != nil {
		return crdBool(cfg.Spec.RepoServer.Git.SubmoduleEnabled)
	}
	return false, false
}

func crdReposerverHelmManifestMaxExtractedSize(cfg *appv1.ArgoCDConfiguration) (int64, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Helm != nil && cfg.Spec.RepoServer.Helm.Manifest != nil {
		return crdInt64FromQty(cfg.Spec.RepoServer.Helm.Manifest.MaxExtractedSize)
	}
	return 0, false
}

func crdReposerverHelmUserAgent(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Helm != nil {
		return crdStr(cfg.Spec.RepoServer.Helm.UserAgent)
	}
	return "", false
}

func crdReposerverIncludeHiddenDirectories(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil {
		return crdBool(cfg.Spec.RepoServer.IncludeHiddenDirectories)
	}
	return false, false
}

func crdReposerverMaxCombinedDirectoryManifestsSize(cfg *appv1.ArgoCDConfiguration) (resource.Quantity, bool) {
	if cfg.Spec.RepoServer != nil {
		return crdQty(cfg.Spec.RepoServer.MaxCombinedDirectoryManifestsSize)
	}
	return resource.Quantity{}, false
}

func crdReposerverOciManifestMaxExtractedSize(cfg *appv1.ArgoCDConfiguration) (int64, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.OCI != nil && cfg.Spec.RepoServer.OCI.Manifest != nil {
		return crdInt64FromQty(cfg.Spec.RepoServer.OCI.Manifest.MaxExtractedSize)
	}
	return 0, false
}

func crdReposerverParallelismLimit(cfg *appv1.ArgoCDConfiguration) (int64, bool) {
	if cfg.Spec.RepoServer != nil {
		return crdInt64(cfg.Spec.RepoServer.ParallelismLimit)
	}
	return 0, false
}

func crdReposerverPluginUseManifestGeneratePaths(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Plugin != nil {
		return crdBool(cfg.Spec.RepoServer.Plugin.UseManifestGeneratePaths)
	}
	return false, false
}

func crdReposerverRepoCacheExpiration(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.Cache != nil {
		return crdDur(cfg.Spec.RepoServer.Cache.RepoExpiration)
	}
	return 0, false
}

func crdReposerverRevisionCacheLockTimeout(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.RepoServer != nil {
		return crdDur(cfg.Spec.RepoServer.RevisionCacheLockTimeout)
	}
	return 0, false
}

func crdReposerverStreamedManifestMaxExtractedSize(cfg *appv1.ArgoCDConfiguration) (int64, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.StreamedManifest != nil {
		return crdInt64FromQty(cfg.Spec.RepoServer.StreamedManifest.MaxExtractedSize)
	}
	return 0, false
}

func crdReposerverStreamedManifestMaxTarSize(cfg *appv1.ArgoCDConfiguration) (int64, bool) {
	if cfg.Spec.RepoServer != nil && cfg.Spec.RepoServer.StreamedManifest != nil {
		return crdInt64FromQty(cfg.Spec.RepoServer.StreamedManifest.MaxTarSize)
	}
	return 0, false
}

func crdResourceCompareOptions(cfg *appv1.ArgoCDConfiguration) (settings.ArgoCDDiffOptions, bool, error) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Diff != nil {
		v, ok := crdCompareOptions(cfg.Spec.Controller.Diff.CompareOptions)
		return v, ok, nil
	}
	return settings.ArgoCDDiffOptions{}, false, nil
}

func crdResourceTrackingMethod(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Controller != nil {
		return crdStr(cfg.Spec.Controller.ResourceTrackingMethod)
	}
	return "", false
}

func crdCfgResourcesFilter(cfg *appv1.ArgoCDConfiguration) (*settings.ResourcesFilter, bool) {
	return crdResourcesFilterExclusions(cfg)
}

func crdCfgRespectRBAC(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Resource != nil {
		return crdRespectRBAC(cfg.Spec.Controller.Resource.RespectRBAC)
	}
	return 0, false
}

func crdServerBasehref(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil {
		return crdStr(cfg.Spec.Server.BaseHref)
	}
	return "", false
}

func crdServerContentSecurityPolicy(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil {
		return crdStr(cfg.Spec.Server.ContentSecurityPolicy)
	}
	return "", false
}

func crdServerDexServer(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.DexConnection != nil {
		return crdStr(cfg.Spec.Server.DexConnection.Address)
	}
	return "", false
}

func crdServerDexServerPlaintext(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.DexConnection != nil {
		return crdBoolNot(cfg.Spec.Server.DexConnection.TLSEnabled)
	}
	return false, false
}

func crdServerDexServerStrictTls(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.DexConnection != nil && cfg.Spec.Server.DexConnection.InsecureSkipVerify != nil {
		return !*cfg.Spec.Server.DexConnection.InsecureSkipVerify, true
	}
	return false, false
}

func crdServerDisableAuth(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil {
		return crdBoolNot(cfg.Spec.Server.AuthEnabled)
	}
	return false, false
}

func crdServerEnableGzip(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Compression != "" {
		return cfg.Spec.Server.Compression == "gzip", true
	}
	return false, false
}

func crdServerEnableProxyExtension(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil {
		return crdBool(cfg.Spec.Server.ProxyExtensionEnabled)
	}
	return false, false
}

func crdServerInsecure(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil {
		return crdBoolNot(cfg.Spec.Server.TLSEnabled)
	}
	return false, false
}

func crdServerListenAddress(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Listen != nil {
		return crdStr(cfg.Spec.Server.Listen.Address)
	}
	return "", false
}

func crdServerListenPort(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Listen != nil {
		return crdInt(cfg.Spec.Server.Listen.Port)
	}
	return 0, false
}

func crdServerMetricsListenAddress(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Listen != nil {
		return crdStr(cfg.Spec.Server.Listen.MetricsAddress)
	}
	return "", false
}

func crdServerMetricsPort(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Listen != nil {
		return crdInt(cfg.Spec.Server.Listen.MetricsPort)
	}
	return 0, false
}

func crdServerRootpath(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil {
		return crdStr(cfg.Spec.Server.RootPath)
	}
	return "", false
}

func crdServerStaticassets(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil {
		return crdStr(cfg.Spec.Server.StaticAssetsPath)
	}
	return "", false
}

func crdServerSyncReplaceAllowed(cfg *appv1.ArgoCDConfiguration) (bool, bool) {
	if cfg.Spec.Server != nil {
		return crdBool(cfg.Spec.Server.SyncReplaceAllowed)
	}
	return false, false
}

func crdServerWebhookParallelismLimit(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Webhook != nil {
		return crdInt(cfg.Spec.Server.Webhook.ParallelismLimit)
	}
	return 0, false
}

func crdServerWebhookRefreshWorkers(cfg *appv1.ArgoCDConfiguration) (int, bool) {
	if cfg.Spec.Server != nil && cfg.Spec.Server.Webhook != nil && cfg.Spec.Server.Webhook.Refresh != nil {
		return crdInt(cfg.Spec.Server.Webhook.Refresh.Workers)
	}
	return 0, false
}

func crdServerXFrameOptions(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Server != nil {
		return crdStr(cfg.Spec.Server.XFrameOptions)
	}
	return "", false
}

func crdSourceHydratorCommitMessageTemplate(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.SourceHydrator != nil {
		return crdStr(cfg.Spec.Controller.SourceHydrator.CommitMessageTemplate)
	}
	return "", false
}

func crdSourceHydratorReadmeMessageTemplate(cfg *appv1.ArgoCDConfiguration) (string, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.SourceHydrator != nil {
		return crdStr(cfg.Spec.Controller.SourceHydrator.ReadmeMessageTemplate)
	}
	return "", false
}

func crdControllerMetricsClusterLabels(cfg *appv1.ArgoCDConfiguration) ([]string, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Metrics != nil && cfg.Spec.Controller.Metrics.Cluster != nil {
		return crdStrSlice(cfg.Spec.Controller.Metrics.Cluster.LabelKeys)
	}
	return nil, false
}

func crdReconciliationTimeout(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Reconciliation != nil {
		return crdDur(cfg.Spec.Controller.Reconciliation.Timeout)
	}
	return 0, false
}

func crdHardReconciliationTimeout(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Reconciliation != nil {
		return crdDur(cfg.Spec.Controller.Reconciliation.HardTimeout)
	}
	return 0, false
}

func crdReconciliationJitter(cfg *appv1.ArgoCDConfiguration) (time.Duration, bool) {
	if cfg.Spec.Controller != nil && cfg.Spec.Controller.Reconciliation != nil {
		return crdDur(cfg.Spec.Controller.Reconciliation.Jitter)
	}
	return 0, false
}

func crdResourceOverrides(cfg *appv1.ArgoCDConfiguration) (map[string]appv1.ResourceOverride, bool, error) {
	if cfg.Spec.Controller == nil || cfg.Spec.Controller.Resource == nil {
		return nil, false, nil
	}
	r := cfg.Spec.Controller.Resource
	if len(r.Health) == 0 &&
		len(r.Actions) == 0 &&
		len(r.IgnoreDifferences) == 0 &&
		len(r.IgnoreResourceUpdates) == 0 &&
		len(r.KnownTypeFields) == 0 {
		return nil, false, nil
	}
	out, err := mergeResourceOverrides(r)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}
