package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(NewInMemoryCache(1 * time.Hour))}
}

func TestCache_GetRevisionMetadata(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetRevisionMetadata("", "")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetRevisionMetadata("foo", "baz", &RevisionMetadata{Message: "foo"})
	assert.NoError(t, err)
	value, err := cache.GetRevisionMetadata("foo", "baz")
	assert.NoError(t, err)
	assert.Equal(t, &RevisionMetadata{Message: "foo"}, value)
}

func TestCache_ListApps(t *testing.T) {
	cache := newFixtures().Cache
	// cach miss
	_, err := cache.ListApps("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetApps("my-repo-url", "my-revision", map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	value, err := cache.ListApps("my-repo-url", "my-revision")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, value)
}

func TestCache_GetAppManagedResources(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetAppManagedResources("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetAppManagedResources("my-appname", []*ResourceDiff{{Name: "my-name"}})
	assert.NoError(t, err)
	value, err := cache.GetAppManagedResources("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, []*ResourceDiff{{Name: "my-name"}}, value)
}

func TestCache_GetAppResourcesTree(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetAppResourcesTree("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetAppResourcesTree("my-appname", &ApplicationTree{Nodes: []ResourceNode{{}}})
	assert.NoError(t, err)
	value, err := cache.GetAppResourcesTree("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, &ApplicationTree{Nodes: []ResourceNode{{}}}, value)
}

func TestCache_GetClusterConnectionState(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetClusterConnectionState("my-server")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetClusterConnectionState("my-server", &ConnectionState{Status: "my-state"})
	assert.NoError(t, err)
	value, err := cache.GetClusterConnectionState("my-server")
	assert.NoError(t, err)
	assert.Equal(t, ConnectionState{Status: "my-state"}, value)
}

func TestCache_GetRepoConnectionState(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetRepoConnectionState("my-repo")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetRepoConnectionState("my-repo", &ConnectionState{Status: "my-state"})
	assert.NoError(t, err)
	value, err := cache.GetRepoConnectionState("my-repo")
	assert.NoError(t, err)
	assert.Equal(t, ConnectionState{Status: "my-state"}, value)
}

func TestCache_GetManifests(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	value := &apiclient.ManifestResponse{}
	err := cache.GetManifests("my-revision", &ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", value)
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	res := &apiclient.ManifestResponse{SourceType: "my-source-type"}
	err = cache.SetManifests("my-revision", &ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", res)
	assert.NoError(t, err)
	err = cache.GetManifests("my-revision", &ApplicationSource{}, "my-namespace", "my-app-label-key", "my-app-label-value", value)
	assert.NoError(t, err)
	assert.Equal(t, &apiclient.ManifestResponse{SourceType: "my-source-type"}, value)
}
