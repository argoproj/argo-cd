package configbus

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

// ControllerLegacy is implemented by *controller.ApplicationController.
// Methods return component-resolved flag/env values already stored on the
// controller (or structs it owns).
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

// MetricsClusterLabels returns controller metrics cluster labels from Legacy.
func (p *Provider) MetricsClusterLabels() ([]string, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacyMetricsClusterLabels(), nil
}

// SelfHealTimeout returns the controller self-heal timeout from Legacy.
func (p *Provider) SelfHealTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySelfHealTimeout(), nil
}

// SyncTimeout returns the controller sync timeout from Legacy.
func (p *Provider) SyncTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySyncTimeout(), nil
}

// RepoErrorGracePeriod returns the repo-error grace period from Legacy.
func (p *Provider) RepoErrorGracePeriod() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyRepoErrorGracePeriod(), nil
}

// PersistResourceHealth returns whether resource health is persisted (Legacy).
func (p *Provider) PersistResourceHealth() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyPersistResourceHealth(), nil
}

// ServerSideDiff returns whether server-side diff is enabled (Legacy).
func (p *Provider) ServerSideDiff() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyServerSideDiff(), nil
}

// IgnoreNormalizerJQTimeout returns the JQ execution timeout from Legacy opts.
func (p *Provider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyIgnoreNormalizerOpts().JQExecutionTimeout, nil
}

// IgnoreNormalizerOpts returns ignore-normalizer options from Legacy.
func (p *Provider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return normalizers.IgnoreNormalizerOpts{}, err
	}
	return c.LegacyIgnoreNormalizerOpts(), nil
}

// SelfHealBackoff returns the self-heal backoff from Legacy.
func (p *Provider) SelfHealBackoff() (*wait.Backoff, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacySelfHealBackoff(), nil
}
