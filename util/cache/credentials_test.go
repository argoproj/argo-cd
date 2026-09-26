package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProviderFactory is a test double for RedisCredentialsProviderFactory
// that lets each test instantiate it under a fresh registry name and
// observe how AddFlags / Build are invoked.
type stubProviderFactory struct {
	name             string
	addFlagsCalls    int
	lastFlagPrefix   string
	lastEnvPrefix    string
	buildCalls       int
	lastLoadedUser   string
	buildErr         error
	providerUsername string
	providerPassword string
}

func (s *stubProviderFactory) Name() string { return s.name }

func (s *stubProviderFactory) AddFlags(_ *cobra.Command, flagPrefix, envPrefix string) {
	s.addFlagsCalls++
	s.lastFlagPrefix = flagPrefix
	s.lastEnvPrefix = envPrefix
}

func (s *stubProviderFactory) Build(loadedUsername string) (RedisCredentialsProvider, error) {
	s.buildCalls++
	s.lastLoadedUser = loadedUsername
	if s.buildErr != nil {
		return nil, s.buildErr
	}
	user := s.providerUsername
	pass := s.providerPassword
	return func(_ context.Context) (string, string, error) {
		return user, pass, nil
	}, nil
}

// withCleanRegistry swaps the global registry for a test-local one and
// restores the original on cleanup. Required because RegisterRedisCredentialsProvider
// panics on duplicate names; tests must not leak factories into each other.
func withCleanRegistry(t *testing.T) {
	t.Helper()
	redisCredentialsProvidersMu.Lock()
	saved := redisCredentialsProviders
	redisCredentialsProviders = map[string]RedisCredentialsProviderFactory{}
	redisCredentialsProvidersMu.Unlock()
	t.Cleanup(func() {
		redisCredentialsProvidersMu.Lock()
		redisCredentialsProviders = saved
		redisCredentialsProvidersMu.Unlock()
	})
}

func TestRegisterRedisCredentialsProvider(t *testing.T) {
	withCleanRegistry(t)

	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "alpha"})
	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "beta"})

	assert.Equal(t, []string{"alpha", "beta"}, registeredRedisCredentialsProviderNames())
}

func TestRegisterRedisCredentialsProvider_DuplicatePanics(t *testing.T) {
	withCleanRegistry(t)

	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "dup"})
	assert.PanicsWithValue(t,
		`redis credentials provider "dup" already registered`,
		func() {
			RegisterRedisCredentialsProvider(&stubProviderFactory{name: "dup"})
		})
}

func TestLookupRedisCredentialsProvider(t *testing.T) {
	withCleanRegistry(t)

	want := &stubProviderFactory{name: "azure"}
	RegisterRedisCredentialsProvider(want)

	got, err := lookupRedisCredentialsProvider("azure")
	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestLookupRedisCredentialsProvider_Unknown(t *testing.T) {
	withCleanRegistry(t)

	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "azure"})

	_, err := lookupRedisCredentialsProvider("gcp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown redis credentials provider "gcp"`)
	assert.Contains(t, err.Error(), "azure", "error must list registered names so users can correct typos")
}

func TestAddRedisCredentialsProviderFlags_DelegatesToAllFactories(t *testing.T) {
	withCleanRegistry(t)

	a := &stubProviderFactory{name: "alpha"}
	b := &stubProviderFactory{name: "beta"}
	RegisterRedisCredentialsProvider(a)
	RegisterRedisCredentialsProvider(b)

	cmd := &cobra.Command{}
	addRedisCredentialsProviderFlags(cmd, "repo-", "REPO_")

	assert.Equal(t, 1, a.addFlagsCalls)
	assert.Equal(t, 1, b.addFlagsCalls)
	assert.Equal(t, "repo-", a.lastFlagPrefix)
	assert.Equal(t, "REPO_", b.lastEnvPrefix)
}

func TestStubFactory_BuildPropagatesError(t *testing.T) {
	withCleanRegistry(t)

	wantErr := errors.New("boom")
	f := &stubProviderFactory{name: "x", buildErr: wantErr}

	_, err := f.Build("ignored")
	require.ErrorIs(t, err, wantErr)
	assert.Equal(t, "ignored", f.lastLoadedUser)
}
