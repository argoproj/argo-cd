package mocks

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"

	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	cacheutilmocks "github.com/argoproj/argo-cd/v2/util/cache/mocks"
)

type MockCacheType int

const (
	MockCacheTypeRedis MockCacheType = iota
	MockCacheTypeInMem
)

type MockRepoCache struct {
	mock.Mock
	RedisClient       *cacheutilmocks.MockCacheClient
	StopRedisCallback func()
}

type MockCacheOptions struct {
	RepoCacheExpiration     time.Duration
	RevisionCacheExpiration time.Duration
	ReadDelay               time.Duration
	WriteDelay              time.Duration
}

type CacheCallCounts struct {
	ExternalSets    int
	ExternalGets    int
	ExternalDeletes int
	ExternalRenames int
}

// Checks that the cache was called the expected number of times
func (mockCache *MockRepoCache) AssertCacheCalledTimes(t *testing.T, calls *CacheCallCounts) {
	mockCache.RedisClient.AssertNumberOfCalls(t, "Get", calls.ExternalGets)
	mockCache.RedisClient.AssertNumberOfCalls(t, "Set", calls.ExternalSets)
	mockCache.RedisClient.AssertNumberOfCalls(t, "Delete", calls.ExternalDeletes)
	mockCache.RedisClient.AssertNumberOfCalls(t, "Rename", calls.ExternalRenames)
}

func (mockCache *MockRepoCache) ConfigureDefaultCallbacks() {
	mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Return(nil)
	mockCache.RedisClient.On("Set", mock.Anything).Return(nil)
	mockCache.RedisClient.On("Delete", mock.Anything).Return(nil)
	mockCache.RedisClient.On("Rename", mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

func NewInMemoryRedis() (*redis.Client, func()) {
	cacheutil.NewInMemoryCache(5 * time.Second)
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr.Close
}

func NewMockRepoCache(cacheOpts *MockCacheOptions) *MockRepoCache {
	redisClient, stopRedis := NewInMemoryRedis()
	redisCacheClient := &cacheutilmocks.MockCacheClient{
		ReadDelay:  cacheOpts.ReadDelay,
		WriteDelay: cacheOpts.WriteDelay,
		BaseCache:  cacheutil.NewRedisCache(redisClient, cacheOpts.RepoCacheExpiration, cacheutil.RedisCompressionNone),
	}
	newMockCache := &MockRepoCache{RedisClient: redisCacheClient, StopRedisCallback: stopRedis}
	newMockCache.ConfigureDefaultCallbacks()
	return newMockCache
}
