package configbus

import (
	"context"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

func (p *SettingsManagerProvider) AllowedNodeLabels(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetAllowedNodeLabels(), nil
	})
}

func (p *SettingsManagerProvider) AppInstanceLabelKey(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetAppInstanceLabelKey)
}

func (p *SettingsManagerProvider) CommitAuthorEmail(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetCommitAuthorEmail)
}

func (p *SettingsManagerProvider) CommitAuthorName(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetCommitAuthorName)
}

func (p *SettingsManagerProvider) EnabledSourceTypes(_ context.Context) (map[string]bool, error) {
	return withMgr(p, (*settings.SettingsManager).GetEnabledSourceTypes)
}

func (p *SettingsManagerProvider) ExcludeEventLabelKeys(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetExcludeEventLabelKeys(), nil
	})
}

func (p *SettingsManagerProvider) GlobalProjectsSettings(_ context.Context) ([]settings.GlobalProjectSettings, error) {
	return withMgr(p, (*settings.SettingsManager).GetGlobalProjectsSettings)
}

func (p *SettingsManagerProvider) HelmSettings(_ context.Context) (*v1alpha1.HelmOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetHelmSettings)
}

func (p *SettingsManagerProvider) HydratorReadmeTemplate(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetHydratorReadmeTemplate)
}

func (p *SettingsManagerProvider) IgnoreResourceUpdatesOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetIgnoreResourceUpdatesOverrides)
}

func (p *SettingsManagerProvider) IncludeEventLabelKeys(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetIncludeEventLabelKeys(), nil
	})
}

func (p *SettingsManagerProvider) InstallationID(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetInstallationID)
}

func (p *SettingsManagerProvider) IsIgnoreResourceUpdatesEnabled(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).GetIsIgnoreResourceUpdatesEnabled)
}

func (p *SettingsManagerProvider) IsImpersonationEnabled(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).IsImpersonationEnabled)
}

func (p *SettingsManagerProvider) IsImpersonationEnforced(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).IsImpersonationEnforced)
}

func (p *SettingsManagerProvider) KustomizeSettings(_ context.Context) (*v1alpha1.KustomizeOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetKustomizeSettings)
}

func (p *SettingsManagerProvider) ResourceCompareOptions(_ context.Context) (settings.ArgoCDDiffOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCompareOptions)
}

func (p *SettingsManagerProvider) ResourceCustomLabels(_ context.Context) ([]string, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCustomLabels)
}

func (p *SettingsManagerProvider) ResourceOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceOverrides)
}

func (p *SettingsManagerProvider) ResourcesFilter(_ context.Context) (*settings.ResourcesFilter, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourcesFilter)
}

func (p *SettingsManagerProvider) RespectRBAC(_ context.Context) (int, error) {
	return withMgr(p, (*settings.SettingsManager).RespectRBAC)
}

func (p *SettingsManagerProvider) SensitiveAnnotations(_ context.Context) (map[string]bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (map[string]bool, error) {
		return mgr.GetSensitiveAnnotations(), nil
	})
}

func (p *SettingsManagerProvider) SourceHydratorCommitMessageTemplate(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetSourceHydratorCommitMessageTemplate)
}

func (p *SettingsManagerProvider) TrackingMethod(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetTrackingMethod)
}
