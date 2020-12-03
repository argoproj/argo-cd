package cache

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/hash"

	log "github.com/sirupsen/logrus"
)

var ErrCacheMiss = cacheutil.ErrCacheMiss

type Cache struct {
	cache               *cacheutil.Cache
	repoCacheExpiration time.Duration
}

func NewCache(cache *cacheutil.Cache, repoCacheExpiration time.Duration) *Cache {
	return &Cache{cache, repoCacheExpiration}
}

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	var repoCacheExpiration time.Duration

	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", 24*time.Hour, "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")

	repoFactory := cacheutil.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := repoFactory()
		if err != nil {
			return nil, err
		}
		return NewCache(cache, repoCacheExpiration), nil
	}
}

func appSourceKey(appSrc *appv1.ApplicationSource) uint32 {
	appSrc = appSrc.DeepCopy()
	if !appSrc.IsHelm() {
		appSrc.RepoURL = ""        // superceded by commitSHA
		appSrc.TargetRevision = "" // superceded by commitSHA
	}
	appSrcStr, _ := json.Marshal(appSrc)
	return hash.FNVa(string(appSrcStr))
}

func listApps(repoURL, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repoURL, revision)
}

func (c *Cache) ListApps(repoUrl, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.cache.GetItem(listApps(repoUrl, revision), &res)
	return res, err
}

func (c *Cache) SetApps(repoUrl, revision string, apps map[string]string) error {
	return c.cache.SetItem(listApps(repoUrl, revision), apps, c.repoCacheExpiration, apps == nil)
}

func manifestCacheKey(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string) string {
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d", appLabelKey, appLabelValue, revision, namespace, appSourceKey(appSrc))
}

func (c *Cache) GetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res *CachedManifestResponse) error {
	err := c.cache.GetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res)

	if err != nil {
		return err
	}

	hash, err := res.generateCacheEntryHash()
	if err != nil {
		return fmt.Errorf("Unable to generate hash value: %s", err)
	}

	// If the expected hash of the cache entry does not match the actual hash value...
	if hash != res.CacheEntryHash {
		log.Warnf("Manifest hash did not match expected value, treating as a cache miss: %s", appLabelValue)

		err = c.DeleteManifests(revision, appSrc, namespace, appLabelKey, appLabelValue)
		if err != nil {
			return fmt.Errorf("Unable to delete manifest after hash mismatch, %v", err)
		}

		// Treat hash mismatches as cache misses, so that the underlying resource is reacquired
		return ErrCacheMiss
	}

	// The expected hash matches the actual hash, so remove the hash from the returned value
	res.CacheEntryHash = ""

	return nil
}

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res *CachedManifestResponse) error {

	// Generate and apply the cache entry hash, before writing
	if res != nil {
		res = res.shallowCopy()
		hash, err := res.generateCacheEntryHash()
		if err != nil {
			return fmt.Errorf("Unable to generate hash value: %s", err)
		}
		res.CacheEntryHash = hash
	}

	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) DeleteManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string) error {
	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), "", c.repoCacheExpiration, true)
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource) string {
	return fmt.Sprintf("appdetails|%s|%d", revision, appSourceKey(appSrc))
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, res *apiclient.RepoAppDetailsResponse) error {
	return c.cache.GetItem(appDetailsCacheKey(revision, appSrc), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, res *apiclient.RepoAppDetailsResponse) error {
	return c.cache.SetItem(appDetailsCacheKey(revision, appSrc), res, c.repoCacheExpiration, res == nil)
}

func revisionMetadataKey(repoURL, revision string) string {
	return fmt.Sprintf("revisionmetadata|%s|%s", repoURL, revision)
}

func (c *Cache) GetRevisionMetadata(repoURL, revision string) (*appv1.RevisionMetadata, error) {
	item := &appv1.RevisionMetadata{}
	return item, c.cache.GetItem(revisionMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetRevisionMetadata(repoURL, revision string, item *appv1.RevisionMetadata) error {
	return c.cache.SetItem(revisionMetadataKey(repoURL, revision), item, c.repoCacheExpiration, false)
}

func (cmr *CachedManifestResponse) shallowCopy() *CachedManifestResponse {
	if cmr == nil {
		return nil
	}

	return &CachedManifestResponse{
		CacheEntryHash:                  cmr.CacheEntryHash,
		FirstFailureTimestamp:           cmr.FirstFailureTimestamp,
		ManifestResponse:                cmr.ManifestResponse,
		MostRecentError:                 cmr.MostRecentError,
		NumberOfCachedResponsesReturned: cmr.NumberOfCachedResponsesReturned,
		NumberOfConsecutiveFailures:     cmr.NumberOfConsecutiveFailures,
	}
}

func (cmr *CachedManifestResponse) generateCacheEntryHash() (string, error) {

	// Copy, then remove the old hash
	copy := cmr.shallowCopy()
	copy.CacheEntryHash = ""

	// Hash the JSON representation into a base-64-encoded FNV 64a (we don't need a cryptographic hash algorithm, since this is only for detecting data corruption)
	bytes, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	h := fnv.New64a()
	_, err = h.Write(bytes)
	if err != nil {
		return "", err
	}
	fnvHash := h.Sum(nil)
	return base64.URLEncoding.EncodeToString(fnvHash), nil

}

// CachedManifestResponse represents a cached result of a previous manifest generation operation, including the caching
// of a manifest generation error, plus additional information on previous failures
type CachedManifestResponse struct {

	// NOTE: When adding fields to this struct, you MUST also update shallowCopy()

	CacheEntryHash                  string                      `json:"cacheEntryHash"`
	ManifestResponse                *apiclient.ManifestResponse `json:"manifestResponse"`
	MostRecentError                 string                      `json:"mostRecentError"`
	FirstFailureTimestamp           int64                       `json:"firstFailureTimestamp"`
	NumberOfConsecutiveFailures     int                         `json:"numberOfConsecutiveFailures"`
	NumberOfCachedResponsesReturned int                         `json:"numberOfCachedResponsesReturned"`
}
