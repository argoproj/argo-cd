package helm

import (
	"crypto/sha256"
	"sync"
	"time"

	"github.com/golang/groupcache/lru"
	"golang.org/x/sync/singleflight"
)

// Keep only successful body digests, never the bodies or their freshness and
// authentication metadata. Every read hashes its bytes, so a changed/corrupt
// body cannot reuse validation through matching timestamps or file sizes.
var validatedIndexBodies = newIndexValidationCache(256, parseIndexBody)

// Decode the pinned Helm index's complete schema without importing Helm's
// command dependencies. Semantic chart validation remains Helm's responsibility.
type indexValidationChart struct {
	*IndexValidationMetadata
	URLs                                         []string
	Created                                      time.Time
	Removed                                      bool
	Digest, Checksum, Engine, TillerVersion, URL string
}

// IndexValidationMetadata is exported so encoding/json can initialize the
// embedded pointer, matching the pinned Helm decoder's treatment of scalars.
type IndexValidationMetadata struct {
	Name, Home, Version, Description, Icon, APIVersion, Condition, Tags, AppVersion, KubeVersion, Type string
	Sources, Keywords                                                                                  []string
	Deprecated                                                                                         bool
	Annotations                                                                                        map[string]string
	Maintainers                                                                                        []*struct{ Name, Email, URL string }
	Dependencies                                                                                       []*struct {
		Name, Version, Repository, Condition, Alias string
		Tags                                        []string
		Enabled                                     bool
		ImportValues                                []any `json:"import-values"`
	}
}

func validateIndexBody(data []byte) error {
	return validatedIndexBodies.validateBody(data)
}

type indexValidationCache struct {
	mu       sync.Mutex
	valid    *lru.Cache
	pending  singleflight.Group
	validate func([]byte) error
}

func newIndexValidationCache(limit int, validate func([]byte) error) *indexValidationCache {
	return &indexValidationCache{valid: lru.New(limit), validate: validate}
}

func (c *indexValidationCache) contains(digest [sha256.Size]byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.valid.Get(digest)
	return ok
}

func (c *indexValidationCache) validateBody(body []byte) error {
	digest := sha256.Sum256(body)
	return c.validateBodyDigest(body, digest)
}

// The caller computes this digest from the bytes, never from cache metadata.
func (c *indexValidationCache) validateBodyDigest(body []byte, digest [sha256.Size]byte) error {
	if c.contains(digest) {
		return nil
	}
	// Coalesce identical cold bodies without holding the cache mutex while
	// parsing. Other warm bodies remain usable during an expensive validation.
	_, err, _ := c.pending.Do(string(digest[:]), func() (any, error) {
		if c.contains(digest) {
			return nil, nil
		}
		if err := c.validate(body); err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.valid.Add(digest, struct{}{})
		c.mu.Unlock()
		return nil, nil
	})
	return err
}
