package controller;

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
    cacheutil "github.com/argoproj/argo-cd/util/cache"
)

var ErrCacheMiss = cacheutil.ErrCacheMiss

type Cache struct {
    cache                           *cacheutil.Cache
	appStateCacheExpiration         time.Duration
	connectionStatusCacheExpiration time.Duration
	oidcCacheExpiration             time.Duration
}

func NewCache(
    cache                           *cacheutil.Cache,
	appStateCacheExpiration         time.Duration,
	connectionStatusCacheExpiration time.Duration,
	oidcCacheExpiration             time.Duration,
) *Cache {
	return &Cache{cache, appStateCacheExpiration, connectionStatusCacheExpiration, oidcCacheExpiration}
}

type OIDCState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	var appStateCacheExpiration time.Duration
	var connectionStatusCacheExpiration time.Duration
	var oidcCacheExpiration time.Duration

	cmd.Flags().DurationVar(&appStateCacheExpiration, "app-state-cache-expiration", 1*time.Hour, "Cache expiration for app state")
	cmd.Flags().DurationVar(&connectionStatusCacheExpiration, "connection-status-cache-expiration", 1*time.Hour, "Cache expiration for cluster/repo connection status")
	cmd.Flags().DurationVar(&oidcCacheExpiration, "oidc-cache-expiration", 3*time.Minute, "Cache expiration for OIDC state")

	fn := cacheutil.AddCacheFlagsToCmd(cmd)

	return func() (*Cache, error) {
		cache, err := fn()
		if err != nil {
		    return nil, err
		}
		return NewCache(            cache,            appStateCacheExpiration,            connectionStatusCacheExpiration,oidcCacheExpiration), nil
	}
}

func appManagedResourcesKey(appName string) string {
	return fmt.Sprintf("app|managed-resources|%s", appName)
}

func (c *Cache) GetAppManagedResources(appName string) ([]*appv1.ResourceDiff, error) {
	res := make([]*appv1.ResourceDiff, 0)
	err := c.cache.GetItem(appManagedResourcesKey(appName), &res)
	return res, err
}

func (c *Cache) SetAppManagedResources(appName string, managedResources []*appv1.ResourceDiff) error {
	return c.cache.SetItem(appManagedResourcesKey(appName), managedResources, c.appStateCacheExpiration, managedResources == nil)
}

func appResourcesTreeKey(appName string) string {
	return fmt.Sprintf("app|resources-tree|%s", appName)
}

func (c *Cache) GetAppResourcesTree(appName string) (*appv1.ApplicationTree, error) {
	var res *appv1.ApplicationTree
	err := c.cache.GetItem(appResourcesTreeKey(appName), &res)
	return res, err
}

func (c *Cache) SetAppResourcesTree(appName string, resourcesTree *appv1.ApplicationTree) error {
	return c.cache.SetItem(appResourcesTreeKey(appName), resourcesTree, c.appStateCacheExpiration, resourcesTree == nil)
}

func clusterConnectionStateKey(server string) string {
	return fmt.Sprintf("cluster|%s|connection-state", server)
}

func (c *Cache) SetRepoConnectionState(repo string, state *appv1.ConnectionState) error {
	return c.cache.SetItem(repoConnectionStateKey(repo), &state, c.connectionStatusCacheExpiration, state == nil)
}

func (c *Cache) GetClusterConnectionState(server string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.cache.GetItem(clusterConnectionStateKey(server), &res)
	return res, err
}

func repoConnectionStateKey(repo string) string {
	return fmt.Sprintf("repo|%s|connection-state", repo)
}

func (c *Cache) GetRepoConnectionState(repo string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.cache.GetItem(repoConnectionStateKey(repo), &res)
	return res, err
}

func (c *Cache) SetClusterConnectionState(server string, state *appv1.ConnectionState) error {
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
