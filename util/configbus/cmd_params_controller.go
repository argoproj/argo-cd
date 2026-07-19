package configbus

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

// Registry names for application-controller settings with a durable home on
// ApplicationController / appStateManager (stable Resolve IDs).
const (
	NameControllerMetricsClusterLabels        = "controllerMetricsClusterLabels"
	NameControllerSelfHealTimeoutSeconds      = "controllerSelfHealTimeoutSeconds"
	NameControllerSyncTimeoutSeconds          = "controllerSyncTimeoutSeconds"
	NameControllerRepoErrorGracePeriodSeconds = "controllerRepoErrorGracePeriodSeconds"
	NameControllerResourceHealthPersist       = "controllerResourceHealthPersist"
	NameControllerDiffServerSide              = "controllerDiffServerSide"
	NameControllerIgnoreNormalizerJQTimeout   = "controllerIgnoreNormalizerJqTimeout"
)

// ControllerLegacy is implemented by *controller.ApplicationController.
// Methods return component-resolved flag/env values already stored on the
// controller (or structs it owns). Setting.Get callbacks read only through
// this interface; runtime code uses these Legacy* methods (or Provider
// getters that Resolve to them).
type ControllerLegacy interface {
	LegacyStatusRefreshTimeout() time.Duration
	LegacyStatusHardRefreshTimeout() time.Duration
	LegacyStatusRefreshJitter() time.Duration
	LegacySyncTimeout() time.Duration
	LegacySelfHealTimeout() time.Duration
	LegacySelfHealBackoff() *wait.Backoff
	LegacyIgnoreNormalizerOpts() normalizers.IgnoreNormalizerOpts
	LegacyMetricsClusterLabels() []string
	LegacyServerSideDiff() bool
	LegacyPersistResourceHealth() bool
	LegacyRepoErrorGracePeriod() time.Duration
}

func requireControllerLegacy(ctx *ResolveContext) (ControllerLegacy, error) {
	if ctx == nil || ctx.Legacy == nil || ctx.Legacy.Controller == nil {
		return nil, fmt.Errorf("config: ControllerLegacy not supplied by component")
	}
	return ctx.Legacy.Controller, nil
}

func init() {
	registerControllerLegacySettings()
}

func registerControllerLegacySettings() {
	MustRegister(Setting[[]string]{
		Name: NameControllerMetricsClusterLabels, CMKeyExact: "controller.metrics.cluster.labels",
		EnvVar: "ARGOCD_APPLICATION_CONTROLLER_METRICS_CLUSTER_LABELS", FlagName: "metrics-cluster-labels",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) ([]string, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return nil, err
			}
			return c.LegacyMetricsClusterLabels(), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name: NameControllerSelfHealTimeoutSeconds, CMKeyExact: "controller.self.heal.timeout.seconds",
		EnvVar: "ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS", FlagName: "self-heal-timeout-seconds",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return 0, err
			}
			return c.LegacySelfHealTimeout(), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name: NameControllerSyncTimeoutSeconds, CMKeyExact: "controller.sync.timeout.seconds",
		EnvVar: "ARGOCD_APPLICATION_CONTROLLER_SYNC_TIMEOUT", FlagName: "sync-timeout",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return 0, err
			}
			return c.LegacySyncTimeout(), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name: NameControllerRepoErrorGracePeriodSeconds, CMKeyExact: "controller.repo.error.grace.period.seconds",
		EnvVar: "ARGOCD_REPO_ERROR_GRACE_PERIOD_SECONDS", FlagName: "repo-error-grace-period-seconds",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return 0, err
			}
			return c.LegacyRepoErrorGracePeriod(), nil
		},
	})
	MustRegister(Setting[bool]{
		Name: NameControllerResourceHealthPersist, CMKeyExact: "controller.resource.health.persist",
		EnvVar: "ARGOCD_APPLICATION_CONTROLLER_PERSIST_RESOURCE_HEALTH", FlagName: "persist-resource-health",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (bool, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return false, err
			}
			return c.LegacyPersistResourceHealth(), nil
		},
	})
	MustRegister(Setting[bool]{
		Name: NameControllerDiffServerSide, CMKeyExact: "controller.diff.server.side",
		EnvVar: "ARGOCD_APPLICATION_CONTROLLER_SERVER_SIDE_DIFF", FlagName: "server-side-diff-enabled",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (bool, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return false, err
			}
			return c.LegacyServerSideDiff(), nil
		},
	})
	MustRegister(Setting[time.Duration]{
		Name: NameControllerIgnoreNormalizerJQTimeout, CMKeyExact: "controller.ignore.normalizer.jq.timeout",
		EnvVar: "ARGOCD_IGNORE_NORMALIZER_JQ_TIMEOUT", FlagName: "ignore-normalizer-jq-execution-timeout-seconds",
		Component: "controller", SourceConfigMap: SourceCmdParamsCM, HotReload: false,
		Get: func(ctx *ResolveContext) (time.Duration, error) {
			c, err := requireControllerLegacy(ctx)
			if err != nil {
				return 0, err
			}
			return c.LegacyIgnoreNormalizerOpts().JQExecutionTimeout, nil
		},
	})
}

// Per-setting getters go through Resolve so each key can later prefer a CRD
// value and warn on deprecated legacy usage independently.

func (p *Provider) MetricsClusterLabels() ([]string, error) {
	return Resolve[[]string](p, NameControllerMetricsClusterLabels)
}
func (p *Provider) SelfHealTimeout() (time.Duration, error) {
	return Resolve[time.Duration](p, NameControllerSelfHealTimeoutSeconds)
}
func (p *Provider) SyncTimeout() (time.Duration, error) {
	return Resolve[time.Duration](p, NameControllerSyncTimeoutSeconds)
}
func (p *Provider) RepoErrorGracePeriod() (time.Duration, error) {
	return Resolve[time.Duration](p, NameControllerRepoErrorGracePeriodSeconds)
}
func (p *Provider) PersistResourceHealth() (bool, error) {
	return Resolve[bool](p, NameControllerResourceHealthPersist)
}
func (p *Provider) ServerSideDiff() (bool, error) {
	return Resolve[bool](p, NameControllerDiffServerSide)
}
func (p *Provider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	return Resolve[time.Duration](p, NameControllerIgnoreNormalizerJQTimeout)
}
func (p *Provider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacyIgnoreNormalizerOpts(), nil
	}
	return normalizers.IgnoreNormalizerOpts{}, fmt.Errorf("config: ControllerLegacy not supplied by component")
}
func (p *Provider) SelfHealBackoff() (*wait.Backoff, error) {
	if p.legacy != nil && p.legacy.Controller != nil {
		return p.legacy.Controller.LegacySelfHealBackoff(), nil
	}
	return nil, fmt.Errorf("config: ControllerLegacy not supplied by component")
}
