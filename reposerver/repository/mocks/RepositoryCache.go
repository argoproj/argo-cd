package mocks

import (
	"context"
	"time"

	"github.com/alicebob/miniredis/v2"
	reposerverCache "github.com/argoproj/argo-cd/v2/reposerver/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/redis/go-redis/v9"
)

type MockCacheType int

const (
	MockCacheTypeRedis MockCacheType = iota
	MockCacheTypeInMem
)

type MockRepoCache struct {
	cache     *cacheutil.Cache
	baseCache *reposerverCache.Cache
	reposerverCache.CacheOpts
	CallCounts     map[string]int
	ErrorResponses map[string]error
}

type MockCacheOptions struct {
	reposerverCache.CacheOpts
	ReadDelay  time.Duration
	WriteDelay time.Duration
	// Map of function name keys to error values that should be returned by the function
	ErrorResponses map[string]error
}

func NewInMemoryRedis() (*redis.Client, func()) {
	cacheutil.NewInMemoryCache(5 * time.Second)
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return redis.NewClient(&redis.Options{Addr: mr.Addr()}), mr.Close
}

func NewMockRepoCache(cacheType MockCacheType, cacheOpts *MockCacheOptions) *MockRepoCache {
	redisClient, _ := NewInMemoryRedis()
	var cacheClient cacheutil.CacheClient
	switch cacheType {
	case MockCacheTypeRedis:
		cacheClient = cacheutil.NewRedisCache(redisClient, 5*time.Second, cacheutil.RedisCompressionNone)
	default:
		cacheClient = cacheutil.NewInMemoryCache(5 * time.Second)
	}
	utilCache := cacheutil.NewCache(&FakeCache{baseCache: cacheClient, ReadDelay: cacheOpts.ReadDelay, WriteDelay: cacheOpts.WriteDelay})
	if cacheOpts.ErrorResponses == nil {
		cacheOpts.ErrorResponses = make(map[string]error)
	}
	baseCacheOptions := reposerverCache.CacheOpts{
		RepoCacheExpiration:           cacheOpts.RepoCacheExpiration,
		RevisionCacheExpiration:       cacheOpts.RevisionCacheExpiration,
		RevisionCacheLockWaitEnabled:  cacheOpts.RevisionCacheLockWaitEnabled,
		RevisionCacheLockTimeout:      cacheOpts.RevisionCacheLockTimeout,
		RevisionCacheLockWaitInterval: cacheOpts.RevisionCacheLockWaitInterval,
	}
	return &MockRepoCache{
		cache:          utilCache,
		baseCache:      reposerverCache.NewCache(utilCache, &baseCacheOptions),
		CacheOpts:      baseCacheOptions,
		CallCounts:     make(map[string]int),
		ErrorResponses: cacheOpts.ErrorResponses,
	}
}

func (c *MockRepoCache) GetGitCache() *cacheutil.Cache {
	return c.cache
}

func (c *MockRepoCache) SetGitReferences(repo string, references []*plumbing.Reference) error {
	if err := c.ErrorResponses["SetGitReferences"]; err != nil {
		return err
	}
	c.CallCounts["SetGitReferences"]++
	return c.baseCache.SetGitReferences(repo, references)
}

func (c *MockRepoCache) GetGitReferences(repo string, references *[]*plumbing.Reference) error {
	if err := c.ErrorResponses["GetGitReferences"]; err != nil {
		return err
	}
	c.CallCounts["GetGitReferences"]++
	return c.baseCache.GetGitReferences(repo, references)
}

func (c *MockRepoCache) UnlockGitReferences(repo string, lockId string) error {
	if err := c.ErrorResponses["UnlockGitReferences"]; err != nil {
		return err
	}
	c.CallCounts["UnlockGitReferences"]++
	return c.baseCache.UnlockGitReferences(repo, lockId)
}

func (c *MockRepoCache) GetOrLockGitReferences(repo string, references *[]*plumbing.Reference) (updateCache bool, lockId string, err error) {
	if err := c.ErrorResponses["GetOrLockGitReferences"]; err != nil {
		return false, "", err
	}
	c.CallCounts["GetOrLockGitReferences"]++
	return c.baseCache.GetOrLockGitReferences(repo, references)
}

type FakeCache struct {
	baseCache  cacheutil.CacheClient
	ReadDelay  time.Duration
	WriteDelay time.Duration
}

func (c *FakeCache) Set(item *cacheutil.Item) error {
	if c.WriteDelay > 0 {
		time.Sleep(c.WriteDelay)
	}
	return c.baseCache.Set(item)
}

func (c *FakeCache) Get(key string, obj interface{}) error {
	if c.ReadDelay > 0 {
		time.Sleep(c.ReadDelay)
	}
	return c.baseCache.Get(key, obj)
}

func (c *FakeCache) Delete(key string) error {
	if c.WriteDelay > 0 {
		time.Sleep(c.WriteDelay)
	}
	return c.baseCache.Delete(key)
}

func (c *FakeCache) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.baseCache.OnUpdated(ctx, key, callback)
}

func (c *FakeCache) NotifyUpdated(key string) error {
	return c.baseCache.NotifyUpdated(key)
}
