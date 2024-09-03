package cache

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/env"
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

func (c *Cache) GetAppResourcesTree(appName string, res *appv1.ApplicationTree) error {
	return c.cache.GetAppResourcesTree(appName, res)
}

func (c *Cache) OnAppResourcesTreeChanged(ctx context.Context, appName string, callback func() error) error {
	return c.cache.OnAppResourcesTreeChanged(ctx, appName, callback)
}

func (c *Cache) GetAppManagedResources(appName string, res *[]*appv1.ResourceDiff) error {
	return c.cache.GetAppManagedResources(appName, res)
}

func (c *Cache) SetRepoConnectionState(repo string, project string, state *appv1.ConnectionState) error {
	return c.cache.SetItem(repoConnectionStateKey(repo, project), &state, c.connectionStatusCacheExpiration, state == nil)
}

func (c *Cache) SetLastApplicationEvent(a *appv1.Application, exp time.Duration) error {
	return c.cache.SetItem(lastApplicationEventKey(a), a, exp, false)
}

func (c *Cache) GetLastApplicationEvent(a *appv1.Application) (*appv1.Application, error) {
	cachedApp := appv1.Application{}
	return &cachedApp, c.cache.GetItem(lastApplicationEventKey(a), &cachedApp)
}

func (c *Cache) SetLastResourceEvent(a *appv1.Application, rs appv1.ResourceStatus, exp time.Duration, revision string) error {
	return c.cache.SetItem(lastResourceEventKey(a, rs, revision), rs, exp, false)
}

func (c *Cache) GetLastResourceEvent(a *appv1.Application, rs appv1.ResourceStatus, revision string) (appv1.ResourceStatus, error) {
	res := appv1.ResourceStatus{}
	return res, c.cache.GetItem(lastResourceEventKey(a, rs, revision), &res)
}

func lastApplicationEventKey(a *appv1.Application) string {
	return fmt.Sprintf("app|%s/%s|last-sent-event", a.Namespace, a.Name)
}

func lastResourceEventKey(a *appv1.Application, rs appv1.ResourceStatus, revision string) string {
	return fmt.Sprintf("app|%s/%s|%s|res|%s/%s/%s/%s/%s|last-sent-event",
		a.Namespace, a.Name, revision, rs.Group, rs.Version, rs.Kind, rs.Name, rs.Namespace)
}

func repoConnectionStateKey(repo string, project string) string {
	return fmt.Sprintf("repo|%s|%s|connection-state", repo, project)
}

func (c *Cache) GetRepoConnectionState(repo string, project string) (appv1.ConnectionState, error) {
	res := appv1.ConnectionState{}
	err := c.cache.GetItem(repoConnectionStateKey(repo, project), &res)
	return res, err
}

func (c *Cache) GetClusterInfo(server string, res *appv1.ClusterInfo) error {
	return c.cache.GetClusterInfo(server, res)
}

func (c *Cache) SetClusterInfo(server string, res *appv1.ClusterInfo) error {
	return c.cache.SetClusterInfo(server, res)
}

func (c *Cache) GetCache() *cacheutil.Cache {
	return c.cache.Cache
}
