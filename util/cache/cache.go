package cache

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"crypto/tls"
	"crypto/x509"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/common"
	certutil "github.com/argoproj/argo-cd/v2/util/cert"
	"github.com/argoproj/argo-cd/v2/util/env"
)

const (
	// envRedisPassword is an env variable name which stores redis password
	envRedisPassword = "REDIS_PASSWORD"
	// envRedisUsername is an env variable name which stores redis username (for acl setup)
	envRedisUsername = "REDIS_USERNAME"
	// envRedisRetryCount is an env variable name which stores redis retry count
	envRedisRetryCount = "REDIS_RETRY_COUNT"
	// defaultRedisRetryCount holds default number of retries
	defaultRedisRetryCount = 3
)

func NewCache(client CacheClient) *Cache {
	return &Cache{client}
}

func buildRedisClient(redisAddress, password, username string, redisDB, maxRetries int, tlsConfig *tls.Config) *redis.Client {
	opts := &redis.Options{
		Addr:       redisAddress,
		Password:   password,
		DB:         redisDB,
		MaxRetries: maxRetries,
		TLSConfig:  tlsConfig,
		Username:   username,
	}

	client := redis.NewClient(opts)

	client.AddHook(redis.Hook(NewArgoRedisHook(func() {
		*client = *buildRedisClient(redisAddress, password, username, redisDB, maxRetries, tlsConfig)
	})))

	return client
}

func buildFailoverRedisClient(sentinelMaster, password, username string, redisDB, maxRetries int, tlsConfig *tls.Config, sentinelAddresses []string) *redis.Client {
	opts := &redis.FailoverOptions{
		MasterName:    sentinelMaster,
		SentinelAddrs: sentinelAddresses,
		DB:            redisDB,
		Password:      password,
		MaxRetries:    maxRetries,
		TLSConfig:     tlsConfig,
		Username:      username,
	}

	client := redis.NewFailoverClient(opts)

	client.AddHook(redis.Hook(NewArgoRedisHook(func() {
		*client = *buildFailoverRedisClient(sentinelMaster, password, username, redisDB, maxRetries, tlsConfig, sentinelAddresses)
	})))

	return client
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0
	redisCACerticate := ""
	redisClientCertificate := ""
	redisClientKey := ""
	redisUseTLS := false
	insecureRedis := false
	var defaultCacheExpiration time.Duration

	cmd.Flags().StringVar(&redisAddress, "redis", env.StringFromEnv("REDIS_SERVER", ""), "Redis server hostname and port (e.g. argocd-redis:6379). ")
	cmd.Flags().IntVar(&redisDB, "redisdb", env.ParseNumFromEnv("REDISDB", 0, 0, math.MaxInt32), "Redis database.")
	cmd.Flags().StringArrayVar(&sentinelAddresses, "sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	cmd.Flags().StringVar(&sentinelMaster, "sentinelmaster", "master", "Redis sentinel master group name.")
	cmd.Flags().DurationVar(&defaultCacheExpiration, "default-cache-expiration", env.ParseDurationFromEnv("ARGOCD_DEFAULT_CACHE_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration default")
	cmd.Flags().BoolVar(&redisUseTLS, "redis-use-tls", false, "Use TLS when connecting to Redis. ")
	cmd.Flags().StringVar(&redisClientCertificate, "redis-client-certificate", "", "Path to Redis client certificate (e.g. /etc/certs/redis/client.crt).")
	cmd.Flags().StringVar(&redisClientKey, "redis-client-key", "", "Path to Redis client key (e.g. /etc/certs/redis/client.crt).")
	cmd.Flags().BoolVar(&insecureRedis, "redis-insecure-skip-tls-verify", false, "Skip Redis server certificate validation.")
	cmd.Flags().StringVar(&redisCACerticate, "redis-ca-certificate", "", "Path to Redis server CA certificate (e.g. /etc/certs/redis/ca.crt). If not specified, system trusted CAs will be used for server certificate validation.")
	return func() (*Cache, error) {
		var tlsConfig *tls.Config = nil
		if redisUseTLS {
			tlsConfig = &tls.Config{}
			if redisClientCertificate != "" {
				clientCert, err := tls.LoadX509KeyPair(redisClientCertificate, redisClientKey)
				if err != nil {
					return nil, err
				}
				tlsConfig.Certificates = []tls.Certificate{clientCert}
			}
			if insecureRedis {
				tlsConfig.InsecureSkipVerify = true
			} else if redisCACerticate != "" {
				redisCA, err := certutil.ParseTLSCertificatesFromPath(redisCACerticate)
				if err != nil {
					return nil, err
				}
				tlsConfig.RootCAs = certutil.GetCertPoolFromPEMData(redisCA)
			} else {
				var err error
				tlsConfig.RootCAs, err = x509.SystemCertPool()
				if err != nil {
					return nil, err
				}
			}
		}
		password := os.Getenv(envRedisPassword)
		username := os.Getenv(envRedisUsername)
		maxRetries := env.ParseNumFromEnv(envRedisRetryCount, defaultRedisRetryCount, 0, math.MaxInt32)
		if len(sentinelAddresses) > 0 {
			client := buildFailoverRedisClient(sentinelMaster, password, username, redisDB, maxRetries, tlsConfig, sentinelAddresses)
			for i := range opts {
				opts[i](client)
			}
			return NewCache(NewRedisCache(client, defaultCacheExpiration)), nil
		}
		if redisAddress == "" {
			redisAddress = common.DefaultRedisAddr
		}

		client := buildRedisClient(redisAddress, password, username, redisDB, maxRetries, tlsConfig)
		for i := range opts {
			opts[i](client)
		}
		return NewCache(NewRedisCache(client, defaultCacheExpiration)), nil
	}
}

// Cache provides strongly types methods to store and retrieve values from shared cache
type Cache struct {
	client CacheClient
}

func (c *Cache) GetClient() CacheClient {
	return c.client
}

func (c *Cache) SetClient(client CacheClient) {
	c.client = client
}

func (c *Cache) SetItem(key string, item interface{}, expiration time.Duration, delete bool) error {
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	if delete {
		return c.client.Delete(key)
	} else {
		if item == nil {
			return fmt.Errorf("cannot set item to nil for key %s", key)
		}
		return c.client.Set(&Item{Object: item, Key: key, Expiration: expiration})
	}
}

func (c *Cache) GetItem(key string, item interface{}) error {
	if item == nil {
		return fmt.Errorf("cannot get item into a nil for key %s", key)
	}
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	return c.client.Get(key, item)
}

func (c *Cache) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.client.OnUpdated(ctx, fmt.Sprintf("%s|%s", key, common.CacheVersion), callback)
}

func (c *Cache) NotifyUpdated(key string) error {
	return c.client.NotifyUpdated(fmt.Sprintf("%s|%s", key, common.CacheVersion))
}
