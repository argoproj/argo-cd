package headless

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	argoapi "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/server"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

func testAPI(clientOpts *argoapi.ClientOptions) error {
	apiClient, err := argoapi.NewClient(clientOpts)
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

// InitCommand allows executing command in a headless mode: on the fly starts Argo CD API server and
// changes provided client options to use started API server port
func InitCommand(cmd *cobra.Command, clientOpts *argoapi.ClientOptions, port *int) *cobra.Command {
	ctx, cancel := context.WithCancel(context.Background())
	flags := pflag.NewFlagSet("tmp", pflag.ContinueOnError)
	clientConfig := cli.AddKubectlFlagsToSet(flags)
	// copy k8s persistent flags into argocd command flags
	flags.VisitAll(func(flag *pflag.Flag) {
		// skip Kubernetes server flags since argocd has it's own server flag
		if flag.Name == "server" {
			return
		}
		cmd.Flags().AddFlag(flag)
	})
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
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
		if port == nil || *port == 0 {
			ln, err := net.Listen("tcp", "localhost:0")
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

		appstateCache := appstatecache.NewCache(cacheutil.NewCache(&forwardCacheClient{namespace: namespace}), time.Hour)
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
			ListenHost:    "localhost",
			RepoClientset: &forwardRepoClientset{namespace: namespace},
		})

		go srv.Run(ctx, *port, 0)
		clientOpts.ServerAddr = fmt.Sprintf("localhost:%d", *port)
		clientOpts.PlainText = true
		if !cache.WaitForCacheSync(ctx.Done(), srv.Initialized) {
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
	cmd.PostRun = func(cmd *cobra.Command, args []string) {
		cancel()
	}
	return cmd
}
