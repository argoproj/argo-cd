package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/cache/mocks"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
)

type MockedCache struct {
	mock.Mock
	*Cache
}

type fixtures struct {
	mockCache *mocks.MockRepoCache
	cache     *MockedCache
}

func newFixtures() *fixtures {
	mockCache := mocks.NewMockRepoCache(&mocks.MockCacheOptions{RevisionCacheExpiration: 1 * time.Minute, RepoCacheExpiration: 1 * time.Minute})
	newBaseCache := cacheutil.NewCache(mockCache.RedisClient)
	baseCache := NewCache(newBaseCache, 1*time.Minute, 1*time.Minute, 10*time.Second)
	return &fixtures{mockCache: mockCache, cache: &MockedCache{Cache: baseCache}}
}

func TestCache_GetRevisionMetadata(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	_, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	mockCache.RedisClient.AssertCalled(t, "Get", mock.Anything, mock.Anything)
	// populate cache
	err = cache.SetRevisionMetadata("my-repo-url", "my-revision", &RevisionMetadata{Message: "my-message"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.GetRevisionMetadata("other-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	_, err = cache.GetRevisionMetadata("my-repo-url", "other-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	require.NoError(t, err)
	assert.Equal(t, &RevisionMetadata{Message: "my-message"}, value)
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 4})
}

func TestCache_ListApps(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	_, err := cache.ListApps("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetApps("my-repo-url", "my-revision", map[string]string{"foo": "bar"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.ListApps("other-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	_, err = cache.ListApps("my-repo-url", "other-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.ListApps("my-repo-url", "my-revision")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, value)
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 4})
}

func TestCache_GetManifests(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	q := &apiclient.ManifestRequest{}
	value := &CachedManifestResponse{}
	err := cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	res := &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}
	err = cache.SetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", res, nil)
	require.NoError(t, err)
	t.Run("expect cache miss because of changed revision", func(t *testing.T) {
		err = cache.GetManifests("other-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache miss because of changed path", func(t *testing.T) {
		err = cache.GetManifests("my-revision", &ApplicationSource{Path: "other-path"}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache miss because of changed namespace", func(t *testing.T) {
		err = cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "other-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache miss because of changed app label key", func(t *testing.T) {
		err = cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "other-app-label-key", "my-app-label-value", value, nil)
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache miss because of changed app label value", func(t *testing.T) {
		err = cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "other-app-label-value", value, nil)
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache miss because of changed referenced source", func(t *testing.T) {
		err = cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "other-app-label-value", value, map[string]string{"my-referenced-source": "my-referenced-revision"})
		assert.Equal(t, ErrCacheMiss, err)
	})
	t.Run("expect cache hit", func(t *testing.T) {
		err = cache.SetManifests(
			"my-revision1", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value",
			&CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type", Revision: "my-revision2"}}, nil)
		require.NoError(t, err)

		err = cache.GetManifests("my-revision1", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
		require.NoError(t, err)

		assert.Equal(t, "my-source-type", value.ManifestResponse.SourceType)
		assert.Equal(t, "my-revision1", value.ManifestResponse.Revision)
	})
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 8})
}

func TestCache_GetAppDetails(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	value := &apiclient.RepoAppDetailsResponse{}
	emptyRefSources := map[string]*RefTarget{}
	err := cache.GetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	res := &apiclient.RepoAppDetailsResponse{Type: "my-type"}
	err = cache.SetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, res, "", nil)
	require.NoError(t, err)
	// cache miss
	err = cache.GetAppDetails("other-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	err = cache.GetAppDetails("my-revision", &ApplicationSource{Path: "other-path"}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	require.NoError(t, err)
	assert.Equal(t, &apiclient.RepoAppDetailsResponse{Type: "my-type"}, value)
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 4})
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.repoCacheExpiration)
}

