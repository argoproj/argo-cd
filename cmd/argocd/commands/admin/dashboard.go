package admin

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/util/cli"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

func NewDashboardCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		port         int
		address      string
		clientConfig clientcmd.ClientConfig
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			clientOpts.Core = true
			errors.CheckError(headless.MaybeStartLocalServer(ctx, clientOpts, initialize.RetrieveContextIfChanged(cmd.Flag("context")), &port, &address, clientConfig))
			println(fmt.Sprintf("Argo CD UI is available at http://%s:%d", address, port))
			<-ctx.Done()
		},
		Example: `# Start the Argo CD Web UI locally on the default port and address
$ argocd admin dashboard

# Start the Argo CD Web UI locally on a custom port and address
$ argocd admin dashboard --port 8080 --address 127.0.0.1

# Start the Argo CD Web UI with GZip compression
$ argocd admin dashboard --redis-compress gzip
  `,
	}
	clientConfig = cli.AddKubectlFlagsToSet(cmd.Flags())
	cmd.Flags().IntVar(&port, "port", common.DefaultPortAPIServer, "Listen on given port")
	cmd.Flags().StringVar(&address, "address", common.DefaultAddressAdminDashboard, "Listen on given address")
	return cmd
}
