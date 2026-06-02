package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.client.(*redisCache).expiration)
}

func NewInMemoryRedis() (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr.Close
}

func TestCacheClient(t *testing.T) {
	clientRedis, stopRedis := NewInMemoryRedis()
	defer stopRedis()
	redisCache := NewRedisCache(clientRedis, 5*time.Second, RedisCompressionNone)
	clientMemCache := NewInMemoryCache(60 * time.Second)
	twoLevelClient := NewTwoLevelClient(redisCache, 5*time.Second)
	// Run tests for both Redis and InMemoryCache
	for _, client := range []CacheClient{clientMemCache, redisCache, twoLevelClient} {
		cache := NewCache(client)
		t.Run("SetItem", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			var output string
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
		})
		t.Run("SetCacheItem W/Disable Overwrite", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			var output string
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output)
			err = cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 60 * time.Second, DisableOverwrite: true, Delete: false})
			require.NoError(t, err)
			err = cache.GetItem("foo", &output)
			require.NoError(t, err)
			assert.Equal(t, "bar", output, "output should not have changed with DisableOverwrite set to true")
		})
		t.Run("GetItem", func(t *testing.T) {
			var val string
			err := cache.GetItem("foo", &val)
			require.NoError(t, err)
			assert.Equal(t, "bar", val)
		})
		t.Run("DeleteItem", func(t *testing.T) {
			err := cache.SetItem("foo", "bar", &CacheActionOpts{Expiration: 0, Delete: true})
			require.NoError(t, err)
			var val string
			err = cache.GetItem("foo", &val)
			require.Error(t, err)
			assert.Empty(t, val)
		})
		t.Run("Check for nil items", func(t *testing.T) {
			err := cache.SetItem("foo", nil, &CacheActionOpts{Expiration: 0, Delete: true})
			require.ErrorContains(t, err, "cannot set nil item")
			err = cache.GetItem("foo", nil)
			assert.ErrorContains(t, err, "cannot get item")
		})
	}
}

// Smoke test to ensure key changes aren't done accidentally
func TestGenerateCacheKey(t *testing.T) {
	client := NewInMemoryCache(60 * time.Second)
	cache := NewCache(client)
	testKey := cache.generateFullKey("testkey")
	assert.Equal(t, "testkey|"+common.CacheVersion, testKey)
}

// Test loading Redis credentials from a file
func TestLoadRedisCreds(t *testing.T) {
	dir := t.TempDir()
	// Helper to write a file
	writeFile := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o400))
	}
	// Write all files
	writeFile("auth", "mypassword\n")
	writeFile("auth_username", "myuser")
	writeFile("sentinel_username", "sentineluser")
	writeFile("sentinel_auth", "sentinelpass")

	username, password, sentinelUsername, sentinelPassword, err := loadRedisCreds(dir, Options{})
	require.NoError(t, err)
	assert.Equal(t, "mypassword", password)
	assert.Equal(t, "myuser", username)
	assert.Equal(t, "sentineluser", sentinelUsername)
	assert.Equal(t, "sentinelpass", sentinelPassword)
}

// Test loading Redis credentials from environment variables
func TestLoadRedisCredsFromEnv(t *testing.T) {
	// Set environment variables
	t.Setenv(envRedisPassword, "mypassword")
	t.Setenv(envRedisUsername, "myuser")
	t.Setenv(envRedisSentinelUsername, "sentineluser")
	t.Setenv(envRedisSentinelPassword, "sentinelpass")

	username, password, sentinelUsername, sentinelPassword, err := loadRedisCreds("", Options{})
	require.NoError(t, err)
	assert.Equal(t, "mypassword", password)
	assert.Equal(t, "myuser", username)
	assert.Equal(t, "sentineluser", sentinelUsername)
	assert.Equal(t, "sentinelpass", sentinelPassword)
}

// Test loading Redis credentials from both environment variables and a file
func TestLoadRedisCredsFromBothEnvAndFile(t *testing.T) {
	// Set environment variables
	t.Setenv(envRedisPassword, "mypassword")
	t.Setenv(envRedisUsername, "myuser")
	t.Setenv(envRedisSentinelUsername, "sentineluser")
	t.Setenv(envRedisSentinelPassword, "sentinelpass")

	dir := t.TempDir()
	// Helper to write a file
	writeFile := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o400))
	}
	// Write all files
	writeFile("auth", "filepassword")
	writeFile("auth_username", "fileuser")
	writeFile("sentinel_username", "filesentineluser")
	writeFile("sentinel_auth", "filesentinelpass")

	username, password, sentinelUsername, sentinelPassword, err := loadRedisCreds(dir, Options{})
	require.NoError(t, err)
	assert.Equal(t, "filepassword", password)
	assert.Equal(t, "fileuser", username)
	assert.Equal(t, "filesentineluser", sentinelUsername)
	assert.Equal(t, "filesentinelpass", sentinelPassword)
}

