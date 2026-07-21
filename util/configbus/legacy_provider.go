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

// ---------------------------------------------------------------------------
// Controller
// ---------------------------------------------------------------------------

func (p *LegacyProvider) HardReconciliationTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusHardRefreshTimeout(), nil
}

func (p *LegacyProvider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyIgnoreNormalizerOpts().JQExecutionTimeout, nil
}

func (p *LegacyProvider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return normalizers.IgnoreNormalizerOpts{}, err
	}
	return c.LegacyIgnoreNormalizerOpts(), nil
}

func (p *LegacyProvider) MetricsClusterLabels() ([]string, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacyMetricsClusterLabels(), nil
}

func (p *LegacyProvider) PersistResourceHealth() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyPersistResourceHealth(), nil
}

func (p *LegacyProvider) ReconciliationJitter() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusRefreshJitter(), nil
}

func (p *LegacyProvider) ReconciliationTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusRefreshTimeout(), nil
}

func (p *LegacyProvider) RepoErrorGracePeriod() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyRepoErrorGracePeriod(), nil
}

func (p *LegacyProvider) SelfHealBackoff() (*wait.Backoff, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacySelfHealBackoff(), nil
}

func (p *LegacyProvider) SelfHealTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySelfHealTimeout(), nil
}

func (p *LegacyProvider) ServerSideDiff() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyServerSideDiff(), nil
}

func (p *LegacyProvider) SyncTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySyncTimeout(), nil
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
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetAllowedNodeLabels(), nil
}

func (p *LegacyProvider) AppInstanceLabelKey() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetAppInstanceLabelKey()
}

func (p *LegacyProvider) CommitAuthorEmail() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetCommitAuthorEmail()
}

func (p *LegacyProvider) CommitAuthorName() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetCommitAuthorName()
}

func (p *LegacyProvider) EnabledSourceTypes() (map[string]bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetEnabledSourceTypes()
}

func (p *LegacyProvider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetHelmSettings()
}

func (p *LegacyProvider) HydratorReadmeTemplate() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetHydratorReadmeTemplate()
}

func (p *LegacyProvider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetIgnoreResourceUpdatesOverrides()
}

func (p *LegacyProvider) InstallationID() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetInstallationID()
}

func (p *LegacyProvider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.GetIsIgnoreResourceUpdatesEnabled()
}

func (p *LegacyProvider) IsImpersonationEnabled() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.IsImpersonationEnabled()
}

func (p *LegacyProvider) IsImpersonationEnforced() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.IsImpersonationEnforced()
}

func (p *LegacyProvider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetKustomizeSettings()
}

func (p *LegacyProvider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return settings.ArgoCDDiffOptions{}, err
	}
	return mgr.GetResourceCompareOptions()
}

func (p *LegacyProvider) ResourceCustomLabels() ([]string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourceCustomLabels()
}

func (p *LegacyProvider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourceOverrides()
}

func (p *LegacyProvider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourcesFilter()
}

func (p *LegacyProvider) RespectRBAC() (int, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return 0, err
	}
	return mgr.RespectRBAC()
}

func (p *LegacyProvider) SensitiveAnnotations() (map[string]bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetSensitiveAnnotations(), nil
}

func (p *LegacyProvider) SourceHydratorCommitMessageTemplate() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetSourceHydratorCommitMessageTemplate()
}

func (p *LegacyProvider) TrackingMethod() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetTrackingMethod()
}
