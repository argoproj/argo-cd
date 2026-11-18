package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/cache/mocks"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/helm"
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
	t.Parallel()
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	_, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
	mockCache.RedisClient.AssertCalled(t, "Get", mock.Anything, mock.Anything)
	// populate cache
	err = cache.SetRevisionMetadata("my-repo-url", "my-revision", &v1alpha1.RevisionMetadata{Message: "my-message"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.GetRevisionMetadata("other-repo-url", "my-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
	// cache miss
	_, err = cache.GetRevisionMetadata("my-repo-url", "other-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
	// cache hit
	value, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	require.NoError(t, err)
	assert.Equal(t, &v1alpha1.RevisionMetadata{Message: "my-message"}, value)
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 4})
}

func TestCache_ListApps(t *testing.T) {
	t.Parallel()
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	_, err := cache.ListApps("my-repo-url", "my-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
	// populate cache
	err = cache.SetApps("my-repo-url", "my-revision", map[string]string{"foo": "bar"})
	require.NoError(t, err)
	// cache miss
	_, err = cache.ListApps("other-repo-url", "my-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
	// cache miss
	_, err = cache.ListApps("my-repo-url", "other-revision")
	require.ErrorIs(t, err, ErrCacheMiss)
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
	newManifestCacheKeyData := func(revision string, appSource *v1alpha1.ApplicationSource, namespace, appLabelKey, appName string, refSourceCommitSHAs ResolvedRevisions) manifestKey {
		return manifestKey{
			Revision:            revision,
			AppSource:           appSource,
			RefSources:          q.RefSources,
			ClusterInfo:         q,
			Namespace:           namespace,
			AppLabelKey:         appLabelKey,
			AppName:             appName,
			RefSourceCommitSHAs: refSourceCommitSHAs,
		}
	}
	err := cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", nil), value)
	require.ErrorIs(t, err, ErrCacheMiss)
	// populate cache
	res := &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}
	err = cache.SetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", nil), res)
	require.NoError(t, err)
	t.Run("expect cache miss because of changed revision", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("other-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", nil), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache miss because of changed path", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{Path: "other-path"}, "my-namespace", "my-app-label-key", "my-app-label-value", nil), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache miss because of changed namespace", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "other-namespace", "my-app-label-key", "my-app-label-value", nil), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache miss because of changed app label key", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "other-app-label-key", "my-app-label-value", nil), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache miss because of changed app label value", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "other-app-label-value", nil), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache miss because of changed referenced source", func(t *testing.T) {
		err = cache.GetManifests(newManifestCacheKeyData("my-revision", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "other-app-label-value", map[string]string{"my-referenced-source": "my-referenced-revision"}), value)
		require.ErrorIs(t, err, ErrCacheMiss)
	})
	t.Run("expect cache hit", func(t *testing.T) {
		err = cache.SetManifests(
			newManifestCacheKeyData("my-revision1", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", nil),
			&CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type", Revision: "my-revision2"}})
		require.NoError(t, err)

		err = cache.GetManifests(newManifestCacheKeyData("my-revision1", &v1alpha1.ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", nil), value)
		require.NoError(t, err)

		assert.Equal(t, "my-source-type", value.ManifestResponse.SourceType)
		assert.Equal(t, "my-revision1", value.ManifestResponse.Revision)
	})
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 8})
}

func TestCache_GetAppDetails(t *testing.T) {
	t.Parallel()
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	cache := fixtures.cache
	mockCache := fixtures.mockCache
	// cache miss
	value := &apiclient.RepoAppDetailsResponse{}
	emptyRefSources := map[string]*v1alpha1.RefTarget{}
	err := cache.GetAppDetails("my-revision", &v1alpha1.ApplicationSource{}, emptyRefSources, value, "", nil)
	require.ErrorIs(t, err, ErrCacheMiss)
	res := &apiclient.RepoAppDetailsResponse{Type: "my-type"}
	err = cache.SetAppDetails("my-revision", &v1alpha1.ApplicationSource{}, emptyRefSources, res, "", nil)
	require.NoError(t, err)
	// cache miss
	err = cache.GetAppDetails("other-revision", &v1alpha1.ApplicationSource{}, emptyRefSources, value, "", nil)
	require.ErrorIs(t, err, ErrCacheMiss)
	// cache miss
	err = cache.GetAppDetails("my-revision", &v1alpha1.ApplicationSource{Path: "other-path"}, emptyRefSources, value, "", nil)
	require.ErrorIs(t, err, ErrCacheMiss)
	// cache hit
	err = cache.GetAppDetails("my-revision", &v1alpha1.ApplicationSource{}, emptyRefSources, value, "", nil)
	require.NoError(t, err)
	assert.Equal(t, &apiclient.RepoAppDetailsResponse{Type: "my-type"}, value)
	mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 4})
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	t.Parallel()
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.repoCacheExpiration)
}

func TestCachedManifestResponse_HashBehavior(t *testing.T) {
	t.Parallel()
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
	appSrc := &v1alpha1.ApplicationSource{}
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
	cacheKeyData := manifestKey{
		Revision:    response.Revision,
		AppSource:   appSrc,
		RefSources:  q.RefSources,
		ClusterInfo: q,
		Namespace:   response.Namespace,
		AppLabelKey: appKey,
		AppName:     appValue,
	}
	err := repoCache.SetManifests(cacheKeyData, store)
	require.NoError(t, err)

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
		require.NoError(t, err)
		assert.Equal(t, cmr.CacheEntryHash, regeneratedHash)
	}

	// Retrieve the value using 'GetManifests' and confirm it works
	retrievedVal := &CachedManifestResponse{}
	err = repoCache.GetManifests(cacheKeyData, retrievedVal)
	require.NoError(t, err)
	assert.Equal(t, retrievedVal, store)

	// Corrupt the hash so that it doesn't match
	{
		newCmr := cmr.shallowCopy()
		newCmr.CacheEntryHash = "!bad-hash!"

		err := inMemCache.Set(&cacheutil.Item{
			Key:    cacheKey,
			Object: &newCmr,
		})
		require.NoError(t, err)
	}

	// Retrieve the value using GetManifests and confirm it returns a cache miss
	retrievedVal = &CachedManifestResponse{}
	err = repoCache.GetManifests(cacheKeyData, retrievedVal)

	assert.Equal(t, err, cacheutil.ErrCacheMiss)

	// Verify that the hash mismatch item has been deleted
	items := getInMemoryCacheContents(t, inMemCache)
	assert.Empty(t, items)
}

func getInMemoryCacheContents(t *testing.T, inMemCache *cacheutil.InMemoryCache) map[string]*CachedManifestResponse {
	t.Helper()
	items, err := inMemCache.Items(func() any { return &CachedManifestResponse{} })
	require.NoError(t, err)

	result := map[string]*CachedManifestResponse{}
	for key, val := range items {
		obj, ok := val.(*CachedManifestResponse)
		require.True(t, ok, "Unexpected type in cache")

		result[key] = obj
	}

	return result
}

func TestCachedManifestResponse_ShallowCopy(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	// Attempt to ensure that the developer updated CachedManifestResponse.shallowCopy(), by doing a sanity test of the structure here

	val := &CachedManifestResponse{}

	str, err := json.Marshal(val)
	if err != nil {
		assert.FailNow(t, "Unable to marshal", err)
		return
	}

	jsonMap := map[string]any{}
	err = json.Unmarshal(str, &jsonMap)
	if err != nil {
		assert.FailNow(t, "Unable to unmarshal", err)
		return
	}

	expectedFields := []string{
		"cacheEntryHash", "manifestResponse", "mostRecentError", "firstFailureTimestamp",
		"numberOfConsecutiveFailures", "numberOfCachedResponsesReturned",
	}

	assert.Len(t, jsonMap, len(expectedFields))

	// If this test failed, you probably also forgot to update CachedManifestResponse.shallowCopy(), so
	// go do that first :)

	for _, expectedField := range expectedFields {
		assert.Containsf(t, string(str), "\""+expectedField+"\"", "Missing field: %s", expectedField)
	}
}

func TestGetGitReferences(t *testing.T) {
	t.Parallel()
	t.Run("Valid args, nothing in cache, in-memory only", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err, "Error is cache miss handled inside function")
		assert.Empty(t, lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})

	t.Run("Valid args, nothing in cache, external only", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err, "Error is cache miss handled inside function")
		assert.Empty(t, lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})

	t.Run("Valid args, value in cache, in-memory only", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
		require.NoError(t, err)
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.NoError(t, err)
		assert.Empty(t, lockOwner, "Lock owner should be empty")
		assert.Len(t, references, 1)
		assert.Equal(t, "test", references[0].Target().String())
		assert.Equal(t, "test-repo", references[0].Name().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("cache error", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Unset()
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Return(errors.New("test cache error"))
		var references []*plumbing.Reference
		lockOwner, err := cache.GetGitReferences("test-repo", &references)
		require.ErrorContains(t, err, "test cache error", "Error should be propagated")
		assert.Empty(t, lockOwner, "Lock owner should be empty")
		assert.Nil(t, references)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
}

func TestGitRefCacheItemToReferences_DataChecks(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	key := "git-refs|" + "my-repo-url"
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
	t.Parallel()
	t.Run("Test cache lock get lock", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEmpty(t, lockId, "Lock id should be set")
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 2})
	})

	t.Run("Test cache lock, cache hit local", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
		require.NoError(t, err)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.NotEqual(t, "test-lock-id", lockId)
		assert.Empty(t, lockId, "Lock id should not be set")
		assert.Equal(t, "test-repo", references[0].Name().String())
		assert.Equal(t, "test", references[0].Target().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("Test cache lock, cache hit remote", func(t *testing.T) {
		t.Parallel()
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
		assert.Empty(t, lockId, "Lock id should not be set")
		assert.Equal(t, "test-repo", references[0].Name().String())
		assert.Equal(t, "test", references[0].Target().String())
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1, ExternalGets: 1})
	})

	t.Run("Test miss, populated by external", func(t *testing.T) {
		// Tests the case where another process populates the external cache when trying
		// to obtain the lock
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Unset()
		fixtures.mockCache.RedisClient.On("Get", mock.Anything, mock.Anything).Return(cacheutil.ErrCacheMiss).Once().Run(func(_ mock.Arguments) {
			err := cache.SetGitReferences("test-repo", *GitRefCacheItemToReferences([][2]string{{"test-repo", "ref: test"}}))
			require.NoError(t, err)
		}).On("Get", mock.Anything, mock.Anything).Return(nil)
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.NotEqual(t, "test-lock-id", lockId)
		assert.Empty(t, lockId, "Lock id should not be set")
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 2, ExternalGets: 2})
	})

	t.Run("Test cache lock timeout", func(t *testing.T) {
		t.Parallel()
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
		assert.NotEmpty(t, lockId, "Lock id should be set")
		cache.revisionCacheLockTimeout = 10 * time.Second
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1})
	})

	t.Run("Test cache lock error", func(t *testing.T) {
		t.Parallel()
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
		assert.NotEmpty(t, lockId, "Lock id should be set")
		fixtures.mockCache.RedisClient.AssertNumberOfCalls(t, "Set", 2)
		fixtures.mockCache.RedisClient.AssertNumberOfCalls(t, "Get", 4)
	})
}

