package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hash"
)

const (
	// envRedisPassword is a env variable name which stores redis password
	envRedisPassword = "REDIS_PASSWORD"
)

// NewCache creates new instance of Cache
func NewCache(
	cacheClient CacheClient,
	repoCacheExpiration time.Duration,
) *Cache {
	return &Cache{
		client:                          cacheClient,
		repoCacheExpiration:             repoCacheExpiration,
	}
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0
	var defaultCacheExpiration time.Duration
	var repoCacheExpiration time.Duration

	cmd.Flags().StringVar(&redisAddress, "redis", "", "Redis server hostname and port (e.g. argocd-redis:6379). ")
	cmd.Flags().IntVar(&redisDB, "redisdb", 0, "Redis database.")
	cmd.Flags().StringArrayVar(&sentinelAddresses, "sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	cmd.Flags().StringVar(&sentinelMaster, "sentinelmaster", "master", "Redis sentinel master group name.")
	cmd.Flags().DurationVar(&defaultCacheExpiration, "default-cache-expiration", 24*time.Hour, "Cache expiration default")
	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", 24*time.Hour, "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")
	return func() (*Cache, error) {
		password := os.Getenv(envRedisPassword)
		if len(sentinelAddresses) > 0 {
			client := redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:    sentinelMaster,
				SentinelAddrs: sentinelAddresses,
				DB:            redisDB,
				Password:      password,
			})
			return NewCache(
				NewRedisCache(client, defaultCacheExpiration),
				repoCacheExpiration,
			), nil
		}

		if redisAddress == "" {
			redisAddress = common.DefaultRedisAddr
		}
		client := redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: password,
			DB:       redisDB,
		})
		return NewCache(
			NewRedisCache(client, defaultCacheExpiration),
			repoCacheExpiration,
		), nil
	}
}

// Cache provides strongly types methods to store and retrieve values from shared cache
type Cache struct {
	client                          CacheClient
	repoCacheExpiration             time.Duration
}

func listApps(repoURL, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repoURL, revision)
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

func manifestCacheKey(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string) string {
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d", appLabelKey, appLabelValue, revision, namespace, appSourceKey(appSrc))
}

func appDetailsCacheKey(revision string, appSrc *appv1.ApplicationSource) string {
	return fmt.Sprintf("appdetails|%s|%d", revision, appSourceKey(appSrc))
}

func revisionMetadataKey(repoURL, revision string) string {
	return fmt.Sprintf("revisionmetadata|%s|%s", repoURL, revision)
}

func (c *Cache) SetItem(key string, item interface{}, expiration time.Duration, delete bool) error {
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	if delete {
		return c.client.Delete(key)
	} else {
		return c.client.Set(&Item{Object: item, Key: key, Expiration: expiration})
	}
}

func (c *Cache) GetItem(key string, item interface{}) error {
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	return c.client.Get(key, item)
}

func (c *Cache) ListApps(repoUrl, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.GetItem(listApps(repoUrl, revision), &res)
	return res, err
}

func (c *Cache) SetApps(repoUrl, revision string, apps map[string]string) error {
	return c.SetItem(listApps(repoUrl, revision), apps, c.repoCacheExpiration, apps == nil)
}

func (c *Cache) GetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.GetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res)
}

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.SetItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
	return c.GetItem(appDetailsCacheKey(revision, appSrc), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
	return c.SetItem(appDetailsCacheKey(revision, appSrc), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) GetRevisionMetadata(repoURL, revision string) (*appv1.RevisionMetadata, error) {
	item := &appv1.RevisionMetadata{}
	return item, c.GetItem(revisionMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetRevisionMetadata(repoURL, revision string, item *appv1.RevisionMetadata) error {
	return c.SetItem(revisionMetadataKey(repoURL, revision), item, c.repoCacheExpiration, false)
}
