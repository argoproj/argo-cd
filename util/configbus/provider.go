package configbus

import (
	"errors"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// LegacyValues holds component Legacy adapters the provider must not re-derive.
// Nil fields mean "not supplied by this component".
type LegacyValues struct {
	// Controller is the live application controller (or test fake).
	Controller ControllerLegacy
}

// Provider is the typed config API for one component process. Methods read
// SettingsManager and LegacyValues directly — there is no global setting registry.
type Provider struct {
	settingsMgr *settings.SettingsManager
	legacy      *LegacyValues
}

// NewProvider constructs a Provider.
func NewProvider(settingsMgr *settings.SettingsManager, legacy *LegacyValues) *Provider {
	if legacy == nil {
		legacy = &LegacyValues{}
	}
	return &Provider{
		settingsMgr: settingsMgr,
		legacy:      legacy,
	}
}

// SettingsManager returns the underlying settings manager (for gradual migration).
func (p *Provider) SettingsManager() *settings.SettingsManager {
	return p.settingsMgr
}

// Legacy returns the component-supplied legacy values.
func (p *Provider) Legacy() *LegacyValues {
	return p.legacy
}

// Subscribe wraps SettingsManager.Subscribe for hot-reload consumers.
func (p *Provider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p.settingsMgr != nil {
		p.settingsMgr.Subscribe(subCh)
	}
}

// Unsubscribe wraps SettingsManager.Unsubscribe.
func (p *Provider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p.settingsMgr != nil {
		p.settingsMgr.Unsubscribe(subCh)
	}
}

func (p *Provider) requireSettingsMgr() (*settings.SettingsManager, error) {
	if p == nil || p.settingsMgr == nil {
		return nil, errors.New("config: SettingsManager is nil")
	}
	return p.settingsMgr, nil
}

func (p *Provider) requireControllerLegacy() (ControllerLegacy, error) {
	if p == nil || p.legacy == nil || p.legacy.Controller == nil {
		return nil, errors.New("config: ControllerLegacy not supplied by component")
	}
	return p.legacy.Controller, nil
}

// ReconciliationTimeout returns the reconciliation / app-resync period.
func (p *Provider) ReconciliationTimeout() time.Duration {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusRefreshTimeout()
	}
	return 0
}

// HardReconciliationTimeout returns the hard resync period.
func (p *Provider) HardReconciliationTimeout() time.Duration {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusHardRefreshTimeout()
	}
	return 0
}

// ReconciliationJitter returns the resync jitter.
func (p *Provider) ReconciliationJitter() time.Duration {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusRefreshJitter()
	}
	return 0
}

// ResourceOverrides returns resource customization overrides.
func (p *Provider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourceOverrides()
}
