package controllers

import (
	"github.com/argoproj/argo-cd/v3/applicationset/generators"
	"github.com/argoproj/argo-cd/v3/applicationset/services"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// InitConfigProvider wires the configbus provider after the reconciler struct is
// built. scmConfig and argoCDService supply SCM / git settings captured into
// StaticFields at construction.
func (r *ApplicationSetReconciler) InitConfigProvider(settingsMgr *settings.SettingsManager, scmConfig *generators.SCMConfig, argoCDService *services.ArgoCDService) {
	r.scmConfig = scmConfig
	r.argoCDService = argoCDService
	r.configProvider = r.newChainProvider(settingsMgr)
	generators.SetDefaultRequeueProvider(r.configProvider)
	if scmConfig != nil {
		scmConfig.SetConfigProvider(r.configProvider)
	}
	if argoCDService != nil {
		argoCDService.SetConfigProvider(r.configProvider)
	}
}

// ensureConfigProvider lazily wires a Static/Env chain. Production always
// calls InitConfigProvider; unit tests that construct the reconciler directly
// hit this path so configProvider getters remain usable.
func (r *ApplicationSetReconciler) ensureConfigProvider() {
	if r.configProvider == nil {
		r.configProvider = r.newChainProvider(nil)
		generators.SetDefaultRequeueProvider(r.configProvider)
		if r.scmConfig != nil {
			r.scmConfig.SetConfigProvider(r.configProvider)
		}
		if r.argoCDService != nil {
			r.argoCDService.SetConfigProvider(r.configProvider)
		}
	}
}

func (r *ApplicationSetReconciler) newChainProvider(settingsMgr *settings.SettingsManager) configbus.Provider {
	static := r.staticFields()
	links := []configbus.Provider{
		&configbus.StaticProvider{Fields: static},
	}
	if settingsMgr != nil {
		links = append(links, configbus.NewSettingsManagerProvider(settingsMgr))
	}
	links = append(links, configbus.NewEnvProvider())
	return configbus.NewChainProvider(links...)
}

func (r *ApplicationSetReconciler) staticFields() configbus.StaticFields {
	//nolint:staticcheck // SA1019: StaticFields capture construction-time opts once at wire-up
	fields := configbus.StaticFields{
		ApplicationsetConcurrentApplicationUpdates: configbus.Ptr(r.ConcurrentApplicationUpdates),
		ApplicationsetEnablePolicyOverride:         configbus.Ptr(r.EnablePolicyOverride),
		ApplicationsetEnableProgressiveSyncs:       configbus.Ptr(r.EnableProgressiveSyncs),
		ApplicationsetGlobalPreservedAnnotations:   configbus.Ptr(r.GlobalPreservedAnnotations),
		ApplicationsetGlobalPreservedLabels:        configbus.Ptr(r.GlobalPreservedLabels),
		ApplicationsetMaxResourcesStatusCount:      configbus.Ptr(r.MaxResourcesStatusCount),
		ApplicationsetMetricsAddr:                  configbus.Ptr(r.MetricsAddr),
		ApplicationsetMetricsApplicationsetLabels:  configbus.Ptr(r.MetricsApplicationsetLabels),
		ApplicationsetNamespaces:                   configbus.Ptr(r.ApplicationSetNamespaces),
		ApplicationsetPolicy:                       configbus.Ptr(string(r.Policy)),
		ApplicationsetProbeAddr:                    configbus.Ptr(r.ProbeAddr),
		ApplicationsetWebhookAddr:                  configbus.Ptr(r.WebhookAddr),
	}
	if r.scmConfig != nil {
		fields.ApplicationsetAllowedScmProviders = configbus.Ptr(r.scmConfig.LegacyAllowedSCMProviders())
		fields.ApplicationsetEnableGitHubAPIMetrics = configbus.Ptr(r.scmConfig.LegacyEnableGitHubAPIMetrics())
		fields.ApplicationsetEnableScmProviders = configbus.Ptr(r.scmConfig.LegacyEnableSCMProviders())
		fields.ApplicationsetScmNoProxy = configbus.Ptr(r.scmConfig.LegacyScmNoProxy())
		fields.ApplicationsetScmProxyURL = configbus.Ptr(r.scmConfig.LegacyScmProxyURL())
		fields.ApplicationsetScmRootCAPath = configbus.Ptr(r.scmConfig.LegacyScmRootCAPath())
		fields.ApplicationsetTokenRefStrictMode = configbus.Ptr(r.scmConfig.LegacyTokenRefStrictMode())
	}
	if r.argoCDService != nil {
		fields.ApplicationsetEnableNewGitFileGlobbing = configbus.Ptr(r.argoCDService.LegacyNewFileGlobbingEnabled())
		fields.ApplicationsetGitSubmoduleEnabled = configbus.Ptr(r.argoCDService.LegacySubmoduleEnabled())
	}
	return fields
}
