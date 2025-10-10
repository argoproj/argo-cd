package cache

import (
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

	creds := loadRedisCreds(dir, Options{})
	assert.Equal(t, "mypassword", creds.password)
	assert.Equal(t, "myuser", creds.username)
	assert.Equal(t, "sentineluser", creds.sentinelUsername)
	assert.Equal(t, "sentinelpass", creds.sentinelPassword)
}

// Test loading Redis credentials from environment variables
func TestLoadRedisCredsFromEnv(t *testing.T) {
	// Set environment variables
	t.Setenv(envRedisPassword, "mypassword")
	t.Setenv(envRedisUsername, "myuser")
	t.Setenv(envRedisSentinelUsername, "sentineluser")
	t.Setenv(envRedisSentinelPassword, "sentinelpass")

	creds := loadRedisCreds("", Options{})
	assert.Equal(t, "mypassword", creds.password)
	assert.Equal(t, "myuser", creds.username)
	assert.Equal(t, "sentineluser", creds.sentinelUsername)
	assert.Equal(t, "sentinelpass", creds.sentinelPassword)
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
	writeFile("auth", "filepassword\n")
	writeFile("auth_username", "fileuser")
	writeFile("sentinel_username", "filesentineluser")
	writeFile("sentinel_auth", "filesentinelpass")

	creds := loadRedisCreds(dir, Options{})
	assert.Equal(t, "filepassword", creds.password)
	assert.Equal(t, "fileuser", creds.username)
	assert.Equal(t, "filesentineluser", creds.sentinelUsername)
	assert.Equal(t, "filesentinelpass", creds.sentinelPassword)
}
