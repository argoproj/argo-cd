package configbus

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

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
