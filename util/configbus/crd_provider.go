package configbus

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// CRDProvider resolves config only from the ArgoCDConfiguration CRD. Until the
// CRD is introduced, every getter returns ErrNotConfigured so HybridProvider
// falls through to LegacyProvider.
type CRDProvider struct {
	// source is reserved until the CRD is introduced. Nil means no CRD is available.
	source any
}

// NewCRDProvider constructs a CRDProvider. Pass nil until the CRD source exists.
func NewCRDProvider(source any) *CRDProvider {
	return &CRDProvider{source: source}
}

// Ensure CRDProvider implements Provider.
var _ Provider = (*CRDProvider)(nil)

func (p *CRDProvider) SettingsManager() (*settings.SettingsManager, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) Subscribe(_ chan<- *settings.ArgoCDSettings) {}

func (p *CRDProvider) Unsubscribe(_ chan<- *settings.ArgoCDSettings) {}

func (p *CRDProvider) AllowedNodeLabels() ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) AppInstanceLabelKey() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) CommitAuthorEmail() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) CommitAuthorName() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) EnabledSourceTypes() (map[string]bool, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) GitRequestTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) HardReconciliationTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) HydratorReadmeTemplate() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	return normalizers.IgnoreNormalizerOpts{}, ErrNotConfigured
}

func (p *CRDProvider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) InstallationID() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) IsImpersonationEnabled() (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) IsImpersonationEnforced() (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) MetricsClusterLabels() ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) PersistResourceHealth() (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) ReconciliationJitter() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) ReconciliationTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) RepoErrorGracePeriod() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	return settings.ArgoCDDiffOptions{}, ErrNotConfigured
}

func (p *CRDProvider) ResourceCustomLabels() ([]string, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) RespectRBAC() (int, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) SelfHealBackoff() (*wait.Backoff, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) SelfHealTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) SensitiveAnnotations() (map[string]bool, error) {
	return nil, ErrNotConfigured
}

func (p *CRDProvider) ServerSideDiff() (bool, error) {
	return false, ErrNotConfigured
}

func (p *CRDProvider) SourceHydratorCommitMessageTemplate() (string, error) {
	return "", ErrNotConfigured
}

func (p *CRDProvider) SyncTimeout() (time.Duration, error) {
	return 0, ErrNotConfigured
}

func (p *CRDProvider) TrackingMethod() (string, error) {
	return "", ErrNotConfigured
}
