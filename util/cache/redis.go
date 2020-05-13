package cache

import (
	"time"

	rediscache "github.com/go-redis/cache"
	"github.com/go-redis/redis"
	"github.com/vmihailenco/msgpack"
)

func NewRedisCache(client *redis.Client, expiration time.Duration) CacheClient {
	return &redisCache{
		expiration: expiration,
		codec: &rediscache.Codec{
			Redis: client,
			Marshal: func(v interface{}) ([]byte, error) {
				return msgpack.Marshal(v)
			},
			Unmarshal: func(b []byte, v interface{}) error {
				return msgpack.Unmarshal(b, v)
			},
		},
	}
}

type redisCache struct {
	expiration time.Duration
	codec      *rediscache.Codec
}

func (r *redisCache) Set(item *Item) error {
	expiration := item.Expiration
	if expiration == 0 {
		expiration = r.expiration
	}
	return r.codec.Set(&rediscache.Item{
		Key:        item.Key,
		Object:     item.Object,
		Expiration: expiration,
	})
}

func (r *redisCache) Get(key string, obj interface{}) error {
	err := r.codec.Get(key, obj)
	if err == rediscache.ErrCacheMiss {
		return ErrCacheMiss
	}
	return err
}

func (r *redisCache) Delete(key string) error {
	return r.codec.Delete(key)
}

type MetricsRegistry interface {
	IncRedisRequest(failed bool)
	ObserveRedisRequestDuration(duration time.Duration)
}

// CollectMetrics add transport wrapper that pushes metrics into the specified metrics registry
func CollectMetrics(client *redis.Client, registry MetricsRegistry) {
	client.WrapProcess(func(oldProcess func(cmd redis.Cmder) error) func(cmd redis.Cmder) error {
		return func(cmd redis.Cmder) error {
			startTime := time.Now()
			err := oldProcess(cmd)
			registry.IncRedisRequest(err != nil && err != redis.Nil)
			duration := time.Since(startTime)
			registry.ObserveRedisRequestDuration(duration)
			return err
		}
	})
}
