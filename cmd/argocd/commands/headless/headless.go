package headless

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	cache2 "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/server"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/io"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

type forwardCacheClient struct {
	namespace string
	context   string
	init      sync.Once
	client    cache.CacheClient
	err       error
}

func (c *forwardCacheClient) doLazy(action func(client cache.CacheClient) error) error {
	c.init.Do(func() {
		overrides := clientcmd.ConfigOverrides{
			CurrentContext: c.context,
		}
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
	context       string
	init          sync.Once
	repoClientset repoapiclient.Clientset
	err           error
}

func (c *forwardRepoClientset) NewRepoServerClient() (io.Closer, repoapiclient.RepoServerServiceClient, error) {
	c.init.Do(func() {
		overrides := clientcmd.ConfigOverrides{
			CurrentContext: c.context,
		}
		repoServerPort, err := kubeutil.PortForward(8081, c.namespace, &overrides, "app.kubernetes.io/name=argocd-repo-server")
		if err != nil {
			c.err = err
			return
		}
		c.repoClientset = repoapiclient.NewRepoServerClientset(fmt.Sprintf("localhost:%d", repoServerPort), 60, repoapiclient.TLSConfiguration{
			DisableTLS: false, StrictValidation: false})
	})
	if c.err != nil {
		return nil, nil, c.err
	}
	return c.repoClientset.NewRepoServerClient()
}

func testAPI(clientOpts *apiclient.ClientOptions) error {
	apiClient, err := apiclient.NewClient(clientOpts)
	if err != nil {
		return err
	}
	closer, versionClient, err := apiClient.NewVersionClient()
	if err != nil {
		return err
	}
	defer io.Close(closer)
	_, err = versionClient.Version(context.Background(), &empty.Empty{})
	return err
}

// StartLocalServer allows executing command in a headless mode: on the fly starts Argo CD API server and
// changes provided client options to use started API server port
func StartLocalServer(clientOpts *apiclient.ClientOptions, ctxStr string, port *int, address *string) error {
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	clientConfig := cli.AddKubectlFlagsToSet(flags)
	startInProcessAPI := clientOpts.Core
	if !startInProcessAPI {
		localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
		if err != nil {
			return err
		}
		if localCfg != nil {
			configCtx, err := localCfg.ResolveContext(clientOpts.Context)
			if err != nil {
				return err
			}
			startInProcessAPI = configCtx.Server.Core
		}
	}
	if !startInProcessAPI {
		return nil
	}

	// get rid of logging error handler
	runtime.ErrorHandlers = runtime.ErrorHandlers[1:]
	cli.SetLogLevel(log.ErrorLevel.String())
	log.SetLevel(log.ErrorLevel)
	os.Setenv(v1alpha1.EnvVarFakeInClusterConfig, "true")
	if address == nil {
		address = pointer.String("localhost")
	}
	if port == nil || *port == 0 {
		addr := fmt.Sprintf("%s:0", *address)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		port = &ln.Addr().(*net.TCPAddr).Port
		io.Close(ln)
	}

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	appClientset, err := appclientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	kubeClientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	namespace, _, err := clientConfig.Namespace()
	if err != nil {
		return err
	}

	mr, err := miniredis.Run()
	if err != nil {
		return err
	}
	ctx := context.Background()
	appstateCache := appstatecache.NewCache(cache.NewCache(&forwardCacheClient{namespace: namespace, context: ctxStr}), time.Hour)
	srv := server.NewServer(ctx, server.ArgoCDServerOpts{
		EnableGZip:    false,
		Namespace:     namespace,
		ListenPort:    *port,
		AppClientset:  appClientset,
		DisableAuth:   true,
		RedisClient:   redis.NewClient(&redis.Options{Addr: mr.Addr()}),
		Cache:         servercache.NewCache(appstateCache, 0, 0, 0),
		KubeClientset: kubeClientset,
		Insecure:      true,
		ListenHost:    *address,
		RepoClientset: &forwardRepoClientset{namespace: namespace, context: ctxStr},
	})

	go srv.Run(ctx, *port, 0)
	clientOpts.ServerAddr = fmt.Sprintf("%s:%d", *address, *port)
	clientOpts.PlainText = true
	if !cache2.WaitForCacheSync(ctx.Done(), srv.Initialized) {
		log.Fatal("Timed out waiting for project cache to sync")
	}
	tries := 5
	for i := 0; i < tries; i++ {
		err = testAPI(clientOpts)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	return err
}

// NewClientOrDie creates a new API client from a set of config options, or fails fatally if the new client creation fails.
func NewClientOrDie(opts *apiclient.ClientOptions, c *cobra.Command) apiclient.Client {
	ctxStr := initialize.RetrieveContextIfChanged(c.Flag("context"))
	err := StartLocalServer(opts, ctxStr, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	client, err := apiclient.NewClient(opts)
	if err != nil {
		log.Fatal(err)
	}
	return client
}
