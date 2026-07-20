package configbus

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// LegacyValues holds component Legacy adapters the LegacyProvider must not
// re-derive. Nil fields mean "not supplied by this component".
type LegacyValues struct {
	// Controller is the live application controller (or test fake).
	Controller ControllerLegacy
}

// ControllerLegacy is implemented by *controller.ApplicationController.
// Methods return component-resolved flag/env values already stored on the
// controller (or structs it owns).
type ControllerLegacy interface {
	LegacyStatusRefreshTimeout() time.Duration
	LegacyStatusHardRefreshTimeout() time.Duration
	LegacyStatusRefreshJitter() time.Duration
	LegacySyncTimeout() time.Duration
	LegacySelfHealTimeout() time.Duration
	LegacySelfHealBackoff() *wait.Backoff
	LegacyIgnoreNormalizerOpts() normalizers.IgnoreNormalizerOpts
	LegacyMetricsClusterLabels() []string
	LegacyServerSideDiff() bool
	LegacyPersistResourceHealth() bool
	LegacyRepoErrorGracePeriod() time.Duration
}

// LegacyProvider resolves config only from SettingsManager, component Legacy
// adapters, and env. It never returns ErrNotConfigured.
type LegacyProvider struct {
	settingsMgr *settings.SettingsManager
	legacy      *LegacyValues
}

// NewLegacyProvider constructs a LegacyProvider.
func NewLegacyProvider(settingsMgr *settings.SettingsManager, legacy *LegacyValues) *LegacyProvider {
	if legacy == nil {
		legacy = &LegacyValues{}
	}
	return &LegacyProvider{
		settingsMgr: settingsMgr,
		legacy:      legacy,
	}
}

// Ensure LegacyProvider implements Provider.
var _ Provider = (*LegacyProvider)(nil)

func (p *LegacyProvider) SettingsManager() (*settings.SettingsManager, error) {
	if p == nil || p.settingsMgr == nil {
		return nil, errors.New("config: SettingsManager is nil")
	}
	return p.settingsMgr, nil
}

func (p *LegacyProvider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p != nil && p.settingsMgr != nil {
		p.settingsMgr.Subscribe(subCh)
	}
}

func (p *LegacyProvider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p != nil && p.settingsMgr != nil {
		p.settingsMgr.Unsubscribe(subCh)
	}
}

func (p *LegacyProvider) requireSettingsMgr() (*settings.SettingsManager, error) {
	return p.SettingsManager()
}

func (p *LegacyProvider) requireControllerLegacy() (ControllerLegacy, error) {
	if p == nil || p.legacy == nil || p.legacy.Controller == nil {
		return nil, errors.New("config: ControllerLegacy not supplied by component")
	}
	return p.legacy.Controller, nil
}