func TestCachedManifestResponse_HashBehavior(t *testing.T) {
	inMemCache := cacheutil.NewInMemoryCache(1 * time.Hour)

	repoCache := NewCache(
		cacheutil.NewCache(inMemCache),
		1*time.Minute,
		1*time.Minute,
		10*time.Second,
	)

	response := apiclient.ManifestResponse{
		Namespace: "default",
		Revision:  "revision",
		Manifests: []string{"sample-text"},
	}
	appSrc := &ApplicationSource{}
	appKey := "key"
	appValue := "value"

	// Set the value in the cache
	store := &CachedManifestResponse{
		FirstFailureTimestamp:           0,
		ManifestResponse:                &response,
		MostRecentError:                 "",
		NumberOfCachedResponsesReturned: 0,
		NumberOfConsecutiveFailures:     0,
	}
	q := &apiclient.ManifestRequest{}
	err := repoCache.SetManifests(response.Revision, appSrc, q.RefSources, q, response.Namespace, "", appKey, appValue, store, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Get the cache entry of the set value directly from the in memory cache, and check the values
	var cacheKey string
	var cmr *CachedManifestResponse
	{
		items := getInMemoryCacheContents(t, inMemCache)

		assert.Len(t, items, 1)

		for key, val := range items {
			cmr = val
			cacheKey = key
		}
		assert.NotEmpty(t, cmr.CacheEntryHash)
		assert.NotNil(t, cmr.ManifestResponse)
		assert.Equal(t, cmr.ManifestResponse, store.ManifestResponse)

		regeneratedHash, err := cmr.generateCacheEntryHash()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, cmr.CacheEntryHash, regeneratedHash)
	}

	// Retrieve the value using 'GetManifests' and confirm it works
	retrievedVal := &CachedManifestResponse{}
	err = repoCache.GetManifests(response.Revision, appSrc, q.RefSources, q, response.Namespace, "", appKey, appValue, retrievedVal, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, retrievedVal, store)

	// Corrupt the hash so that it doesn't match
	{
		newCmr := cmr.shallowCopy()
		newCmr.CacheEntryHash = "!bad-hash!"

		err := inMemCache.Set(&cacheutil.Item{
			Key:    cacheKey,
			Object: &newCmr,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Retrieve the value using GetManifests and confirm it returns a cache miss
	retrievedVal = &CachedManifestResponse{}
	err = repoCache.GetManifests(response.Revision, appSrc, q.RefSources, q, response.Namespace, "", appKey, appValue, retrievedVal, nil)

	assert.Equal(t, err, cacheutil.ErrCacheMiss)

	// Verify that the hash mismatch item has been deleted
	items := getInMemoryCacheContents(t, inMemCache)
	assert.Empty(t, items)
}

func getInMemoryCacheContents(t *testing.T, inMemCache *cacheutil.InMemoryCache) map[string]*CachedManifestResponse {
	items, err := inMemCache.Items(func() interface{} { return &CachedManifestResponse{} })
	if err != nil {
		t.Fatal(err)
	}

	result := map[string]*CachedManifestResponse{}
	for key, val := range items {
		obj, ok := val.(*CachedManifestResponse)
		if !ok {
			t.Fatal(errors.New("Unexpected type in cache"))
		}

		result[key] = obj
	}

	return result
}

func TestCachedManifestResponse_ShallowCopy(t *testing.T) {
	pre := &CachedManifestResponse{
		CacheEntryHash:        "value",
		FirstFailureTimestamp: 1,
		ManifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{"one", "two"},
		},
		MostRecentError:                 "error",
		NumberOfCachedResponsesReturned: 2,
		NumberOfConsecutiveFailures:     3,
	}

	post := pre.shallowCopy()
	assert.Equal(t, pre, post)

	unequal := &CachedManifestResponse{
		CacheEntryHash:        "diff-value",
		FirstFailureTimestamp: 1,
		ManifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{"one", "two"},
		},
		MostRecentError:                 "error",
		NumberOfCachedResponsesReturned: 2,
		NumberOfConsecutiveFailures:     3,
	}
	assert.NotEqual(t, pre, unequal)
}

func TestCachedManifestResponse_ShallowCopyExpectedFields(t *testing.T) {
	// Attempt to ensure that the developer updated CachedManifestResponse.shallowCopy(), by doing a sanity test of the structure here

	val := &CachedManifestResponse{}

	str, err := json.Marshal(val)
	if err != nil {
		assert.FailNow(t, "Unable to marshal", err)
		return
	}

	jsonMap := map[string]interface{}{}
	err = json.Unmarshal(str, &jsonMap)
	if err != nil {
		assert.FailNow(t, "Unable to unmarshal", err)
		return
	}

	expectedFields := []string{
		"cacheEntryHash", "manifestResponse", "mostRecentError", "firstFailureTimestamp",
		"numberOfConsecutiveFailures", "numberOfCachedResponsesReturned",
	}

	assert.Equal(t, len(jsonMap), len(expectedFields))

	// If this test failed, you probably also forgot to update CachedManifestResponse.shallowCopy(), so
	// go do that first :)

	for _, expectedField := range expectedFields {
		assert.Containsf(t, string(str), "\""+expectedField+"\"", "Missing field: %s", expectedField)
	}
}

func TestGetGitReferences(t *testing.T) {
	t.Run("Valid args, nothing in cache, in-memory only", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err, "Error is cache miss handled inside function")
		assert.Equal(t, "", lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})

	t.Run("Valid args, nothing in cache, external only", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err, "Error is cache miss handled inside function")
		assert.Equal(t, "", lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})

	t.Run("Valid args, value in cache, in-memory only", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
		require.NoError(t, err)
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err)
		assert.Equal(t, "", lockOwner, "Lock owner should be empty")
		assert.Len(t, references, 1)
		assert.Equal(t, "test", (references)[0].Target().String())
		assert.Equal(t, "test-repo", (references)[0].Name().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("cache error", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Unset()
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Return(errors.New("test cache error"))
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.ErrorContains(t, err, "test cache error", "Error should be propagated")
		assert.Equal(t, "", lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
}

func TestGitRefCacheItemToReferences_DataChecks(t *testing.T) {
	references := *GitRefCacheItemToReferences(nil)
	assert.Empty(t, references, "No data should be handled gracefully by returning an empty slice")
	references = *GitRefCacheItemToReferences([][2]string{{"", ""}})
	assert.Empty(t, references, "Empty data should be discarded")
	references = *GitRefCacheItemToReferences([][2]string{{"test", ""}})
	assert.Len(t, references, 1, "Just the key being set should not be discarded")
	assert.Equal(t, "test", references[0].Name().String(), "Name should be set and equal test")
	references = *GitRefCacheItemToReferences([][2]string{{"", "ref: test1"}})
	assert.Len(t, references, 1, "Just the value being set should not be discarded")
	assert.Equal(t, "test1", references[0].Target().String(), "Target should be set and equal test1")
	references = *GitRefCacheItemToReferences([][2]string{{"test2", "ref: test2"}})
	assert.Len(t, references, 1, "Valid data is should be preserved")
	assert.Equal(t, "test2", references[0].Name().String(), "Name should be set and equal test2")
	assert.Equal(t, "test2", references[0].Target().String(), "Target should be set and equal test2")
	references = *GitRefCacheItemToReferences([][2]string{{"test3", "ref: test3"}, {"test4", "ref: test4"}})
	assert.Len(t, references, 2, "Valid data is should be preserved")
	assert.Equal(t, "test3", references[0].Name().String(), "Name should be set and equal test3")
	assert.Equal(t, "test3", references[0].Target().String(), "Target should be set and equal test3")
	assert.Equal(t, "test4", references[1].Name().String(), "Name should be set and equal test4")
	assert.Equal(t, "test4", references[1].Target().String(), "Target should be set and equal test4")
}

func TestTryLockGitRefCache_OwnershipFlows(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	utilCache := cache.cache
	var references []*plumbing.Reference
	// Test setting the lock
	_, err := cache.TryLockGitRefCache("my-repo-url", "my-lock-id", &references)
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	require.NoError(t, err)
	var output [][2]string
	key := fmt.Sprintf("git-refs|%s", "my-repo-url")
	err = utilCache.GetItem(key, &output)
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 2})
	require.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should be set")
	assert.Equal(t, "my-lock-id", output[0][1], "The lock should be set to the provided lock id")
	// Test not being able to overwrite the lock
	_, err = cache.TryLockGitRefCache("my-repo-url", "other-lock-id", &references)
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 3})
	require.NoError(t, err)
	err = utilCache.GetItem(key, &output)
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 4})
	require.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should not have changed")
	assert.Equal(t, "my-lock-id", output[0][1], "The lock should not have changed")
	// Test can overwrite once there is nothing set
	err = utilCache.SetItem(key, [][2]string{}, &cacheutil.CacheActionOpts{Expiration: 0, Delete: true})
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 4, ExternalDeletes: 1})
	require.NoError(t, err)
	_, err = cache.TryLockGitRefCache("my-repo-url", "other-lock-id", &references)
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 3, ExternalGets: 5, ExternalDeletes: 1})
	require.NoError(t, err)
	err = utilCache.GetItem(key, &output)
	require.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should be set")
	assert.Equal(t, "other-lock-id", output[0][1], "The lock id should have changed to other-lock-id")
	fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 3, ExternalGets: 6, ExternalDeletes: 1})
}

