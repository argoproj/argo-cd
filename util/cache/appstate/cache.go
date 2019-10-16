package appstate

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
)

var ErrCacheMiss = cacheutil.ErrCacheMiss

type Cache struct {
	cache                   *cacheutil.Cache
	appStateCacheExpiration time.Duration
}

func NewCache(cache *cacheutil.Cache, appStateCacheExpiration time.Duration) *Cache {
	return &Cache{cache, appStateCacheExpiration}
}

type OIDCState struct {
	// ReturnURL is the URL in which to redirect a user back to after completing an OAuth2 login
	ReturnURL string `json:"returnURL"`
}

func AddCacheFlagsToCmd(cmd *cobra.Command) func() (*Cache, error) {
	var appStateCacheExpiration time.Duration

	cmd.Flags().DurationVar(&appStateCacheExpiration, "app-state-cache-expiration", 1*time.Hour, "Cache expiration for app state")

	cacheFactory := cacheutil.AddCacheFlagsToCmd(cmd)

	return func() (*Cache, error) {
		cache, err := cacheFactory()
		if err != nil {
			return nil, err
		}
		return NewCache(cache, appStateCacheExpiration), nil
	}
}

func (c *Cache) GetItem(key string, item interface{}) error {
	return c.cache.GetItem(key, item)
}

func (c *Cache) SetItem(key string, item interface{}, expiration time.Duration, delete bool) error {
	return c.cache.SetItem(key, item, expiration, delete)
}

func appManagedResourcesKey(appName string) string {
	return fmt.Sprintf("app|managed-resources|%s", appName)
}

func (c *Cache) GetAppManagedResources(appName string, res *[]*appv1.ResourceDiff) error {
	err := c.GetItem(appManagedResourcesKey(appName), &res)
	return err
}

func (c *Cache) SetAppManagedResources(appName string, managedResources []*appv1.ResourceDiff) error {
	return c.SetItem(appManagedResourcesKey(appName), managedResources, c.appStateCacheExpiration, managedResources == nil)
}

func appResourcesTreeKey(appName string) string {
	return fmt.Sprintf("app|resources-tree|%s", appName)
}

func (c *Cache) GetAppResourcesTree(appName string, res *appv1.ApplicationTree) error {
	err := c.GetItem(appResourcesTreeKey(appName), &res)
	return err
}

func (c *Cache) SetAppResourcesTree(appName string, resourcesTree *appv1.ApplicationTree) error {
	return c.SetItem(appResourcesTreeKey(appName), resourcesTree, c.appStateCacheExpiration, resourcesTree == nil)
}
