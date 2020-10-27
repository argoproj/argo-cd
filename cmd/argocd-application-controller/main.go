package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// load the azure plugin (required to authenticate with AKS clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/controller/sharding"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/env"
	"github.com/argoproj/argo-cd/util/errors"
	kubeutil "github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-application-controller"
	// Default time in seconds for application resync period
	defaultAppResyncPeriod = 180
)

func newCommand() *cobra.Command {
	var (
		clientConfig             clientcmd.ClientConfig
		appResyncPeriod          int64
		repoServerAddress        string
		repoServerTimeoutSeconds int
		selfHealTimeoutSeconds   int
		statusProcessors         int
		operationProcessors      int
		logFormat                string
		logLevel                 string
		glogLevel                int
		metricsPort              int
		kubectlParallelismLimit  int64
		cacheSrc                 func() (*appstatecache.Cache, error)
		redisClient              *redis.Client
	)
	var command = cobra.Command{
		Use:   cliName,
		Short: "application-controller is a controller to operate on applications CRD",
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogFormat(logFormat)
			cli.SetLogLevel(logLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			kubeClient := kubernetes.NewForConfigOrDie(config)
			appClient := appclientset.NewForConfigOrDie(config)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			resyncDuration := time.Duration(appResyncPeriod) * time.Second
			repoClientset := apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cache, err := cacheSrc()
			errors.CheckError(err)

			settingsMgr := settings.NewSettingsManager(ctx, kubeClient, namespace)
			kubectl := kubeutil.NewKubectl()
			clusterFilter := getClusterFilter()
			appController, err := controller.NewApplicationController(
				namespace,
				settingsMgr,
				kubeClient,
				appClient,
				repoClientset,
				cache,
				kubectl,
				resyncDuration,
				time.Duration(selfHealTimeoutSeconds)*time.Second,
				metricsPort,
				kubectlParallelismLimit,
				clusterFilter)
			errors.CheckError(err)
			cacheutil.CollectMetrics(redisClient, appController.GetMetricsServer())

			vers := common.GetVersion()
			log.Infof("Application Controller (version: %s, built: %s) starting (namespace: %s)", vers.Version, vers.BuildDate, namespace)
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			go appController.Run(ctx, statusProcessors, operationProcessors)

			// Wait forever
			select {}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().Int64Var(&appResyncPeriod, "app-resync", defaultAppResyncPeriod, "Time period in seconds for application resync.")
	command.Flags().StringVar(&repoServerAddress, "repo-server", common.DefaultRepoServerAddr, "Repo server address.")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "repo-server-timeout-seconds", 60, "Repo server RPC call timeout seconds.")
	command.Flags().IntVar(&statusProcessors, "status-processors", 1, "Number of application status processors")
	command.Flags().IntVar(&operationProcessors, "operation-processors", 1, "Number of application operation processors")
	command.Flags().StringVar(&logFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortArgoCDMetrics, "Start metrics server on given port")
	command.Flags().IntVar(&selfHealTimeoutSeconds, "self-heal-timeout-seconds", 5, "Specifies timeout between application self heal attempts")
	command.Flags().Int64Var(&kubectlParallelismLimit, "kubectl-parallelism-limit", 20, "Number of allowed concurrent kubectl fork/execs. Any value less the 1 means no limit.")
	cacheSrc = appstatecache.AddCacheFlagsToCmd(&command, func(client *redis.Client) {
		redisClient = client
	})
	return &command
}

func getClusterFilter() func(cluster *v1alpha1.Cluster) bool {
	replicas := env.ParseNumFromEnv(common.EnvControllerReplicas, 0, 0, math.MaxInt32)
	shard := env.ParseNumFromEnv(common.EnvControllerShard, -1, -math.MaxInt32, math.MaxInt32)
	var clusterFilter func(cluster *v1alpha1.Cluster) bool
	if replicas > 1 {
		if shard < 0 {
			var err error
			shard, err = sharding.InferShard()
			errors.CheckError(err)
		}
		log.Infof("Processing clusters from shard %d", shard)
		clusterFilter = sharding.GetClusterFilter(replicas, shard)
	} else {
		log.Info("Processing all cluster shards")
	}
	return clusterFilter
}

func main() {
	if err := newCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
