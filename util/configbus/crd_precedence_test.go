package configbus

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Tests apply an ArgoCDConfiguration only in fixtures — production installs leave the CR absent.

func TestResolve_CRDPrecedencesOverStatic(t *testing.T) {
	trueVal := true
	crdObj := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Reconciliation: &appv1.ReconciliationConfig{
					Timeout:     &metav1.Duration{Duration: 45 * time.Second},
					HardTimeout: &metav1.Duration{Duration: 10 * time.Minute},
					Jitter:      &metav1.Duration{Duration: 15 * time.Second},
				},
				SelfHeal: &appv1.SelfHealConfig{
					Timeout: &metav1.Duration{Duration: 90 * time.Second},
				},
				ResourceHealthPersist: &trueVal,
			},
			InstallationID: "from-crd",
		},
	}
	fallback := &StaticProvider{Fields: StaticFields{
		ReconciliationTimeout:     Ptr(120 * time.Second),
		HardReconciliationTimeout: Ptr(300 * time.Second),
		ReconciliationJitter:      Ptr(60 * time.Second),
		SelfHealTimeout:           Ptr(5 * time.Second),
		PersistResourceHealth:     Ptr(false),
	}}
	p := NewChainProvider(NewCRDProvider(StaticCRDSource{Object: crdObj}), fallback)

	timeout, err := p.ReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 45*time.Second, timeout)

	hard, err := p.HardReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 10*time.Minute, hard)

	jitter, err := p.ReconciliationJitter(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, jitter)

	selfHeal, err := p.SelfHealTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 90*time.Second, selfHeal)

	persist, err := p.PersistResourceHealth(context.Background())
	require.NoError(t, err)
	assert.True(t, persist)

	installID, err := p.InstallationID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "from-crd", installID)
}

func TestResolve_AbsentCRDFallsBackToStatic(t *testing.T) {
	legacyTO := 120 * time.Second
	fallback := &StaticProvider{Fields: StaticFields{
		ReconciliationTimeout:     Ptr(legacyTO),
		HardReconciliationTimeout: Ptr(300 * time.Second),
		ReconciliationJitter:      Ptr(60 * time.Second),
	}}
	p := NewChainProvider(NewCRDProvider(StaticCRDSource{}), fallback)

	timeout, err := p.ReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, legacyTO, timeout)

	hard, err := p.HardReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 300*time.Second, hard)

	jitter, err := p.ReconciliationJitter(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, jitter)
}

func TestResolve_PartialCRDOnlyOverridesSetFields(t *testing.T) {
	legacyTO := 120 * time.Second
	crdObj := &appv1.ArgoCDConfiguration{
		Spec: appv1.ArgoCDConfigurationSpec{
			Controller: &appv1.ControllerConfig{
				Reconciliation: &appv1.ReconciliationConfig{
					Timeout: &metav1.Duration{Duration: 30 * time.Second},
				},
			},
		},
	}
	fallback := &StaticProvider{Fields: StaticFields{
		ReconciliationTimeout:     Ptr(legacyTO),
		HardReconciliationTimeout: Ptr(300 * time.Second),
		ReconciliationJitter:      Ptr(60 * time.Second),
	}}
	p := NewChainProvider(NewCRDProvider(StaticCRDSource{Object: crdObj}), fallback)

	timeout, err := p.ReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, timeout)

	hard, err := p.HardReconciliationTimeout(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 300*time.Second, hard)

	jitter, err := p.ReconciliationJitter(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 60*time.Second, jitter)
}

func TestChainProvider_ConfigurationAndSubscribeCRD(t *testing.T) {
	crdObj := &appv1.ArgoCDConfiguration{Spec: appv1.ArgoCDConfigurationSpec{InstallationID: "cfg"}}
	src := StaticCRDSource{Object: crdObj}
	chain := NewChainProvider(NewCRDProvider(src), &StaticProvider{})

	cfg, err := chain.Configuration(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "cfg", cfg.Spec.InstallationID)

	// no-op subscribe on StaticCRDSource (not a notifier) must not panic
	ch := make(chan struct{}, 1)
	chain.SubscribeCRD(ch)
	chain.UnsubscribeCRD(ch)
}
