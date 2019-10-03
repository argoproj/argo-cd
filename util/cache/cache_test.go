package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

type fixtures struct {
	*Cache
}

func newFixtures() *fixtures {
	return &fixtures{NewCache(NewInMemoryCache(1 * time.Hour))}
}

func TestCacheRevisionMetadata(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetRevisionMetadata("", "")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetRevisionMetadata("foo", "baz", &RevisionMetadata{Message: "foo"})
	assert.NoError(t, err)
	metadata, err := cache.GetRevisionMetadata("foo", "baz")

	assert.NoError(t, err)
	assert.Equal(t, &RevisionMetadata{Message: "foo"}, metadata)
}

func TestCache_Apps(t *testing.T) {
	cache := newFixtures().Cache
	// cach miss
	_, err := cache.ListApps("my-repo-url", "my-revision")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetApps("my-repo-url", "my-revision", map[string]string{"foo": "bar"})
	assert.NoError(t, err)
	apps, err := cache.ListApps("my-repo-url", "my-revision")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"foo": "bar"}, apps)
}

func TestCache_AppManagedResources(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetAppManagedResources("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetAppManagedResources("my-appname", []*ResourceDiff{{Name: "my-name"}})
	assert.NoError(t, err)
	diffs, err := cache.GetAppManagedResources("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, []*ResourceDiff{{Name: "my-name"}}, diffs)
}

func TestCache_AppResourcesTree(t *testing.T) {
	cache := newFixtures().Cache
	// cache miss
	_, err := cache.GetAppResourcesTree("my-appname")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetAppResourcesTree("my-appname", &ApplicationTree{Nodes: []ResourceNode{{}}})
	assert.NoError(t, err)
	tree, err := cache.GetAppResourcesTree("my-appname")
	assert.NoError(t, err)
	assert.Equal(t, &ApplicationTree{Nodes: []ResourceNode{{}}}, tree)
}