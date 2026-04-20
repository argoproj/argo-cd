package cache

// This file is the home for black-box tests against the ClusterCache
// interface — tests that do not reach into the unexported state of the
// concrete implementation. White-box tests that poke at internal maps,
// locks, or private helpers live in cluster_test.go and construct the
// concrete legacy impl via a type assertion.
//
// When the informer implementation lands (see issue #19199), this file
// is where the mode matrix (`forEachMode`) will live, so any test added
// here must work against both impls.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/kubetest"
)

// newInterfaceTestCache constructs the cluster cache and returns it through
// the public interface. Use this helper for tests that should run
// unchanged against every ClusterCache implementation.
func newInterfaceTestCache(opts ...UpdateSettingsFunc) ClusterCache {
	opts = append([]UpdateSettingsFunc{
		SetKubectl(&kubetest.MockKubectlCmd{}),
	}, opts...)
	return NewClusterCache(&rest.Config{Host: "https://test"}, opts...)
}

func TestNewClusterCache_DefaultsToLegacyMode(t *testing.T) {
	// Construction without SetMode must not panic. ModeInformer panics
	// until the informer impl lands, so reaching this line proves the
	// default is ModeLegacy.
	cache := newInterfaceTestCache()
	require.NotNil(t, cache)
	assert.Equal(t, "https://test", cache.GetClusterInfo().Server)
}

func TestNewClusterCache_InformerModeConstructs(t *testing.T) {
	// Construction under ModeInformer must succeed. EnsureSynced will route
	// through syncInformers() when called; that's covered by the informer
	// lifecycle tests in informer_test.go.
	cache := newInterfaceTestCache(SetMode(ModeInformer))
	require.NotNil(t, cache)
}