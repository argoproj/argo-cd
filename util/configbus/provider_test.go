package configbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestEnvProviderGitRequestTimeoutDefault(t *testing.T) {
	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "")
	p := NewEnvProvider()
	d, err := p.GitRequestTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, d)

	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "30s")
	d, err = p.GitRequestTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)
}

func TestEnvProviderUnownedReturnsErrNotConfigured(t *testing.T) {
	p := NewEnvProvider()
	_, err := p.SyncTimeout(context.Background())
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestSettingsManagerProviderRequiresMgr(t *testing.T) {
	assert.Panics(t, func() {
		NewSettingsManagerProvider(nil)
	})
}

func TestStaticProviderRoundTrip(t *testing.T) {
	syncTimeout := 5 * time.Minute
	labels := []string{"team"}
	backoff := &wait.Backoff{Duration: time.Second}
	p := &StaticProvider{Fields: StaticFields{
		SyncTimeout:               &syncTimeout,
		MetricsClusterLabels:      &labels,
		IgnoreNormalizerJQTimeout: Ptr(2 * time.Second),
		ServerSideDiff:            Ptr(true),
		SelfHealRetry:             Ptr(SelfHealRetry{Backoff: backoff}),
	}}

	d, err := p.SyncTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)

	gotLabels, err := p.MetricsClusterLabels(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"team"}, gotLabels)

	jqTimeout, err := p.IgnoreNormalizerJQTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, jqTimeout)

	b, err := p.ServerSideDiff(context.Background())
	require.NoError(t, err)
	assert.True(t, b)

	gotRetry, err := p.SelfHealRetry(context.Background())
	require.NoError(t, err)
	assert.Equal(t, backoff, gotRetry.Backoff)

	_, err = p.AppInstanceLabelKey(context.Background())
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestStaticProviderConfiguredNilSelfHealBackoff(t *testing.T) {
	p := &StaticProvider{Fields: StaticFields{SelfHealRetry: Ptr(SelfHealRetry{Backoff: nil})}}
	got, err := p.SelfHealRetry(context.Background())
	require.NoError(t, err)
	assert.Nil(t, got.Backoff)
}

func TestChainProviderPrecedence(t *testing.T) {
	overrideTimeout := 10 * time.Second
	fallbackTimeout := 90 * time.Second
	override := &StaticProvider{Fields: StaticFields{SyncTimeout: &overrideTimeout}}
	fallback := &StaticProvider{Fields: StaticFields{SyncTimeout: &fallbackTimeout, SelfHealTimeout: Ptr(30 * time.Second)}}
	chain := NewChainProvider(override, fallback, NewEnvProvider())

	d, err := chain.SyncTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, d, "override Static must beat fallback")

	d, err = chain.SelfHealTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d, "fallback Static supplies unset override fields")

	d, err = chain.GitRequestTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, d, "EnvProvider supplies env-only fields")
}

func TestChainProviderSkipsErrNotConfigured(t *testing.T) {
	unset := &StaticProvider{}
	chain := NewChainProvider(unset, &StaticProvider{Fields: StaticFields{ReconciliationTimeout: Ptr(120 * time.Second)}})
	d, err := chain.ReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 120*time.Second, d)
}
