package configbus

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// ErrNotConfigured is returned by CRDProvider when a field is absent from the
// ArgoCDConfiguration CR (or the CR itself is absent). HybridProvider treats
// this sentinel as "fall back to LegacyProvider".
var ErrNotConfigured = errors.New("config: not configured")

// Provider is the typed config API for one Argo CD process.
//
// Construction rules (for reviewers and contributors):
//
//   - Each method is the smallest migrateable unit: when its backing CRD field
//     is set, every nested value under that field is considered migrated.
//   - Method names are alphabetical so each component layer can insert receivers
//     in a predictable place and PRs stay skimmable.
//   - Every config getter returns (T, error). Even legacy-guaranteed values use
//     this shape because CRD-backed reads can fail via a Kubernetes client or
//     informer. Implementations must never omit the error return.
//
// Production processes use HybridProvider (CRD first, Legacy fallback on
// ErrNotConfigured). Tests typically inject mocks.Provider from mockery.
type Provider interface {
	AllowedNodeLabels() ([]string, error)
	AppInstanceLabelKey() (string, error)
	CommitAuthorEmail() (string, error)
	CommitAuthorName() (string, error)
	EnabledSourceTypes() (map[string]bool, error)
	GitRequestTimeout() (time.Duration, error)
	HardReconciliationTimeout() (time.Duration, error)
	HelmSettings() (*v1alpha1.HelmOptions, error)
	HydratorReadmeTemplate() (string, error)
	IgnoreNormalizerJQTimeout() (time.Duration, error)
	IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error)
	IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error)
	InstallationID() (string, error)
	IsIgnoreResourceUpdatesEnabled() (bool, error)
	IsImpersonationEnabled() (bool, error)
	IsImpersonationEnforced() (bool, error)
	KustomizeSettings() (*v1alpha1.KustomizeOptions, error)
	MetricsClusterLabels() ([]string, error)
	PersistResourceHealth() (bool, error)
	ReconciliationJitter() (time.Duration, error)
	ReconciliationTimeout() (time.Duration, error)
	RepoErrorGracePeriod() (time.Duration, error)
	ResourceCompareOptions() (settings.ArgoCDDiffOptions, error)
	ResourceCustomLabels() ([]string, error)
	ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error)
	ResourcesFilter() (*settings.ResourcesFilter, error)
	RespectRBAC() (int, error)
	SelfHealBackoff() (*wait.Backoff, error)
	SelfHealTimeout() (time.Duration, error)
	SensitiveAnnotations() (map[string]bool, error)
	ServerSideDiff() (bool, error)
	// SettingsManager is a temporary escape hatch for call sites still needing
	// the raw manager during migration. Prefer typed getters when possible.
	SettingsManager() (*settings.SettingsManager, error)
	SourceHydratorCommitMessageTemplate() (string, error)
	// Subscribe registers for argocd-cm/secret change notifications when the
	// backing implementation supports it (LegacyProvider / HybridProvider).
	Subscribe(subCh chan<- *settings.ArgoCDSettings)
	SyncTimeout() (time.Duration, error)
	TrackingMethod() (string, error)
	Unsubscribe(subCh chan<- *settings.ArgoCDSettings)
}

// configured tries the CRD-backed getter first and falls back to Legacy when
// the CRD source reports ErrNotConfigured. Other CRD errors propagate.
func configured[T any](crdFn, legacyFn func() (T, error)) (T, error) {
	v, err := crdFn()
	if errors.Is(err, ErrNotConfigured) {
		return legacyFn()
	}
	return v, err
}
