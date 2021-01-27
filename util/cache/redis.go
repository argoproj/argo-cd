package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ioutil "github.com/argoproj/argo-cd/util/io"

	rediscache "github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
)

func NewRedisCache(client *redis.Client, expiration time.Duration) CacheClient {
	return &redisCache{
		client:     client,
		expiration: expiration,
		cache:      rediscache.New(&rediscache.Options{Redis: client}),
		hashByKey:  map[string]prevSetInfo{},
	}
}

type prevSetInfo struct {
	hash  string
	setAt time.Time
}

type redisCache struct {
	expiration    time.Duration
	client        *redis.Client
	cache         *rediscache.Cache
	hashByKey     map[string]prevSetInfo
	hashByKeyLock sync.Mutex
}

func (r *redisCache) isNewKeyValue(key string, data []byte) bool {
	r.hashByKeyLock.Lock()
	defer r.hashByKeyLock.Unlock()
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if prevSet, ok := r.hashByKey[key]; !ok || hash != prevSet.hash || prevSet.setAt.Add(r.expiration).Before(time.Now()) {
		r.hashByKey[key] = prevSetInfo{setAt: time.Now(), hash: hash}
		return true
	}
	return false
}

func (r *redisCache) Set(item *Item) error {
	expiration := item.Expiration
	if expiration == 0 {
		expiration = r.expiration
	}

	val, err := json.Marshal(item.Object)
	if err != nil {
		return err
	}

	if r.isNewKeyValue(item.Key, val) {
		return r.cache.Set(&rediscache.Item{
			Key:   item.Key,
			Value: val,
			TTL:   expiration,
		})
	}
	return nil
}

func (r *redisCache) Get(key string, obj interface{}) error {
	var data []byte
	err := r.cache.Get(context.TODO(), key, &data)
	if err == rediscache.ErrCacheMiss {
		err = ErrCacheMiss
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func (r *redisCache) Delete(key string) error {
	return r.cache.Delete(context.TODO(), key)
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

var metricStartTimeKey = struct{}{}

type redisHook struct {
	registry MetricsRegistry
}

func (rh *redisHook) BeforeProcess(ctx context.Context, cmd redis.Cmder) (context.Context, error) {
	return context.WithValue(ctx, metricStartTimeKey, time.Now()), nil
}

func (rh *redisHook) AfterProcess(ctx context.Context, cmd redis.Cmder) error {
	cmdErr := cmd.Err()
	rh.registry.IncRedisRequest(cmdErr != nil && cmdErr != redis.Nil)

	startTime := ctx.Value(metricStartTimeKey).(time.Time)
	duration := time.Since(startTime)
	rh.registry.ObserveRedisRequestDuration(duration)

	return nil
}

func (redisHook) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (redisHook) AfterProcessPipeline(_ context.Context, _ []redis.Cmder) error {
	return nil
}

// CollectMetrics add transport wrapper that pushes metrics into the specified metrics registry
func CollectMetrics(client *redis.Client, registry MetricsRegistry) {
	client.AddHook(&redisHook{registry: registry})
}
