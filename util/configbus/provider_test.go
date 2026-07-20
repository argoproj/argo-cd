package configbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

func TestProviderReconciliationTimeoutLegacy(t *testing.T) {
	d := 120 * time.Second
	p := NewProvider(nil, &LegacyValues{ReconciliationTimeout: &d})
	assert.Equal(t, d, p.ReconciliationTimeout())
}

func TestProviderResourceOverridesRequiresSettingsMgr(t *testing.T) {
	p := NewProvider(nil, nil)
	_, err := p.ResourceOverrides()
	require.Error(t, err)
}

func TestProviderHardTimeoutAndJitterLegacy(t *testing.T) {
	hard := 300 * time.Second
	jitter := 60 * time.Second
	p := NewProvider(nil, &LegacyValues{
		HardReconciliationTimeout: &hard,
		ReconciliationJitter:      &jitter,
	})
	assert.Equal(t, hard, p.HardReconciliationTimeout())
	assert.Equal(t, jitter, p.ReconciliationJitter())
}

func TestStandaloneEnvGitRequestTimeoutDefault(t *testing.T) {
	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "")
	p := NewProvider(nil, nil)
	assert.Equal(t, 15*time.Second, p.GitRequestTimeout())

	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "30s")
	assert.Equal(t, 30*time.Second, p.GitRequestTimeout())
}

func TestControllerLegacyRoundTrip(t *testing.T) {
	stub := &stubControllerLegacy{
		statusRefresh:    120 * time.Second,
		syncTimeout:      5 * time.Minute,
		serverSideDiff:   true,
		metricsLabels:    []string{"team"},
		selfHealTimeout:  30 * time.Second,
		persistHealth:    true,
		repoErrorGrace:   90 * time.Second,
		ignoreNormalizer: normalizers.IgnoreNormalizerOpts{JQExecutionTimeout: 2 * time.Second},
	}
	p := NewProvider(nil, &LegacyValues{Controller: stub})

	assert.Equal(t, 120*time.Second, p.ReconciliationTimeout())

	d, err := p.SyncTimeout()
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)

	b, err := p.ServerSideDiff()
	require.NoError(t, err)
	assert.True(t, b)

	labels, err := p.MetricsClusterLabels()
	require.NoError(t, err)
	assert.Equal(t, []string{"team"}, labels)

	jq, err := p.IgnoreNormalizerJQTimeout()
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, jq)
}

type stubControllerLegacy struct {
	statusRefresh, statusHard, statusJitter time.Duration
	syncTimeout, selfHealTimeout            time.Duration
	selfHealBackoff                         *wait.Backoff
	ignoreNormalizer                        normalizers.IgnoreNormalizerOpts
	metricsLabels                           []string
	serverSideDiff, persistHealth           bool
	repoErrorGrace                          time.Duration
}

func (s *stubControllerLegacy) LegacyStatusRefreshTimeout() time.Duration {
	return s.statusRefresh
}

func (s *stubControllerLegacy) LegacyStatusHardRefreshTimeout() time.Duration {
	return s.statusHard
}
func (s *stubControllerLegacy) LegacyStatusRefreshJitter() time.Duration { return s.statusJitter }
func (s *stubControllerLegacy) LegacySyncTimeout() time.Duration         { return s.syncTimeout }
func (s *stubControllerLegacy) LegacySelfHealTimeout() time.Duration     { return s.selfHealTimeout }

func (s *stubControllerLegacy) LegacySelfHealBackoff() *wait.Backoff { return s.selfHealBackoff }

func (s *stubControllerLegacy) LegacyIgnoreNormalizerOpts() normalizers.IgnoreNormalizerOpts {
	return s.ignoreNormalizer
}
func (s *stubControllerLegacy) LegacyMetricsClusterLabels() []string { return s.metricsLabels }
func (s *stubControllerLegacy) LegacyServerSideDiff() bool           { return s.serverSideDiff }
func (s *stubControllerLegacy) LegacyPersistResourceHealth() bool    { return s.persistHealth }
func (s *stubControllerLegacy) LegacyRepoErrorGracePeriod() time.Duration {
	return s.repoErrorGrace
}
