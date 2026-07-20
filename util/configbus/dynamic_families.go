package configbus

import (
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func init() {
	registerDynamicFamilies()
}

func registerDynamicFamilies() {
	MustRegisterDynamic(DynamicSetting[*v1alpha1.KustomizeOptions]{
		Name:        "kustomizeVersions",
		CMKeyPrefix: "kustomize.version.",
		KeyFunc:     suffixAfterPrefixKeyFunc("kustomize.version."),
		Get: func(ctx *ResolveContext) (*v1alpha1.KustomizeOptions, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			return mgr.GetKustomizeSettings()
		},
	})
	MustRegisterDynamic(DynamicSetting[*v1alpha1.KustomizeOptions]{
		Name:        "kustomizePaths",
		CMKeyPrefix: "kustomize.path.",
		KeyFunc:     suffixAfterPrefixKeyFunc("kustomize.path."),
		Get: func(ctx *ResolveContext) (*v1alpha1.KustomizeOptions, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			return mgr.GetKustomizeSettings()
		},
	})
	MustRegisterDynamic(DynamicSetting[*v1alpha1.KustomizeOptions]{
		Name:        "kustomizeBuildOptionsVersions",
		CMKeyPrefix: "kustomize.buildOptions.",
		KeyFunc:     suffixAfterPrefixKeyFunc("kustomize.buildOptions."),
		Get: func(ctx *ResolveContext) (*v1alpha1.KustomizeOptions, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			return mgr.GetKustomizeSettings()
		},
	})
	MustRegisterDynamic(DynamicSetting[*settings.Help]{
		Name:        "helpDownload",
		CMKeyPrefix: "help.download.",
		KeyFunc:     suffixAfterPrefixKeyFunc("help.download."),
		Get: func(ctx *ResolveContext) (*settings.Help, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			return mgr.GetHelp()
		},
	})
	MustRegisterDynamic(DynamicSetting[map[string]string]{
		Name:        "extensionConfig",
		CMKeyPrefix: "extension.config",
		KeyFunc:     extensionConfigKeyFunc,
		Get: func(ctx *ResolveContext) (map[string]string, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			s, err := mgr.GetSettings()
			if err != nil {
				return nil, err
			}
			return s.ExtensionConfig, nil
		},
	})
	MustRegisterDynamic(DynamicSetting[map[string]settings.Account]{
		Name:        "accounts",
		CMKeyPrefix: "accounts.",
		KeyFunc:     accountsKeyFunc,
		Get: func(ctx *ResolveContext) (map[string]settings.Account, error) {
			mgr, err := requireSettingsMgr(ctx)
			if err != nil {
				return nil, err
			}
			return mgr.GetAccounts()
		},
	})
}

// suffixAfterPrefixKeyFunc claims any key under prefix; elementID is the suffix.
func suffixAfterPrefixKeyFunc(prefix string) KeyFunc {
	return func(key string) (elementID, subField string, ok bool) {
		if key == strings.TrimSuffix(prefix, ".") {
			return "*", "", true
		}
		if !strings.HasPrefix(key, prefix) {
			return "", "", false
		}
		return strings.TrimPrefix(key, prefix), "", true
	}
}

func extensionConfigKeyFunc(key string) (elementID, subField string, ok bool) {
	const prefix = "extension.config"
	if key == prefix {
		return "*", "", true
	}
	if !strings.HasPrefix(key, prefix+".") && !strings.HasPrefix(key, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, prefix)
	rest = strings.TrimPrefix(rest, ".")
	if rest == "" {
		return "*", "", true
	}
	return rest, "", true
}

func accountsKeyFunc(key string) (elementID, subField string, ok bool) {
	if !strings.HasPrefix(key, "accounts.") {
		return "", "", false
	}
	parts := strings.Split(key, ".")
	// accounts.<name> or accounts.<name>.enabled
	if len(parts) < 2 {
		return "", "", false
	}
	name := parts[1]
	sub := ""
	if len(parts) >= 3 {
		sub = strings.Join(parts[2:], ".")
	}
	return name, sub, true
}
