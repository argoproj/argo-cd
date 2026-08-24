package configbus

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// AppInstanceLabelKey returns application.instanceLabelKey.
func (p *Provider) AppInstanceLabelKey() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetAppInstanceLabelKey()
}

// TrackingMethod returns application.resourceTrackingMethod.
func (p *Provider) TrackingMethod() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetTrackingMethod()
}

// InstallationID returns installationID.
func (p *Provider) InstallationID() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetInstallationID()
}

// ResourcesFilter returns resource.inclusions / resource.exclusions.
func (p *Provider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourcesFilter()
}

// ResourceCompareOptions returns resource.compareoptions.
func (p *Provider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return settings.ArgoCDDiffOptions{}, err
	}
	return mgr.GetResourceCompareOptions()
}

// IsIgnoreResourceUpdatesEnabled returns resource.ignoreResourceUpdatesEnabled.
func (p *Provider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.GetIsIgnoreResourceUpdatesEnabled()
}

// IgnoreResourceUpdatesOverrides combines compare options with resource overrides
// for ignore-resource-updates behavior (delegates to SettingsManager).
func (p *Provider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetIgnoreResourceUpdatesOverrides()
}

// ResourceCustomLabels returns resource.customLabels.
func (p *Provider) ResourceCustomLabels() ([]string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetResourceCustomLabels()
}

// SensitiveAnnotations returns resource.sensitive.mask.annotations.
func (p *Provider) SensitiveAnnotations() (map[string]bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetSensitiveAnnotations(), nil
}

// RespectRBAC returns resource.respectRBAC.
func (p *Provider) RespectRBAC() (int, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return 0, err
	}
	return mgr.RespectRBAC()
}

// AllowedNodeLabels returns application.allowedNodeLabels.
func (p *Provider) AllowedNodeLabels() []string {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil
	}
	return mgr.GetAllowedNodeLabels()
}

// IsImpersonationEnabled returns application.sync.impersonation.enabled.
func (p *Provider) IsImpersonationEnabled() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.IsImpersonationEnabled()
}

// IsImpersonationEnforced returns application.sync.impersonation.enforced.
func (p *Provider) IsImpersonationEnforced() (bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return false, err
	}
	return mgr.IsImpersonationEnforced()
}

// EnabledSourceTypes returns kustomize/helm/jsonnet.enable map.
func (p *Provider) EnabledSourceTypes() (map[string]bool, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetEnabledSourceTypes()
}

// KustomizeSettings returns kustomize settings (build options + versions).
func (p *Provider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetKustomizeSettings()
}

// HelmSettings returns helm.valuesFileSchemes settings.
func (p *Provider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return nil, err
	}
	return mgr.GetHelmSettings()
}

// SourceHydratorCommitMessageTemplate returns sourceHydrator.commitMessageTemplate.
func (p *Provider) SourceHydratorCommitMessageTemplate() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetSourceHydratorCommitMessageTemplate()
}

// HydratorReadmeTemplate returns sourceHydrator.readmeMessageTemplate.
func (p *Provider) HydratorReadmeTemplate() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetHydratorReadmeTemplate()
}

// CommitAuthorName returns commit.author.name.
func (p *Provider) CommitAuthorName() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetCommitAuthorName()
}

// CommitAuthorEmail returns commit.author.email.
func (p *Provider) CommitAuthorEmail() (string, error) {
	mgr, err := p.requireSettingsMgr()
	if err != nil {
		return "", err
	}
	return mgr.GetCommitAuthorEmail()
}
