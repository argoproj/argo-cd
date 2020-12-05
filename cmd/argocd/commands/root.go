package commands

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/localconfig"
)

func init() {
	cobra.OnInitialize(initConfig)
}

var (
	logFormat string
	logLevel  string
)

func initConfig() {
	cli.SetLogFormat(logFormat)
	cli.SetLogLevel(logLevel)
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		clientOpts argocdclient.ClientOptions
		pathOpts   = clientcmd.NewDefaultPathOptions()
	)

	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd controls a Argo CD server",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
	}

	command.AddCommand(NewCompletionCommand())
	command.AddCommand(NewVersionCmd(&clientOpts))
	command.AddCommand(NewClusterCommand(&clientOpts, pathOpts))
	command.AddCommand(NewApplicationCommand(&clientOpts))
	command.AddCommand(NewLoginCommand(&clientOpts))
	command.AddCommand(NewReloginCommand(&clientOpts))
	command.AddCommand(NewRepoCommand(&clientOpts))
	command.AddCommand(NewRepoCredsCommand(&clientOpts))
	command.AddCommand(NewContextCommand(&clientOpts))
	command.AddCommand(NewProjectCommand(&clientOpts))
	command.AddCommand(NewAccountCommand(&clientOpts))
	command.AddCommand(NewLogoutCommand(&clientOpts))
	command.AddCommand(NewCertCommand(&clientOpts))
	command.AddCommand(NewGPGCommand(&clientOpts))

	defaultLocalConfigPath, err := localconfig.DefaultLocalConfigPath()
	errors.CheckError(err)
	command.PersistentFlags().StringVar(&clientOpts.ConfigPath, "config", config.GetFlag("config", defaultLocalConfigPath), "Path to Argo CD config")
	command.PersistentFlags().StringVar(&clientOpts.ServerAddr, "server", config.GetFlag("server", ""), "Argo CD server address")
	command.PersistentFlags().BoolVar(&clientOpts.PlainText, "plaintext", config.GetBoolFlag("plaintext"), "Disable TLS")
	command.PersistentFlags().BoolVar(&clientOpts.Insecure, "insecure", config.GetBoolFlag("insecure"), "Skip server certificate and domain verification")
	command.PersistentFlags().StringVar(&clientOpts.CertFile, "server-crt", config.GetFlag("server-crt", ""), "Server certificate file")
	command.PersistentFlags().StringVar(&clientOpts.ClientCertFile, "client-crt", config.GetFlag("client-crt", ""), "Client certificate file")
	command.PersistentFlags().StringVar(&clientOpts.ClientCertKeyFile, "client-crt-key", config.GetFlag("client-crt-key", ""), "Client certificate key file")
	command.PersistentFlags().StringVar(&clientOpts.AuthToken, "auth-token", config.GetFlag("auth-token", ""), "Authentication token")
	command.PersistentFlags().BoolVar(&clientOpts.GRPCWeb, "grpc-web", config.GetBoolFlag("grpc-web"), "Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2.")
	command.PersistentFlags().StringVar(&clientOpts.GRPCWebRootPath, "grpc-web-root-path", config.GetFlag("grpc-web-root-path", ""), "Enables gRPC-web protocol. Useful if Argo CD server is behind proxy which does not support HTTP2. Set web root.")
	command.PersistentFlags().StringVar(&logFormat, "logformat", config.GetFlag("logformat", "text"), "Set the logging format. One of: text|json")
	command.PersistentFlags().StringVar(&logLevel, "loglevel", config.GetFlag("loglevel", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.PersistentFlags().StringSliceVarP(&clientOpts.Headers, "header", "H", []string{}, "Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)")
	command.PersistentFlags().BoolVar(&clientOpts.PortForward, "port-forward", config.GetBoolFlag("port-forward"), "Connect to a random argocd-server port using port forwarding")
	command.PersistentFlags().StringVar(&clientOpts.PortForwardNamespace, "port-forward-namespace", config.GetFlag("port-forward-namespace", ""), "Namespace name which should be used for port forwarding")
	return command
}
