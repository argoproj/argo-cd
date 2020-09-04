package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/oidc"
	"github.com/argoproj/argo-cd/util/session"
)

var ErrCacheMiss = appstatecache.ErrCacheMiss

type Cache struct {
	cache                           *appstatecache.Cache
	connectionStatusCacheExpiration time.Duration
	oidcCacheExpiration             time.Duration
	loginAttemptsExpiration         time.Duration
}

func NewCache(
	cache *appstatecache.Cache,
	connectionStatusCacheExpiration time.Duration,
	oidcCacheExpiration time.Duration,
	loginAttemptsExpiration time.Duration,
) *Cache {
	return &Cache{cache, connectionStatusCacheExpiration, oidcCacheExpiration, loginAttemptsExpiration}
}

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...func(client *redis.Client)) func() (*Cache, error) {
	var connectionStatusCacheExpiration time.Duration
	var oidcCacheExpiration time.Duration
	var loginAttemptsExpiration time.Duration

	cmd.Flags().DurationVar(&connectionStatusCacheExpiration, "connection-status-cache-expiration", 1*time.Hour, "Cache expiration for cluster/repo connection status")
	cmd.Flags().DurationVar(&oidcCacheExpiration, "oidc-cache-expiration", 3*time.Minute, "Cache expiration for OIDC state")
	cmd.Flags().DurationVar(&loginAttemptsExpiration, "login-attempts-expiration", 24*time.Hour, "Cache expiration for failed login attempts")

	fn := appstatecache.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := fn()
		if err != nil {
			return nil, err
		}

		return NewCache(cache, connectionStatusCacheExpiration, oidcCacheExpiration, loginAttemptsExpiration), nil
	}
}

func (c *Cache) GetAppResourcesTree(appName string, res *appv1.ApplicationTree) error {
	return c.cache.GetAppResourcesTree(appName, res)
}

func (c *Cache) OnAppResourcesTreeChanged(ctx context.Context, appName string, callback func() error) error {
	return c.cache.OnAppResourcesTreeChanged(ctx, appName, callback)
}

func (c *Cache) GetAppManagedResources(appName string, res *[]*appv1.ResourceDiff) error {
	return c.cache.GetAppManagedResources(appName, res)
}

func (c *Cache) GetLoginAttempts(attempts *map[string]session.LoginAttempts) error {
	return c.cache.GetItem("session|login.attempts", attempts)
}

func (c *Cache) SetLoginAttempts(attempts map[string]session.LoginAttempts) error {
	return c.cache.SetItem("session|login.attempts", attempts, c.loginAttemptsExpiration, attempts == nil)
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

func (c *Cache) GetClusterInfo(server string, res *appv1.ClusterInfo) error {
	return c.cache.GetClusterInfo(server, res)
}

func (c *Cache) SetClusterInfo(server string, res *appv1.ClusterInfo) error {
	return c.cache.SetClusterInfo(server, res)
}

func oidcStateKey(key string) string {
	return fmt.Sprintf("oidc|%s", key)
}

func (c *Cache) GetOIDCState(key string) (*oidc.OIDCState, error) {
	res := oidc.OIDCState{}
	err := c.cache.GetItem(oidcStateKey(key), &res)
	return &res, err
}

func (c *Cache) SetOIDCState(key string, state *oidc.OIDCState) error {
	return c.cache.SetItem(oidcStateKey(key), state, c.oidcCacheExpiration, state == nil)
}

func (c *Cache) GetCache() *cacheutil.Cache {
	return c.cache.Cache
}
