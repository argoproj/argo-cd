package configbus

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/env"
)

func init() {
	registerRBACCM()
	registerStandaloneEnv()
}

func registerRBACCM() {
	MustRegister(Setting[string]{
		Name:            "rbacPolicyCSV",
		CMKeyExact:      "policy.csv",
		HotReload:       true,
		SourceConfigMap: SourceRBACCM,
		Component:       "server",
		Get:             rbacCMStringGet("policy.csv"),
	})
	MustRegister(Setting[string]{
		Name:            "rbacPolicyDefault",
		CMKeyExact:      "policy.default",
		HotReload:       true,
		SourceConfigMap: SourceRBACCM,
		Component:       "server",
		Get:             rbacCMStringGet("policy.default"),
	})
	MustRegister(Setting[string]{
		Name:            "rbacPolicyMatchMode",
		CMKeyExact:      "policy.matchMode",
		HotReload:       true,
		SourceConfigMap: SourceRBACCM,
		Component:       "server",
		Get:             rbacCMStringGet("policy.matchMode"),
	})
	MustRegister(Setting[string]{
		Name:            "rbacScopes",
		CMKeyExact:      "scopes",
		HotReload:       true,
		SourceConfigMap: SourceRBACCM,
		Component:       "server",
		Get:             rbacCMStringGet("scopes"),
	})
	// Overlay merge semantics match util/rbac.PolicyCSV (excluding the primary policy.csv key).
	MustRegisterDynamic(DynamicSetting[string]{
		Name:            "rbacPolicyOverlayCSV",
		CMKeyPrefix:     "policy.",
		HotReload:       true,
		SourceConfigMap: SourceRBACCM,
		Component:       "server",
		KeyFunc:         policyOverlayCSVKeyFunc,
		Get: func(ctx *ResolveContext) (string, error) {
			cm, err := requireRBACCM(ctx)
			if err != nil {
				return "", err
			}
			var keys []string
			for k := range cm {
				if k != "policy.csv" && strings.HasPrefix(k, "policy.") && strings.HasSuffix(k, ".csv") {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			var b strings.Builder
			for i, k := range keys {
				if i > 0 {
					b.WriteString("\n")
				}
				b.WriteString(cm[k])
			}
			return b.String(), nil
		},
	})
}

func rbacCMStringGet(key string) func(*ResolveContext) (string, error) {
	return func(ctx *ResolveContext) (string, error) {
		cm, err := requireRBACCM(ctx)
		if err != nil {
			return "", err
		}
		return cm[key], nil
	}
}

func requireRBACCM(ctx *ResolveContext) (map[string]string, error) {
	mgr, err := requireSettingsMgr(ctx)
	if err != nil {
		return nil, err
	}
	cm, err := mgr.GetConfigMapByName(common.ArgoCDRBACConfigMapName)
	if err != nil {
		return nil, fmt.Errorf("config: reading %s: %w", common.ArgoCDRBACConfigMapName, err)
	}
	return cm.Data, nil
}

func policyOverlayCSVKeyFunc(key string) (elementID, subField string, ok bool) {
	// Covers policy.<name>.csv overlays (not policy.csv, policy.default, policy.matchMode).
	if key == "policy.csv" || key == "policy.default" || key == "policy.matchMode" {
		return "", "", false
	}
	if !strings.HasPrefix(key, "policy.") || !strings.HasSuffix(key, ".csv") {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, "policy.")
	rest = strings.TrimSuffix(rest, ".csv")
	return rest, "csv", true
}

// registerStandaloneEnv registers the truly one-off env vars. Defaults mirror the
// canonical read sites (util/git, util/exec, etc.); those packages are not yet
// routed through the provider.
func registerStandaloneEnv() {
	MustRegister(Setting[time.Duration]{
		Name:            "gitRequestTimeout",
		EnvVar:          "ARGOCD_GIT_REQUEST_TIMEOUT",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (time.Duration, error) {
			// Canonical: util/git/client.go
			return env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name:            "execTimeout",
		EnvVar:          "ARGOCD_EXEC_TIMEOUT",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (time.Duration, error) {
			// Canonical: util/exec/exec.go initTimeout (os.Getenv + ParseDuration, fallback 90s)
			d, err := time.ParseDuration(os.Getenv("ARGOCD_EXEC_TIMEOUT"))
			if err != nil {
				return 90 * time.Second, nil
			}
			return d, nil
		},
	})
	MustRegister(Setting[int64]{
		Name:            "gitLsRemoteParallelismLimit",
		EnvVar:          "ARGOCD_GIT_LS_REMOTE_PARALLELISM_LIMIT",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (int64, error) {
			// Canonical: reposerver/metrics/githandlers.go
			return env.ParseInt64FromEnv("ARGOCD_GIT_LS_REMOTE_PARALLELISM_LIMIT", 0, 0, math.MaxInt64), nil
		},
	})
	MustRegister(Setting[string]{
		Name:            "enableProfilerFilePath",
		EnvVar:          "ARGOCD_ENABLE_PROFILER_FILE_PATH",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (string, error) {
			// Canonical: util/profile/profile.go
			return env.StringFromEnv("ARGOCD_ENABLE_PROFILER_FILE_PATH", "/home/argocd/params/profiler.enabled"), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name:            "applicationsetRequeueAfter",
		EnvVar:          "ARGOCD_APPLICATIONSET_CONTROLLER_REQUEUE_AFTER",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (time.Duration, error) {
			// Canonical: applicationset/generators/interface.go (DefaultRequeueAfter = 3m)
			return env.ParseDurationFromEnv("ARGOCD_APPLICATIONSET_CONTROLLER_REQUEUE_AFTER", 3*time.Minute, 1*time.Second, 8760*time.Hour), nil
		},
	})
	MustRegister(Setting[int64]{
		Name:            "applicationTreeShardSize",
		EnvVar:          "ARGOCD_APPLICATION_TREE_SHARD_SIZE",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (int64, error) {
			// Canonical: util/cache/appstate/cache.go
			return env.ParseInt64FromEnv("ARGOCD_APPLICATION_TREE_SHARD_SIZE", 0, 0, 1000), nil
		},
	})
	MustRegister(Setting[string]{
		Name:            "azureArmTokenResource",
		EnvVar:          "AZURE_ARM_TOKEN_RESOURCE",
		HotReload:       false,
		SourceConfigMap: SourceEnvOnly,
		Get: func(*ResolveContext) (string, error) {
			// Canonical: util/helm/creds.go
			return env.StringFromEnv("AZURE_ARM_TOKEN_RESOURCE", "https://management.core.windows.net"), nil
		},
	})
}
