package cache

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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

func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	var repoCacheExpiration time.Duration

	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", 24*time.Hour, "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")

	repoFactory := cacheutil.AddCacheFlagsToCmd(cmd)

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

func manifestCacheKey(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, additionalHelmValues string) string {
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d|%d", appLabelKey, appLabelValue, commitSHA, namespace, appSourceKey(appSrc), hash.FNVa(additionalHelmValues))
}

func (c *Cache) GetManifests(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, additionalHelmValues string, res interface{}) error {
	return c.cache.GetItem(manifestCacheKey(commitSHA, appSrc, namespace, appLabelKey, appLabelValue, additionalHelmValues), res)
}

func (c *Cache) SetManifests(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, additionalHelmValues string, res interface{}) error {
	return c.cache.SetItem(manifestCacheKey(commitSHA, appSrc, namespace, appLabelKey, appLabelValue, additionalHelmValues), res, c.repoCacheExpiration, res == nil)
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
