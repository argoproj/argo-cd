package configbus

import (
	"errors"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// CRDSource is the Phase 1 extension slot for reading config from an
// ArgoCDConfiguration (or successor) CR. In Phase 0 every method returns
// "not present" so legacy sources alone determine values.
type CRDSource interface {
	HasReconciliationTimeout() bool
	ReconciliationTimeout() time.Duration
	HasResourceOverrides() bool
	ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error)
}

type noopCRDSource struct{}

func (noopCRDSource) HasReconciliationTimeout() bool { return false }
func (noopCRDSource) ReconciliationTimeout() time.Duration {
	return 0
}
func (noopCRDSource) HasResourceOverrides() bool { return false }
func (noopCRDSource) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return nil, nil
}

// LegacyValues holds component-resolved flag/env/default values that the
// provider must not re-derive. Nil fields mean "not supplied by this component".
type LegacyValues struct {
	ReconciliationTimeout     *time.Duration
	HardReconciliationTimeout *time.Duration
	ReconciliationJitter      *time.Duration
	// Controller is the live application controller (or test fake).
	Controller ControllerLegacy
}

// Provider is the typed config API for one component process. Methods read
// SettingsManager, LegacyValues, and (later) CRDSource directly — there is no
// global setting registry.
type Provider struct {
	settingsMgr *settings.SettingsManager
	legacy      *LegacyValues
	crd         CRDSource
}

// NewProvider constructs a Provider. crd may be nil (Phase 0 empty slot).
func NewProvider(settingsMgr *settings.SettingsManager, legacy *LegacyValues, crd CRDSource) *Provider {
	if crd == nil {
		crd = noopCRDSource{}
	}
	if legacy == nil {
		legacy = &LegacyValues{}
	}
	return &Provider{
		settingsMgr: settingsMgr,
		legacy:      legacy,
		crd:         crd,
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
// Phase 0: CRD slot empty → ControllerLegacy, else pointer fallback (or zero).
func (p *Provider) ReconciliationTimeout() time.Duration {
	if p.crd.HasReconciliationTimeout() {
		return p.crd.ReconciliationTimeout()
	}
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusRefreshTimeout()
	}
	if p.legacy != nil && p.legacy.ReconciliationTimeout != nil {
		return *p.legacy.ReconciliationTimeout
	}
	return 0
}

// HardReconciliationTimeout returns the hard resync period.
func (p *Provider) HardReconciliationTimeout() time.Duration {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusHardRefreshTimeout()
	}
	if p.legacy != nil && p.legacy.HardReconciliationTimeout != nil {
		return *p.legacy.HardReconciliationTimeout
	}
	return 0
}

// ReconciliationJitter returns the resync jitter.
func (p *Provider) ReconciliationJitter() time.Duration {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyStatusRefreshJitter()
	}
	if p.legacy != nil && p.legacy.ReconciliationJitter != nil {
		return *p.legacy.ReconciliationJitter
	}
	return 0
}

// ResourceOverrides returns resource customization overrides.
func (p *Provider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	if p.crd.HasResourceOverrides() {
		return p.crd.ResourceOverrides()
	}
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourceOverrides()
}
