package configbus

import (
	"context"
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// ErrNotConfigured is returned by a leaf Provider when it does not own / does
// not have a value for a field. ChainProvider skips links that return this
// sentinel and continues to the next link.
var ErrNotConfigured = errors.New("config: not configured")

// Provider is the typed config API for one Argo CD process.
//
// Construction rules (for reviewers and contributors):
//
//   - Each method is the smallest migrateable unit: when its backing CRD field
//     is set, every nested value under that field is considered migrated.
//   - Method names are alphabetical so each component layer can insert receivers
//     in a predictable place and PRs stay skimmable.
//   - Every config getter returns (T, error) and accepts context.Context for
//     future Kubernetes/informer-backed reads and logging. Implementations must
//     never omit the error return.
//
// Production processes compose leaf providers with ChainProvider (Static /
// SettingsManagerProvider / Env; CRD is inserted once wired). Tests typically
// inject mocks.Provider from mockery, or a StaticProvider literal.
type Provider interface {
	// Subscribe registers for argocd-cm/secret change notifications when the
	// backing implementation supports it (SettingsManagerProvider / ChainProvider).
	Subscribe(subCh chan<- *settings.ArgoCDSettings)
	// Unsubscribe unregisters a settings change subscriber.
	Unsubscribe(subCh chan<- *settings.ArgoCDSettings)

	AllowedNodeLabels(ctx context.Context) ([]string, error)
	AppInstanceLabelKey(ctx context.Context) (string, error)
	CommitAuthorEmail(ctx context.Context) (string, error)
	CommitAuthorName(ctx context.Context) (string, error)
	EnabledSourceTypes(ctx context.Context) (map[string]bool, error)
	GitRequestTimeout(ctx context.Context) (time.Duration, error)
	HardReconciliationTimeout(ctx context.Context) (time.Duration, error)
	HelmSettings(ctx context.Context) (*v1alpha1.HelmOptions, error)
	HydratorReadmeTemplate(ctx context.Context) (string, error)
	IgnoreNormalizerJQTimeout(ctx context.Context) (time.Duration, error)
	IgnoreResourceUpdatesOverrides(ctx context.Context) (map[string]v1alpha1.ResourceOverride, error)
	InstallationID(ctx context.Context) (string, error)
	IsIgnoreResourceUpdatesEnabled(ctx context.Context) (bool, error)
	IsImpersonationEnabled(ctx context.Context) (bool, error)
	IsImpersonationEnforced(ctx context.Context) (bool, error)
	KustomizeSettings(ctx context.Context) (*v1alpha1.KustomizeOptions, error)
	MetricsClusterLabels(ctx context.Context) ([]string, error)
	PersistResourceHealth(ctx context.Context) (bool, error)
	ReconciliationJitter(ctx context.Context) (time.Duration, error)
	ReconciliationTimeout(ctx context.Context) (time.Duration, error)
	RepoErrorGracePeriod(ctx context.Context) (time.Duration, error)
	ResourceCompareOptions(ctx context.Context) (settings.ArgoCDDiffOptions, error)
	ResourceCustomLabels(ctx context.Context) ([]string, error)
	ResourceOverrides(ctx context.Context) (map[string]v1alpha1.ResourceOverride, error)
	ResourcesFilter(ctx context.Context) (*settings.ResourcesFilter, error)
	RespectRBAC(ctx context.Context) (int, error)
	SelfHealBackoff(ctx context.Context) (*wait.Backoff, error)
	SelfHealTimeout(ctx context.Context) (time.Duration, error)
	SensitiveAnnotations(ctx context.Context) (map[string]bool, error)
	ServerSideDiff(ctx context.Context) (bool, error)
	SourceHydratorCommitMessageTemplate(ctx context.Context) (string, error)
	SyncTimeout(ctx context.Context) (time.Duration, error)
	TrackingMethod(ctx context.Context) (string, error)
}

// firstConfigured tries each link in order and returns the first result that is
// not ErrNotConfigured. Other errors propagate immediately. If every link
// returns ErrNotConfigured, that sentinel is returned.
func firstConfigured[T any](fn func(Provider) (T, error), links []Provider) (T, error) {
	var zero T
	var lastNotConfigured error
	for _, link := range links {
		v, err := fn(link)
		if err == nil {
			return v, nil
		}
		if errors.Is(err, ErrNotConfigured) {
			lastNotConfigured = err
			continue
		}
		return zero, err
	}
	if lastNotConfigured != nil {
		return zero, lastNotConfigured
	}
	return zero, ErrNotConfigured
}
