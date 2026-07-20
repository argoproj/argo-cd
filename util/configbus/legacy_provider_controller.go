package configbus

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func (p *LegacyProvider) HardReconciliationTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusHardRefreshTimeout(), nil
}

func (p *LegacyProvider) IgnoreNormalizerJQTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyIgnoreNormalizerOpts().JQExecutionTimeout, nil
}

func (p *LegacyProvider) IgnoreNormalizerOpts() (normalizers.IgnoreNormalizerOpts, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return normalizers.IgnoreNormalizerOpts{}, err
	}
	return c.LegacyIgnoreNormalizerOpts(), nil
}

func (p *LegacyProvider) MetricsClusterLabels() ([]string, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacyMetricsClusterLabels(), nil
}

func (p *LegacyProvider) PersistResourceHealth() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyPersistResourceHealth(), nil
}

func (p *LegacyProvider) ReconciliationJitter() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusRefreshJitter(), nil
}

func (p *LegacyProvider) ReconciliationTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyStatusRefreshTimeout(), nil
}

func (p *LegacyProvider) RepoErrorGracePeriod() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacyRepoErrorGracePeriod(), nil
}

func (p *LegacyProvider) SelfHealBackoff() (*wait.Backoff, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return nil, err
	}
	return c.LegacySelfHealBackoff(), nil
}

func (p *LegacyProvider) SelfHealTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySelfHealTimeout(), nil
}

func (p *LegacyProvider) ServerSideDiff() (bool, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return false, err
	}
	return c.LegacyServerSideDiff(), nil
}

func (p *LegacyProvider) SyncTimeout() (time.Duration, error) {
	c, err := p.requireControllerLegacy()
	if err != nil {
		return 0, err
	}
	return c.LegacySyncTimeout(), nil
}
