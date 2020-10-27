package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/hash"
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
	return c.cache.GetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res)
}

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res *CachedManifestResponse) error {
	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) DeleteManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string) error {
	return c.cache.SetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), "", c.repoCacheExpiration, true)
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource) string {
	return fmt.Sprintf("appdetails|%s|%d", revision, appSourceKey(appSrc))
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
	return c.cache.GetItem(appDetailsCacheKey(revision, appSrc), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
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

// CachedManifestResponse represents a cached result of a previous manifest generation operation, including the caching
// of a manifest generation error, plus additional information on previous failures
type CachedManifestResponse struct {
	ManifestResponse                *apiclient.ManifestResponse
	MostRecentError                 string
	FirstFailureTimestamp           int64
	NumberOfConsecutiveFailures     int
	NumberOfCachedResponsesReturned int
}
