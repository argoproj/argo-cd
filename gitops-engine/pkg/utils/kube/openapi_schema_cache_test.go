package kube

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/openapi"
)

type fakeOpenAPIGroupVersion string

func (f fakeOpenAPIGroupVersion) Schema(string) ([]byte, error) { return nil, nil }
func (f fakeOpenAPIGroupVersion) ServerRelativeURL() string     { return string(f) }

func TestOpenAPISchemaCache_Index(t *testing.T) {
	t.Parallel()
	now := time.Unix(0, 0)
	cache := NewOpenAPISchemaCache()
	cache.now = func() time.Time { return now }
	index := map[string]openapi.GroupVersion{"apis/apps/v1": fakeOpenAPIGroupVersion("/openapi/v3/apis/apps/v1?hash=1")}

	assert.Nil(t, cache.indexURLs())

	cache.setIndex(index)
	assert.Equal(t, map[string]string{"apis/apps/v1": "/openapi/v3/apis/apps/v1?hash=1"}, cache.indexURLs())

	t.Run("expires after the TTL", func(t *testing.T) {
		now = now.Add(openAPIIndexTTL - time.Second)
		assert.NotNil(t, cache.indexURLs())
		now = now.Add(time.Second)
		assert.Nil(t, cache.indexURLs())
	})

	t.Run("an index fetched right after an invalidation expires when the API server settles", func(t *testing.T) {
		cache.setIndex(index)
		cache.Invalidate()
		assert.Nil(t, cache.indexURLs())

		now = now.Add(time.Second)
		cache.setIndex(index)
		assert.NotNil(t, cache.indexURLs())
		now = now.Add(openAPIIndexSettle - time.Second)
		assert.Nil(t, cache.indexURLs())

		cache.setIndex(index)
		now = now.Add(openAPIIndexTTL - time.Second)
		assert.NotNil(t, cache.indexURLs())
	})

	t.Run("a nil cache can be invalidated", func(t *testing.T) {
		var nilCache *OpenAPISchemaCache
		nilCache.Invalidate()
	})
}
