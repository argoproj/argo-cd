package configbus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestEnvProviderGitRequestTimeoutDefault(t *testing.T) {
	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "")
	p := NewEnvProvider()
	d, err := p.GitRequestTimeout()
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, d)

	t.Setenv("ARGOCD_GIT_REQUEST_TIMEOUT", "30s")
	d, err = p.GitRequestTimeout()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d)
}

func TestEnvProviderUnownedReturnsErrNotConfigured(t *testing.T) {
	p := NewEnvProvider()
	_, err := p.SyncTimeout()
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestSettingsManagerProviderRequiresMgr(t *testing.T) {
	p := NewSettingsManagerProvider(nil)
	_, err := p.ResourceOverrides()
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotConfigured)

	_, err = p.SyncTimeout()
	assert.ErrorIs(t, err, ErrNotConfigured)
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
		SelfHealBackoff:           PtrPtr(backoff),
	}}

	d, err := p.SyncTimeout()
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, d)

	gotLabels, err := p.MetricsClusterLabels()
	require.NoError(t, err)
	assert.Equal(t, []string{"team"}, gotLabels)

	jqTimeout, err := p.IgnoreNormalizerJQTimeout()
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, jqTimeout)

	b, err := p.ServerSideDiff()
	require.NoError(t, err)
	assert.True(t, b)

	gotBackoff, err := p.SelfHealBackoff()
	require.NoError(t, err)
	assert.Equal(t, backoff, gotBackoff)

	_, err = p.AppInstanceLabelKey()
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestStaticProviderConfiguredNilPointer(t *testing.T) {
	var nilBackoff *wait.Backoff
	p := &StaticProvider{Fields: StaticFields{SelfHealBackoff: PtrPtr(nilBackoff)}}
	got, err := p.SelfHealBackoff()
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCRDProviderReturnsErrNotConfigured(t *testing.T) {
	p := NewCRDProvider(nil)

	_, err := p.ReconciliationTimeout()
	assert.ErrorIs(t, err, ErrNotConfigured)

	_, err = p.ResourceOverrides()
	assert.ErrorIs(t, err, ErrNotConfigured)

	_, err = p.GitRequestTimeout()
	assert.ErrorIs(t, err, ErrNotConfigured)
}

func TestChainProviderPrecedence(t *testing.T) {
	overrideTimeout := 10 * time.Second
	fallbackTimeout := 90 * time.Second
	override := &StaticProvider{Fields: StaticFields{SyncTimeout: &overrideTimeout}}
	fallback := &StaticProvider{Fields: StaticFields{SyncTimeout: &fallbackTimeout, SelfHealTimeout: Ptr(30 * time.Second)}}
	chain := NewChainProvider(override, NewCRDProvider(nil), fallback, NewEnvProvider())

	d, err := chain.SyncTimeout()
	require.NoError(t, err)
	assert.Equal(t, 10*time.Second, d, "override Static must beat fallback")

	d, err = chain.SelfHealTimeout()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, d, "fallback Static supplies unset override fields")

	d, err = chain.GitRequestTimeout()
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, d, "EnvProvider supplies env-only fields")
}

func TestChainProviderSkipsErrNotConfigured(t *testing.T) {
	chain := NewChainProvider(NewCRDProvider(nil), &StaticProvider{Fields: StaticFields{ReconciliationTimeout: Ptr(120 * time.Second)}})
	d, err := chain.ReconciliationTimeout()
	require.NoError(t, err)
	assert.Equal(t, 120*time.Second, d)
}
