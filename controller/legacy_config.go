package controller

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	"github.com/argoproj/argo-cd/v3/util/configbus"
)

// Ensure ApplicationController satisfies configbus.ControllerLegacy.
var _ configbus.ControllerLegacy = (*ApplicationController)(nil)

// Legacy accessors are the sole allowed readers of deprecated config fields on
// ApplicationController / appStateManager. They implement configbus.ControllerLegacy.

//nolint:staticcheck // SA1019: sole allowed reader of deprecated statusRefreshTimeout
func (ctrl *ApplicationController) LegacyStatusRefreshTimeout() time.Duration {
	return ctrl.statusRefreshTimeout
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated statusHardRefreshTimeout
func (ctrl *ApplicationController) LegacyStatusHardRefreshTimeout() time.Duration {
	return ctrl.statusHardRefreshTimeout
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated statusRefreshJitter
func (ctrl *ApplicationController) LegacyStatusRefreshJitter() time.Duration {
	return ctrl.statusRefreshJitter
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated syncTimeout
func (ctrl *ApplicationController) LegacySyncTimeout() time.Duration {
	return ctrl.syncTimeout
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated selfHealTimeout
func (ctrl *ApplicationController) LegacySelfHealTimeout() time.Duration {
	return ctrl.selfHealTimeout
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated selfHealBackoff
func (ctrl *ApplicationController) LegacySelfHealBackoff() *wait.Backoff {
	return ctrl.selfHealBackoff
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated ignoreNormalizerOpts
func (ctrl *ApplicationController) LegacyIgnoreNormalizerOpts() normalizers.IgnoreNormalizerOpts {
	return ctrl.ignoreNormalizerOpts
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated metricsClusterLabels
func (ctrl *ApplicationController) LegacyMetricsClusterLabels() []string {
	return ctrl.metricsClusterLabels
}

func (ctrl *ApplicationController) LegacyServerSideDiff() bool {
	if m, ok := ctrl.appStateManager.(*appStateManager); ok {
		return m.LegacyServerSideDiff()
	}
	return false
}

func (ctrl *ApplicationController) LegacyPersistResourceHealth() bool {
	if m, ok := ctrl.appStateManager.(*appStateManager); ok {
		return m.LegacyPersistResourceHealth()
	}
	return false
}

func (ctrl *ApplicationController) LegacyRepoErrorGracePeriod() time.Duration {
	if m, ok := ctrl.appStateManager.(*appStateManager); ok {
		return m.LegacyRepoErrorGracePeriod()
	}
	return 0
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated serverSideDiff
func (m *appStateManager) LegacyServerSideDiff() bool {
	return m.serverSideDiff
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated persistResourceHealth
func (m *appStateManager) LegacyPersistResourceHealth() bool {
	return m.persistResourceHealth
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated repoErrorGracePeriod
func (m *appStateManager) LegacyRepoErrorGracePeriod() time.Duration {
	return m.repoErrorGracePeriod
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated statusRefreshTimeout
func (m *appStateManager) LegacyStatusRefreshTimeout() time.Duration {
	return m.statusRefreshTimeout
}

//nolint:staticcheck // SA1019: sole allowed reader of deprecated ignoreNormalizerOpts
func (m *appStateManager) LegacyIgnoreNormalizerOpts() normalizers.IgnoreNormalizerOpts {
	return m.ignoreNormalizerOpts
}

// Test/constructor writers for deprecated fields (SA1019). Prefer constructing via
// NewApplicationController; these exist for tests that mutate config after init.

//nolint:staticcheck // SA1019: sole allowed writer of deprecated syncTimeout
func (ctrl *ApplicationController) setLegacySyncTimeout(d time.Duration) {
	ctrl.syncTimeout = d
}

//nolint:staticcheck // SA1019: sole allowed writer of deprecated selfHealBackoff
func (ctrl *ApplicationController) setLegacySelfHealBackoff(b *wait.Backoff) {
	ctrl.selfHealBackoff = b
}
