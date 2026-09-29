//nolint:staticcheck // SA1019: this file is the allowed bridge to deprecated SettingsManager product getters.
package configbus

import (
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// SettingsManagerProvider resolves ConfigMap-backed product settings from a
// SettingsManager. Unowned field getters return ErrNotConfigured via the
// embedded empty ChainProvider.
type SettingsManagerProvider struct {
	// ChainProvider is embedded with no links on purpose: an empty chain
	// resolves every promoted field getter to ErrNotConfigured, so this leaf
	// only implements the fields it owns. Do not populate its links.
	ChainProvider
	mgr *settings.SettingsManager
}

// NewSettingsManagerProvider constructs a SettingsManagerProvider.
// mgr must be non-nil; a nil manager panics so callers fail fast at wiring time
// instead of on every getter.
func NewSettingsManagerProvider(mgr *settings.SettingsManager) *SettingsManagerProvider {
	if mgr == nil {
		panic("configbus: NewSettingsManagerProvider requires a non-nil SettingsManager")
	}
	return &SettingsManagerProvider{mgr: mgr}
}

// Ensure SettingsManagerProvider implements Provider.
var _ Provider = (*SettingsManagerProvider)(nil)

func (p *SettingsManagerProvider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	p.mgr.Subscribe(subCh)
}
func (p *SettingsManagerProvider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	p.mgr.Unsubscribe(subCh)
}

func withMgr[T any](p *SettingsManagerProvider, fn func(*settings.SettingsManager) (T, error)) (T, error) {
	return fn(p.mgr)
}
