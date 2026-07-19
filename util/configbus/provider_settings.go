package configbus

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// AppInstanceLabelKey returns application.instanceLabelKey.
func (p *Provider) AppInstanceLabelKey() (string, error) {
	return Resolve[string](p, "appInstanceLabelKey")
}

// TrackingMethod returns application.resourceTrackingMethod.
func (p *Provider) TrackingMethod() (string, error) {
	return Resolve[string](p, "resourceTrackingMethod")
}

// InstallationID returns installationID.
func (p *Provider) InstallationID() (string, error) {
	return Resolve[string](p, "installationID")
}

// ResourcesFilter returns resource.inclusions / resource.exclusions.
func (p *Provider) ResourcesFilter() (*settings.ResourcesFilter, error) {
	return Resolve[*settings.ResourcesFilter](p, "resourcesFilter")
}

// ResourceCompareOptions returns resource.compareoptions.
func (p *Provider) ResourceCompareOptions() (settings.ArgoCDDiffOptions, error) {
	return Resolve[settings.ArgoCDDiffOptions](p, "resourceCompareOptions")
}

// IsIgnoreResourceUpdatesEnabled returns resource.ignoreResourceUpdatesEnabled.
func (p *Provider) IsIgnoreResourceUpdatesEnabled() (bool, error) {
	return Resolve[bool](p, "ignoreResourceUpdatesEnabled")
}

// IgnoreResourceUpdatesOverrides combines compare options with resource overrides
// for ignore-resource-updates behavior (delegates to SettingsManager).
func (p *Provider) IgnoreResourceUpdatesOverrides() (map[string]v1alpha1.ResourceOverride, error) {
	if p.settingsMgr == nil {
		return nil, errSettingsMgrNil
	}
	return p.settingsMgr.GetIgnoreResourceUpdatesOverrides()
}

// ResourceCustomLabels returns resource.customLabels.
func (p *Provider) ResourceCustomLabels() ([]string, error) {
	return Resolve[[]string](p, "resourceCustomLabels")
}

// SensitiveAnnotations returns resource.sensitive.mask.annotations.
func (p *Provider) SensitiveAnnotations() (map[string]bool, error) {
	return Resolve[map[string]bool](p, "sensitiveMaskAnnotations")
}

// RespectRBAC returns resource.respectRBAC.
func (p *Provider) RespectRBAC() (int, error) {
	return Resolve[int](p, "respectRBAC")
}

// AllowedNodeLabels returns application.allowedNodeLabels.
func (p *Provider) AllowedNodeLabels() []string {
	v, err := Resolve[[]string](p, "allowedNodeLabels")
	if err != nil {
		return nil
	}
	return v
}

// IsImpersonationEnabled returns application.sync.impersonation.enabled.
func (p *Provider) IsImpersonationEnabled() (bool, error) {
	return Resolve[bool](p, "impersonationEnabled")
}

// IsImpersonationEnforced returns application.sync.impersonation.enforced.
func (p *Provider) IsImpersonationEnforced() (bool, error) {
	return Resolve[bool](p, "impersonationEnforced")
}

// EnabledSourceTypes returns kustomize/helm/jsonnet.enable map.
func (p *Provider) EnabledSourceTypes() (map[string]bool, error) {
	return Resolve[map[string]bool](p, "kustomizeEnable")
}

// KustomizeSettings returns kustomize settings (build options + versions).
func (p *Provider) KustomizeSettings() (*v1alpha1.KustomizeOptions, error) {
	return Resolve[*v1alpha1.KustomizeOptions](p, "kustomizeBuildOptions")
}

// HelmSettings returns helm.valuesFileSchemes settings.
func (p *Provider) HelmSettings() (*v1alpha1.HelmOptions, error) {
	return Resolve[*v1alpha1.HelmOptions](p, "helmSettings")
}

// SourceHydratorCommitMessageTemplate returns sourceHydrator.commitMessageTemplate.
func (p *Provider) SourceHydratorCommitMessageTemplate() (string, error) {
	return Resolve[string](p, "sourceHydratorCommitMessageTemplate")
}

// HydratorReadmeTemplate returns sourceHydrator.readmeMessageTemplate.
func (p *Provider) HydratorReadmeTemplate() (string, error) {
	return Resolve[string](p, "sourceHydratorReadmeMessageTemplate")
}

// CommitAuthorName returns commit.author.name.
func (p *Provider) CommitAuthorName() (string, error) {
	return Resolve[string](p, "commitAuthorName")
}

// CommitAuthorEmail returns commit.author.email.
func (p *Provider) CommitAuthorEmail() (string, error) {
	return Resolve[string](p, "commitAuthorEmail")
}

var errSettingsMgrNil = errString("config: SettingsManager is nil")

type errString string

func (e errString) Error() string { return string(e) }
