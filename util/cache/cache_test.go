package cache

import (
	"context"
	"crypto/tls"
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

// Test the 4 possible Redis client types
func TestBuildRedisClient(t *testing.T) {
	tests := []struct {
		name              string
		description       string
		redisClusterMode  bool
		sentinelAddresses []string
		sentinelMaster    string
		sentinelUsername  string
		sentinelPassword  string
		redisAddress      string
		username          string
		password          string
		redisDB           int
		maxRetries        int
		tlsConfig         *tls.Config
	}{
		{
			name:              "FailoverClusterClient",
			description:       "Redis sentinels in cluster mode",
			redisClusterMode:  true,
			sentinelAddresses: []string{"invalidsentinel1.invalid:12345", "invalidsentinel2.invalid:12345"},
			sentinelMaster:    "master",
			sentinelUsername:  "sentinel-user",
			sentinelPassword:  "sentinel-pass",
			redisAddress:      "", // ignored when using sentinels
			username:          "redis-user",
			password:          "redis-pass",
			redisDB:           0, // must be 0 in cluster mode
			maxRetries:        3,
			tlsConfig:         nil,
		},
		{
			name:              "FailoverClient",
			description:       "Redis sentinels not in cluster mode",
			redisClusterMode:  false,
			sentinelAddresses: []string{"invalidsentinel1.invalid:12345", "invalidsentinel2.invalid:12345"},
			sentinelMaster:    "master",
			sentinelUsername:  "sentinel-user",
			sentinelPassword:  "sentinel-pass",
			redisAddress:      "", // ignored when using sentinels
			username:          "redis-user",
			password:          "redis-pass",
			redisDB:           1,
			maxRetries:        3,
			tlsConfig:         nil,
		},
		{
			name:              "ClusterClient",
			description:       "Redis client in cluster mode",
			redisClusterMode:  true,
			sentinelAddresses: []string{},
			sentinelMaster:    "",
			sentinelUsername:  "",
			sentinelPassword:  "",
			redisAddress:      "invalidredis1.invalid:12345,invalidredis2.invalid:12345,invalidredis3.invalid:12345",
			username:          "redis-user",
			password:          "redis-pass",
			redisDB:           0, // must be 0 in cluster mode
			maxRetries:        2,
			tlsConfig:         &tls.Config{InsecureSkipVerify: true},
		},
		{
			name:              "SingleClient",
			description:       "Redis client in single server mode",
			redisClusterMode:  false,
			sentinelAddresses: []string{},
			sentinelMaster:    "",
			sentinelUsername:  "",
			sentinelPassword:  "",
			redisAddress:      "invalidredishost.invalid:12345",
			username:          "redis-user",
			password:          "redis-pass",
			redisDB:           15,
			maxRetries:        2,
			tlsConfig:         &tls.Config{ServerName: "invalidredishost.invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := buildRedisClient(
				tt.redisClusterMode,
				tt.sentinelAddresses,
				tt.sentinelMaster,
				tt.sentinelUsername,
				tt.sentinelPassword,
				tt.redisAddress,
				tt.username,
				tt.password,
				tt.redisDB,
				tt.maxRetries,
				tt.tlsConfig,
			)
			assert.NotNil(t, client, "Client should not be nil for scenario: %s", tt.description)

			if tt.redisClusterMode {
				_, isClusterClient := client.(*redis.ClusterClient)
				assert.True(t, isClusterClient, "Should be *redis.ClusterClient for cluster scenario: %s", tt.description)
			} else {
				_, isNonClusterClient := client.(*redis.Client)
				assert.True(t, isNonClusterClient, "Should be *redis.Client for non-cluster scenario: %s", tt.description)
			}

			cmd := client.Ping(context.Background())
			assert.NotNil(t, cmd, "Should be able to create Ping command for scenario: %s", tt.description)
		})
	}
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
