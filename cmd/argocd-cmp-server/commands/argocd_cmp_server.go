package commands

import (
	"time"

	"github.com/argoproj/pkg/stats"
	"github.com/spf13/cobra"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/cmpserver"
	"github.com/argoproj/argo-cd/v2/cmpserver/plugin"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-cmp-server"
)

func NewCommand() *cobra.Command {
	var (
		configFilePath string
	)
	var command = cobra.Command{
		Use:               cliName,
		Short:             "Run ArgoCD ConfigManagementPlugin Server",
		Long:              "ArgoCD ConfigManagementPlugin Server is an internal service which runs as sidecar container in reposerver deployment. It can be configured by following options.",
		DisableAutoGenTag: true,
		RunE: func(c *cobra.Command, args []string) error {
			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			config, err := plugin.ReadPluginConfig(configFilePath)
			errors.CheckError(err)

			server, err := cmpserver.NewServer(plugin.CMPServerInitConstants{
				PluginConfig: *config,
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
	return &command
}
