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

type OIDCState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

// NewCache creates new instance of Cache
func NewCache(
	cacheClient CacheClient,
	connectionStatusCacheExpiration time.Duration,
	appStateCacheExpiration time.Duration,
	repoCacheExpiration time.Duration,
	oidcCacheExpiration time.Duration,
) *Cache {
	return &Cache{
		client:                          cacheClient,
		connectionStatusCacheExpiration: connectionStatusCacheExpiration,
		appStateCacheExpiration:         appStateCacheExpiration,
		repoCacheExpiration:             repoCacheExpiration,
		oidcCacheExpiration:             oidcCacheExpiration,
	}
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0
	var defaultCacheExpiration time.Duration
	var connectionStatusCacheExpiration time.Duration
	var appStateCacheExpiration time.Duration
	var repoCacheExpiration time.Duration
	var oidcCacheExpiration time.Duration

	cmd.Flags().StringVar(&redisAddress, "redis", "", "Redis server hostname and port (e.g. argocd-redis:6379). ")
	cmd.Flags().IntVar(&redisDB, "redisdb", 0, "Redis database.")
	cmd.Flags().StringArrayVar(&sentinelAddresses, "sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	cmd.Flags().StringVar(&sentinelMaster, "sentinelmaster", "master", "Redis sentinel master group name.")
	cmd.Flags().DurationVar(&defaultCacheExpiration, "default-cache-expiration", 24*time.Hour, "Cache expiration default")
	cmd.Flags().DurationVar(&connectionStatusCacheExpiration, "connection-status-cache-expiration", 1*time.Hour, "Cache expiration for cluster/repo connection status")
	cmd.Flags().DurationVar(&appStateCacheExpiration, "app-state-cache-expiration", 1*time.Hour, "Cache expiration for app state")
	cmd.Flags().DurationVar(&repoCacheExpiration, "repo-cache-expiration", 24*time.Hour, "Cache expiration for repo state, incl. app lists, app details, manifest generation, revision meta-data")
	cmd.Flags().DurationVar(&oidcCacheExpiration, "oidc-cache-expiration", 3*time.Minute, "Cache expiration for OIDC state")
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
				connectionStatusCacheExpiration,
				appStateCacheExpiration,
				repoCacheExpiration,
				oidcCacheExpiration,
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
			connectionStatusCacheExpiration,
			appStateCacheExpiration,
			repoCacheExpiration,
			oidcCacheExpiration,
		), nil
	}
}

// Cache provides strongly types methods to store and retrieve values from shared cache
type Cache struct {
	client                          CacheClient
	appStateCacheExpiration         time.Duration
	connectionStatusCacheExpiration time.Duration
	repoCacheExpiration             time.Duration
	oidcCacheExpiration             time.Duration
}

func appManagedResourcesKey(appName string) string {
	return fmt.Sprintf("app|managed-resources|%s", appName)
}

func appResourcesTreeKey(appName string) string {
	return fmt.Sprintf("app|resources-tree|%s", appName)
}

func clusterConnectionStateKey(server string) string {
	return fmt.Sprintf("cluster|%s|connection-state", server)
}

func repoConnectionStateKey(repo string) string {
	return fmt.Sprintf("repo|%s|connection-state", repo)
}

func listApps(repoURL, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repoURL, revision)
}

func oidcStateKey(key string) string {
	return fmt.Sprintf("oidc|%s", key)
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

func (c *Cache) setItem(key string, item interface{}, expiration time.Duration, delete bool) error {
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	if delete {
		return c.client.Delete(key)
	} else {
		return c.client.Set(&Item{Object: item, Key: key, Expiration: expiration})
	}
}

func (c *Cache) getItem(key string, item interface{}) error {
	key = fmt.Sprintf("%s|%s", key, common.CacheVersion)
	return c.client.Get(key, item)
}

func (c *Cache) GetAppManagedResources(appName string) ([]*appv1.ResourceDiff, error) {
	res := make([]*appv1.ResourceDiff, 0)
	err := c.getItem(appManagedResourcesKey(appName), &res)
	return res, err
}

func (c *Cache) SetAppManagedResources(appName string, managedResources []*appv1.ResourceDiff) error {
	return c.setItem(appManagedResourcesKey(appName), managedResources, c.appStateCacheExpiration, managedResources == nil)
}

func (c *Cache) GetAppResourcesTree(appName string) (*appv1.ApplicationTree, error) {
	var res *appv1.ApplicationTree
	err := c.getItem(appResourcesTreeKey(appName), &res)
	return res, err
}

func (c *Cache) SetAppResourcesTree(appName string, resourcesTree *appv1.ApplicationTree) error {
	return c.setItem(appResourcesTreeKey(appName), resourcesTree, c.appStateCacheExpiration, resourcesTree == nil)
}

func (c *Cache) GetClusterConnectionState(server string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.getItem(clusterConnectionStateKey(server), &res)
	return res, err
}

func (c *Cache) SetClusterConnectionState(server string, state *appv1.ConnectionState) error {
	return c.setItem(clusterConnectionStateKey(server), &state, c.connectionStatusCacheExpiration, state == nil)
}

func (c *Cache) GetRepoConnectionState(repo string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.getItem(repoConnectionStateKey(repo), &res)
	return res, err
}

func (c *Cache) SetRepoConnectionState(repo string, state *appv1.ConnectionState) error {
	return c.setItem(repoConnectionStateKey(repo), &state, c.connectionStatusCacheExpiration, state == nil)
}
func (c *Cache) ListApps(repoUrl, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.getItem(listApps(repoUrl, revision), &res)
	return res, err
}

func (c *Cache) SetApps(repoUrl, revision string, apps map[string]string) error {
	return c.setItem(listApps(repoUrl, revision), apps, c.repoCacheExpiration, apps == nil)
}

func (c *Cache) GetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.getItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res)
}

func (c *Cache) SetManifests(revision string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.setItem(manifestCacheKey(revision, appSrc, namespace, appLabelKey, appLabelValue), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) GetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
	return c.getItem(appDetailsCacheKey(revision, appSrc), res)
}

func (c *Cache) SetAppDetails(revision string, appSrc *appv1.ApplicationSource, res interface{}) error {
	return c.setItem(appDetailsCacheKey(revision, appSrc), res, c.repoCacheExpiration, res == nil)
}

func (c *Cache) GetRevisionMetadata(repoURL, revision string) (*appv1.RevisionMetadata, error) {
	item := &appv1.RevisionMetadata{}
	return item, c.getItem(revisionMetadataKey(repoURL, revision), item)
}

func (c *Cache) SetRevisionMetadata(repoURL, revision string, item *appv1.RevisionMetadata) error {
	return c.setItem(revisionMetadataKey(repoURL, revision), item, c.repoCacheExpiration, false)
}

func (c *Cache) GetOIDCState(key string) (*OIDCState, error) {
	res := OIDCState{}
	err := c.getItem(oidcStateKey(key), &res)
	return &res, err
}

func (c *Cache) SetOIDCState(key string, state *OIDCState) error {
	return c.setItem(oidcStateKey(key), state, c.oidcCacheExpiration, state == nil)
}