func TestUnlockGitReferences(t *testing.T) {
	t.Run("Test not locked", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		err := cache.UnlockGitReferences("test-repo", "")
		// UnlockGitReferences returns nil when the key doesn't exist (cache miss is not an error in this path)
		if err != nil {
			assert.ErrorContains(t, err, "key is missing")
		}
	})

	t.Run("Test unlock", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		var references []*plumbing.Reference
		lockId, err := cache.GetOrLockGitReferences("test-repo", "test-lock-id", &references)
		require.NoError(t, err)
		assert.Equal(t, "test-lock-id", lockId)
		assert.NotEmpty(t, lockId, "Lock id should be set")
		// Release lock
		err = cache.UnlockGitReferences("test-repo", lockId)
		require.NoError(t, err)
	})
}

func TestSetHelmIndex(t *testing.T) {
	t.Parallel()
	t.Run("SetHelmIndex with valid data", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		err := fixtures.cache.SetHelmIndex("test-repo", []byte("test-data"))
		require.NoError(t, err)
		// The legacy mirror uses Set; body and generation use one atomic publication.
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalSets: 1})
		record, err := fixtures.cache.GetHelmIndexRecord("test-repo")
		require.NoError(t, err)
		require.Equal(t, []byte("test-data"), record.Body)
		require.NotEmpty(t, record.Generation)
	})
	t.Run("SetHelmIndex with nil", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		err := fixtures.cache.SetHelmIndex("test-repo", nil)
		require.Error(t, err, "nil data should not be cached")
		var indexData []byte
		err = fixtures.cache.GetHelmIndex("test-repo", &indexData)
		require.Error(t, err)
		// A missing timestamped record cannot be fresh through a raw-key fallback.
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
}

