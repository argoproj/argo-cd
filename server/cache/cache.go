package cache

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/env"
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

func AddCacheFlagsToCmd(cmd *cobra.Command, opts ...cacheutil.Options) func() (*Cache, error) {
	var connectionStatusCacheExpiration time.Duration
	var oidcCacheExpiration time.Duration
	var loginAttemptsExpiration time.Duration

	cmd.Flags().DurationVar(&connectionStatusCacheExpiration, "connection-status-cache-expiration", env.ParseDurationFromEnv("ARGOCD_SERVER_CONNECTION_STATUS_CACHE_EXPIRATION", 1*time.Hour, 0, math.MaxInt64), "Cache expiration for cluster/repo connection status")
	cmd.Flags().DurationVar(&oidcCacheExpiration, "oidc-cache-expiration", env.ParseDurationFromEnv("ARGOCD_SERVER_OIDC_CACHE_EXPIRATION", 3*time.Minute, 0, math.MaxInt64), "Cache expiration for OIDC state")
	cmd.Flags().DurationVar(&loginAttemptsExpiration, "login-attempts-expiration", env.ParseDurationFromEnv("ARGOCD_SERVER_LOGIN_ATTEMPTS_EXPIRATION", 24*time.Hour, 0, math.MaxInt64), "Cache expiration for failed login attempts")

	fn := appstatecache.AddCacheFlagsToCmd(cmd, opts...)

	return func() (*Cache, error) {
		cache, err := fn()
		if err != nil {
			return nil, err
		}

		return NewCache(cache, connectionStatusCacheExpiration, oidcCacheExpiration, loginAttemptsExpiration), nil
	}
}

func (c *Cache) GetAppResourcesTree(ctx context.Context, appName string, res *appv1.ApplicationTree) error {
	return c.cache.GetAppResourcesTree(ctx, appName, res)
}

func (c *Cache) OnAppResourcesTreeChanged(ctx context.Context, appName string, callback func() error) error {
	return c.cache.OnAppResourcesTreeChanged(ctx, appName, callback)
}

func (c *Cache) GetAppManagedResources(ctx context.Context, appName string, res *[]*appv1.ResourceDiff) error {
	return c.cache.GetAppManagedResources(ctx, appName, res)
}

func (c *Cache) SetRepoConnectionState(ctx context.Context, repo string, project string, state *appv1.ConnectionState) error {
	return c.cache.SetItem(ctx, repoConnectionStateKey(repo, project), &state, c.connectionStatusCacheExpiration, state == nil)
}

func repoConnectionStateKey(repo string, project string) string {
	return fmt.Sprintf("repo|%s|%s|connection-state", repo, project)
}

func (c *Cache) GetRepoConnectionState(ctx context.Context, repo string, project string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.cache.GetItem(ctx, repoConnectionStateKey(repo, project), &res)
	return res, err
}

func (c *Cache) GetClusterInfo(ctx context.Context, server string, res *appv1.ClusterInfo) error {
	return c.cache.GetClusterInfo(ctx, server, res)
}

func (c *Cache) SetClusterInfo(ctx context.Context, server string, res *appv1.ClusterInfo) error {
	return c.cache.SetClusterInfo(ctx, server, res)
}

func (c *Cache) GetCache() *cacheutil.Cache {
	return c.cache.Cache
}
