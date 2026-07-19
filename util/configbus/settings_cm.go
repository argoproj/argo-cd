package configbus

import (
	"errors"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func init() {
	registerSettingsCM()
}

// registerCMSetting registers a flat argocd-cm setting with a real Get.
func registerCMSetting[T any](name, cmKey string, hotReload bool, get func(*ResolveContext) (T, error)) {
	MustRegister(Setting[T]{
		Name:            name,
		CMKeyExact:      cmKey,
		HotReload:       hotReload,
		SourceConfigMap: SourceArgoCDCM,
		Get:             get,
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
	registerCMSetting("appInstanceLabelKey", "application.instanceLabelKey", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetAppInstanceLabelKey()
	})
	registerCMSetting("resourceTrackingMethod", "application.resourceTrackingMethod", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetTrackingMethod()
	})
	registerCMSetting("installationID", "installationID", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetInstallationID()
	})
	registerCMSetting("passwordPattern", "passwordPattern", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetPasswordPattern()
	})
	registerCMSetting("resourcesFilter", "resource.exclusions", true, func(ctx *ResolveContext) (*settings.ResourcesFilter, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourcesFilter()
	})
	// resource.inclusions shares GetResourcesFilter; register a sibling that covers the key.
	registerCMSetting("resourceInclusions", "resource.inclusions", true, func(ctx *ResolveContext) (*settings.ResourcesFilter, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourcesFilter()
	})
	registerCMSetting("resourceCompareOptions", "resource.compareoptions", true, func(ctx *ResolveContext) (settings.ArgoCDDiffOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return settings.ArgoCDDiffOptions{}, err
		}
		return mgr.GetResourceCompareOptions()
	})
	registerCMSetting("ignoreResourceUpdatesEnabled", "resource.ignoreResourceUpdatesEnabled", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.GetIsIgnoreResourceUpdatesEnabled()
	})
	registerCMSetting("resourceCustomLabels", "resource.customLabels", true, func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetResourceCustomLabels()
	})
	registerCMSetting("resourceIncludeEventLabelKeys", "resource.includeEventLabelKeys", true, func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetIncludeEventLabelKeys(), nil
	})
	registerCMSetting("resourceExcludeEventLabelKeys", "resource.excludeEventLabelKeys", true, func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetExcludeEventLabelKeys(), nil
	})
	registerCMSetting("sensitiveMaskAnnotations", "resource.sensitive.mask.annotations", true, func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetSensitiveAnnotations(), nil
	})
	registerCMSetting("respectRBAC", "resource.respectRBAC", true, func(ctx *ResolveContext) (int, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.RespectRBAC()
	})
	registerCMSetting("helmSettings", "helm.valuesFileSchemes", true, func(ctx *ResolveContext) (*v1alpha1.HelmOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelmSettings()
	})
	registerCMSetting("kustomizeBuildOptions", "kustomize.buildOptions", true, func(ctx *ResolveContext) (*v1alpha1.KustomizeOptions, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetKustomizeSettings()
	})
	registerCMSetting("googleAnalytics", "ga.trackingid", true, func(ctx *ResolveContext) (*settings.GoogleAnalytics, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGoogleAnalytics()
	})
	registerCMSetting("gaAnonymizeUsers", "ga.anonymizeusers", true, func(ctx *ResolveContext) (*settings.GoogleAnalytics, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGoogleAnalytics()
	})
	registerCMSetting("helpChatURL", "help.chatUrl", true, func(ctx *ResolveContext) (*settings.Help, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelp()
	})
	registerCMSetting("helpChatText", "help.chatText", true, func(ctx *ResolveContext) (*settings.Help, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetHelp()
	})
	registerCMSetting("sourceHydratorCommitMessageTemplate", "sourceHydrator.commitMessageTemplate", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetSourceHydratorCommitMessageTemplate()
	})
	registerCMSetting("sourceHydratorReadmeMessageTemplate", "sourceHydrator.readmeMessageTemplate", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetHydratorReadmeTemplate()
	})
	registerCMSetting("commitAuthorName", "commit.author.name", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetCommitAuthorName()
	})
	registerCMSetting("commitAuthorEmail", "commit.author.email", true, func(ctx *ResolveContext) (string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return "", err
		}
		return mgr.GetCommitAuthorEmail()
	})
	registerCMSetting("globalProjects", "globalProjects", true, func(ctx *ResolveContext) ([]settings.GlobalProjectSettings, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetGlobalProjectsSettings()
	})
	registerCMSetting("maxWebhookPayloadSizeMB", "webhook.maxPayloadSizeMB", true, func(ctx *ResolveContext) (int64, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetMaxWebhookPayloadSize(), nil
	})
	registerCMSetting("webhookRefreshJitter", "webhook.refresh.jitter", true, func(ctx *ResolveContext) (time.Duration, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetWebhookRefreshJitter(), nil
	})
	registerCMSetting("webhookRefreshJitterThreshold", "webhook.refresh.jitter.threshold", true, func(ctx *ResolveContext) (int, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetWebhookRefreshJitterThreshold(), nil
	})
	registerCMSetting("impersonationEnabled", "application.sync.impersonation.enabled", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsImpersonationEnabled()
	})
	registerCMSetting("impersonationEnforced", "application.sync.impersonation.enforced", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsImpersonationEnforced()
	})
	registerCMSetting("requireOverridePrivilegeForRevisionSync", "application.sync.requireOverridePrivilegeForRevisionSync", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.RequireOverridePrivilegeForRevisionSync()
	})
	registerCMSetting("allowedNodeLabels", "application.allowedNodeLabels", true, func(ctx *ResolveContext) ([]string, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetAllowedNodeLabels(), nil
	})
	registerCMSetting("inClusterEnabled", "cluster.inClusterEnabled", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.IsInClusterEnabled()
	})
	registerCMSetting("disableApplicationFineGrainedRBACInheritance", "server.rbac.disableApplicationFineGrainedRBACInheritance", true, func(ctx *ResolveContext) (bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return false, err
		}
		return mgr.ApplicationFineGrainedRBACInheritanceDisabled()
	})
	registerCMSetting("maxPodLogsToRender", "server.maxPodLogsToRender", true, func(ctx *ResolveContext) (int64, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return 0, err
		}
		return mgr.GetMaxPodLogsToRender()
	})
	registerCMSetting("applicationLinks", "application.links", true, func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ApplicationDeepLinks)
	})
	registerCMSetting("projectLinks", "project.links", true, func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ProjectDeepLinks)
	})
	registerCMSetting("resourceLinks", "resource.links", true, func(ctx *ResolveContext) ([]settings.DeepLink, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetDeepLinks(settings.ResourceDeepLinks)
	})
	registerCMSetting("kustomizeEnable", "kustomize.enable", true, func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("helmEnable", "helm.enable", true, func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("jsonnetEnable", "jsonnet.enable", true, func(ctx *ResolveContext) (map[string]bool, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetEnabledSourceTypes()
	})
	registerCMSetting("adminEnabled", "admin.enabled", true, func(ctx *ResolveContext) (map[string]settings.Account, error) {
		mgr, err := requireSettingsMgr(ctx)
		if err != nil {
			return nil, err
		}
		return mgr.GetAccounts()
	})

	// --- GetSettings()-backed fields ---
	registerSettingsField("url", "url", true, func(s *settings.ArgoCDSettings) string { return s.URL })
	registerSettingsField("additionalUrls", "additionalUrls", true, func(s *settings.ArgoCDSettings) []string { return s.AdditionalURLs })
	registerSettingsField("dexConfig", "dex.config", true, func(s *settings.ArgoCDSettings) string { return s.DexConfig })
	registerSettingsField("oidcConfig", "oidc.config", true, func(s *settings.ArgoCDSettings) string { return s.OIDCConfigRAW })
	registerSettingsField("statusBadgeEnabled", "statusbadge.enabled", true, func(s *settings.ArgoCDSettings) bool { return s.StatusBadgeEnabled })
	registerSettingsField("statusBadgeURL", "statusbadge.url", true, func(s *settings.ArgoCDSettings) string { return s.StatusBadgeRootUrl })
	registerSettingsField("anonymousUserEnabled", "users.anonymous.enabled", true, func(s *settings.ArgoCDSettings) bool { return s.AnonymousUserEnabled })
	registerSettingsField("userSessionDuration", "users.session.duration", true, func(s *settings.ArgoCDSettings) time.Duration { return s.UserSessionDuration })
	registerSettingsField("uiCssURL", "ui.cssurl", true, func(s *settings.ArgoCDSettings) string { return s.UiCssURL })
	registerSettingsField("uiBannerContent", "ui.bannercontent", true, func(s *settings.ArgoCDSettings) string { return s.UiBannerContent })
	registerSettingsField("uiBannerURL", "ui.bannerurl", true, func(s *settings.ArgoCDSettings) string { return s.UiBannerURL })
	registerSettingsField("uiBannerPermanent", "ui.bannerpermanent", true, func(s *settings.ArgoCDSettings) bool { return s.UiBannerPermanent })
	registerSettingsField("uiBannerPosition", "ui.bannerposition", true, func(s *settings.ArgoCDSettings) string { return s.UiBannerPosition })
	registerSettingsField("uiLoginButtonText", "ui.loginButtonText", true, func(s *settings.ArgoCDSettings) string { return s.UiLoginButtonText })
	registerSettingsField("execEnabled", "exec.enabled", true, func(s *settings.ArgoCDSettings) bool { return s.ExecEnabled })
	registerSettingsField("execShells", "exec.shells", true, func(s *settings.ArgoCDSettings) []string { return s.ExecShells })
	registerSettingsField("oidcTLSInsecureSkipVerify", "oidc.tls.insecure.skip.verify", true, func(s *settings.ArgoCDSettings) bool {
		return s.OIDCTLSInsecureSkipVerify
	})

	// Secret-backed keys (values live in argocd-secret; names appear in settings.go).
	registerSecretBytes("serverSecretKey", "server.secretkey", func(s *settings.ArgoCDSettings) []byte { return s.ServerSignature })
	// tls.crt / tls.key: only the in-argocd-secret source is covered here; the external
	// argocd-server-tls secret path is not resolved by these descriptors.
	registerSecretDataKey("tlsCert", "tls.crt")
	registerSecretDataKey("tlsKey", "tls.key")
	registerSecretString("webhookGitHubSecret", "webhook.github.secret", func(s *settings.ArgoCDSettings) string { return s.WebhookGitHubSecret })
	registerSecretString("webhookGitLabSecret", "webhook.gitlab.secret", func(s *settings.ArgoCDSettings) string { return s.WebhookGitLabSecret })
	registerSecretString("webhookBitbucketUUID", "webhook.bitbucket.uuid", func(s *settings.ArgoCDSettings) string { return s.WebhookBitbucketUUID })
	registerSecretString("webhookBitbucketServerSecret", "webhook.bitbucketserver.secret", func(s *settings.ArgoCDSettings) string {
		return s.WebhookBitbucketServerSecret
	})
	registerSecretString("webhookGogsSecret", "webhook.gogs.secret", func(s *settings.ArgoCDSettings) string { return s.WebhookGogsSecret })
	registerSecretString("webhookAzureDevOpsUsername", "webhook.azuredevops.username", func(s *settings.ArgoCDSettings) string {
		return s.WebhookAzureDevOpsUsername
	})
	registerSecretString("webhookAzureDevOpsPassword", "webhook.azuredevops.password", func(s *settings.ArgoCDSettings) string {
		return s.WebhookAzureDevOpsPassword
	})
}

func registerSettingsField[T any](name, cmKey string, hotReload bool, extract func(*settings.ArgoCDSettings) T) {
	MustRegister(Setting[T]{
		Name:            name,
		CMKeyExact:      cmKey,
		HotReload:       hotReload,
		SourceConfigMap: SourceArgoCDCM,
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

func registerSecretString(name, cmKey string, extract func(*settings.ArgoCDSettings) string) {
	MustRegister(Setting[string]{
		Name:            name,
		CMKeyExact:      cmKey,
		HotReload:       true,
		Secret:          true,
		SourceConfigMap: SourceArgoCDCM,
		Get: func(ctx *ResolveContext) (string, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return "", err
			}
			s, err := mgr.GetSettings()
			if err != nil {
				return "", err
			}
			return extract(s), nil
		},
	})
}

func registerSecretBytes(name, cmKey string, extract func(*settings.ArgoCDSettings) []byte) {
	MustRegister(Setting[[]byte]{
		Name:            name,
		CMKeyExact:      cmKey,
		HotReload:       true,
		Secret:          true,
		SourceConfigMap: SourceArgoCDCM,
		Get: func(ctx *ResolveContext) ([]byte, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			s, err := mgr.GetSettings()
			if err != nil {
				return nil, err
			}
			return extract(s), nil
		},
	})
}

func registerSecretDataKey(name, dataKey string) {
	MustRegister(Setting[[]byte]{
		Name:            name,
		CMKeyExact:      dataKey,
		HotReload:       true,
		Secret:          true,
		SourceConfigMap: SourceArgoCDCM,
		Get: func(ctx *ResolveContext) ([]byte, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			secret, err := mgr.GetSecretByName(common.ArgoCDSecretName)
			if err != nil {
				return nil, fmt.Errorf("config: reading %s: %w", common.ArgoCDSecretName, err)
			}
			return secret.Data[dataKey], nil
		},
	})
}