func TestRevisionChartDetails(t *testing.T) {
	t.Parallel()
	t.Run("GetRevisionChartDetails cache miss", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		details, err := fixtures.cache.GetRevisionChartDetails("test-repo", "test-revision", "v1.0.0")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Equal(t, &v1alpha1.ChartDetails{}, details)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetRevisionChartDetails cache miss local", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := &v1alpha1.ChartDetails{
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
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := &v1alpha1.ChartDetails{
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
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := &v1alpha1.ChartDetails{
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

func TestOCIMetadata(t *testing.T) {
	t.Parallel()
	t.Run("GetOCIMetadata cache miss", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		metadata, err := fixtures.cache.GetOCIMetadata("test-repo", "sha256:abc")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Equal(t, &v1alpha1.OCIMetadata{}, metadata)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})

	t.Run("SetOCIMetadata round-trips", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := &v1alpha1.OCIMetadata{
			CreatedAt:   "2024-01-02T03:04:05Z",
			Authors:     "me@example.com",
			Version:     "1.2.3",
			Description: "test description",
		}
		err := fixtures.cache.SetOCIMetadata("test-repo", "sha256:abc", expectedItem)
		require.NoError(t, err)
		metadata, err := fixtures.cache.GetOCIMetadata("test-repo", "sha256:abc")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, metadata)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestGetGitDirectories(t *testing.T) {
	t.Parallel()
	t.Run("GetGitDirectories cache miss", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetGitDirectories("test-repo", "test-revision")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetGitDirectories cache miss local", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()
	t.Run("GetGitFiles cache miss", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetGitFiles("test-repo", "test-revision", "*.json")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetGitFiles cache hit", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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

func TestGetGitFilesChanges(t *testing.T) {
	t.Parallel()
	t.Run("GetGitFilesChanges cache miss", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetGitFilesChanges("test-repo", "test-revision", "syncedRevision")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetGitFilesChanges cache hit", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := []string{"test/path/file-1.yaml", "test/path1/file-2.yaml"}
		err := cache.cache.SetItem(
			getGitFilesChangesKey("test-repo", "test-revision", "syncedRevision"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		files, err := fixtures.cache.GetGitFilesChanges("test-repo", "test-revision", "syncedRevision")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetGitFilesChanges", func(t *testing.T) {
		t.Parallel()
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := []string{"test/path/file-1.yaml", "test/path1/file-2.yaml"}
		err := fixtures.cache.SetGitFilesChanges("test-repo", "test-revision", "syncedRevision", expectedItem)
		require.NoError(t, err)
		files, err := fixtures.cache.GetGitFilesChanges("test-repo", "test-revision", "syncedRevision")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestGetOciDirectories(t *testing.T) {
	t.Run("GetOciDirectories cache miss", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		directories, err := fixtures.cache.GetOciDirectories("oci://ghcr.io/example/manifests", "v1.0.0")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetOciDirectories cache miss local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := []string{"test/dir", "test/dir2"}
		err := cache.cache.SetItem(
			ociDirectoriesKey("oci://ghcr.io/example/manifests", "v1.0.0"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		directories, err := fixtures.cache.GetOciDirectories("oci://ghcr.io/example/manifests", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("GetOciDirectories cache hit local", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := []string{"test/dir", "test/dir2"}
		err := cache.cache.SetItem(
			ociDirectoriesKey("oci://ghcr.io/example/manifests", "v1.0.0"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		directories, err := fixtures.cache.GetOciDirectories("oci://ghcr.io/example/manifests", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetOciDirectories", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := []string{"test/dir", "test/dir2"}
		err := fixtures.cache.SetOciDirectories("oci://ghcr.io/example/manifests", "v1.0.0", expectedItem)
		require.NoError(t, err)
		directories, err := fixtures.cache.GetOciDirectories("oci://ghcr.io/example/manifests", "v1.0.0")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, directories)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestGetOciFiles(t *testing.T) {
	t.Run("GetOciFiles cache miss", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		files, err := fixtures.cache.GetOciFiles("oci://ghcr.io/example/manifests", "v1.0.0", "*.json")
		require.ErrorIs(t, err, ErrCacheMiss)
		assert.Empty(t, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1})
	})
	t.Run("GetOciFiles cache hit", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		cache := fixtures.cache
		expectedItem := map[string][]byte{"test/file.json": []byte("\"test\":\"contents\""), "test/file1.json": []byte("\"test1\":\"contents1\"")}
		err := cache.cache.SetItem(
			ociFilesKey("oci://ghcr.io/example/manifests", "v1.0.0", "*.json"),
			expectedItem,
			&cacheutil.CacheActionOpts{Expiration: 30 * time.Second})
		require.NoError(t, err)
		files, err := fixtures.cache.GetOciFiles("oci://ghcr.io/example/manifests", "v1.0.0", "*.json")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})

	t.Run("SetOciFiles", func(t *testing.T) {
		fixtures := newFixtures()
		t.Cleanup(fixtures.mockCache.StopRedisCallback)
		expectedItem := map[string][]byte{"test/file.json": []byte("\"test\":\"contents\""), "test/file1.json": []byte("\"test1\":\"contents1\"")}
		err := fixtures.cache.SetOciFiles("oci://ghcr.io/example/manifests", "v1.0.0", "*.json", expectedItem)
		require.NoError(t, err)
		files, err := fixtures.cache.GetOciFiles("oci://ghcr.io/example/manifests", "v1.0.0", "*.json")
		require.NoError(t, err)
		assert.Equal(t, expectedItem, files)
		fixtures.mockCache.AssertCacheCalledTimes(t, &mocks.CacheCallCounts{ExternalGets: 1, ExternalSets: 1})
	})
}

func TestHelmIndexValidators(t *testing.T) {
	fixtures := newFixtures()
	t.Cleanup(fixtures.mockCache.StopRedisCallback)
	err := fixtures.cache.SetHelmIndexValidators("test-repo", `"etag-1"`, "Wed, 21 Oct 2026 07:28:00 GMT")
	require.NoError(t, err)
	etag, lastModified, err := fixtures.cache.GetHelmIndexValidators("test-repo")
	require.NoError(t, err)
	assert.Equal(t, `"etag-1"`, etag)
	assert.Equal(t, "Wed, 21 Oct 2026 07:28:00 GMT", lastModified)

	// Missing validators → cache miss error, empty strings.
	_, _, err = fixtures.cache.GetHelmIndexValidators("missing-repo")
	assert.Error(t, err)
}

func TestHelmIndexLegacyValidatorsRoundTripWithRecord(t *testing.T) {
	cache, _ := newIndexProtocolCache(t)
	require.NoError(t, cache.SetHelmIndex("repo", helmIndexContractBody("1.0.0")))
	require.NoError(t, cache.SetHelmIndexValidators("repo", `"split"`, "date"))
	etag, modified, err := cache.GetHelmIndexValidators("repo")
	require.NoError(t, err)
	require.Equal(t, `"split"`, etag)
	require.Equal(t, "date", modified)
	record, err := cache.GetHelmIndexRecord("repo")
	require.NoError(t, err)
	require.Empty(t, record.ETag, "a split-key setter must not change body-bound validators")
}

func TestHelmIndexLegacyUnknownAgeRequiresValidation(t *testing.T) {
	cache, server, _, _ := helmIndexContractSetup(t)
	cache.SetHelmIndexCacheExpiration(30 * time.Second)
	body := helmIndexContractBody("1.0.0")
	require.NoError(t, cache.cache.SetItem(helmIndexRefsKey("repo"), body, &cacheutil.CacheActionOpts{Expiration: 3 * time.Minute}))
	server.FastForward(45 * time.Second)
	record, fresh, err := cache.GetHelmIndexSnapshotContext(t.Context(), "repo")
	require.NoError(t, err)
	require.Equal(t, body, record.Body, "legacy data remains available for validation")
	require.False(t, fresh, "a legacy writer's longer TTL cannot establish the reader's freshness")
	var cached []byte
	require.ErrorIs(t, cache.GetHelmIndex("repo", &cached), ErrCacheMiss)
}

// This fixture implements actual conditional responses. A server that always
// returns 200 cannot detect a mismatched body/ETag or prove retention saves bytes.
type helmIndexContractOrigin struct {
	mu       sync.Mutex
	version  string
	statuses []int
	bytes    []int
}

func helmIndexContractBody(version string) []byte {
	return []byte(fmt.Sprintf("apiVersion: v1\nentries:\n  child:\n    - version: %s\n", version))
}

func (o *helmIndexContractOrigin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	defer o.mu.Unlock()
	etag := `"` + o.version + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		o.statuses = append(o.statuses, http.StatusNotModified)
		o.bytes = append(o.bytes, 0)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	body := helmIndexContractBody(o.version)
	o.statuses = append(o.statuses, http.StatusOK)
	o.bytes = append(o.bytes, len(body))
	_, _ = w.Write(body)
}

func helmIndexContractSetup(t *testing.T) (*Cache, *miniredis.Miniredis, *helmIndexContractOrigin, string) {
	t.Helper()
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, redisClient.Close()) })
	base := cacheutil.NewCache(cacheutil.NewRedisCache(redisClient, time.Hour, cacheutil.RedisCompressionNone))
	cache := NewCache(base, time.Hour, time.Minute, 10*time.Second)
	origin := &helmIndexContractOrigin{version: "1.0.0"}
	server := httptest.NewServer(origin)
	t.Cleanup(server.Close)
	return cache, redisServer, origin, server.URL
}

func TestHelmIndexRedisContractRetainsBodyAndValidators(t *testing.T) {
	cache, redisServer, origin, url := helmIndexContractSetup(t)
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	redisServer.FastForward(2 * time.Minute)
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", index.Entries["child"][0].Version)
	origin.mu.Lock()
	defer origin.mu.Unlock()
	require.Equal(t, []int{http.StatusOK, http.StatusNotModified}, origin.statuses,
		"after freshness expires, retained body and validators must permit 304")
	assert.Zero(t, origin.bytes[1])
}

func TestHelmIndexRedisContractLegacyWriterCannotPoison304(t *testing.T) {
	cache, _, origin, url := helmIndexContractSetup(t)
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	origin.mu.Lock()
	origin.version = "2.0.0"
	origin.mu.Unlock()
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", index.Entries["child"][0].Version)
	// Simulate a delayed old replica: it only knows the legacy raw-body key.
	// It must not be able to change the new representation associated with v2.
	require.NoError(t, cache.cache.SetItem(helmIndexRefsKey(url), helmIndexContractBody("1.0.0"), &cacheutil.CacheActionOpts{Expiration: time.Hour}))
	index, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version,
		"a matching origin ETag must never authorize an old replica's body")
	origin.mu.Lock()
	defer origin.mu.Unlock()
	require.Equal(t, http.StatusNotModified, origin.statuses[len(origin.statuses)-1])
}

type operationCredentials struct {
	helm.HelmCreds
	mu     sync.Mutex
	tokens []string
	calls  int
	err    error
}

func (c *operationCredentials) GetPassword() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.tokens[min(c.calls-1, len(c.tokens)-1)], nil
}

func credentialCacheKey(url, password string) string {
	return url + "\x00" + helm.CredsScope(helm.HelmCreds{Username: "reader", Password: password})
}

func TestHelmIndexCredentialsStayConsistentAcrossOperations(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	var requestLock sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, _ := r.BasicAuth()
		requestLock.Lock()
		requests = append(requests, password)
		requestLock.Unlock()
		version := "1.0.0"
		if password == "token-B" {
			version = "2.0.0"
		}
		w.Header().Set("ETag", `"`+version+`"`)
		_, _ = w.Write(helmIndexContractBody(version))
	}))
	t.Cleanup(server.Close)
	provider := &operationCredentials{
		Username: "reader",
		tokens:   []string{"token-A", "token-B"},
	}
	client := helm.NewClient(server.URL, provider, false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	for i, version := range []string{"1.0.0", "2.0.0", "2.0.0"} {
		index, err := client.GetIndex(t.Context(), i == 0, 1<<20)
		require.NoError(t, err)
		require.Equal(t, version, index.Entries["child"][0].Version)
	}
	requestLock.Lock()
	gotRequests := append([]string(nil), requests...)
	requestLock.Unlock()
	assert.Equal(t, []string{"token-A", "token-B"}, gotRequests, "the second token must fetch its own view; the third operation should hit that cache")
	assert.Equal(t, 3, provider.calls, "resolve the provider exactly once for every operation, including cache hits")
	for password, version := range map[string]string{"token-A": "1.0.0", "token-B": "2.0.0"} {
		record, err := cache.GetHelmIndexRecord(credentialCacheKey(server.URL, password))
		require.NoError(t, err)
		assert.Equal(t, helmIndexContractBody(version), record.Body)
		assert.Equal(t, `"`+version+`"`, record.ETag)
	}
}

func TestHelmIndexCredentialFailureCannotUseWarmCache(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	url := "https://example.invalid"
	require.NoError(t, cache.SetHelmIndexRecord(credentialCacheKey(url, "token-A"), helmIndexContractBody("1.0.0"), `"one"`, ""))
	provider := &operationCredentials{Username: "reader", tokens: []string{"token-A"}}
	client := helm.NewClient(url, provider, false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	_, err := client.GetIndex(t.Context(), false, 1<<20)
	require.NoError(t, err)
	provider.err = errors.New("credential lookup failed")
	_, err = client.GetIndex(t.Context(), false, 1<<20)
	require.ErrorContains(t, err, "credential lookup failed", "a previous operation's warm entry cannot bypass current credential resolution")
}

func TestHelmTagsCacheUsesCurrentCredentialView(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	url := "registry.example.invalid/child"
	require.NoError(t, cache.SetOCITags(credentialCacheKey(url, "token-A")+"\x00https", []byte(`{"Tags":["1.0.0"]}`)))
	require.NoError(t, cache.SetOCITags(credentialCacheKey(url, "token-B")+"\x00https", []byte(`{"Tags":["2.0.0"]}`)))
	provider := &operationCredentials{Username: "reader", tokens: []string{"token-A", "token-B"}}
	client := helm.NewClient("registry.example.invalid", provider, true, "", "", helm.WithIndexCacheScopedByCreds(cache))
	for _, version := range []string{"1.0.0", "2.0.0"} {
		tags, err := client.GetTags(t.Context(), "child", false)
		require.NoError(t, err)
		assert.Equal(t, []string{version}, tags)
	}
	assert.Equal(t, 2, provider.calls)
}

func TestHelmIndexCacheExpirationIndependentFromRevisions(t *testing.T) {
	for _, expiration := range []time.Duration{0, 30 * time.Second, time.Minute, 3 * time.Minute, 48 * time.Hour} {
		t.Run(expiration.String(), func(t *testing.T) {
			cache, redisServer, _, _ := helmIndexContractSetup(t)
			require.Equal(t, time.Minute, cache.GetHelmIndexCacheExpiration())
			cache.revisionCacheExpiration = 7 * time.Minute
			cache.SetHelmIndexCacheExpiration(expiration)
			require.Equal(t, expiration, cache.GetHelmIndexCacheExpiration())
			validatedAt := time.Now().Add(-45 * time.Second)
			record := &helm.HelmIndexRecord{
				Body: helmIndexContractBody("1.0.0"), ETag: `"1.0.0"`,
				StoredAt: validatedAt.UTC().Format(time.RFC3339Nano),
			}
			require.NoError(t, cache.SetHelmIndexRetainedRecord("record", record))
			var body []byte
			err := cache.GetHelmIndex("record", &body)
			if expiration > 45*time.Second {
				require.NoError(t, err)
				require.Equal(t, record.Body, body)
			} else {
				require.ErrorIs(t, err, ErrCacheMiss, "expired Helm bodies must miss despite the longer Git revision interval")
			}
			retained, err := cache.GetHelmIndexRetainedRecord("record")
			require.NoError(t, err)
			require.NotEmpty(t, retained.Generation)
			expected := *record
			expected.Generation = retained.Generation
			expected.BodyDigest = retained.BodyDigest
			require.Equal(t, &expected, retained, "validators and body survive together without changing validation time")
			generation, err := cache.GetHelmIndexGenerationContext(t.Context(), "record")
			require.NoError(t, err)
			require.Equal(t, retained.Generation, generation)
			require.NoError(t, cache.SetGitReferences("git", nil))
			require.NoError(t, cache.SetOCITags("oci", []byte("tags")))
			require.NoError(t, cache.SetHelmIndex("legacy", record.Body))
			checked := 0
			for _, key := range redisServer.Keys() {
				switch {
				case strings.Contains(key, helmIndexRecordKey("record")), strings.Contains(key, helmIndexBodyReferenceKey("record")):
					want := time.Until(validatedAt.Add(expiration + max(24*time.Hour, expiration)))
					require.InDelta(t, float64(want), float64(redisServer.TTL(key)), float64(time.Second))
					checked++
				case strings.Contains(key, "git-refs|git"), strings.Contains(key, "oci-tags|oci"):
					require.Equal(t, 7*time.Minute, redisServer.TTL(key), "Helm configuration must not change Git/OCI TTLs")
					checked++
				case strings.Contains(key, "helm-index|legacy") && expiration > 0:
					require.Equal(t, expiration, redisServer.TTL(key))
					checked++
				}
			}
			wantKeys := 5
			if expiration == 0 {
				wantKeys-- // Zero uses the Redis backend's default legacy-key TTL.
			}
			require.Equal(t, wantKeys, checked)
		})
	}
}

// These tests came out of the code review of the Helm index cache (issue
// #30949): they assert the Redis layer's externally relevant guarantees and
// deliberately fail on the reviewed regressions. They live in this package
// because util/helm cannot import reposerver/cache (import cycle).

func regressionRedisCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	base := cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone))
	return NewCache(base, time.Hour, time.Minute, 10*time.Second), server
}

// regressionChartRepo serves a minimal helm repo whose index can be
// re-published with a new chart version between requests.
type regressionChartRepo struct {
	server   *httptest.Server
	mu       sync.Mutex
	index    string
	statuses []int
}

func (f *regressionChartRepo) publish(version string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.index = fmt.Sprintf(`apiVersion: v1
entries:
  child:
    - name: child
      version: %s
      urls:
        - %s/child-%s.tgz
`, version, f.server.URL, version)
}

func newRegressionChartRepo(t *testing.T) *regressionChartRepo {
	t.Helper()
	f := &regressionChartRepo{}
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		index := f.index
		f.mu.Unlock()
		// Version-derived ETag: strong validator that changes when the
		// index is re-published, enabling 304 revalidation in tests.
		etag := `"` + f.indexVersion(t) + `"`
		w.Header().Set("ETag", etag)
		status := http.StatusOK
		if r.Header.Get("If-None-Match") == etag {
			status = http.StatusNotModified
		}
		f.mu.Lock()
		f.statuses = append(f.statuses, status)
		f.mu.Unlock()
		w.WriteHeader(status)
		if status == http.StatusNotModified {
			return
		}
		_, _ = w.Write([]byte(index))
	})
	f.server = httptest.NewServer(mux)
	f.publish("1.0.0")
	t.Cleanup(f.server.Close)
	return f
}

func TestIndexCacheRegressionRedisRetainsRevalidationDataAfterFreshness(t *testing.T) {
	cache, redisServer := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	// Advance Redis time without sleeping. The body's freshness expires, but
	// validators must be retained longer so the next read can revalidate
	// conditionally instead of re-downloading.
	redisServer.FastForward(2 * time.Minute)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
}

func TestIndexCacheRegressionLegacyRedisWriterCannotPoison304(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	var oldBody []byte
	require.NoError(t, cache.GetHelmIndex(fixture.server.URL, &oldBody))
	fixture.publish("2.0.0")
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", fixture.indexVersion(t), "origin now serves 2.0.0")
	// A delayed old replica writes only the legacy body, without the new
	// validator protocol. New records must be isolated from that legacy key:
	// a 304 must never pair a new validator with the old body.
	require.NoError(t, cache.cache.SetItem(helmIndexRefsKey(fixture.server.URL), oldBody, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", index.Entries["child"][0].Version, "an old replica must not make the client serve an old body under a new validator")
}

// indexVersion returns the version currently published by the fixture.
func (f *regressionChartRepo) indexVersion(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for line := range strings.SplitSeq(f.index, "\n") {
		if after, ok := strings.CutPrefix(line, "      version: "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// --- Round-2 review regressions (findings 2/3/4/6 of the second review) ---

// Finding 4: redis keys carry only the repo URL, so a private-credentialed
// connectivity check would populate cache entries that an anonymous reader
// consumes. With scoped clients, the two views must see disjoint keys.
func TestIndexCacheRegressionRedisKeysScopedByCredentials(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)

	private := helm.NewClient(fixture.server.URL,
		helm.HelmCreds{Username: "reader", Password: "secret"},
		false, "", "", helm.WithIndexCacheScopedByCreds(cache))
	_, err := private.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// The private fetch publishes under a scoped key; the legacy bare-URL
	// keys must remain untouched (that is what an anonymous reader uses).
	var body []byte
	require.ErrorIs(t, cache.GetHelmIndex(fixture.server.URL, &body), cacheutil.ErrCacheMiss,
		"a private-credentialed fetch must not populate the shared bare-URL cache entry")
	_, _, err = cache.GetHelmIndexValidators(fixture.server.URL)
	require.Error(t, err, "validators must also stay scoped: an anonymous reader must not inherit private-fetch metadata")

	// An anonymous client with the SAME empty-credential scope shares keys
	// with other anonymous clients (that is the point: same view = same key).
	anon := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err = anon.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
}

// Finding 2: the v2 record must stop serving once body freshness expires
// (age > revision-cache TTL), even though the record itself is retained
// longer for 304 revalidation. A stale body served to an ordinary reader
// would extend staleness from minutes to the repo-cache window.
func TestIndexCacheRegressionRedisStaleBodyNotServedAfterFreshness(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "",
		helm.WithIndexCacheScopedByCreds(cache))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)

	// The client publishes under the SCOPED key — assert on the populated
	// key, not the bare URL (a bare-URL miss would pass regardless of
	// freshness and prove nothing).
	scopedKey := fixture.server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})

	// Freshness is measured from the record's StoredAt with time.Now() —
	// miniredis FastForward does NOT move that clock. Backdate the stored
	// validation timestamp to simulate real aging instead.
	rec, err := cache.GetHelmIndexRecord(scopedKey)
	require.NoError(t, err)
	require.NotEmpty(t, rec.Body, "the test must exercise the populated key")
	// Backdate the validation timestamp in place (package-internal SetItem).
	rec.StoredAt = time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339Nano)
	require.NoError(t, cache.cache.SetItem(helmIndexRecordKey(scopedKey), rec,
		&cacheutil.CacheActionOpts{Expiration: cache.repoCacheExpiration}))

	// Past freshness: ordinary readers must NOT get the stale body...
	var body []byte
	err = cache.GetHelmIndex(scopedKey, &body)
	require.ErrorIs(t, err, cacheutil.ErrCacheMiss, "stale body must not be served to ordinary readers after freshness expiry")

	// ...while the revalidation path can still read the retained record.
	retained, err := cache.GetHelmIndexRetainedRecord(scopedKey)
	require.NoError(t, err, "the retained record must survive freshness expiry for 304 revalidation")
	assert.NotEmpty(t, retained.Body)
	assert.Equal(t, rec.ETag, retained.ETag)
}

// Finding 6: a fresh client instance must keep validators across a 304 so
// the chain is 200 → 304 → 304 (not 200 → 304 → 200). Fresh clients arise
// naturally: repo-server builds a new client per request.
func TestIndexCacheRegressionFreshClientKeepsValidatorsAfter304(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	fixture := newRegressionChartRepo(t)
	scopedKey := fixture.server.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})
	for attempt := range 3 {
		if attempt > 0 {
			record, err := cache.GetHelmIndexRecord(scopedKey)
			require.NoError(t, err)
			record.StoredAt = time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)
			require.NoError(t, cache.SetHelmIndexRetainedRecord(scopedKey, record))
		}
		client := helm.NewClient(fixture.server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCacheScopedByCreds(cache))
		index, err := client.GetIndex(t.Context(), false, 1<<20)
		require.NoError(t, err)
		require.Equal(t, "1.0.0", index.Entries["child"][0].Version)
		record, err := cache.GetHelmIndexRecord(scopedKey)
		require.NoError(t, err)
		require.Equal(t, `"1.0.0"`, record.ETag)
		require.True(t, record.Fresh(time.Minute), "304 must renew the same shared generation")
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	require.Equal(t, []int{http.StatusOK, http.StatusNotModified, http.StatusNotModified}, fixture.statuses)
}

func TestHelmOCITagsKeepRevisionExpiration(t *testing.T) {
	cache, redisServer, _, _ := helmIndexContractSetup(t)
	cache.revisionCacheExpiration = 7 * time.Minute
	cache.SetHelmIndexCacheExpiration(30 * time.Second)
	var requests, version atomic.Int32
	version.Store(1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"name":"child","tags":["%d.0.0"]}`, version.Load())
	}))
	t.Cleanup(server.Close)
	t.Setenv("DOCKER_CONFIG", t.TempDir())
	host := strings.TrimPrefix(server.URL, "http://")
	client := helm.NewClient(host, helm.HelmCreds{}, true, "", "", helm.WithPlainHTTP(), helm.WithIndexCache(cache))
	tags, err := client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags)
	// Expose the former HTTP-record route without depending on wall-clock sleeps.
	if record, err := cache.GetHelmIndexRecord(host + "/child"); err == nil {
		record.StoredAt = time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339Nano)
		require.NoError(t, cache.SetHelmIndexRetainedRecord(host+"/child", record))
	}
	redisServer.FastForward(90 * time.Second)
	version.Store(2)
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"1.0.0"}, tags, "HTTP index expiry must not expire OCI tags")
	require.EqualValues(t, 1, requests.Load())
	redisServer.FastForward(6 * time.Minute)
	tags, err = client.GetTags(t.Context(), "child", false)
	require.NoError(t, err)
	require.Equal(t, []string{"2.0.0"}, tags)
	require.EqualValues(t, 2, requests.Load())
}

type generationReadBarrier struct {
	*Cache
	observed, resume chan struct{}
	once             sync.Once
}

func (c *generationReadBarrier) GetHelmIndexRetainedRecord(key string) (*HelmIndexRecord, error) {
	record, err := c.Cache.GetHelmIndexRetainedRecord(key)
	c.once.Do(func() { close(c.observed); <-c.resume })
	return record, err
}

func (c *generationReadBarrier) GetHelmIndexRetainedRecordContext(ctx context.Context, key string) (*HelmIndexRecord, error) {
	record, err := c.Cache.GetHelmIndexRetainedRecordContext(ctx, key)
	c.once.Do(func() {
		close(c.observed)
		select {
		case <-c.resume:
		case <-ctx.Done():
			err = ctx.Err()
		}
	})
	return record, err
}

func TestIndexRecordReadInterleavingKeepsGeneration(t *testing.T) {
	cache, _, origin, url := helmIndexContractSetup(t)
	body := helmIndexContractBody("1.0.0")
	require.NoError(t, cache.SetHelmIndexRetainedRecord(url, &HelmIndexRecord{Body: body, ETag: `"1.0.0"`, URLIdentity: helm.IndexURLIdentity(url + "/index.yaml"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}))
	view := &generationReadBarrier{Cache: cache, observed: make(chan struct{}), resume: make(chan struct{})}
	release := sync.OnceFunc(func() { close(view.resume) })
	t.Cleanup(release)
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(view))
	type result struct {
		index *helm.Index
		err   error
	}
	done := make(chan result, 1)
	go func() { index, err := client.GetIndex(t.Context(), true, 1<<20); done <- result{index, err} }()
	select {
	case <-view.observed:
	case <-time.After(5 * time.Second):
		t.Fatal("client never observed the retained record")
	}
	require.NoError(t, cache.SetHelmIndexRecord(url, helmIndexContractBody("2.0.0"), `"2.0.0"`, ""))
	release()
	var got result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("revalidation did not finish")
	}
	require.NoError(t, got.err)
	require.Equal(t, "1.0.0", got.index.Entries["child"][0].Version, "304 must return the generation whose validator was sent")
	record, err := cache.GetHelmIndexRecord(url)
	require.NoError(t, err)
	require.Equal(t, helmIndexContractBody("2.0.0"), record.Body, "a delayed 304 must leave the concurrent generation intact")
	require.Equal(t, `"2.0.0"`, record.ETag)
	origin.mu.Lock()
	defer origin.mu.Unlock()
	require.Equal(t, []int{http.StatusNotModified}, origin.statuses)
}

type legacyRecordCache struct {
	cache     *Cache
	published []*HelmIndexRecord
}

func (c *legacyRecordCache) GetHelmIndex(k string, b *[]byte) error {
	return c.cache.GetHelmIndex(k, b)
}

func (c *legacyRecordCache) SetHelmIndex(k string, b []byte) error { return c.cache.SetHelmIndex(k, b) }

func (c *legacyRecordCache) GetHelmIndexValidators(k string) (string, string, error) {
	return c.cache.GetHelmIndexValidators(k)
}

func (c *legacyRecordCache) SetHelmIndexValidators(k, e, m string) error {
	return c.cache.SetHelmIndexValidators(k, e, m)
}

func (c *legacyRecordCache) SetHelmIndexRecord(k string, b []byte, e, m string) error {
	c.published = append(c.published, &HelmIndexRecord{Body: append([]byte(nil), b...), ETag: e, LastModified: m})
	return c.cache.SetHelmIndexRecord(k, b, e, m)
}

func TestLegacy304KeepsRequestBodyAndValidators(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	body := helmIndexContractBody("1.0.0")
	updates := make(chan error, 1)
	headers := make(chan string, 1)
	var url string
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch attempts.Add(1) {
		case 1:
			w.Header().Set("ETag", `"1.0.0"`)
			_, _ = w.Write(body)
			return
		case 2:
			// The only expected revalidation request.
		default:
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		headers <- r.Header.Get("If-None-Match")
		updates <- cache.SetHelmIndexRecord(url, helmIndexContractBody("2.0.0"), `"2.0.0"`, "")
		// A 304 can omit validators; they must be inherited from the request's
		// held generation, never recovered from the concurrently replaced record.
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	url = server.URL
	require.NoError(t, cache.SetHelmIndexRecord(url, body, `"1.0.0"`, ""))
	view := &legacyRecordCache{cache: cache}
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(view))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	view.published = nil
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.EqualValues(t, 2, attempts.Load(), "observe the second request before consuming handler results")
	require.NoError(t, <-updates)
	require.Equal(t, `"1.0.0"`, <-headers)
	require.Equal(t, "1.0.0", index.Entries["child"][0].Version)
	require.Empty(t, view.published, "a cache without generation comparison cannot safely republish a 304")
	final, err := cache.GetHelmIndexRecord(url)
	require.NoError(t, err)
	require.Equal(t, helmIndexContractBody("2.0.0"), final.Body)
	require.Equal(t, `"2.0.0"`, final.ETag)
}

type splitIndexCache struct {
	cache           *Cache
	validatorWrites int
}

func (c *splitIndexCache) GetHelmIndex(k string, b *[]byte) error { return c.cache.GetHelmIndex(k, b) }

func (c *splitIndexCache) SetHelmIndex(k string, b []byte) error { return c.cache.SetHelmIndex(k, b) }

func (c *splitIndexCache) GetHelmIndexValidators(k string) (string, string, error) {
	return c.cache.GetHelmIndexValidators(k)
}

func (c *splitIndexCache) SetHelmIndexValidators(k, e, m string) error {
	c.validatorWrites++
	return c.cache.SetHelmIndexValidators(k, e, m)
}

func TestSplitIndexCacheNeverPublishesUnboundValidators(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	view := &splitIndexCache{cache: cache}
	var requests atomic.Int32
	var publication error
	var url string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("ETag", `"one"`)
			_, _ = w.Write(helmIndexContractBody("1.0.0"))
			return
		}
		publication = cache.SetHelmIndexRecord(url, helmIndexContractBody("2.0.0"), `"two"`, "")
		if r.Header.Get("If-None-Match") != `"one"` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	url = server.URL
	require.NoError(t, cache.SetHelmIndex(url, helmIndexContractBody("1.0.0")))
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(view))
	_, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	_, err = client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.NoError(t, publication)
	require.Zero(t, view.validatorWrites, "split caches cannot prove the generation of a validator-only write")
	record, err := cache.GetHelmIndexRecord(url)
	require.NoError(t, err)
	require.Equal(t, helmIndexContractBody("2.0.0"), record.Body)
	require.Equal(t, `"two"`, record.ETag)
}

func TestIndexDelayed304PreservesConcurrentGeneration(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	entered, release := make(chan struct{}), make(chan struct{})
	resume := sync.OnceFunc(func() { close(release) })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"one"` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(server.Close)
	t.Cleanup(resume)
	url := server.URL
	require.NoError(t, cache.SetHelmIndexRetainedRecord(url, &HelmIndexRecord{Body: helmIndexContractBody("1.0.0"), ETag: `"one"`, URLIdentity: helm.IndexURLIdentity(url + "/index.yaml"), StoredAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}))
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	result := make(chan error, 1)
	go func() { _, err := client.GetIndex(t.Context(), true, 1<<20); result <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request never reached origin")
	}
	require.NoError(t, cache.SetHelmIndexRecord(url, helmIndexContractBody("2.0.0"), `"two"`, ""))
	resume()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("revalidation did not finish")
	}
	record, err := cache.GetHelmIndexRecord(url)
	require.NoError(t, err)
	require.Equal(t, helmIndexContractBody("2.0.0"), record.Body, "a delayed 304 cannot roll back a newer shared generation")
	require.Equal(t, `"two"`, record.ETag)
}

func TestIndexRedirectTargetOwnsValidators(t *testing.T) {
	cache, _ := regressionRedisCache(t)
	var target atomic.Int32
	target.Store(1)
	var mu sync.Mutex
	var statuses []int
	var redirectValidators []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			mu.Lock()
			redirectValidators = append(redirectValidators, r.Header.Get("If-None-Match")+r.Header.Get("If-Modified-Since"))
			mu.Unlock()
			http.Redirect(w, r, fmt.Sprintf("/v%d/index.yaml", target.Load()), http.StatusFound)
			return
		}
		version := "1.0.0"
		if r.URL.Path == "/v2/index.yaml" {
			version = "2.0.0"
		}
		status := http.StatusOK
		w.Header().Set("ETag", `"shared-tag"`)
		if r.Header.Get("If-None-Match") == `"shared-tag"` {
			status = http.StatusNotModified
		}
		mu.Lock()
		statuses = append(statuses, status)
		mu.Unlock()
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write(helmIndexContractBody(version))
		}
	}))
	t.Cleanup(server.Close)
	client := helm.NewClient(server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	for _, version := range []string{"1.0.0", "1.0.0", "2.0.0", "2.0.0"} {
		if version == "2.0.0" {
			target.Store(2)
		}
		index, err := client.GetIndex(t.Context(), true, 1<<20)
		require.NoError(t, err)
		require.Equal(t, version, index.Entries["child"][0].Version, "an ETag from another redirect target cannot authorize these bytes")
	}
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []int{200, 304, 200, 304}, statuses)
	require.Equal(t, []string{"", "", "", ""}, redirectValidators)
}

