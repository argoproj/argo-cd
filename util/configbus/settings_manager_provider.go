//nolint:staticcheck // SA1019: this file is the allowed bridge to deprecated SettingsManager product getters.
package configbus

import (
	"context"
	"time"

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

func (p *SettingsManagerProvider) Accounts(_ context.Context) (map[string]settings.Account, error) {
	return withMgr(p, (*settings.SettingsManager).GetAccounts)
}

func (p *SettingsManagerProvider) AdditionalURLs(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return nil, err
		}
		return sett.AdditionalURLs, nil
	})
}

func (p *SettingsManagerProvider) AllowedNodeLabels(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		return mgr.GetAllowedNodeLabels(), nil
	})
}

func (p *SettingsManagerProvider) AnonymousUserEnabled(_ context.Context) (bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (bool, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return false, err
		}
		return sett.AnonymousUserEnabled, nil
	})
}

func (p *SettingsManagerProvider) AppInstanceLabelKey(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetAppInstanceLabelKey)
}

func (p *SettingsManagerProvider) ApplicationDeepLinks(_ context.Context) ([]settings.DeepLink, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]settings.DeepLink, error) {
		return mgr.GetDeepLinks(settings.ApplicationDeepLinks)
	})
}

func (p *SettingsManagerProvider) ApplicationFineGrainedRBACInheritanceDisabled(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).ApplicationFineGrainedRBACInheritanceDisabled)
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

func (p *SettingsManagerProvider) ExecEnabled(_ context.Context) (bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (bool, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return false, err
		}
		return sett.ExecEnabled, nil
	})
}

func (p *SettingsManagerProvider) ExecShells(_ context.Context) ([]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]string, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return nil, err
		}
		return sett.ExecShells, nil
	})
}

func (p *SettingsManagerProvider) ExtensionConfig(_ context.Context) (map[string]string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (map[string]string, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return nil, err
		}
		return sett.ExtensionConfig, nil
	})
}

func (p *SettingsManagerProvider) GlobalProjectsSettings(_ context.Context) ([]settings.GlobalProjectSettings, error) {
	return withMgr(p, (*settings.SettingsManager).GetGlobalProjectsSettings)
}

func (p *SettingsManagerProvider) GoogleAnalytics(_ context.Context) (settings.GoogleAnalytics, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (settings.GoogleAnalytics, error) {
		ga, err := mgr.GetGoogleAnalytics()
		if err != nil {
			return settings.GoogleAnalytics{}, err
		}
		if ga == nil {
			return settings.GoogleAnalytics{}, nil
		}
		return *ga, nil
	})
}

func (p *SettingsManagerProvider) HelmSettings(_ context.Context) (v1alpha1.HelmOptions, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (v1alpha1.HelmOptions, error) {
		opts, err := mgr.GetHelmSettings()
		if err != nil {
			return v1alpha1.HelmOptions{}, err
		}
		if opts == nil {
			return v1alpha1.HelmOptions{}, nil
		}
		return *opts, nil
	})
}

func (p *SettingsManagerProvider) Help(_ context.Context) (settings.Help, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (settings.Help, error) {
		help, err := mgr.GetHelp()
		if err != nil {
			return settings.Help{}, err
		}
		if help == nil {
			return settings.Help{}, nil
		}
		return *help, nil
	})
}

func (p *SettingsManagerProvider) HydratorReadmeTemplate(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetHydratorReadmeTemplate)
}

func (p *SettingsManagerProvider) IgnoreResourceUpdatesOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetIgnoreResourceUpdatesOverrides)
}

func (p *SettingsManagerProvider) InClusterEnabled(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).IsInClusterEnabled)
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

func (p *SettingsManagerProvider) KustomizeSettings(_ context.Context) (v1alpha1.KustomizeOptions, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (v1alpha1.KustomizeOptions, error) {
		opts, err := mgr.GetKustomizeSettings()
		if err != nil {
			return v1alpha1.KustomizeOptions{}, err
		}
		if opts == nil {
			return v1alpha1.KustomizeOptions{}, nil
		}
		return *opts, nil
	})
}

