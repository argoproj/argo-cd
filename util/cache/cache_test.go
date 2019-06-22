package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestCacheRevisionMetadata(t *testing.T) {
	cache := NewCache(NewInMemoryCache(1 * time.Hour))
	// cache miss
	_, err := cache.GetRevisionMetadata("", "")
	assert.Equal(t, ErrCacheMiss, err)
	// cache hit
	err = cache.SetRevisionMetadata("foo", "bar", &v1alpha1.RevisionMetadata{Message: "foo"})
	assert.NoError(t, err)
	metadata, err := cache.GetRevisionMetadata("foo", "bar")
	assert.NoError(t, err)
	assert.Equal(t, &v1alpha1.RevisionMetadata{Message: "foo"}, metadata)
}
