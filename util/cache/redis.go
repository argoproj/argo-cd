package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	ioutil "github.com/argoproj/argo-cd/v2/util/io"

	rediscache "github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
)

type RedisCompressionType string

var (
	RedisCompressionNone RedisCompressionType = "none"
	RedisCompressionGZip RedisCompressionType = "gzip"
)

func CompressionTypeFromString(s string) (RedisCompressionType, error) {
	switch s {
	case string(RedisCompressionNone):
		return RedisCompressionNone, nil
	case string(RedisCompressionGZip):
		return RedisCompressionGZip, nil
	}
	return "", fmt.Errorf("unknown compression type: %s", s)
}

func NewRedisCache(client *redis.Client, expiration time.Duration, compressionType RedisCompressionType) CacheClient {
	return &redisCache{
		client:               client,
		expiration:           expiration,
		cache:                rediscache.New(&rediscache.Options{Redis: client}),
		redisCompressionType: compressionType,
	}
}

// compile-time validation of adherance of the CacheClient contract
var _ CacheClient = &redisCache{}

type redisCache struct {
	expiration           time.Duration
	client               *redis.Client
	cache                *rediscache.Cache
	redisCompressionType RedisCompressionType
}

func (r *redisCache) getKey(key string) string {
	switch r.redisCompressionType {
	case RedisCompressionGZip:
		return key + ".gz"
	default:
		return key
	}
}

func (r *redisCache) marshal(obj interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	var w io.Writer = buf
	if r.redisCompressionType == RedisCompressionGZip {
		w = gzip.NewWriter(buf)
	}
	encoder := json.NewEncoder(w)

	if err := encoder.Encode(obj); err != nil {
		return nil, err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func (r *redisCache) unmarshal(data []byte, obj interface{}) error {
	buf := bytes.NewReader(data)
	var reader io.Reader = buf
	if r.redisCompressionType == RedisCompressionGZip {
		if gzipReader, err := gzip.NewReader(buf); err != nil {
			return err
		} else {
			reader = gzipReader
		}
	}
	if err := json.NewDecoder(reader).Decode(obj); err != nil {
		return fmt.Errorf("failed to decode cached data: %w", err)
	}
	return nil
}

func (r *redisCache) Rename(oldKey string, newKey string, _ time.Duration) error {
	return r.client.Rename(context.TODO(), r.getKey(oldKey), r.getKey(newKey)).Err()
}

func (r *redisCache) Set(item *Item) error {
	expiration := item.Expiration
	if expiration == 0 {
		expiration = r.expiration
	}

	val, err := r.marshal(item.Object)
	if err != nil {
		return err
	}

	return r.cache.Set(&rediscache.Item{
		Key:   r.getKey(item.Key),
		Value: val,
		TTL:   expiration,
	})
}

func (r *redisCache) Get(key string, obj interface{}) error {
	var data []byte
	err := r.cache.Get(context.TODO(), r.getKey(key), &data)
	if err == rediscache.ErrCacheMiss {
		err = ErrCacheMiss
	}
	if err != nil {
		return err
	}
	return r.unmarshal(data, obj)
}

func (r *redisCache) Delete(key string) error {
	return r.cache.Delete(context.TODO(), r.getKey(key))
}

func (r *redisCache) OnUpdated(ctx context.Context, key string, callback func() error) error {
	pubsub := r.client.Subscribe(ctx, key)
	defer ioutil.Close(pubsub)

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ch:
			if err := callback(); err != nil {
				return err
			}
		}
	}
}

func (r *redisCache) NotifyUpdated(key string) error {
	return r.client.Publish(context.TODO(), key, "").Err()
}

type MetricsRegistry interface {
	IncRedisRequest(failed bool)
	ObserveRedisRequestDuration(duration time.Duration)
}

type redisHook struct {
	registry MetricsRegistry
}

func (rh *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := next(ctx, network, addr)
		return conn, err
	}
}

func (rh *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		startTime := time.Now()

		err := next(ctx, cmd)
		rh.registry.IncRedisRequest(err != nil && err != redis.Nil)
		rh.registry.ObserveRedisRequestDuration(time.Since(startTime))

		return err
	}
}

func (redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return nil
}

// CollectMetrics add transport wrapper that pushes metrics into the specified metrics registry
func CollectMetrics(client *redis.Client, registry MetricsRegistry) {
	client.AddHook(&redisHook{registry: registry})
}
