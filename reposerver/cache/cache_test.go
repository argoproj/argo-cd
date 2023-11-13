package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
		&CacheOpts{
			RepoCacheExpiration:           1 * time.Minute,
			RevisionCacheExpiration:       1 * time.Minute,
			RevisionCacheLockWaitEnabled:  true,
			RevisionCacheLockTimeout:      30 * time.Second,
			RevisionCacheLockWaitInterval: 1 * time.Second,
		},
	)}
}

func TestCache_GetRevisionMetadata(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetRevisionMetadata("my-repo-url", "my-revision", &RevisionMetadata{Message: "my-message"})
	assert.NoError(t, err)
	// cache miss
	_, err = cache.GetRevisionMetadata("other-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	_, err = cache.GetRevisionMetadata("my-repo-url", "other-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.GetRevisionMetadata("my-repo-url", "my-revision")
	assert.NoError(t, err)
	assert.Equal(t, &RevisionMetadata{Message: "my-message"}, value)
}

func TestCache_ListApps(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.ListApps("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	err = cache.SetApps("my-repo-url", "my-revision", map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	// cache miss
	_, err = cache.ListApps("other-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	_, err = cache.ListApps("my-repo-url", "other-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	value, err := cache.ListApps("my-repo-url", "my-revision")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, value)
}

func TestCache_GetManifests(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	q := &apiclient.ManifestRequest{}
	value := &CachedManifestResponse{}
	err := cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	res := &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}
	err = cache.SetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", res, nil)
	assert.NoError(t, err)
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
		err = cache.GetManifests("my-revision", &ApplicationSource{}, q.RefSources, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value, nil)
		assert.NoError(t, err)
		assert.Equal(t, &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}, value)
	})
}

func TestCache_GetAppDetails(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	value := &apiclient.RepoAppDetailsResponse{}
	emptyRefSources := map[string]*RefTarget{}
	err := cache.GetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	res := &apiclient.RepoAppDetailsResponse{Type: "my-type"}
	err = cache.SetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, res, "", nil)
	assert.NoError(t, err)
	//cache miss
	err = cache.GetAppDetails("other-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	//cache miss
	err = cache.GetAppDetails("my-revision", &ApplicationSource{Path: "other-path"}, emptyRefSources, value, "", nil)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetAppDetails("my-revision", &ApplicationSource{}, emptyRefSources, value, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, &apiclient.RepoAppDetailsResponse{Type: "my-type"}, value)
}

func TestAddCacheFlagsToCmd(t *testing.T) {
	cache, err := AddCacheFlagsToCmd(&cobra.Command{})()
	assert.NoError(t, err)
	assert.Equal(t, 24*time.Hour, cache.repoCacheExpiration)
}

func TestCachedManifestResponse_HashBehavior(t *testing.T) {

	inMemCache := cacheutil.NewInMemoryCache(1 * time.Hour)

	repoCache := NewCache(
		cacheutil.NewCache(inMemCache),
		&CacheOpts{
			RepoCacheExpiration:           1 * time.Minute,
			RevisionCacheExpiration:       1 * time.Minute,
			RevisionCacheLockWaitEnabled:  true,
			RevisionCacheLockTimeout:      10 * time.Second,
			RevisionCacheLockWaitInterval: 1 * time.Second,
		},
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

		assert.Equal(t, len(items), 1)

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

	assert.True(t, err == cacheutil.ErrCacheMiss)

	// Verify that the hash mismatch item has been deleted
	items := getInMemoryCacheContents(t, inMemCache)
	assert.Equal(t, len(items), 0)

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

	expectedFields := []string{"cacheEntryHash", "manifestResponse", "mostRecentError", "firstFailureTimestamp",
		"numberOfConsecutiveFailures", "numberOfCachedResponsesReturned"}

	assert.Equal(t, len(jsonMap), len(expectedFields))

	// If this test failed, you probably also forgot to update CachedManifestResponse.shallowCopy(), so
	// go do that first :)

	for _, expectedField := range expectedFields {
		assert.Truef(t, strings.Contains(string(str), "\""+expectedField+"\""), "Missing field: %s", expectedField)
	}

}

func TestGitRefCacheItemToReferences_DataChecks(t *testing.T) {
	references := GitRefCacheItemToReferences(nil)
	assert.Equal(t, 0, len(references), "No data should be handled gracefully by returning an empty slice")
	references = GitRefCacheItemToReferences([][2]string{{"", ""}})
	assert.Equal(t, 0, len(references), "Empty data should be discarded")
	references = GitRefCacheItemToReferences([][2]string{{"test", ""}})
	assert.Equal(t, 1, len(references), "Just the key being set should not be discarded")
	assert.Equal(t, "test", references[0].Name().String(), "Name should be set and equal test")
	references = GitRefCacheItemToReferences([][2]string{{"", "ref: test1"}})
	assert.Equal(t, 1, len(references), "Just the value being set should not be discarded")
	assert.Equal(t, "test1", references[0].Target().String(), "Target should be set and equal test1")
	references = GitRefCacheItemToReferences([][2]string{{"test2", "ref: test2"}})
	assert.Equal(t, 1, len(references), "Valid data is should be preserved")
	assert.Equal(t, "test2", references[0].Name().String(), "Name should be set and equal test2")
	assert.Equal(t, "test2", references[0].Target().String(), "Target should be set and equal test2")
	references = GitRefCacheItemToReferences([][2]string{{"test3", "ref: test3"}, {"test4", "ref: test4"}})
	assert.Equal(t, 2, len(references), "Valid data is should be preserved")
	assert.Equal(t, "test3", references[0].Name().String(), "Name should be set and equal test3")
	assert.Equal(t, "test3", references[0].Target().String(), "Target should be set and equal test3")
	assert.Equal(t, "test4", references[1].Name().String(), "Name should be set and equal test4")
	assert.Equal(t, "test4", references[1].Target().String(), "Target should be set and equal test4")
}

func TestTryLockGitRefCache_OwnershipFlows(t *testing.T) {
	cache := newFixtures().Cache
	// Test setting the lock
	cache.TryLockGitRefCache("my-repo-url", "my-lock-id")
	var output [][2]string
	key := fmt.Sprintf("git-refs|%s", "my-repo-url")
	err := cache.cache.GetItem(key, &output)
	assert.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should be set")
	assert.Equal(t, "my-lock-id", output[0][1], "The lock should be set to the provided lock id")
	// Test not being able to overwrite the lock
	cache.TryLockGitRefCache("my-repo-url", "other-lock-id")
	err = cache.cache.GetItem(key, &output)
	assert.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should not have changed")
	assert.Equal(t, "my-lock-id", output[0][1], "The lock should not have changed")
	// Test can overwrite once there is nothing set
	err = cache.cache.SetItem(key, [][2]string{}, 0, true)
	assert.NoError(t, err)
	cache.TryLockGitRefCache("my-repo-url", "other-lock-id")
	err = cache.cache.GetItem(key, &output)
	assert.NoError(t, err)
	assert.Equal(t, "locked", output[0][0], "The lock should be set")
	assert.Equal(t, "other-lock-id", output[0][1], "The lock id should have changed to other-lock-id")
}
