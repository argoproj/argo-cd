package cache

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/cobra"
)

// RedisCredentialsProvider matches the signature go-redis expects for
// dynamic per-connection credentials (redis.Options.CredentialsProviderContext).
// It is invoked by go-redis on every new connection (including reconnects),
// which makes it the natural extension point for short-lived, automatically
// rotated credentials such as cloud-provider-issued OAuth tokens.
type RedisCredentialsProvider = func(ctx context.Context) (username, password string, err error)

// RedisCredentialsProviderFactory builds a RedisCredentialsProvider for a
// specific authentication backend (Microsoft Entra ID, GCP Memorystore IAM,
// AWS ElastiCache IAM, …).
//
// Implementations register themselves via package-level init() calls to
// RegisterRedisCredentialsProvider. The active backend is selected at runtime
// by --redis-credentials-provider; the factory whose Name() matches that flag
// is asked to Build the provider closure that will be wired into the redis
// client. Backends that aren't selected are inert: their AddFlags-registered
// flags are still parseable for documentation purposes but have no effect.
type RedisCredentialsProviderFactory interface {
	// Name returns the identifier used to select this factory via the
	// --redis-credentials-provider flag (e.g. "azure", "gcp", "aws").
	Name() string

	// AddFlags lets the factory register its own backend-specific flags on
	// the supplied cobra command. Flags should be prefixed with `flagPrefix`
	// (the FlagPrefix from Options, usually empty) and namespaced under the
	// provider name to avoid collisions across backends — e.g.
	// "redis-azure-client-id" rather than "redis-client-id".
	//
	// envPrefix is the matching upper-snake-cased prefix applied to env vars
	// that back the flag defaults (e.g. "REPOSERVER_" producing
	// "REPOSERVER_REDIS_AZURE_CLIENT_ID").
	AddFlags(cmd *cobra.Command, flagPrefix, envPrefix string)

	// Build is invoked when the user has selected this factory by name. It
	// returns the closure that will be assigned to
	// redis.Options.CredentialsProviderContext.
	//
	// loadedUsername is the Redis username already resolved from
	// --redis-username, REDIS_USERNAME or the credentials-mount file. The
	// factory is free to honour, override, or ignore it; for AAD-style flows
	// the username is typically derived from the issued token instead.
	Build(loadedUsername string) (RedisCredentialsProvider, error)
}

var (
	redisCredentialsProvidersMu sync.RWMutex
	redisCredentialsProviders   = map[string]RedisCredentialsProviderFactory{}
)

// RegisterRedisCredentialsProvider registers a factory in the package-level
// registry. Intended to be called from init() functions of provider
// implementations. Panics on duplicate registration so that double-registers
// surface as a fail-fast process start rather than as silently shadowed
// behaviour at runtime.
func RegisterRedisCredentialsProvider(f RedisCredentialsProviderFactory) {
	redisCredentialsProvidersMu.Lock()
	defer redisCredentialsProvidersMu.Unlock()
	if _, exists := redisCredentialsProviders[f.Name()]; exists {
		panic(fmt.Sprintf("redis credentials provider %q already registered", f.Name()))
	}
	redisCredentialsProviders[f.Name()] = f
}

// lookupRedisCredentialsProvider returns the registered factory for `name`,
// or an error listing the registered names if no match exists.
func lookupRedisCredentialsProvider(name string) (RedisCredentialsProviderFactory, error) {
	redisCredentialsProvidersMu.RLock()
	defer redisCredentialsProvidersMu.RUnlock()
	f, ok := redisCredentialsProviders[name]
	if !ok {
		return nil, fmt.Errorf("unknown redis credentials provider %q (registered: %v)", name, registeredRedisCredentialsProviderNamesLocked())
	}
	return f, nil
}

// registeredRedisCredentialsProviderNames returns the sorted list of
// registered factory names. Useful for --help and error messages.
func registeredRedisCredentialsProviderNames() []string {
	redisCredentialsProvidersMu.RLock()
	defer redisCredentialsProvidersMu.RUnlock()
	return registeredRedisCredentialsProviderNamesLocked()
}

// registeredRedisCredentialsProviderNamesLocked is the lock-free body of
// registeredRedisCredentialsProviderNames; the caller must hold the registry
// read or write lock.
func registeredRedisCredentialsProviderNamesLocked() []string {
	names := make([]string, 0, len(redisCredentialsProviders))
	for n := range redisCredentialsProviders {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// addRedisCredentialsProviderFlags invokes AddFlags on every registered
// factory in a stable, sorted order so that --help output is deterministic.
func addRedisCredentialsProviderFlags(cmd *cobra.Command, flagPrefix, envPrefix string) {
	redisCredentialsProvidersMu.RLock()
	names := registeredRedisCredentialsProviderNamesLocked()
	factories := make([]RedisCredentialsProviderFactory, 0, len(names))
	for _, n := range names {
		factories = append(factories, redisCredentialsProviders[n])
	}
	redisCredentialsProvidersMu.RUnlock()
	for _, f := range factories {
		f.AddFlags(cmd, flagPrefix, envPrefix)
	}
}