func TestLoadRedisCreds_MountPathMissing(t *testing.T) {
	_, _, _, _, err := loadRedisCreds("not_existing_path", Options{})
	require.Error(t, err)
	require.ErrorContains(t, err, "failed to access Redis credentials")
}

func TestCredentialFileHandling(t *testing.T) {
	t.Run("ReadAuthDetailsFromFile_Missing", func(t *testing.T) {
		dir := t.TempDir()
		val, err := readAuthDetailsFromFile(dir, "not_existing_path")
		require.NoError(t, err)
		assert.Empty(t, val)
	})

	t.Run("ReadAuthDetailsFromFile_Unreadable", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "auth")
		require.NoError(t, os.WriteFile(file, []byte("value"), 0o000))
		_, err := readAuthDetailsFromFile(dir, "auth")
		require.Error(t, err)
	})

	t.Run("ReadAuthDetailsFromFile_Normal", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "auth")
		require.NoError(t, os.WriteFile(file, []byte("value"), 0o400))
		val, err := readAuthDetailsFromFile(dir, "auth")
		require.NoError(t, err)
		assert.Equal(t, "value", val)
	})
}

// TestAddCacheFlagsToCmd_RegistersProviderFlags verifies that every registered
// factory's AddFlags callback is invoked, so backend-specific flags exist on
// the command regardless of which provider (if any) is selected at runtime.
func TestAddCacheFlagsToCmd_RegistersProviderFlags(t *testing.T) {
	withCleanRegistry(t)
	stub := &stubProviderFactory{name: "stubcloud"}
	RegisterRedisCredentialsProvider(stub)

	cmd := &cobra.Command{}
	_ = AddCacheFlagsToCmd(cmd)

	assert.Equal(t, 1, stub.addFlagsCalls,
		"AddCacheFlagsToCmd must walk the credentials-provider registry on flag setup")
	require.NotNil(t, cmd.Flags().Lookup("redis-credentials-provider"),
		"the generic selector flag must be registered")
}

// TestAddCacheFlagsToCmd_NoProviderSelected covers the default code path:
// when --redis-credentials-provider is empty, the registry is not consulted
// and the cache is built without a credentials closure.
func TestAddCacheFlagsToCmd_NoProviderSelected(t *testing.T) {
	withCleanRegistry(t)
	stub := &stubProviderFactory{name: "stubcloud"}
	RegisterRedisCredentialsProvider(stub)

	cmd := &cobra.Command{}
	cacheFn := AddCacheFlagsToCmd(cmd)
	cache, err := cacheFn()
	require.NoError(t, err)
	assert.Equal(t, 0, stub.buildCalls,
		"Build must not run when --redis-credentials-provider is unset")
	assert.NotNil(t, cache)
}

// TestAddCacheFlagsToCmd_SelectsRegisteredProvider verifies the happy path:
// when --redis-credentials-provider matches a registered factory, the
// factory's Build is invoked exactly once and is handed the loaded username.
func TestAddCacheFlagsToCmd_SelectsRegisteredProvider(t *testing.T) {
	withCleanRegistry(t)
	stub := &stubProviderFactory{
		name:             "stubcloud",
		providerUsername: "from-provider",
		providerPassword: "from-provider-tok",
	}
	RegisterRedisCredentialsProvider(stub)

	t.Setenv(envRedisUsername, "loaded-user")
	t.Setenv(envRedisPassword, "static-password")

	cmd := &cobra.Command{}
	cacheFn := AddCacheFlagsToCmd(cmd)
	require.NoError(t, cmd.Flags().Set("redis-credentials-provider", "stubcloud"))

	cache, err := cacheFn()
	require.NoError(t, err)
	require.NotNil(t, cache)

	assert.Equal(t, 1, stub.buildCalls)
	assert.Equal(t, "loaded-user", stub.lastLoadedUser,
		"factory must receive the username already resolved from env/file")
}

