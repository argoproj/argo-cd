package configbus

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// HybridProvider tries CRDProvider first and falls back to LegacyProvider when
// the CRD returns ErrNotConfigured. Other CRD errors are returned as-is.
type HybridProvider struct {
	crd    *CRDProvider
	legacy *LegacyProvider
}

// NewHybridProvider constructs a HybridProvider.
func NewHybridProvider(crd *CRDProvider, legacy *LegacyProvider) *HybridProvider {
	if crd == nil {
		crd = NewCRDProvider(nil)
	}
	if legacy == nil {
		legacy = NewLegacyProvider(nil, nil)
	}
	return &HybridProvider{crd: crd, legacy: legacy}
}

// Ensure HybridProvider implements Provider.
var _ Provider = (*HybridProvider)(nil)

func (h *HybridProvider) SettingsManager() (*settings.SettingsManager, error) {
	return configured(h.crd.SettingsManager, h.legacy.SettingsManager)
}

func (h *HybridProvider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	h.legacy.Subscribe(subCh)
}

func (h *HybridProvider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	h.legacy.Unsubscribe(subCh)
}

func (h *HybridProvider) AllowedNodeLabels() ([]string, error) {
	return configured(h.crd.AllowedNodeLabels, h.legacy.AllowedNodeLabels)
}

func (h *HybridProvider) AppInstanceLabelKey() (string, error) {
	return configured(h.crd.AppInstanceLabelKey, h.legacy.AppInstanceLabelKey)
}

func (h *HybridProvider) CommitAuthorEmail() (string, error) {
	return configured(h.crd.CommitAuthorEmail, h.legacy.CommitAuthorEmail)
}

func (h *HybridProvider) CommitAuthorName() (string, error) {
	return configured(h.crd.CommitAuthorName, h.legacy.CommitAuthorName)
}

func (h *HybridProvider) EnabledSourceTypes() (map[string]bool, error) {
	return configured(h.crd.EnabledSourceTypes, h.legacy.EnabledSourceTypes)
}

func (h *HybridProvider) GitRequestTimeout() (time.Duration, error) {
	return configured(h.crd.GitRequestTimeout, h.legacy.GitRequestTimeout)
}

func (h *HybridProvider) HardReconciliationTimeout() (time.Duration, error) {
	return configured(h.crd.HardReconciliationTimeout, h.legacy.HardReconciliationTimeout)
}

func (h *HybridProvider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	return configured(h.crd.HelmSettings, h.legacy.HelmSettings)
}

func (h *HybridProvider) HydratorReadmeTemplate() (string, error) {
	return configured(h.crd.HydratorReadmeTemplate, h.legacy.HydratorReadmeTemplate)
}

func (h *HybridProvider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	return configured(h.crd.IgnoreNormalizerJQTimeout, h.legacy.IgnoreNormalizerJQTimeout)
}

func (h *HybridProvider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	return configured(h.crd.IgnoreNormalizerOpts, h.legacy.IgnoreNormalizerOpts)
}

func (h *HybridProvider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return configured(h.crd.IgnoreResourceUpdatesOverrides, h.legacy.IgnoreResourceUpdatesOverrides)
}

func (h *HybridProvider) InstallationID() (string, error) {
	return configured(h.crd.InstallationID, h.legacy.InstallationID)
}

func (h *HybridProvider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	return configured(h.crd.IsIgnoreResourceUpdatesEnabled, h.legacy.IsIgnoreResourceUpdatesEnabled)
}

func (h *HybridProvider) IsImpersonationEnabled() (bool, error) {
	return configured(h.crd.IsImpersonationEnabled, h.legacy.IsImpersonationEnabled)
}

func (h *HybridProvider) IsImpersonationEnforced() (bool, error) {
	return configured(h.crd.IsImpersonationEnforced, h.legacy.IsImpersonationEnforced)
}

func (h *HybridProvider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	return configured(h.crd.KustomizeSettings, h.legacy.KustomizeSettings)
}

func (h *HybridProvider) MetricsClusterLabels() ([]string, error) {
	return configured(h.crd.MetricsClusterLabels, h.legacy.MetricsClusterLabels)
}

func (h *HybridProvider) PersistResourceHealth() (bool, error) {
	return configured(h.crd.PersistResourceHealth, h.legacy.PersistResourceHealth)
}

func (h *HybridProvider) ReconciliationJitter() (time.Duration, error) {
	return configured(h.crd.ReconciliationJitter, h.legacy.ReconciliationJitter)
}

func (h *HybridProvider) ReconciliationTimeout() (time.Duration, error) {
	return configured(h.crd.ReconciliationTimeout, h.legacy.ReconciliationTimeout)
}

func (h *HybridProvider) RepoErrorGracePeriod() (time.Duration, error) {
	return configured(h.crd.RepoErrorGracePeriod, h.legacy.RepoErrorGracePeriod)
}

func (h *HybridProvider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	return configured(h.crd.ResourceCompareOptions, h.legacy.ResourceCompareOptions)
}

func (h *HybridProvider) ResourceCustomLabels() ([]string, error) {
	return configured(h.crd.ResourceCustomLabels, h.legacy.ResourceCustomLabels)
}

func (h *HybridProvider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return configured(h.crd.ResourceOverrides, h.legacy.ResourceOverrides)
}

func (h *HybridProvider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	return configured(h.crd.ResourcesFilter, h.legacy.ResourcesFilter)
}

func (h *HybridProvider) RespectRBAC() (int, error) {
	return configured(h.crd.RespectRBAC, h.legacy.RespectRBAC)
}

func (h *HybridProvider) SelfHealBackoff() (*wait.Backoff, error) {
	return configured(h.crd.SelfHealBackoff, h.legacy.SelfHealBackoff)
}

func (h *HybridProvider) SelfHealTimeout() (time.Duration, error) {
	return configured(h.crd.SelfHealTimeout, h.legacy.SelfHealTimeout)
}

func (h *HybridProvider) SensitiveAnnotations() (map[string]bool, error) {
	return configured(h.crd.SensitiveAnnotations, h.legacy.SensitiveAnnotations)
}

func (h *HybridProvider) ServerSideDiff() (bool, error) {
	return configured(h.crd.ServerSideDiff, h.legacy.ServerSideDiff)
}

func (h *HybridProvider) SourceHydratorCommitMessageTemplate() (string, error) {
	return configured(h.crd.SourceHydratorCommitMessageTemplate, h.legacy.SourceHydratorCommitMessageTemplate)
}

func (h *HybridProvider) SyncTimeout() (time.Duration, error) {
	return configured(h.crd.SyncTimeout, h.legacy.SyncTimeout)
}

func (h *HybridProvider) TrackingMethod() (string, error) {
	return configured(h.crd.TrackingMethod, h.legacy.TrackingMethod)
}
