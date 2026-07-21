package configbus

import (
	"errors"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// SettingsManagerProvider resolves ConfigMap-backed product settings from a
// SettingsManager. Unowned field getters return ErrNotConfigured via the
// embedded notConfiguredProvider.
type SettingsManagerProvider struct {
	notConfiguredProvider
	mgr *settings.SettingsManager
}

// NewSettingsManagerProvider constructs a SettingsManagerProvider.
func NewSettingsManagerProvider(mgr *settings.SettingsManager) *SettingsManagerProvider {
	return &SettingsManagerProvider{mgr: mgr}
}

// Ensure SettingsManagerProvider implements Provider.
var _ Provider = (*SettingsManagerProvider)(nil)

func (p *SettingsManagerProvider) SettingsManager() (*settings.SettingsManager, error) {
	if p == nil || p.mgr == nil {
		return nil, errors.New("config: SettingsManager is nil")
	}
	return p.mgr, nil
}

func (p *SettingsManagerProvider) Subscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p != nil && p.mgr != nil {
		p.mgr.Subscribe(subCh)
	}
}

func (p *SettingsManagerProvider) Unsubscribe(subCh chan<- *settings.ArgoCDSettings) {
	if p != nil && p.mgr != nil {
		p.mgr.Unsubscribe(subCh)
	}
}

func (p *SettingsManagerProvider) requireMgr() (*settings.SettingsManager, error) {
	return p.SettingsManager()
}

func withMgr[T any](p *SettingsManagerProvider, fn func(*settings.SettingsManager) (T, error)) (T, error) {
	var zero T
	mgr, err := p.requireMgr()
	if err != nil {
		return zero, err
	}
	return fn(mgr)
}

func (p *SettingsManagerProvider) AllowedNodeLabels() ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetAllowedNodeLabels(), nil
	})
}

func (p *SettingsManagerProvider) AppInstanceLabelKey() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetAppInstanceLabelKey)
}

func (p *SettingsManagerProvider) CommitAuthorEmail() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetCommitAuthorEmail)
}

func (p *SettingsManagerProvider) CommitAuthorName() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetCommitAuthorName)
}

func (p *SettingsManagerProvider) EnabledSourceTypes() (map[string]bool, error) {
	return withMgr(p, (*settings.SettingsManager).GetEnabledSourceTypes)
}

func (p *SettingsManagerProvider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetHelmSettings)
}

func (p *SettingsManagerProvider) HydratorReadmeTemplate() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetHydratorReadmeTemplate)
}

func (p *SettingsManagerProvider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetIgnoreResourceUpdatesOverrides)
}

func (p *SettingsManagerProvider) InstallationID() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetInstallationID)
}

func (p *SettingsManagerProvider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	return withMgr(p, (*settings.SettingsManager).GetIsIgnoreResourceUpdatesEnabled)
}

func (p *SettingsManagerProvider) IsImpersonationEnabled() (bool, error) {
	return withMgr(p, (*settings.SettingsManager).IsImpersonationEnabled)
}

func (p *SettingsManagerProvider) IsImpersonationEnforced() (bool, error) {
	return withMgr(p, (*settings.SettingsManager).IsImpersonationEnforced)
}

func (p *SettingsManagerProvider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetKustomizeSettings)
}

func (p *SettingsManagerProvider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCompareOptions)
}

func (p *SettingsManagerProvider) ResourceCustomLabels() ([]string, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCustomLabels)
}

func (p *SettingsManagerProvider) ResourceOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceOverrides)
}

func (p *SettingsManagerProvider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourcesFilter)
}

func (p *SettingsManagerProvider) RespectRBAC() (int, error) {
	return withMgr(p, (*settings.SettingsManager).RespectRBAC)
}

func (p *SettingsManagerProvider) SensitiveAnnotations() (map[string]bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (map[string]bool, error) {
		return mgr.GetSensitiveAnnotations(), nil
	})
}

func (p *SettingsManagerProvider) SourceHydratorCommitMessageTemplate() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetSourceHydratorCommitMessageTemplate)
}

func (p *SettingsManagerProvider) TrackingMethod() (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetTrackingMethod)
}
