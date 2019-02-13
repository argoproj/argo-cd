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
