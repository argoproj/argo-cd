package commands

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/admin"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/config"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cli.SetLogFormat(cmdutil.LogFormat)
	cli.SetLogLevel(cmdutil.LogLevel)
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
	command.AddCommand(initialize.InitCommand(NewVersionCmd(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewClusterCommand(&clientOpts, pathOpts)))
	command.AddCommand(initialize.InitCommand(NewApplicationCommand(&clientOpts)))
	command.AddCommand(NewLoginCommand(&clientOpts))
	command.AddCommand(NewReloginCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewRepoCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewRepoCredsCommand(&clientOpts)))
	command.AddCommand(NewContextCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewProjectCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewAccountCommand(&clientOpts)))
	command.AddCommand(NewLogoutCommand(&clientOpts))
	command.AddCommand(initialize.InitCommand(NewCertCommand(&clientOpts)))
	command.AddCommand(initialize.InitCommand(NewGPGCommand(&clientOpts)))
	command.AddCommand(admin.NewAdminCommand())

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
	command.PersistentFlags().StringVar(&cmdutil.LogFormat, "logformat", config.GetFlag("logformat", "text"), "Set the logging format. One of: text|json")
	command.PersistentFlags().StringVar(&cmdutil.LogLevel, "loglevel", config.GetFlag("loglevel", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.PersistentFlags().StringSliceVarP(&clientOpts.Headers, "header", "H", []string{}, "Sets additional header to all requests made by Argo CD CLI. (Can be repeated multiple times to add multiple headers, also supports comma separated headers)")
	command.PersistentFlags().BoolVar(&clientOpts.PortForward, "port-forward", config.GetBoolFlag("port-forward"), "Connect to a random argocd-server port using port forwarding")
	command.PersistentFlags().StringVar(&clientOpts.PortForwardNamespace, "port-forward-namespace", config.GetFlag("port-forward-namespace", ""), "Namespace name which should be used for port forwarding")
	command.PersistentFlags().IntVar(&clientOpts.HttpRetryMax, "http-retry-max", 0, "Maximum number of retries to establish http connection to Argo CD server")
	command.PersistentFlags().BoolVar(&clientOpts.Core, "core", false, "If set to true then CLI talks directly to Kubernetes instead of talking to Argo CD API server")
	return command
}
