package cache

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/image-updater/tag"
	memcache "github.com/patrickmn/go-cache"
)

type MemCache struct {
	cache *memcache.Cache
}

// NewMemCache returns a new instance of MemCache
func NewMemCache() ImageTagCache {
	mc := MemCache{}
	c := memcache.New(0, 0)
	mc.cache = c
	return &mc
}

// HasTag returns true if cache has entry for given tag, false if not
func (mc *MemCache) HasTag(imageName string, tagName string) bool {
	tag, err := mc.GetTag(imageName, tagName)
	if err != nil || tag == nil {
		return false
	} else {
		return true
	}
}

// SetTag sets a tag entry into the cache
func (mc *MemCache) SetTag(imageName string, imgTag *tag.ImageTag) {
	mc.cache.Set(tagCacheKey(imageName, imgTag.TagName), *imgTag, -1)
}

// GetTag gets a tag entry from the cache
func (mc *MemCache) GetTag(imageName string, tagName string) (*tag.ImageTag, error) {
	var imgTag tag.ImageTag
	e, ok := mc.cache.Get(tagCacheKey(imageName, tagName))
	if !ok {
		return nil, nil
	}
	imgTag, ok = e.(tag.ImageTag)
	if !ok {
		return nil, fmt.Errorf("")
	}
	return &imgTag, nil
}

func (mc *MemCache) SetImage(imageName, application string) {
	mc.cache.Set(imageCacheKey(imageName), application, -1)
}

// ClearCache clears the cache
func (mc *MemCache) ClearCache() {
	for k := range mc.cache.Items() {
		mc.cache.Delete(k)
	}
}

// NumEntries returns the number of entries in the cache
func (mc *MemCache) NumEntries() int {
	return mc.cache.ItemCount()
}

func tagCacheKey(imageName, imageTag string) string {
	return fmt.Sprintf("tags:%s:%s", imageName, imageTag)
}

func imageCacheKey(imageName string) string {
	return fmt.Sprintf("image:%s", imageName)
}
