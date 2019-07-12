package commands

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/stats"
	"github.com/argoproj/argo-cd/util/tls"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		insecure                 bool
		listenPort               int
		metricsPort              int
		logLevel                 string
		glogLevel                int
		clientConfig             clientcmd.ClientConfig
		repoServerTimeoutSeconds int
		staticAssetsDir          string
		baseHRef                 string
		repoServerAddress        string
		dexServerAddress         string
		disableAuth              bool
		tlsConfigCustomizerSrc   func() (tls.ConfigCustomizer, error)
		cacheSrc                 func() (*cache.Cache, error)
	)
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Run the argocd API server",
		Long:  "Run the argocd API server",
		Run: func(c *cobra.Command, args []string) {
			cli.SetLogLevel(logLevel)
			cli.SetGLogLevel(glogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			config.QPS = common.K8sClientConfigQPS
			config.Burst = common.K8sClientConfigBurst

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			tlsConfigCustomizer, err := tlsConfigCustomizerSrc()
			errors.CheckError(err)
			cache, err := cacheSrc()
			errors.CheckError(err)

			kubeclientset := kubernetes.NewForConfigOrDie(config)
			appclientset := appclientset.NewForConfigOrDie(config)
			repoclientset := apiclient.NewRepoServerClientset(repoServerAddress, repoServerTimeoutSeconds)

			argoCDOpts := server.ArgoCDServerOpts{
				Insecure:            insecure,
				ListenPort:          listenPort,
				MetricsPort:         metricsPort,
				Namespace:           namespace,
				StaticAssetsDir:     staticAssetsDir,
				BaseHRef:            baseHRef,
				KubeClientset:       kubeclientset,
				AppClientset:        appclientset,
				RepoClientset:       repoclientset,
				DexServerAddr:       dexServerAddress,
				DisableAuth:         disableAuth,
				TLSConfigCustomizer: tlsConfigCustomizer,
				Cache:               cache,
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
	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().IntVar(&glogLevel, "gloglevel", 0, "Set the glog logging level")
	command.Flags().StringVar(&repoServerAddress, "repo-server", common.DefaultRepoServerAddr, "Repo server address")
	command.Flags().StringVar(&dexServerAddress, "dex-server", common.DefaultDexServerAddr, "Dex server address")
	command.Flags().BoolVar(&disableAuth, "disable-auth", false, "Disable client authentication")
	command.AddCommand(cli.NewVersionCmd(cliName))
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortAPIServer, "Listen on given port")
	command.Flags().IntVar(&metricsPort, "metrics-port", common.DefaultPortArgoCDAPIServerMetrics, "Start metrics on given port")
	command.Flags().IntVar(&repoServerTimeoutSeconds, "repo-server-timeout-seconds", 60, "Repo server RPC call timeout seconds.")
	tlsConfigCustomizerSrc = tls.AddTLSFlagsToCmd(command)
	cacheSrc = cache.AddCacheFlagsToCmd(command)
	return command
}