func TestIndexDelayed200PreservesConcurrentGeneration(t *testing.T) {
	for _, warm := range []bool{false, true} {
		t.Run(strconv.FormatBool(warm), func(t *testing.T) {
			cache, _ := regressionRedisCache(t)
			entered, release := make(chan struct{}), make(chan struct{})
			resume := sync.OnceFunc(func() { close(release) })
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"one"`)
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				_, _ = w.Write(helmIndexContractBody("1.0.0"))
			}))
			t.Cleanup(server.Close)
			t.Cleanup(resume)
			if warm {
				require.NoError(t, cache.SetHelmIndexRecord(server.URL, helmIndexContractBody("0.0.1"), `"old"`, ""))
			}
			client := helm.NewClient(server.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
			done := make(chan error, 1)
			go func() { _, err := client.GetIndex(t.Context(), true, 1<<20); done <- err }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("200 never started")
			}
			require.NoError(t, cache.SetHelmIndexRecord(server.URL, helmIndexContractBody("2.0.0"), `"two"`, ""))
			resume()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("200 did not finish")
			}
			record, err := cache.GetHelmIndexRecord(server.URL)
			require.NoError(t, err)
			require.Equal(t, helmIndexContractBody("2.0.0"), record.Body)
			require.Equal(t, `"two"`, record.ETag)
		})
	}
}

func TestIndexLegacyRecordWithoutURLFetchesBody(t *testing.T) {
	cache, _, origin, url := helmIndexContractSetup(t)
	origin.mu.Lock()
	origin.version = "2.0.0"
	origin.mu.Unlock()
	// The old ETag may have belonged to an earlier redirect target.
	require.NoError(t, cache.SetHelmIndexRecord(url, helmIndexContractBody("1.0.0"), `"2.0.0"`, ""))
	client := helm.NewClient(url, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(cache))
	index, err := client.GetIndex(t.Context(), true, 1<<20)
	require.NoError(t, err)
	require.Equal(t, "2.0.0", index.Entries["child"][0].Version)
	origin.mu.Lock()
	defer origin.mu.Unlock()
	require.Equal(t, []int{http.StatusOK}, origin.statuses, "a legacy record without URL provenance cannot authorize a 304")
}

func TestHelmIndexObservationBudgetLeavesTimeForOrigin(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, force := range []bool{false, true} {
		t.Run(strconv.FormatBool(force), func(t *testing.T) {
			listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
			require.NoError(t, err)
			var mu sync.Mutex
			var connections []net.Conn
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					mu.Lock()
					connections = append(connections, conn)
					mu.Unlock()
				}
			}()
			t.Cleanup(func() {
				_ = listener.Close()
				<-done
				mu.Lock()
				defer mu.Unlock()
				for _, conn := range connections {
					_ = conn.Close()
				}
			})
			client := redis.NewClient(&redis.Options{Addr: listener.Addr().String(), ReadTimeout: time.Second, MaxRetries: -1, ContextTimeoutEnabled: true})
			t.Cleanup(func() { require.NoError(t, client.Close()) })
			shared := NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Minute, cacheutil.RedisCompressionNone)), time.Hour, time.Minute, time.Minute)
			archive, err := os.ReadFile(filepath.Join("..", "..", "test", "e2e", "testdata", "helm-repo", "helm-1.0.0.tgz"))
			require.NoError(t, err)
			var requests, archives atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/helm-1.0.0.tgz" {
					archives.Add(1)
					_, _ = w.Write(archive)
					return
				}
				requests.Add(1)
				_, _ = w.Write([]byte("apiVersion: v1\nentries:\n  helm:\n    - name: helm\n      apiVersion: v1\n      version: 1.0.0\n      urls: [helm-1.0.0.tgz]\n"))
			}))
			t.Cleanup(origin.Close)
			store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
			// The public store methods take the private client type; exercise the
			// same path through dependency build with a real declared repository.
			parent := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: helm\n    version: 1.0.0\n    repository: "+origin.URL+"\n"), 0o600))
			app, err := helm.NewHelmAppWithIndexStore(parent, []helm.HelmRepository{{Name: "charts", Repo: origin.URL, Creds: helm.HelmCreds{}}}, false, "", "", "", false, false, store, force)
			require.NoError(t, err)
			t.Cleanup(app.Dispose)
			// This fixture measures stalled Redis I/O, independently of cold
			// credential KDF scheduling covered by the cancellation tests.
			identity := origin.URL + "\x00" + helm.CredsScope(helm.HelmCreds{})
			_, err = helmIndexKeyPrefix(t.Context(), identity)
			require.NoError(t, err)
			_ = helm.IndexURLIdentity(origin.URL + "/index.yaml")
			ctx, cancel := context.WithTimeout(t.Context(), 800*time.Millisecond)
			defer cancel()
			err = app.DependencyBuild(ctx)
			require.EqualValues(t, 1, requests.Load(), "a stalled Redis must not consume the origin request deadline")
			require.NoError(t, err, "a healthy origin must complete the dependency build during a Redis stall")
			require.EqualValues(t, 1, archives.Load())
			require.NoError(t, ctx.Err(), "the request budget must remain available after the bounded observation")
			_, err = store.Get(origin.URL + "\x00" + helm.CredsScope(helm.HelmCreds{}))
			require.Error(t, err, "unconfirmed observation must not publish a fresh disk record")
		})
	}
}

func requirePinnedHelmForCacheTest(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("requires repository-pinned Helm")
	}
	versions, err := os.ReadFile(filepath.Join("..", "..", "hack", "tool-versions.sh"))
	require.NoError(t, err)
	var pinned string
	for line := range strings.SplitSeq(string(versions), "\n") {
		if value, ok := strings.CutPrefix(line, "HELM_VERSION="); ok {
			pinned = strings.Trim(value, "\"'")
		}
	}
	require.NotEmpty(t, pinned)
	output, err := exec.CommandContext(t.Context(), "helm", "version", "--short").Output()
	require.NoError(t, err)
	actual, _, _ := strings.Cut(strings.TrimSpace(string(output)), "+")
	require.Equal(t, "v"+pinned, actual)
}

// Delays surround real Redis operations so publication, key encoding and
// ownership checks still run through the production backend.
type indexProtocolBackend struct {
	cacheutil.CacheClient
	stallMetadata    bool
	stallBody        bool
	bodyDelay        time.Duration
	missDelay        time.Duration
	bodyReads        int
	stalls           int
	publicationReply string
	afterPublication func()
	beforeBody       func()
	lostClaimReply   bool
	failedRelease    bool
	bodyWrites       int
	readDelays       map[string]time.Duration
	readKeys         []string
	readDeadlines    []time.Time
	readStarted      []time.Time
}

func (b *indexProtocolBackend) GetContext(ctx context.Context, key string, obj any) error {
	delay := b.missDelay
	if b.readDelays != nil {
		kind := "v3"
		if strings.HasPrefix(key, "helm-index-v2|") {
			kind = "v2"
		} else if strings.HasPrefix(key, "helm-index|") {
			kind = "raw"
		}
		delay = b.readDelays[kind]
		b.readKeys = append(b.readKeys, kind)
		deadline, _ := ctx.Deadline()
		b.readDeadlines = append(b.readDeadlines, deadline)
		b.readStarted = append(b.readStarted, time.Now())
	}
	if strings.Contains(key, "|body") {
		b.bodyReads++
		delay = b.bodyDelay
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return b.CacheClient.(interface {
		GetContext(context.Context, string, any) error
	}).GetContext(ctx, key, obj)
}

func (b *indexProtocolBackend) BeginUpdate(ctx context.Context, key, token string, expiration time.Duration) (bool, error) {
	current, err := b.CacheClient.(interface {
		BeginUpdate(context.Context, string, string, time.Duration) (bool, error)
	}).BeginUpdate(ctx, key, token, expiration)
	if current && b.lostClaimReply {
		return false, io.ErrUnexpectedEOF
	}
	return current, err
}

func (b *indexProtocolBackend) ReleaseUpdate(ctx context.Context, key, token string) (bool, error) {
	if b.failedRelease {
		return false, io.ErrUnexpectedEOF
	}
	return b.CacheClient.(interface {
		ReleaseUpdate(context.Context, string, string) (bool, error)
	}).ReleaseUpdate(ctx, key, token)
}

func (b *indexProtocolBackend) StoreVersioned(ctx context.Context, item cacheutil.VersionedItem) (bool, error) {
	if b.stallMetadata {
		b.stalls++
		<-ctx.Done()
		return false, ctx.Err()
	}
	updated, err := b.CacheClient.(interface {
		StoreVersioned(context.Context, cacheutil.VersionedItem) (bool, error)
	}).StoreVersioned(ctx, item)
	if err == nil && updated && b.publicationReply != "" {
		mode := b.publicationReply
		b.publicationReply = ""
		if b.afterPublication != nil {
			b.afterPublication()
		}
		switch mode {
		case "retry":
			return false, nil // A replay sees an already-consumed owner.
		case "timeout":
			<-ctx.Done()
			return false, ctx.Err()
		default:
			return false, io.ErrUnexpectedEOF
		}
	}
	return updated, err
}

func (b *indexProtocolBackend) StoreVersionedBody(ctx context.Context, referenceKey, bodyKey, referenceValue string, body any, expiration time.Duration) (bool, error) {
	b.bodyWrites++
	if callback := b.beforeBody; callback != nil {
		b.beforeBody = nil
		callback()
	}
	if b.stallBody {
		b.stalls++
		<-ctx.Done()
		return false, ctx.Err()
	}
	return b.CacheClient.(interface {
		StoreVersionedBody(context.Context, string, string, string, any, time.Duration) (bool, error)
	}).StoreVersionedBody(ctx, referenceKey, bodyKey, referenceValue, body, expiration)
}

func newIndexProtocolCache(t *testing.T) (*Cache, *indexProtocolBackend) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true, MaxRetries: -1})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	backend := &indexProtocolBackend{CacheClient: cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone)}
	return NewCache(cacheutil.NewCache(backend), time.Hour, time.Minute, time.Minute), backend
}

func indexProtocolOrigin(t *testing.T) (string, *atomic.Int32, *atomic.Int32, []byte) {
	t.Helper()
	archive, err := os.ReadFile(filepath.Join("..", "..", "test", "e2e", "testdata", "helm-repo", "helm-1.0.0.tgz"))
	require.NoError(t, err)
	body := []byte("apiVersion: v1\nentries:\n  helm:\n    - name: helm\n      apiVersion: v1\n      version: 1.0.0\n      urls: [helm-1.0.0.tgz]\n")
	var requests, archives atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/helm-1.0.0.tgz" {
			archives.Add(1)
			_, _ = w.Write(archive)
			return
		}
		requests.Add(1)
		w.Header().Set("ETag", `"one"`)
		_, _ = w.Write(body)
	}))
	t.Cleanup(origin.Close)
	return origin.URL, &requests, &archives, body
}

func indexProtocolBuild(t *testing.T, ctx context.Context, url string, store *helm.IndexStore, force bool) {
	t.Helper()
	parent := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(parent, "Chart.yaml"), []byte("apiVersion: v2\nname: parent\nversion: 0.1.0\ndependencies:\n  - name: helm\n    version: 1.0.0\n    repository: "+url+"\n"), 0o600))
	app, err := helm.NewHelmAppWithIndexStore(parent, []helm.HelmRepository{{Name: "charts", Repo: url, Creds: helm.HelmCreds{}}}, false, "", "", "", false, false, store, force)
	require.NoError(t, err)
	t.Cleanup(app.Dispose)
	require.NoError(t, app.DependencyBuild(ctx))
	require.NoError(t, ctx.Err(), "cache work must leave time for the Helm subprocess")
}

func TestHelmIndexPublicationBudgets(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, force := range []bool{false, true} {
		for _, stage := range []string{"metadata", "body"} {
			t.Run(stage+"/force="+strconv.FormatBool(force), func(t *testing.T) {
				shared, backend := newIndexProtocolCache(t)
				url, requests, archives, body := indexProtocolOrigin(t)
				identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
				if stage == "metadata" {
					require.NoError(t, shared.SetHelmIndexRetainedRecord(identity, &HelmIndexRecord{Body: body, StoredAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)}))
					backend.stallMetadata = true
				} else {
					backend.stallBody = true
				}
				store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				indexProtocolBuild(t, ctx, url, store, force)
				require.EqualValues(t, 1, requests.Load())
				require.EqualValues(t, 1, archives.Load())
				require.Equal(t, 1, backend.stalls)
				_, err := store.Get(identity)
				if stage == "metadata" {
					require.Error(t, err, "unconfirmed publication cannot advance disk freshness")
					return
				}
				require.NoError(t, err, "confirmed metadata permits local warming when body sharing times out")
				_, _, err = shared.GetHelmIndexSnapshotContext(t.Context(), identity)
				require.ErrorIs(t, err, ErrCacheMiss, "cold readers cannot treat a missing shared body as a hit")
				reads := backend.bodyReads
				indexProtocolBuild(t, t.Context(), url, store, false)
				require.EqualValues(t, 1, requests.Load(), "the next build must use its confirmed local body")
				require.Equal(t, reads, backend.bodyReads, "matching metadata must avoid full Redis body reads")
			})
		}
	}
}

func TestHelmIndexSlowBodyDoesNotCauseRepeatedDownloads(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	shared, backend := newIndexProtocolCache(t)
	url, requests, _, body := indexProtocolOrigin(t)
	identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
	require.NoError(t, shared.SetHelmIndexRecord(identity, body, `"one"`, ""))
	backend.bodyDelay = 1200 * time.Millisecond
	store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	indexProtocolBuild(t, ctx, url, store, false)
	indexProtocolBuild(t, ctx, url, store, false)
	require.EqualValues(t, 0, requests.Load(), "a shared body taking over one second must remain usable")
	require.Equal(t, 1, backend.bodyReads, "the second build must only read metadata")
}

func TestHelmIndexLegacyMissesShareOneBudget(t *testing.T) {
	shared, backend := newIndexProtocolCache(t)
	backend.readDelays = map[string]time.Duration{"v3": 0, "v2": 40 * time.Millisecond, "raw": 300 * time.Millisecond}
	var requests atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(helmIndexContractBody("1.0.0"))
	}))
	t.Cleanup(origin.Close)
	// Isolate fallback I/O budgeting from cold KDF scheduling. Cancellation
	// of queued/running key derivation has separate deterministic coverage.
	_, err := helmIndexKeyPrefix(t.Context(), origin.URL)
	require.NoError(t, err)
	helm.IndexURLIdentity(origin.URL + "/index.yaml")
	client := helm.NewClient(origin.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(shared))
	ctx, cancel := context.WithTimeout(t.Context(), 800*time.Millisecond)
	defer cancel()
	_, err = client.GetIndex(ctx, false, 1<<20)
	require.NoError(t, err)
	require.NoError(t, ctx.Err())
	require.EqualValues(t, 1, requests.Load())
	require.Equal(t, []string{"v3", "v2", "raw", "v3"}, backend.readKeys)
	callerDeadline, _ := ctx.Deadline()
	require.True(t, backend.readDeadlines[0].Before(callerDeadline))
	require.LessOrEqual(t, backend.readDeadlines[0].Sub(backend.readStarted[0]), 800*time.Millisecond/4, "the first observation must reserve the rest of the caller budget")
	// The final read observes again after claiming; only the initial
	// v3/v2/raw fallback chain must share the first observation's budget.
	for _, deadline := range backend.readDeadlines[:3] {
		require.Equal(t, backend.readDeadlines[0], deadline, "every fallback must use the initial bounded read deadline")
	}
}

func TestHelmIndexChangedGenerationRequiresUsableBody(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, corrupt := range []bool{false, true} {
		t.Run("corrupt="+strconv.FormatBool(corrupt), func(t *testing.T) {
			shared, _ := newIndexProtocolCache(t)
			url, requests, _, body := indexProtocolOrigin(t)
			identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
			store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
			indexProtocolBuild(t, t.Context(), url, store, false)
			require.EqualValues(t, 1, requests.Load())
			// A newer shared generation invalidates the still-fresh local one.
			// An old legacy body must not hide a missing/corrupt current body.
			require.NoError(t, shared.cache.SetItem(helmIndexRefsKey(identity), body, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
			require.NoError(t, shared.SetHelmIndexRecord(identity, append(append([]byte{}, body...), []byte("# newer\n")...), `"newer"`, ""))
			_, err := shared.GetHelmIndexRecord(identity)
			require.NoError(t, err)
			key := helmIndexBodyKey(identity)
			require.NoError(t, shared.cache.SetItem(key, []byte("wrong body"), &cacheutil.CacheActionOpts{Expiration: time.Hour, Delete: !corrupt}))
			_, _, err = shared.GetHelmIndexSnapshotContext(t.Context(), identity)
			require.Error(t, err, "legacy data must not hide an unusable current generation")
			indexProtocolBuild(t, t.Context(), url, store, false)
			require.EqualValues(t, 2, requests.Load(), "a changed generation without a usable body requires origin validation")
		})
	}
}

func TestHelmIndexMetadataCannotSelectAnOlderBody(t *testing.T) {
	shared, _ := newIndexProtocolCache(t)
	repo := "https://charts.example.test"
	require.NoError(t, shared.SetHelmIndex(repo, helmIndexContractBody("1.0.0")))
	owner, err := shared.BeginHelmIndexFetch(t.Context(), repo)
	require.NoError(t, err)
	updated, err := shared.PublishHelmIndexRecord(t.Context(), repo, owner, &HelmIndexRecord{Body: helmIndexContractBody("2.0.0"), StoredAt: time.Now().UTC().Format(time.RFC3339Nano)})
	require.NoError(t, err)
	require.True(t, updated)
	record, err := shared.GetHelmIndexRecord(repo)
	require.NoError(t, err)
	bodyKey := helmIndexBodyKey(repo)
	for _, corrupt := range []bool{false, true} {
		if corrupt {
			require.NoError(t, shared.cache.SetItem(bodyKey, helmIndexContractBody("1.0.0"), nil))
		} else {
			require.NoError(t, shared.cache.SetItem(bodyKey, record.Body, &cacheutil.CacheActionOpts{Delete: true}))
		}
		_, fresh, err := shared.GetHelmIndexSnapshotContext(t.Context(), repo)
		require.Error(t, err)
		require.False(t, fresh, "legacy raw data cannot replace the current generation's missing or corrupt body")
	}
}

func TestHelmIndexZeroExpirationDoesNotReuseLegacyFreshness(t *testing.T) {
	shared, _ := newIndexProtocolCache(t)
	shared.helmIndexCacheExpiration = 0
	repo := "https://charts.example.test"
	body := helmIndexContractBody("1.0.0")
	require.NoError(t, shared.cache.SetItem(helmIndexRefsKey(repo), body, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
	record, fresh, err := shared.GetHelmIndexSnapshotContext(t.Context(), repo)
	require.NoError(t, err)
	require.Equal(t, body, record.Body)
	require.False(t, fresh, "zero expiration must require origin validation even for a legacy hit")
	var cached []byte
	require.ErrorIs(t, shared.GetHelmIndex(repo, &cached), ErrCacheMiss)
}

func TestHelmIndexSharedBodyStorageIsBounded(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), ContextTimeoutEnabled: true})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	shared := NewCache(cacheutil.NewCache(cacheutil.NewRedisCache(client, time.Hour, cacheutil.RedisCompressionNone)), time.Hour, time.Minute, time.Minute)
	repo := "https://charts.example.test"
	for version := range 10 {
		require.NoError(t, shared.SetHelmIndexRecord(repo, helmIndexContractBody(strconv.Itoa(version)+".0.0"), "", ""))
		// Evict only the small keys. Losing them must not orphan a large body
		// for every subsequent generation during the retention window.
		for _, key := range server.Keys() {
			if strings.Contains(key, "|record") || strings.Contains(key, "|body-reference") {
				server.Del(key)
			}
		}
		require.Len(t, server.Keys(), 2, "only the current body and its digest may remain after metadata eviction")
	}
}

func TestHelmIndexLocalBodiesStayWarmAcrossReplicasWhenSharingStalls(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	shared, backend := newIndexProtocolCache(t)
	backend.stallBody = true
	url, requests, _, _ := indexProtocolOrigin(t)
	stores := []*helm.IndexStore{
		helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared),
		helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared),
	}
	coldBodyReads := 0
	for build := range 4 {
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		indexProtocolBuild(t, ctx, url, stores[build%2], false)
		cancel()
		require.EqualValues(t, min(build+1, 2), requests.Load(), "a new shared validation of identical bytes must not make an already-warm replica cold")
		if build == 1 {
			coldBodyReads = backend.bodyReads
		} else if build > 1 {
			require.Equal(t, coldBodyReads, backend.bodyReads, "warm builds must avoid full shared-body reads")
		}
	}
	require.Equal(t, 2, backend.stalls, "only the first cold build on each replica attempts body sharing")
	require.Positive(t, coldBodyReads, "exercise missing shared bodies during the cold builds")
}

func TestHelmIndexRecoversCommittedPublicationAfterLostReply(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, mode := range []string{"error", "retry", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			shared, backend := newIndexProtocolCache(t)
			url, requests, _, body := indexProtocolOrigin(t)
			identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
			backend.publicationReply = mode
			store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			indexProtocolBuild(t, ctx, url, store, false)
			cached, err := store.Get(identity)
			require.NoError(t, err, "confirmed committed metadata must warm disk despite a lost reply")
			require.Equal(t, body, cached)
			require.Equal(t, 1, backend.bodyWrites)
			indexProtocolBuild(t, t.Context(), url, store, false)
			require.EqualValues(t, 1, requests.Load(), "the next build must stay warm after successful confirmation")
		})
	}
}

func TestHelmIndexPublicationConfirmationRequiresCurrentMetadata(t *testing.T) {
	for _, mode := range []string{"missing", "newer", "changed-fields"} {
		t.Run(mode, func(t *testing.T) {
			shared, backend := newIndexProtocolCache(t)
			repo := "https://charts.example.test"
			owner, err := shared.BeginHelmIndexFetch(t.Context(), repo)
			require.NoError(t, err)
			require.NotEmpty(t, owner)
			backend.publicationReply = "error"
			backend.afterPublication = func() {
				switch mode {
				case "missing":
					require.NoError(t, shared.cache.SetItem(helmIndexRecordKey(repo), 0, &cacheutil.CacheActionOpts{Delete: true}))
				case "newer":
					require.NoError(t, shared.SetHelmIndexRecord(repo, helmIndexContractBody("2.0.0"), `"two"`, ""))
				case "changed-fields":
					metadata, err := shared.GetHelmIndexMetadataContext(t.Context(), repo)
					require.NoError(t, err)
					metadata.ETag = `"different"`
					require.NoError(t, shared.cache.SetItem(helmIndexRecordKey(repo), metadata, &cacheutil.CacheActionOpts{Expiration: time.Hour}))
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			updated, _ := shared.PublishHelmIndexRecord(ctx, repo, owner, &HelmIndexRecord{Body: helmIndexContractBody("1.0.0"), ETag: `"one"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)})
			require.False(t, updated, "absent or different metadata cannot confirm this publication")
			wantWrites := 0
			if mode == "newer" {
				wantWrites = 1 // Only the maintenance publication shares its body.
			}
			require.Equal(t, wantWrites, backend.bodyWrites)
		})
	}
}