func TestGetOrLockGitReferences(t *testing.T) {
	t.Run("Test cache lock get lock", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEqual(t, "", lockId, "Lock id should be set")
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 2})
	})

	t.Run("Test cache lock, cache hit local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
		require.NoError(t, err)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.NotEqual(t, "test-lock-id", lockId)
		assert.Equal(t, "", lockId, "Lock id should not be set")
		assert.Equal(t, "test-repo", references[0].Name().String())
		assert.Equal(t, "test", references[0].Target().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("Test cache lock, cache hit remote", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := fixtures.cache.cache.SetItem(
			"git-refs|test-repo",
			[][2]string{{"test-repo", "ref: test"}},
			&cacheutil.CacheActionOpts{
				Expiration: 30 * time.Second,
			})
		require.NoError(t, err)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.NotEqual(t, "test-lock-id", lockId)
		assert.Equal(t, "", lockId, "Lock id should not be set")
		assert.Equal(t, "test-repo", references[0].Name().String())
		assert.Equal(t, "test", references[0].Target().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("Test miss, populated by external", func(t *testing.T) {
		// Tests the case where another process populates the external cache when trying
		// to obtain the lock
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Unset()
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Return(cacheutil.ErrCacheMiss).Once().Run(func(args mock.Arguments) {
			err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
			require.NoError(t, err)
		}).On("Get", mock.Anything, mock.Anything).Return(nil)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.NotEqual(t, "test-lock-id", lockId)
		assert.Equal(t, "", lockId, "Lock id should not be set")
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 2})
	})

	t.Run("Test cache lock timeout", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		// Create conditions for cache hit, which would result in false on updateCache if we weren't reaching the timeout
		err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
		require.NoError(t, err)
		cache.revisionCacheLockTimeout = -1 * time.Second
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEqual(t, "", lockId, "Lock id should be set")
		cache.revisionCacheLockTimeout = 10 * time.Second
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1})
	})

	t.Run("Test cache lock error", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		fixtures.cache.revisionCacheLockTimeout = 10 * time.Second
		fixtures.mockCache.RedisClient.On("Set", mock.Anything).Unset()
		fixtures.mockCache.RedisClient.On("Set", mock.Anything).Return(errors.New("test cache error")).Once().
			On("Set", mock.Anything).Return(nil)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEqual(t, "", lockId, "Lock id should be set")
		fixtures.mockCache.RedisClient.AssertNumberOfCalls(t, "Set", 2)
		fixtures.mockCache.RedisClient.AssertNumberOfCalls(t, "Get", 4)
	})
}

