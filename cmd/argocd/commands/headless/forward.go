package headless

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/io"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
)

type forwardCacheClient struct {
	namespace string
	init      sync.Once
	client    cache.CacheClient
	err       error
}

func (c *forwardCacheClient) doLazy(action func(client cache.CacheClient) error) error {
	c.init.Do(func() {
		overrides := clientcmd.ConfigOverrides{}
		redisPort, err := kubeutil.PortForward(6379, c.namespace, &overrides,
			"app.kubernetes.io/name=argocd-redis-ha-haproxy", "app.kubernetes.io/name=argocd-redis")
		if err != nil {
			c.err = err
			return
		}

		redisClient := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("localhost:%d", redisPort)})
		c.client = cache.NewRedisCache(redisClient, time.Hour)
	})
	if c.err != nil {
		return c.err
	}
	return action(c.client)
}

func (c *forwardCacheClient) Set(item *cache.Item) error {
	return c.doLazy(func(client cache.CacheClient) error {
		return client.Set(item)
	})
}

func (c *forwardCacheClient) Get(key string, obj interface{}) error {
	return c.doLazy(func(client cache.CacheClient) error {
		return client.Get(key, obj)
	})
}

func (c *forwardCacheClient) Delete(key string) error {
	return c.doLazy(func(client cache.CacheClient) error {
		return client.Delete(key)
	})
}

func (c *forwardCacheClient) OnUpdated(ctx context.Context, key string, callback func() error) error {
	return c.doLazy(func(client cache.CacheClient) error {
		return client.OnUpdated(ctx, key, callback)
	})
}

func (c *forwardCacheClient) NotifyUpdated(key string) error {
	return c.doLazy(func(client cache.CacheClient) error {
		return client.NotifyUpdated(key)
	})
}

type forwardRepoClientset struct {
	namespace     string
	init          sync.Once
	repoClientset repoapiclient.Clientset
	err           error
}

func (c *forwardRepoClientset) NewRepoServerClient() (io.Closer, repoapiclient.RepoServerServiceClient, error) {
	c.init.Do(func() {
		overrides := clientcmd.ConfigOverrides{}
		repoServerPort, err := kubeutil.PortForward(8081, c.namespace, &overrides, "app.kubernetes.io/name=argocd-repo-server")
		if err != nil {
			c.err = err
			return
		}
		c.repoClientset = apiclient.NewRepoServerClientset(fmt.Sprintf("localhost:%d", repoServerPort), 60, apiclient.TLSConfiguration{
			DisableTLS: false, StrictValidation: false})
	})
	if c.err != nil {
		return nil, nil, c.err
	}
	return c.repoClientset.NewRepoServerClient()
}
