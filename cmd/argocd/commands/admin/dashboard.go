package admin

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/util/cli"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v3/common"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// DashboardConfig holds the configuration for starting the dashboard
type DashboardConfig struct {
	Port         int
	Address      string
	ClientOpts   *argocdclient.ClientOptions
	ClientConfig clientcmd.ClientConfig
	Context      string
}

type dashboard struct {
	startLocalServer func(ctx context.Context, clientOpts *argocdclient.ClientOptions, contextName string, port *int, address *string, clientConfig clientcmd.ClientConfig) (func(), error)
}

// NewDashboard initializes a new dashboard with default dependencies
func NewDashboard() *dashboard {
	return &dashboard{
		startLocalServer: headless.MaybeStartLocalServer,
	}
}

// Run runs the dashboard and blocks until context is done
func (ds *dashboard) Run(ctx context.Context, config *DashboardConfig) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	config.ClientOpts.Core = true
	println("starting dashboard")
	shutDownFunc, err := ds.startLocalServer(ctx, config.ClientOpts, config.Context, &config.Port, &config.Address, config.ClientConfig)
	if err != nil {
		return fmt.Errorf("could not start dashboard: %w", err)
	}
	fmt.Printf("Argo CD UI is available at http://%s:%d\n", config.Address, config.Port)
	<-ctx.Done()
	stop() // unregister the signal handler as soon as we receive a signal
	println("signal received, shutting down dashboard")
	if shutDownFunc != nil {
		shutDownFunc()
	}
	println("clean shutdown")
	return nil
}

func NewDashboardCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	config := &DashboardConfig{ClientOpts: clientOpts}
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, _ []string) {
			config.Context = initialize.RetrieveContextIfChanged(cmd.Flag("context"))
			errors.CheckError(NewDashboard().Run(cmd.Context(), config))
		},
		Example: `# Start the Argo CD Web UI locally on the default port and address
$ argocd admin dashboard

# Start the Argo CD Web UI locally on a custom port and address
$ argocd admin dashboard --port 8080 --address 127.0.0.1

# Start the Argo CD Web UI with GZip compression
$ argocd admin dashboard --redis-compress gzip
  `,
	}
	config.ClientConfig = cli.AddKubectlFlagsToSet(cmd.Flags())
	cmd.Flags().IntVar(&config.Port, "port", common.DefaultPortAPIServer, "Listen on given port")
	cmd.Flags().StringVar(&config.Address, "address", common.DefaultAddressAdminDashboard, "Listen on given address")
	return cmd
}