func TestUnlockGitReferences(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache

	t.Run("Test not locked", func(t *testing.T) {
		err := cache.UnlockGitReferences("test-repo", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key is missing")
	})

	t.Run("Test unlock", func(t *testing.T) {
		// Get lock
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEqual(t, "", lockId, "Lock id should be set")
		// Release lock
		err = cache.UnlockGitReferences("test-repo", lockId)
		require.NoError(t, err)
	})
}

func TestSetHelmIndex(t *testing.T) {
	t.Run("SetHelmIndex with valid data", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		err := fixtures.cache.SetHelmIndex("test-repo", []byte("test-data"))
		require.NoError(t, err)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1})
	})
	t.Run("SetHelmIndex with nil", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		err := fixtures.cache.SetHelmIndex("test-repo", nil)
		require.Error(t, err, "nil data should not be cached")
		var indexData []byte
		err = fixtures.cache.GetHelmIndex("test-repo", &indexData)
		require.Error(t, err)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
}

func TestRevisionChartDetails(t *testing.T) {
	t.Run("GetRevisionChartDetails cache miss", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		details, err := fixtures.cache.GetRevisionChartDetails("test-repo", "test-revision", "v1.0.0")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Equal(t, &appv1.ChartDetails{}, details)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetRevisionChartDetails cache miss local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := &appv1.ChartDetails{
			Description: "test-chart",
			Home:        "v1.0.0",
			Maintainers: []string{"test-maintainer"},
		}
		err := cache.cache.SetItem(
			revisionChartDetailsKey("test-repo", "test-revision", "v1.0.0"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		details, err := fixtures.cache.GetRevisionChartDetails("test-repo", "test-revision", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, details)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("GetRevisionChartDetails cache hit local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := &appv1.ChartDetails{
			Description: "test-chart",
			Home:        "v1.0.0",
			Maintainers: []string{"test-maintainer"},
		}
		err := cache.cache.SetItem(
			revisionChartDetailsKey("test-repo", "test-revision", "v1.0.0"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		details, err := fixtures.cache.GetRevisionChartDetails("test-repo", "test-revision", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, details)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetRevisionChartDetails", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := &appv1.ChartDetails{
			Description: "test-chart",
			Home:        "v1.0.0",
			Maintainers: []string{"test-maintainer"},
		}
		err := fixtures.cache.SetRevisionChartDetails("test-repo", "test-revision", "v1.0.0", expectedItem)
		require.NoError(t, err)
		details, err := fixtures.cache.GetRevisionChartDetails("test-repo", "test-revision", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, details)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestGetGitDirectories(t *testing.T) {
	t.Run("GetGitDirectories cache miss", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetGitDirectories("test-repo", "test-revision")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetGitDirectories cache miss local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := []string{"test/dir", "test/dir2"}
		err := cache.cache.SetItem(
			gitDirectoriesKey("test-repo", "test-revision"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		directories, err := fixtures.cache.GetGitDirectories("test-repo", "test-revision")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("GetGitDirectories cache hit local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := []string{"test/dir", "test/dir2"}
		err := cache.cache.SetItem(
			gitDirectoriesKey("test-repo", "test-revision"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		directories, err := fixtures.cache.GetGitDirectories("test-repo", "test-revision")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetGitDirectories", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := []string{"test/dir", "test/dir2"}
		err := fixtures.cache.SetGitDirectories("test-repo", "test-revision", expectedItem)
		require.NoError(t, err)
		directories, err := fixtures.cache.GetGitDirectories("test-repo", "test-revision")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestGetGitFiles(t *testing.T) {
	t.Run("GetGitFiles cache miss", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetGitFiles("test-repo", "test-revision", "*.json")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetGitFiles cache hit", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := map[string][]byte{"test/file.json": []byte("\"test\":\"contents\""), "test/file1.json": []byte("\"test1\":\"contents1\"")}
		err := cache.cache.SetItem(
			gitFilesKey("test-repo", "test-revision", "*.json"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		files, err := fixtures.cache.GetGitFiles("test-repo", "test-revision", "*.json")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetGitFiles", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := map[string][]byte{"test/file.json": []byte("\"test\":\"contents\""), "test/file1.json": []byte("\"test1\":\"contents1\"")}
		err := fixtures.cache.SetGitFiles("test-repo", "test-revision", "*.json", expectedItem)
		require.NoError(t, err)
		files, err := fixtures.cache.GetGitFiles("test-repo", "test-revision", "*.json")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}
