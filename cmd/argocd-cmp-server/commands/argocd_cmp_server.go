package commands

import (
	"time"

	"github.com/argoproj/pkg/stats"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/cmpserver"
	"github.com/argoproj/argo-cd/v2/cmpserver/plugin"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	traceutil "github.com/argoproj/argo-cd/v2/util/trace"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-cmp-server"
)

func NewCommand() *cobra.Command {
	var (
		configFilePath string
		otlpAddress string
		execTimeout time.Duration
	)
	var command = cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD ConfigManagementPlugin Server",
		Long:              "ArgoCD ConfigManagementPlugin Server is an internal service which runs as sidecar container in reposerver deployment. It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			vers := common.GetVersion()
			vers.LogStartupInfo("ArgoCD ConfigManagementPlugin Server", nil)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			config, err := plugin.ReadPluginConfig(configFilePath)
			errors.CheckError(err)

			if otlpAddress != "" {
				var closer func()
				var err error
				closer, err = traceutil.InitTracer(ctx, "argocd-cmp-server", otlpAddress)
				if err != nil {
					log.Fatalf("failed to initialize tracing: %v", err)
				}
				defer closer()
			}

			server, err := cmpserver.NewServer(plugin.CMPServerInitConstants{
				PluginConfig: *config,
				ExecTimeout:  execTimeout,
			})
			errors.CheckError(err)

			// register dumper
			stats.RegisterStackDumper()
			stats.StartStatsTicker(10 * time.Minute)
			stats.RegisterHeapDumper("memprofile")

			// run argocd-cmp-server server
			server.Run()
			return nil
		},
	}

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&configFilePath, "config-dir-path", common.DefaultPluginConfigFilePath, "Config management plugin configuration file location, Default is '/home/argocd/cmp-server/config/'")
	command.Flags().StringVar(&otlpAddress, "otlp-address", env.StringFromEnv("ARGOCD_CMP_SERVER_OTLP_ADDRESS", ""), "OpenTelemetry collector address to send traces to")
	// Fall back to ARGOCD_EXEC_TIMEOUT for backwards compatibility.
	durationFromEnv := env.ParseDurationFromEnvs(common.DefaultExecTimeout, 0*time.Second, 24*time.Hour, "ARGOCD_CMP_SERVER_EXEC_TIMEOUT", "ARGOCD_EXEC_TIMEOUT")
	command.Flags().DurationVar(&execTimeout, "exec-timeout", durationFromEnv, "per-command timeout for external commands invoked by the CMP server (such as git)")

	return &command
}
