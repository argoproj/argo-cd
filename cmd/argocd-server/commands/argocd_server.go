package commands

import (
	"context"
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/go-redis/redis/v8"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server"
	servercache "github.com/argoproj/argo-cd/server/cache"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/env"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/tls"
)

const (
	failureRetryCountEnv              = "ARGOCD_K8S_RETRY_COUNT"
	failureRetryPeriodMilliSecondsEnv = "ARGOCD_K8S_RETRY_DURATION_MILLISECONDS"
)

var (
	failureRetryCount              = 0
	failureRetryPeriodMilliSeconds = 100
)

func init() {
	failureRetryCount = env.ParseNumFromEnv(failureRetryCountEnv, failureRetryCount, 0, 10)
	failureRetryPeriodMilliSeconds = env.ParseNumFromEnv(failureRetryPeriodMilliSecondsEnv, failureRetryPeriodMilliSeconds, 0, 1000)
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		redisClient              *redis.Client
		insecure                 bool
		listenPort               int
		metricsPort              int
		glogLevel                int
		clientConfig             clientcmd.ClientConfig
		repoServerTimeoutSeconds int
		staticAssetsDir          string
		baseHRef                 string
		rootPath                 string
		repoServerAddress        string
		dexServerAddress         string
		disableAuth              bool
		enableGZip               bool
		tlsConfigCustomizerSrc   func() (tls.ConfigCustomizer, error)
		cacheSrc                 func() (*servercache.Cache, error)
		frameOptions             string
	)
	var command = &cobra.Command{
		Use:               cliName,
		Short:             "Run the ArgoCD API server",
		Long:              "The API server is a gRPC/REST server which exposes the API consumed by the Web UI, CLI, and CI/CD systems.  This command runs API server in the foreground.  It can be configured by following options.",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(config))

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)
			cache, err := cacheSrc()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)

			appclientsetConfig, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			errors.CheckError(v1alpha1.SetK8SConfigDefaults(appclientsetConfig))

			if failureRetryCount > 0 {
				appclientsetConfig = kube.AddFailureRetryWrapper(appclientsetConfig, failureRetryCount, failureRetryPeriodMilliSeconds)
			}
			appclientset := appclientset.NewForConfigOrDie(appclientsetConfig)
			repoclientset := apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds)

			if rootPath != "" {
				if baseHRef != "" && baseHRef != rootPath {
					log.Warnf("--basehref and --rootpath had conflict: basehref: %s rootpath: %s", baseHRef, rootPath)
				}
				baseHRef = rootPath
			}

			argoCDOpts := server.ArgoCDServerOpts{
				Insecure:            insecure,
				ListenPort:          listenPort,
				MetricsPort:         metricsPort,
				Namespace:           namespace,
				StaticAssetsDir:     staticAssetsDir,
				BaseHRef:            baseHRef,
				RootPath:            rootPath,
				KubeClientset:       kubeclientset,
				AppClientset:        appclientset,
				RepoClientset:       repoclientset,
				DexServerAddr:       dexServerAddress,
				DisableAuth:         disableAuth,
				EnableGZip:          enableGZip,
				TLSConfigCustomizer: tlsConfigCustomizer,
				Cache:               cache,
				XFrameOptions:       frameOptions,
				RedisClient:         redisClient,
			}

			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			for {
				ctx := context.Background()
				ctx, cancel := context.WithCancel(ctx)
				argocd := server.NewServer(ctx, argoCDOpts)
				argocd.Run(ctx, listenPort, metricsPort)
				cancel()
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().BoolVar(&insecure, "insecure", false, "Run server without TLS")
	command.Flags().StringVar(&staticAssetsDir, "staticassets", "", "Static assets directory path")
	command.Flags().StringVar(&baseHRef, "basehref", "/", "Value for base href in index.html. Used if Argo CD is running behind reverse proxy under subpath different from /")
	command.Flags().StringVar(&rootPath, "rootpath", "", "Used if Argo CD is running behind reverse proxy under subpath different from /")
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().StringVar(&repoServerAddress, "repo-server", common.DefaultRepoServerAddr, "Repo server address")
	command.Flags().StringVar(&dexServerAddress, "dex-server", common.DefaultDexServerAddr, "Dex server address")
	command.Flags().BoolVar(&disableAuth, "disable-auth", false, "Disable client authentication")
	command.Flags().BoolVar(&enableGZip, "enable-gzip", false, "Enable GZIP compression")
	command.AddCommand(cli.NewVersionCmd(cliName))
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortAPIServer, "Listen on given port")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortArgoCDAPIServerMetrics, "Start metrics on given port")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "repo-server-timeout-seconds", 60, "Repo server RPC call timeout seconds.")
	command.Flags().StringVar(&frameOptions, "x-frame-options", "sameorigin", "Set X-Frame-Options header in HTTP responses to `value`. To disable, set to \"\".")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(command)
	cacheSrc = servercache.AddCacheFlagsToCmd(command, func(client *redis.Client) {
		redisClient = client
	})
	return command
}
