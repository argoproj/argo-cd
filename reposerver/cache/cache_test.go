package cache

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Hour)),
		1*time.Minute,
		1*time.Minute,
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
	err := cache.GetManifests("my-revision", &ApplicationSource{}, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// populate cache
	res := &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}
	err = cache.SetManifests("my-revision", &ApplicationSource{}, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", res)
	assert.NoError(t, err)
	// cache miss
	err = cache.GetManifests("other-revision", &ApplicationSource{}, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	err = cache.GetManifests("my-revision", &ApplicationSource{Path: "other-path"}, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	err = cache.GetManifests("my-revision", &ApplicationSource{}, q, "other-namespace", "", "my-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	err = cache.GetManifests("my-revision", &ApplicationSource{}, q, "my-namespace", "", "other-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache miss
	err = cache.GetManifests("my-revision", &ApplicationSource{}, q, "my-namespace", "", "my-app-label-key", "other-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetManifests("my-revision", &ApplicationSource{}, q, "my-namespace", "", "my-app-label-key", "my-app-label-value", value)
	assert.NoError(t, err)
	assert.Equal(t, &CachedManifestResponse{ManifestResponse: &apiclient.ManifestResponse{SourceType: "my-source-type"}}, value)
}

func TestCache_GetAppDetails(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	value := &apiclient.RepoAppDetailsResponse{}
	err := cache.GetAppDetails("my-revision", &ApplicationSource{}, value, "")
	assert.Equal(t, ErrCacheMiss, err)
	res := &apiclient.RepoAppDetailsResponse{Type: "my-type"}
	err = cache.SetAppDetails("my-revision", &ApplicationSource{}, res, "")
	assert.NoError(t, err)
	//cache miss
	err = cache.GetAppDetails("other-revision", &ApplicationSource{}, value, "")
	assert.Equal(t, ErrCacheMiss, err)
	//cache miss
	err = cache.GetAppDetails("my-revision", &ApplicationSource{Path: "other-path"}, value, "")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.GetAppDetails("my-revision", &ApplicationSource{}, value, "")
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
		1*time.Minute,
		1*time.Minute,
	)

	response := apiclient.ManifestResponse{
		Namespace: "default",
		Revision:  "revision",
		Manifests: []string{"sample-text"},
	}
	appSrc := &appv1.ApplicationSource{}
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
	err := repoCache.SetManifests(response.Revision, appSrc, &apiclient.ManifestRequest{}, response.Namespace, "", appKey, appValue, store)
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
	err = repoCache.GetManifests(response.Revision, appSrc, &apiclient.ManifestRequest{}, response.Namespace, "", appKey, appValue, retrievedVal)
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
	err = repoCache.GetManifests(response.Revision, appSrc, &apiclient.ManifestRequest{}, response.Namespace, "", appKey, appValue, retrievedVal)

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
