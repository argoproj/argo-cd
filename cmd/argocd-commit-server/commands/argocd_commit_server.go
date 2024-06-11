package commands

import (
	"fmt"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/commitserver"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/spf13/cobra"
	"net"
)

func NewCommand() *cobra.Command {
	var listenHost string
	var listenPort int
	var command = &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			vers := common.GetVersion()
			vers.LogStartupInfo(
				"Argo CD Commit Server",
				map[string]any{
					"port": listenPort,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			server := commitserver.NewServer()
			grpc := server.CreateGRPC()

			listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenHost, listenPort))
			errors.CheckError(err)

			err = grpc.Serve(listener)
			errors.CheckError(err)
			return nil
		},
	}
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVar(&listenHost, "address", env.StringFromEnv("ARGOCD_COMMIT_SERVER_LISTEN_ADDRESS", common.DefaultAddressCommitServer), "Listen on given address for incoming connections")
	command.Flags().IntVar(&listenPort, "port", common.DefaultPortCommitServer, "Listen on given port for incoming connections")
	return command
}
