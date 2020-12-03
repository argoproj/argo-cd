package cache

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/env"
)

const (
	// envRedisPassword is a env variable name which stores redis password
	envRedisPassword = "REDIS_PASSWORD"
	// envRedisRetryCount is a env variable name which stores redis retry count
	envRedisRetryCount = "REDIS_RETRY_COUNT"
	// defaultRedisRetryCount holds default number of retries
	defaultRedisRetryCount = 3
)

func NewCache(client CacheClient) *Cache {
	return &Cache{client}
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0
	var defaultCacheExpiration time.Duration

	cmd.Flags().StringVar(&redisAddress, "redis", "", "Redis server hostname and port (e.g. argocd-redis:6379). ")
	cmd.Flags().IntVar(&redisDB, "redisdb", 0, "Redis database.")
	cmd.Flags().StringArrayVar(&sentinelAddresses, "sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	cmd.Flags().StringVar(&sentinelMaster, "sentinelmaster", "master", "Redis sentinel master group name.")
	cmd.Flags().DurationVar(&defaultCacheExpiration, "default-cache-expiration", 24*time.Hour, "Cache expiration default")
	return func() (*Cache, error) {
		password := os.Getenv(envRedisPassword)
		maxRetries := env.ParseNumFromEnv(envRedisRetryCount, defaultRedisRetryCount, 0, math.MaxInt32)
		if len(sentinelAddresses) > 0 {
			client := redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:    sentinelMaster,
				SentinelAddrs: sentinelAddresses,
				DB:            redisDB,
				Password:      password,
				MaxRetries:    maxRetries,
			})
			for i := range opts {
				opts[i](client)
			}
			return NewCache(NewRedisCache(client, defaultCacheExpiration)), nil
		}

		if redisAddress == "" {
			redisAddress = common.DefaultRedisAddr
		}
		client := redis.NewClient(&redis.Options{
			Addr:       redisAddress,
			Password:   password,
			DB:         redisDB,
			MaxRetries: maxRetries,
		})
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