func TestHelmIndexFailedOriginReleasesClaim(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(strconv.FormatBool(canceled), func(t *testing.T) {
			shared, _ := newIndexProtocolCache(t)
			started := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			resume := func() { once.Do(func() { close(release) }) }
			var requests atomic.Int32
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					close(started)
					select {
					case <-r.Context().Done():
					case <-release:
						w.WriteHeader(http.StatusServiceUnavailable)
					}
					return
				}
				_, _ = w.Write(helmIndexContractBody("1.0.0"))
			}))
			t.Cleanup(origin.Close)
			t.Cleanup(resume)
			client := helm.NewClient(origin.URL, helm.HelmCreds{}, false, "", "", helm.WithIndexCache(shared))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			result := make(chan error, 1)
			go func() { _, err := client.GetIndex(ctx, true, 1<<20); result <- err }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				t.Fatal("origin request did not start")
			}
			contender, err := shared.BeginHelmIndexFetch(t.Context(), origin.URL)
			require.NoError(t, err)
			require.Empty(t, contender, "the origin request must hold a live claim before release is tested")
			if canceled {
				cancel()
			} else {
				resume()
			}
			select {
			case err := <-result:
				require.Error(t, err)
			case <-time.After(3 * time.Second):
				t.Fatal("failed origin did not release its request")
			}
			probe, err := shared.BeginHelmIndexFetch(t.Context(), origin.URL)
			require.NoError(t, err)
			require.NotEmpty(t, probe, "failure or cancellation must release the old claim")
			require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), origin.URL, probe))
			_, err = client.GetIndex(t.Context(), true, 1<<20)
			require.NoError(t, err)
			_, err = client.GetIndex(t.Context(), false, 1<<20)
			require.NoError(t, err)
			require.EqualValues(t, 2, requests.Load(), "a successful retry must publish and make the next call warm")
		})
	}
}

