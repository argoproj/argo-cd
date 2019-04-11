package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/hash"
)

const (
	defaultCacheExpiration          = 24 * time.Hour
	connectionStatusCacheExpiration = 1 * time.Hour
	appStateCacheExpiration         = 1 * time.Hour
	repoCacheExpiration             = 24 * time.Hour
	oidcCacheExpiration             = 3 * time.Minute

	// envRedisPassword is a env variable name which stores redis password
	envRedisPassword = "REDIS_PASSWORD"
)

type OIDCState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

// NewCache creates new instance of Cache
func NewCache(cacheClient CacheClient) *Cache {
	return &Cache{client: cacheClient}
}

// AddCacheFlagsToCmd adds flags which control caching to the specified command
func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	redisAddress := ""
	sentinelAddresses := make([]string, 0)
	sentinelMaster := ""
	redisDB := 0

	cmd.Flags().StringVar(&redisAddress, "redis", "", "Redis server hostname and port (e.g. argocd-redis:6379). ")
	cmd.Flags().IntVar(&redisDB, "redisdb", 0, "Redis database.")
	cmd.Flags().StringArrayVar(&sentinelAddresses, "sentinel", []string{}, "Redis sentinel hostname and port (e.g. argocd-redis-ha-announce-0:6379). ")
	cmd.Flags().StringVar(&sentinelMaster, "sentinelmaster", "master", "Redis sentinel master group name.")
	return func() (*Cache, error) {
		password := os.Getenv(envRedisPassword)
		if len(sentinelAddresses) > 0 {
			client := redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:    sentinelMaster,
				SentinelAddrs: sentinelAddresses,
				DB:            redisDB,
				Password:      password,
			})
			return NewCache(NewRedisCache(client, defaultCacheExpiration)), nil
		}

		if redisAddress == "" {
			redisAddress = common.DefaultRedisAddr
		}
		client := redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: password,
			DB:       redisDB,
		})
		return NewCache(NewRedisCache(client, defaultCacheExpiration)), nil
	}
}

// Cache provides strongly types methods to store and retrieve values from shared cache
type Cache struct {
	client CacheClient
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

func appTemplatesKey(repo, revision string) string {
	return fmt.Sprintf("ldir|%s|%s", repo, revision)
}

func appTemplateKey(repo, path, revision string) string {
	return fmt.Sprintf("gfile|%s|%s|%s", repo, path, revision)
}

func oidcStateKey(key string) string {
	return fmt.Sprintf("oidc|%s", key)
}

func manifestCacheKey(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string) string {
	appSrc = appSrc.DeepCopy()
	appSrc.RepoURL = ""        // superceded by commitSHA
	appSrc.TargetRevision = "" // superceded by commitSHA
	appSrcStr, _ := json.Marshal(appSrc)
	fnva := hash.FNVa(string(appSrcStr))
	return fmt.Sprintf("mfst|%s|%s|%s|%s|%d", appLabelKey, appLabelValue, commitSHA, namespace, fnva)
}

func appDetailsCacheKey(commitSHA, path string, valueFiles []string) string {
	valuesStr := strings.Join(valueFiles, ",")
	return fmt.Sprintf("appdetails|%s|%s|%s", commitSHA, path, valuesStr)
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
	return c.setItem(appManagedResourcesKey(appName), managedResources, appStateCacheExpiration, managedResources == nil)
}

func (c *Cache) GetAppResourcesTree(appName string) (*appv1.ApplicationTree, error) {
	var res *appv1.ApplicationTree
	err := c.getItem(appResourcesTreeKey(appName), &res)
	return res, err
}

func (c *Cache) SetAppResourcesTree(appName string, resourcesTree *appv1.ApplicationTree) error {
	return c.setItem(appResourcesTreeKey(appName), resourcesTree, appStateCacheExpiration, resourcesTree == nil)
}

func (c *Cache) GetClusterConnectionState(server string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.getItem(clusterConnectionStateKey(server), &res)
	return res, err
}

func (c *Cache) SetClusterConnectionState(server string, state *appv1.ConnectionState) error {
	return c.setItem(clusterConnectionStateKey(server), &state, connectionStatusCacheExpiration, state == nil)
}

func (c *Cache) GetRepoConnectionState(repo string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.getItem(repoConnectionStateKey(repo), &res)
	return res, err
}

func (c *Cache) SetRepoConnectionState(repo string, state *appv1.ConnectionState) error {
	return c.setItem(repoConnectionStateKey(repo), &state, connectionStatusCacheExpiration, state == nil)
}

func (c *Cache) ListApps(repo, revision string) (map[string]string, error) {
	res := make(map[string]string)
	err := c.getItem(appTemplatesKey(repo, revision), &res)
	return res, err
}

func (c *Cache) SetListApps(repo, revision string, appTemplates map[string]string) error {
	return c.setItem(appTemplatesKey(repo, revision), appTemplates, repoCacheExpiration, appTemplates == nil)
}

func (c *Cache) GetRepoApp(repo, path, revision string) (string, error) {
	res := ""
	err := c.getItem(appTemplateKey(repo, path, revision), &res)
	return res, err
}

func (c *Cache) SetRepoApp(repo, path, revision string, appType string) error {
	return c.setItem(appTemplateKey(repo, path, revision), appType, repoCacheExpiration, appType == "")
}

func (c *Cache) GetManifests(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.getItem(manifestCacheKey(commitSHA, appSrc, namespace, appLabelKey, appLabelValue), res)
}

func (c *Cache) SetManifests(commitSHA string, appSrc *appv1.ApplicationSource, namespace string, appLabelKey string, appLabelValue string, res interface{}) error {
	return c.setItem(manifestCacheKey(commitSHA, appSrc, namespace, appLabelKey, appLabelValue), res, repoCacheExpiration, res == nil)
}

func (c *Cache) GetAppDetails(commitSHA, path string, valueFiles []string, res interface{}) error {
	return c.getItem(appDetailsCacheKey(commitSHA, path, valueFiles), res)
}

func (c *Cache) SetAppDetails(commitSHA, path string, valueFiles []string, res interface{}) error {
	return c.setItem(appDetailsCacheKey(commitSHA, path, valueFiles), res, repoCacheExpiration, res == nil)
}

func (c *Cache) GetOIDCState(key string) (*OIDCState, error) {
	res := OIDCState{}
	err := c.getItem(oidcStateKey(key), &res)
	return &res, err
}

func (c *Cache) SetOIDCState(key string, state *OIDCState) error {
	return c.setItem(oidcStateKey(key), state, oidcCacheExpiration, state == nil)
}