// TestAddCacheFlagsToCmd_UnknownProvider asserts that selecting an unregistered
// provider name fails fast with a message that lists the registered names —
// vital for typo diagnosis on a single-line config.
func TestAddCacheFlagsToCmd_UnknownProvider(t *testing.T) {
	withCleanRegistry(t)
	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "stubcloud"})

	cmd := &cobra.Command{}
	cacheFn := AddCacheFlagsToCmd(cmd)
	require.NoError(t, cmd.Flags().Set("redis-credentials-provider", "typo"))

	_, err := cacheFn()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown redis credentials provider "typo"`)
	assert.Contains(t, err.Error(), "stubcloud",
		"error must enumerate registered providers so users can correct the typo")
}

// TestAddCacheFlagsToCmd_ProviderBuildErrorPropagates ensures factory failures
// surface to the caller rather than being swallowed.
func TestAddCacheFlagsToCmd_ProviderBuildErrorPropagates(t *testing.T) {
	withCleanRegistry(t)
	wantErr := errors.New("federated token unavailable")
	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "stubcloud", buildErr: wantErr})

	cmd := &cobra.Command{}
	cacheFn := AddCacheFlagsToCmd(cmd)
	require.NoError(t, cmd.Flags().Set("redis-credentials-provider", "stubcloud"))

	_, err := cacheFn()
	require.ErrorIs(t, err, wantErr)
}

// TestAddCacheFlagsToCmd_ProviderRejectsSentinel asserts that selecting a
// credentials provider while sentinel addresses are configured fails with a
// clear, actionable message — sentinel mode is incompatible with the
// cloud-managed Redis services these providers target.
func TestAddCacheFlagsToCmd_ProviderRejectsSentinel(t *testing.T) {
	withCleanRegistry(t)
	RegisterRedisCredentialsProvider(&stubProviderFactory{name: "stubcloud"})

	cmd := &cobra.Command{}
	cacheFn := AddCacheFlagsToCmd(cmd)
	require.NoError(t, cmd.Flags().Set("redis-credentials-provider", "stubcloud"))
	require.NoError(t, cmd.Flags().Set("sentinel", "sentinel-1:26379"))

	_, err := cacheFn()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible with sentinel-mode")
	assert.Contains(t, err.Error(), "stubcloud")
}

// TestBuildRedisClient_SetsCredentialsProvider verifies that a non-nil
// provider is propagated into the underlying go-redis Options. We can't
// inspect the struct field directly (it's a closure), but we can compare
// pointers to the same closure to prove the wiring.
func TestBuildRedisClient_SetsCredentialsProvider(t *testing.T) {
	called := 0
	provider := func(_ context.Context) (string, string, error) {
		called++
		return "u", "p", nil
	}

	client := buildRedisClient("redis:6379", "static", "static-user", 0, 0, nil, provider)
	require.NotNil(t, client)
	require.NotNil(t, client.Options().CredentialsProviderContext,
		"provider closure must be wired into go-redis Options")

	_, _, err := client.Options().CredentialsProviderContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, called, "go-redis must invoke our provider, not bypass it")
}

// TestBuildRedisClient_NilProviderLeavesOptionsAlone makes sure the default
// (no provider) code path doesn't accidentally install a nil
// CredentialsProviderContext, which would crash go-redis on every connect.
func TestBuildRedisClient_NilProviderLeavesOptionsAlone(t *testing.T) {
	client := buildRedisClient("redis:6379", "static", "static-user", 0, 0, nil, nil)
	require.NotNil(t, client)
	assert.Nil(t, client.Options().CredentialsProviderContext,
		"no provider configured -> CredentialsProviderContext must remain nil")
}

// TestBuildFailoverRedisClient_SetsCredentialsProvider mirrors
// TestBuildRedisClient_SetsCredentialsProvider for the sentinel client. We
// keep the wiring even though the public flow rejects sentinel + provider —
// future providers (e.g. self-hosted Redis Enterprise behind sentinel with
// ACLs) may want it.
func TestBuildFailoverRedisClient_SetsCredentialsProvider(t *testing.T) {
	provider := func(_ context.Context) (string, string, error) {
		return "u", "p", nil
	}
	client := buildFailoverRedisClient("master", "", "", "static", "user", 0, 0, nil,
		[]string{"s-0:26379"}, provider)
	require.NotNil(t, client)
	require.NotNil(t, client.Options().CredentialsProviderContext,
		"provider closure must be wired into go-redis FailoverOptions too")
}