func TestHelmIndexClaimCarriesReleaseKey(t *testing.T) {
	shared, _ := newIndexProtocolCache(t)
	owner, err := shared.BeginHelmIndexFetch(t.Context(), "original-repository")
	require.NoError(t, err)
	require.NotEmpty(t, owner)
	// The acquired claim is a capability carrying its resolved Redis key.
	// Cleanup must not need the credential-scope KDF again after LRU eviction.
	require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), "unused-raw-identity", owner))
	next, err := shared.BeginHelmIndexFetch(t.Context(), "original-repository")
	require.NoError(t, err)
	require.NotEmpty(t, next)
	require.NotEqual(t, owner, next)
	// An old capability cannot release a subsequent fetch's claim.
	require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), "original-repository", owner))
	contender, err := shared.BeginHelmIndexFetch(t.Context(), "original-repository")
	require.NoError(t, err)
	require.Empty(t, contender)
	require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), "original-repository", next))
}

func TestHelmIndexRejectedBodyReferenceDoesNotWarmDisk(t *testing.T) {
	requirePinnedHelmForCacheTest(t)
	for _, force := range []bool{false, true} {
		t.Run(strconv.FormatBool(force), func(t *testing.T) {
			shared, backend := newIndexProtocolCache(t)
			url, _, archives, body := indexProtocolOrigin(t)
			identity := url + "\x00" + helm.CredsScope(helm.HelmCreds{})
			newer := bytes.ReplaceAll(body, []byte("version: 1.0.0"), []byte("version: 2.0.0"))
			require.NotEqual(t, body, newer)
			backend.beforeBody = func() {
				require.NoError(t, shared.SetHelmIndexRetainedRecord(identity, &HelmIndexRecord{Body: newer, ETag: `"newer"`, StoredAt: time.Now().UTC().Format(time.RFC3339Nano)}))
			}
			store := helm.NewIndexStore(t.TempDir(), time.Minute, 1<<20, shared)
			indexProtocolBuild(t, t.Context(), url, store, force)
			require.EqualValues(t, 1, archives.Load(), "the successful live response still builds")
			_, err := store.Get(identity)
			require.Error(t, err, "an explicitly rejected reference cannot warm an older local generation")
			record, err := shared.GetHelmIndexRecord(identity)
			require.NoError(t, err)
			require.Equal(t, newer, record.Body)
		})
	}
}

