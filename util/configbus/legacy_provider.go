package configbus

import (
	"errors"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/env"
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

func withControllerLegacy[T any](p *LegacyProvider, fn func(ControllerLegacy) T) (T, error) {
	var zero T
	c, err := p.requireControllerLegacy()
	if err != nil {
		return zero, err
	}
	return fn(c), nil
}

func withSettingsMgr[T any](p *LegacyProvider, fn func(*settings.SettingsManager) (T, error)) (T, error) {
	var zero T
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return zero, err
	}
	return fn(mgr)
}

// ---------------------------------------------------------------------------
// Controller
// ---------------------------------------------------------------------------

func (p *LegacyProvider) HardReconciliationTimeout() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyStatusHardRefreshTimeout)
}

func (p *LegacyProvider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	return withControllerLegacy(p, func(c ControllerLegacy) time.Duration {
		return c.LegacyIgnoreNormalizerOpts().JQExecutionTimeout
	})
}

func (p *LegacyProvider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyIgnoreNormalizerOpts)
}

func (p *LegacyProvider) MetricsClusterLabels() ([]string, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyMetricsClusterLabels)
}

func (p *LegacyProvider) PersistResourceHealth() (bool, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyPersistResourceHealth)
}

func (p *LegacyProvider) ReconciliationJitter() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyStatusRefreshJitter)
}

func (p *LegacyProvider) ReconciliationTimeout() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyStatusRefreshTimeout)
}

func (p *LegacyProvider) RepoErrorGracePeriod() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyRepoErrorGracePeriod)
}

func (p *LegacyProvider) SelfHealBackoff() (*wait.Backoff, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacySelfHealBackoff)
}

func (p *LegacyProvider) SelfHealTimeout() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacySelfHealTimeout)
}

func (p *LegacyProvider) ServerSideDiff() (bool, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacyServerSideDiff)
}

func (p *LegacyProvider) SyncTimeout() (time.Duration, error) {
	return withControllerLegacy(p, ControllerLegacy.LegacySyncTimeout)
}

// ---------------------------------------------------------------------------
// Env
// ---------------------------------------------------------------------------

func (p *LegacyProvider) GitRequestTimeout() (time.Duration, error) {
	return env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64), nil
}

// ---------------------------------------------------------------------------
// SettingsManager
// ---------------------------------------------------------------------------

func (p *LegacyProvider) AllowedNodeLabels() ([]string, error) {
	return withSettingsMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetAllowedNodeLabels(), nil
	})
}

func (p *LegacyProvider) AppInstanceLabelKey() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetAppInstanceLabelKey)
}

func (p *LegacyProvider) CommitAuthorEmail() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetCommitAuthorEmail)
}

func (p *LegacyProvider) CommitAuthorName() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetCommitAuthorName)
}

func (p *LegacyProvider) EnabledSourceTypes() (map[string]bool, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetEnabledSourceTypes)
}

func (p *LegacyProvider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetHelmSettings)
}

func (p *LegacyProvider) HydratorReadmeTemplate() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetHydratorReadmeTemplate)
}

func (p *LegacyProvider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetIgnoreResourceUpdatesOverrides)
}

func (p *LegacyProvider) InstallationID() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetInstallationID)
}

func (p *LegacyProvider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetIsIgnoreResourceUpdatesEnabled)
}

func (p *LegacyProvider) IsImpersonationEnabled() (bool, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).IsImpersonationEnabled)
}

func (p *LegacyProvider) IsImpersonationEnforced() (bool, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).IsImpersonationEnforced)
}

func (p *LegacyProvider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetKustomizeSettings)
}

func (p *LegacyProvider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetResourceCompareOptions)
}

func (p *LegacyProvider) ResourceCustomLabels() ([]string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetResourceCustomLabels)
}

func (p *LegacyProvider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetResourceOverrides)
}

func (p *LegacyProvider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetResourcesFilter)
}

func (p *LegacyProvider) RespectRBAC() (int, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).RespectRBAC)
}

func (p *LegacyProvider) SensitiveAnnotations() (map[string]bool, error) {
	return withSettingsMgr(p, func(mgr *settings.SettingsManager) (map[string]bool, error) {
		return mgr.GetSensitiveAnnotations(), nil
	})
}

func (p *LegacyProvider) SourceHydratorCommitMessageTemplate() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetSourceHydratorCommitMessageTemplate)
}

func (p *LegacyProvider) TrackingMethod() (string, error) {
	return withSettingsMgr(p, (*settings.SettingsManager).GetTrackingMethod)
}
