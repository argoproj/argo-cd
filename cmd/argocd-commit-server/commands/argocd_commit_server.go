package commands

import (
	"net/http"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/commitserver"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v3/util/askpass"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/env"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// NewCommand returns a new instance of an argocd-commit-server command
func NewCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		cfg          commitserver.Config
	)
	command := &cobra.Command{
		Use:   common.CommandCommitServer,
		Short: "Run Argo CD Commit Server",
		Long:  "Argo CD Commit Server is an internal service which commits and pushes hydrated manifests to git. This command runs Commit Server in the foreground.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg.GrpcEnableTxtServiceConfig = env.ParseBoolFromEnv("GRPC_ENABLE_TXT_SERVICE_CONFIG", false)

			vers := common.GetVersion()
			vers.LogStartupInfo(
				"Argo CD Commit Server",
				map[string]any{
					"port": cfg.ListenPort,
				},
			)

			cli.SetLogFormat(cfg.LogFormat)
			cli.SetLogLevel(cfg.LogLevel)

			metricsServer := metrics.NewMetricsServer()
			http.Handle("/metrics", metricsServer.GetHandler())

			askPassServer := askpass.NewServer(askpass.CommitServerSocketPath)
			go func() { errors.CheckError(askPassServer.Run()) }()

			var crdSource configbus.CRDSource
			if restConfig, err := clientConfig.ClientConfig(); err != nil {
				log.WithError(err).Warn("kubeconfig unavailable; continuing without CRD config source")
			} else {
				errors.CheckError(v1alpha1.SetK8SConfigDefaults(restConfig))
				namespace, _, err := clientConfig.Namespace()
				errors.CheckError(err)
				appClient := appclientset.NewForConfigOrDie(restConfig)
				crdSource = configbus.NewOptionalInformerCRDSource(ctx, appClient, namespace)
			}

			server := commitserver.NewServer(cfg, askPassServer, metricsServer, crdSource)
			return server.Run(ctx)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	command.Flags().StringVar(&cfg.LogFormat, "logformat", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGFORMAT", "json"), "Set the logging format. One of: json|text")
	command.Flags().StringVar(&cfg.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&cfg.ListenHost, "address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LISTEN_ADDRESS", common.DefaultAddressCommitServer), "Listen on given address for incoming connections")
	command.Flags().IntVar(&cfg.ListenPort, "port", common.DefaultPortCommitServer, "Listen on given port for incoming connections")
	command.Flags().StringVar(&cfg.MetricsHost, "metrics-address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_METRICS_LISTEN_ADDRESS", common.DefaultAddressCommitServerMetrics), "Listen on given address for metrics")
	command.Flags().IntVar(&cfg.MetricsPort, "metrics-port", common.DefaultPortCommitServerMetrics, "Start metrics server on given port")

	return command
}