func TestHelmIndexAbandonedClaimAndRenewal(t *testing.T) {
	shared, server := regressionRedisCache(t)
	backend := &indexProtocolBackend{CacheClient: shared.cache.GetClient(), lostClaimReply: true, failedRelease: true}
	shared.cache = cacheutil.NewCache(backend)
	_, err := shared.BeginHelmIndexFetch(t.Context(), "lease-test")
	require.Error(t, err)
	backend.lostClaimReply, backend.failedRelease = false, false
	owner, err := shared.BeginHelmIndexFetch(t.Context(), "lease-test")
	require.NoError(t, err)
	require.Empty(t, owner)
	server.FastForward(helm.IndexFetchLease + time.Second)
	owner, err = shared.BeginHelmIndexFetch(t.Context(), "lease-test")
	require.NoError(t, err)
	require.NotEmpty(t, owner, "an abandoned claim must recover within the short lease")
	// Use the real backend for renewal, retaining its production key encoding.
	shared.cache = cacheutil.NewCache(backend.CacheClient)
	server.FastForward(helm.IndexFetchLease - time.Second)
	renewed, err := shared.RenewHelmIndexFetch(t.Context(), "lease-test", owner)
	require.NoError(t, err)
	require.True(t, renewed)
	server.FastForward(2 * time.Second)
	contender, err := shared.BeginHelmIndexFetch(t.Context(), "lease-test")
	require.NoError(t, err)
	require.Empty(t, contender, "renewal protects a live fetch beyond its original expiry")
	require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), "lease-test", owner))
	next, err := shared.BeginHelmIndexFetch(t.Context(), "lease-test")
	require.NoError(t, err)
	require.NotEmpty(t, next)
	renewed, err = shared.RenewHelmIndexFetch(t.Context(), "lease-test", owner)
	require.NoError(t, err)
	require.False(t, renewed, "a late renewal must not renew a later owner's claim")
	require.NoError(t, shared.ReleaseHelmIndexFetch(t.Context(), "lease-test", next))
}
