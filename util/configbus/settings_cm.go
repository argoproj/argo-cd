package configbus

import (
	"errors"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func init() {
	registerSettingsCM()
}

// registerCMSetting registers a flat argocd-cm setting with a real Get.
func registerCMSetting[T any](name, cmKey string, get func(*ResolveContext) (T, error)) {
	MustRegister(Setting[T]{
		Name:       name,
		CMKeyExact: cmKey,
		Get:        get,
	})
}

func requireSettingsMgr(ctx *ResolveContext) (*settings.SettingsManager, error) {
	if ctx == nil || ctx.SettingsMgr == nil {
		return nil, errors.New("config: SettingsManager is required")
	}
	return ctx.SettingsMgr, nil
}

func registerSettingsCM() {
	// --- Typed getters ---
	registerCMSetting("appInstanceLabelKey", "application.instanceLabelKey", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetAppInstanceLabelKey()
	})
	registerCMSetting("resourceTrackingMethod", "application.resourceTrackingMethod", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetTrackingMethod()
	})
	registerCMSetting("installationID", "installationID", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetInstallationID()
	})
	registerCMSetting("passwordPattern", "passwordPattern", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetPasswordPattern()
	})
	registerCMSetting("resourcesFilter", "resource.exclusions", func(ctx *ResolveContext) (*settings.ResourcesFilter, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourcesFilter()
	})
	// resource.inclusions shares GetResourcesFilter; register a sibling that covers the key.
	registerCMSetting("resourceInclusions", "resource.inclusions", func(ctx *ResolveContext) (*settings.ResourcesFilter, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourcesFilter()
	})
	registerCMSetting("resourceCompareOptions", "resource.compareoptions", func(ctx *ResolveContext) (settings.ArgoCDDiffOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return settings.ArgoCDDiffOptions{}, err
		}
		return mgr.GetResourceCompareOptions()
	})
	registerCMSetting("ignoreResourceUpdatesEnabled", "resource.ignoreResourceUpdatesEnabled", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.GetIsIgnoreResourceUpdatesEnabled()
	})
	registerCMSetting("resourceCustomLabels", "resource.customLabels", func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourceCustomLabels()
	})
	registerCMSetting("resourceIncludeEventLabelKeys", "resource.includeEventLabelKeys", func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetIncludeEventLabelKeys(), nil
	})
	registerCMSetting("resourceExcludeEventLabelKeys", "resource.excludeEventLabelKeys", func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetExcludeEventLabelKeys(), nil
	})
	registerCMSetting("sensitiveMaskAnnotations", "resource.sensitive.mask.annotations", func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetSensitiveAnnotations(), nil
	})
	registerCMSetting("respectRBAC", "resource.respectRBAC", func(ctx *ResolveContext) (int, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.RespectRBAC()
	})
	registerCMSetting("helmSettings", "helm.valuesFileSchemes", func(ctx *ResolveContext) (*v1alpha1.HelmOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelmSettings()
	})
	registerCMSetting("kustomizeBuildOptions", "kustomize.buildOptions", func(ctx *ResolveContext) (*v1alpha1.KustomizeOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetKustomizeSettings()
	})
	registerCMSetting("googleAnalytics", "ga.trackingid", func(ctx *ResolveContext) (*settings.GoogleAnalytics, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGoogleAnalytics()
	})
	registerCMSetting("gaAnonymizeUsers", "ga.anonymizeusers", func(ctx *ResolveContext) (*settings.GoogleAnalytics, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGoogleAnalytics()
	})
	registerCMSetting("helpChatURL", "help.chatUrl", func(ctx *ResolveContext) (*settings.Help, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelp()
	})
	registerCMSetting("helpChatText", "help.chatText", func(ctx *ResolveContext) (*settings.Help, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelp()
	})
	registerCMSetting("sourceHydratorCommitMessageTemplate", "sourceHydrator.commitMessageTemplate", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetSourceHydratorCommitMessageTemplate()
	})
	registerCMSetting("sourceHydratorReadmeMessageTemplate", "sourceHydrator.readmeMessageTemplate", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetHydratorReadmeTemplate()
	})
	registerCMSetting("commitAuthorName", "commit.author.name", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetCommitAuthorName()
	})
	registerCMSetting("commitAuthorEmail", "commit.author.email", func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetCommitAuthorEmail()
	})
	registerCMSetting("globalProjects", "globalProjects", func(ctx *ResolveContext) ([]settings.GlobalProjectSettings, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGlobalProjectsSettings()
	})
	registerCMSetting("maxWebhookPayloadSizeMB", "webhook.maxPayloadSizeMB", func(ctx *ResolveContext) (int64, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetMaxWebhookPayloadSize(), nil
	})
	registerCMSetting("webhookRefreshJitter", "webhook.refresh.jitter", func(ctx *ResolveContext) (time.Duration, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetWebhookRefreshJitter(), nil
	})
	registerCMSetting("webhookRefreshJitterThreshold", "webhook.refresh.jitter.threshold", func(ctx *ResolveContext) (int, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetWebhookRefreshJitterThreshold(), nil
	})
	registerCMSetting("impersonationEnabled", "application.sync.impersonation.enabled", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsImpersonationEnabled()
	})
	registerCMSetting("impersonationEnforced", "application.sync.impersonation.enforced", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsImpersonationEnforced()
	})
	registerCMSetting("requireOverridePrivilegeForRevisionSync", "application.sync.requireOverridePrivilegeForRevisionSync", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.RequireOverridePrivilegeForRevisionSync()
	})
	registerCMSetting("allowedNodeLabels", "application.allowedNodeLabels", func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetAllowedNodeLabels(), nil
	})
	registerCMSetting("inClusterEnabled", "cluster.inClusterEnabled", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsInClusterEnabled()
	})
	registerCMSetting("disableApplicationFineGrainedRBACInheritance", "server.rbac.disableApplicationFineGrainedRBACInheritance", func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.ApplicationFineGrainedRBACInheritanceDisabled()
	})
	registerCMSetting("maxPodLogsToRender", "server.maxPodLogsToRender", func(ctx *ResolveContext) (int64, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetMaxPodLogsToRender()
	})
	registerCMSetting("applicationLinks", "application.links", func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ApplicationDeepLinks)
	})
	registerCMSetting("projectLinks", "project.links", func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ProjectDeepLinks)
	})
	registerCMSetting("resourceLinks", "resource.links", func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ResourceDeepLinks)
	})
	registerCMSetting("kustomizeEnable", "kustomize.enable", func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("helmEnable", "helm.enable", func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("jsonnetEnable", "jsonnet.enable", func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("adminEnabled", "admin.enabled", func(ctx *ResolveContext) (map[string]settings.Account, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetAccounts()
	})

	// --- GetSettings()-backed fields ---
	registerSettingsField("url", "url", func(s *settings.ArgoCDSettings) string { return s.URL })
	registerSettingsField("additionalUrls", "additionalUrls", func(s *settings.ArgoCDSettings) []string { return s.AdditionalURLs })
	registerSettingsField("dexConfig", "dex.config", func(s *settings.ArgoCDSettings) string { return s.DexConfig })
	registerSettingsField("oidcConfig", "oidc.config", func(s *settings.ArgoCDSettings) string { return s.OIDCConfigRAW })
	registerSettingsField("statusBadgeEnabled", "statusbadge.enabled", func(s *settings.ArgoCDSettings) bool { return s.StatusBadgeEnabled })
	registerSettingsField("statusBadgeURL", "statusbadge.url", func(s *settings.ArgoCDSettings) string { return s.StatusBadgeRootUrl })
	registerSettingsField("anonymousUserEnabled", "users.anonymous.enabled", func(s *settings.ArgoCDSettings) bool { return s.AnonymousUserEnabled })
	registerSettingsField("userSessionDuration", "users.session.duration", func(s *settings.ArgoCDSettings) time.Duration { return s.UserSessionDuration })
	registerSettingsField("uiCssURL", "ui.cssurl", func(s *settings.ArgoCDSettings) string { return s.UiCssURL })
	registerSettingsField("uiBannerContent", "ui.bannercontent", func(s *settings.ArgoCDSettings) string { return s.UiBannerContent })
	registerSettingsField("uiBannerURL", "ui.bannerurl", func(s *settings.ArgoCDSettings) string { return s.UiBannerURL })
	registerSettingsField("uiBannerPermanent", "ui.bannerpermanent", func(s *settings.ArgoCDSettings) bool { return s.UiBannerPermanent })
	registerSettingsField("uiBannerPosition", "ui.bannerposition", func(s *settings.ArgoCDSettings) string { return s.UiBannerPosition })
	registerSettingsField("uiLoginButtonText", "ui.loginButtonText", func(s *settings.ArgoCDSettings) string { return s.UiLoginButtonText })
	registerSettingsField("execEnabled", "exec.enabled", func(s *settings.ArgoCDSettings) bool { return s.ExecEnabled })
	registerSettingsField("execShells", "exec.shells", func(s *settings.ArgoCDSettings) []string { return s.ExecShells })
	registerSettingsField("oidcTLSInsecureSkipVerify", "oidc.tls.insecure.skip.verify", func(s *settings.ArgoCDSettings) bool {
		return s.OIDCTLSInsecureSkipVerify
	})

}

func registerSettingsField[T any](name, cmKey string, extract func(*settings.ArgoCDSettings) T) {
	MustRegister(Setting[T]{
		Name:       name,
		CMKeyExact: cmKey,
		Get: func(ctx *ResolveContext) (T, error) {
			var zero T
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return zero, err
			}
			s, err := mgr.GetSettings()
			if err != nil {
				return zero, err
			}
			return extract(s), nil
		},
	})
}
