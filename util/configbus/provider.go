package configbus

import (
	"errors"

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