func (p *SettingsManagerProvider) MaxPodLogsToRender(_ context.Context) (int64, error) {
	return withMgr(p, (*settings.SettingsManager).GetMaxPodLogsToRender)
}

func (p *SettingsManagerProvider) MaxWebhookPayloadSize(_ context.Context) (int64, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (int64, error) {
		return mgr.GetMaxWebhookPayloadSize(), nil
	})
}

func (p *SettingsManagerProvider) OIDCLogoutURL(_ context.Context) (string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (string, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return "", err
		}
		if oidcConfig := sett.OIDCConfig(); oidcConfig != nil {
			return oidcConfig.LogoutURL, nil
		}
		return "", nil
	})
}

func (p *SettingsManagerProvider) PasswordPattern(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetPasswordPattern)
}

func (p *SettingsManagerProvider) ProjectDeepLinks(_ context.Context) ([]settings.DeepLink, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]settings.DeepLink, error) {
		return mgr.GetDeepLinks(settings.ProjectDeepLinks)
	})
}

func (p *SettingsManagerProvider) RequireOverridePrivilegeForRevisionSync(_ context.Context) (bool, error) {
	return withMgr(p, (*settings.SettingsManager).RequireOverridePrivilegeForRevisionSync)
}

func (p *SettingsManagerProvider) ResourceCompareOptions(_ context.Context) (settings.ArgoCDDiffOptions, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCompareOptions)
}

func (p *SettingsManagerProvider) ResourceCustomLabels(_ context.Context) ([]string, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceCustomLabels)
}

func (p *SettingsManagerProvider) ResourceDeepLinks(_ context.Context) ([]settings.DeepLink, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) ([]settings.DeepLink, error) {
		return mgr.GetDeepLinks(settings.ResourceDeepLinks)
	})
}

func (p *SettingsManagerProvider) ResourceOverrides(_ context.Context) (map[string]v1alpha1.ResourceOverride, error) {
	return withMgr(p, (*settings.SettingsManager).GetResourceOverrides)
}

func (p *SettingsManagerProvider) ResourcesFilter(_ context.Context) (settings.ResourcesFilter, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (settings.ResourcesFilter, error) {
		f, err := mgr.GetResourcesFilter()
		if err != nil {
			return settings.ResourcesFilter{}, err
		}
		if f == nil {
			return settings.ResourcesFilter{}, nil
		}
		return *f, nil
	})
}

func (p *SettingsManagerProvider) RespectRBAC(_ context.Context) (int, error) {
	return withMgr(p, (*settings.SettingsManager).RespectRBAC)
}

func (p *SettingsManagerProvider) SensitiveAnnotations(_ context.Context) (map[string]bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (map[string]bool, error) {
		return mgr.GetSensitiveAnnotations(), nil
	})
}

func (p *SettingsManagerProvider) ServerURL(_ context.Context) (string, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (string, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return "", err
		}
		return sett.URL, nil
	})
}

func (p *SettingsManagerProvider) SourceHydratorCommitMessageTemplate(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetSourceHydratorCommitMessageTemplate)
}

func (p *SettingsManagerProvider) StatusBadgeEnabled(_ context.Context) (bool, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (bool, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return false, err
		}
		return sett.StatusBadgeEnabled, nil
	})
}

func (p *SettingsManagerProvider) TrackingMethod(_ context.Context) (string, error) {
	return withMgr(p, (*settings.SettingsManager).GetTrackingMethod)
}

func (p *SettingsManagerProvider) UserSessionDuration(_ context.Context) (time.Duration, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (time.Duration, error) {
		sett, err := mgr.GetSettings()
		if err != nil {
			return 0, err
		}
		return sett.UserSessionDuration, nil
	})
}

func (p *SettingsManagerProvider) WebhookRefreshJitter(_ context.Context) (time.Duration, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (time.Duration, error) {
		return mgr.GetWebhookRefreshJitter(), nil
	})
}

func (p *SettingsManagerProvider) WebhookRefreshJitterThreshold(_ context.Context) (int, error) {
	return withMgr(p, func(mgr *settings.SettingsManager) (int, error) {
		return mgr.GetWebhookRefreshJitterThreshold(), nil
	})
}
