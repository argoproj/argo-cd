package cache

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
)

var ErrCacheMiss = appstatecache.ErrCacheMiss

type Cache struct {
	cache                           *appstatecache.Cache
	connectionStatusCacheExpiration time.Duration
	oidcCacheExpiration             time.Duration
}

func NewCache(
	cache *appstatecache.Cache,
	connectionStatusCacheExpiration time.Duration,
	oidcCacheExpiration time.Duration,
) *Cache {
	return &Cache{cache, connectionStatusCacheExpiration, oidcCacheExpiration}
}

type OIDCState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

type ClusterInfo struct {
	appv1.ConnectionState
	Version string
}

func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	var connectionStatusCacheExpiration time.Duration
	var oidcCacheExpiration time.Duration

	cmd.Flags().DurationVar(&connectionStatusCacheExpiration, "connection-status-cache-expiration", 1*time.Hour, "Cache expiration for cluster/repo connection status")
	cmd.Flags().DurationVar(&oidcCacheExpiration, "oidc-cache-expiration", 3*time.Minute, "Cache expiration for OIDC state")

	fn := appstatecache.AddCacheFlagsToCmd(cmd)

	return func() (*Cache, error) {
		cache, err := fn()
		if err != nil {
			return nil, err
		}

		return NewCache(cache, connectionStatusCacheExpiration, oidcCacheExpiration), nil
	}
}

func (c *Cache) GetAppResourcesTree(appName string, res *appv1.ApplicationTree) error {
	return c.cache.GetAppResourcesTree(appName, res)
}

func (c *Cache) GetAppManagedResources(appName string, res *[]*appv1.ResourceDiff) error {
	return c.cache.GetAppManagedResources(appName, res)
}

func clusterConnectionStateKey(server string) string {
	return fmt.Sprintf("cluster|%s|connection-state", server)
}

func (c *Cache) GetClusterInfo(server string) (ClusterInfo, error) {
	res := ClusterInfo{}
	err := c.cache.GetItem(clusterConnectionStateKey(server), &res)
	return res, err
}

func (c *Cache) SetRepoConnectionState(repo string, state *appv1.ConnectionState) error {
	return c.cache.SetItem(repoConnectionStateKey(repo), &state, c.connectionStatusCacheExpiration, state == nil)
}

func repoConnectionStateKey(repo string) string {
	return fmt.Sprintf("repo|%s|connection-state", repo)
}

func (c *Cache) GetRepoConnectionState(repo string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.cache.GetItem(repoConnectionStateKey(repo), &res)
	return res, err
}

func (c *Cache) SetClusterInfo(server string, state *ClusterInfo) error {
	return c.cache.SetItem(clusterConnectionStateKey(server), &state, c.connectionStatusCacheExpiration, state == nil)
}

func oidcStateKey(key string) string {
	return fmt.Sprintf("oidc|%s", key)
}

func (c *Cache) GetOIDCState(key string) (*OIDCState, error) {
	res := OIDCState{}
	err := c.cache.GetItem(oidcStateKey(key), &res)
	return &res, err
}

func (c *Cache) SetOIDCState(key string, state *OIDCState) error {
	return c.cache.SetItem(oidcStateKey(key), state, c.oidcCacheExpiration, state == nil)
}
