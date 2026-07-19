package configbus

import (
	"errors"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	// Setting names (stable registry IDs).
	NameReconciliationTimeout  = "reconciliationTimeout"
	NameResourceCustomizations = "resourceCustomizations"

	// Legacy keys / env (current sources).
	CMKeyTimeoutReconciliation     = "timeout.reconciliation"
	EnvReconciliationTimeout       = "ARGOCD_RECONCILIATION_TIMEOUT"
	CMKeyResourceCustomizations    = "resource.customizations"
	CMPrefixResourceCustomizations = "resource.customizations."
)

func init() {
	registerFirstSlice()
}

func registerFirstSlice() {
	MustRegister(Setting[time.Duration]{
		Name:            NameReconciliationTimeout,
		CMKeyExact:      CMKeyTimeoutReconciliation,
		EnvVar:          EnvReconciliationTimeout,
		HotReload:       false,          // cmd-params / env; cold until Phase 2
		SourceConfigMap: SourceArgoCDCM, // manifests still mount from argocd-cm
		Component:       "controller",
		FlagName:        "app-resync",
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.Controller != nil {
				return ctx.Legacy.Controller.LegacyStatusRefreshTimeout(), nil
			}
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.ReconciliationTimeout != nil {
				return *ctx.Legacy.ReconciliationTimeout, nil
			}
			return 0, errors.New("config: reconciliation timeout not supplied by component")
		},
	})
	MustRegister(Setting[time.Duration]{
		Name:            "hardReconciliationTimeout",
		CMKeyExact:      "timeout.hard.reconciliation",
		EnvVar:          "ARGOCD_HARD_RECONCILIATION_TIMEOUT",
		HotReload:       false,
		SourceConfigMap: SourceArgoCDCM,
		Component:       "controller",
		FlagName:        "app-hard-resync",
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.Controller != nil {
				return ctx.Legacy.Controller.LegacyStatusHardRefreshTimeout(), nil
			}
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.HardReconciliationTimeout != nil {
				return *ctx.Legacy.HardReconciliationTimeout, nil
			}
			return 0, errors.New("config: hard reconciliation timeout not supplied by component")
		},
	})
	MustRegister(Setting[time.Duration]{
		Name:            "reconciliationJitter",
		CMKeyExact:      "timeout.reconciliation.jitter",
		EnvVar:          "ARGOCD_RECONCILIATION_JITTER",
		HotReload:       false,
		SourceConfigMap: SourceArgoCDCM,
		Component:       "controller",
		FlagName:        "app-resync-jitter",
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.Controller != nil {
				return ctx.Legacy.Controller.LegacyStatusRefreshJitter(), nil
			}
			if ctx != nil && ctx.Legacy != nil && ctx.Legacy.ReconciliationJitter != nil {
				return *ctx.Legacy.ReconciliationJitter, nil
			}
			return 0, errors.New("config: reconciliation jitter not supplied by component")
		},
	})

	MustRegisterDynamic(DynamicSetting[map[string]v1alpha1.ResourceOverride]{
		Name:            NameResourceCustomizations,
		CMKeyPrefix:     CMPrefixResourceCustomizations,
		HotReload:       true,
		SourceConfigMap: SourceArgoCDCM,
		KeyFunc:         resourceCustomizationsKeyFunc,
		Get: func(ctx *ResolveContext) (map[string]v1alpha1.ResourceOverride, error) {
			if ctx == nil || ctx.SettingsMgr == nil {
				return nil, errors.New("config: SettingsManager required for resource customizations")
			}
			return ctx.SettingsMgr.GetResourceOverrides()
		},
	})
}

// resourceCustomizationsKeyFunc covers both the monolithic blob key and split keys.
func resourceCustomizationsKeyFunc(key string) (elementID, subField string, ok bool) {
	if key == CMKeyResourceCustomizations {
		return "*", "", true
	}
	if !strings.HasPrefix(key, CMPrefixResourceCustomizations) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, CMPrefixResourceCustomizations)
	// Forms:
	//   resource.customizations.<group_kind>
	//   resource.customizations.health.<group_kind>
	//   resource.customizations.actions.<group_kind>
	//   resource.customizations.ignoreDifferences.<group_kind>
	//   resource.customizations.knownTypeFields.<group_kind>
	//   resource.customizations.useOpenLibs.<group_kind>
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) == 1 {
		return parts[0], "", true
	}
	sub := parts[0]
	switch sub {
	case "health", "actions", "ignoreDifferences", "ignoreResourceUpdates", "knownTypeFields", "useOpenLibs":
		return parts[1], sub, true
	default:
		// Treat as group_kind that itself contains dots (unusual) or unknown — still claim.
		return rest, "", true
	}
}
