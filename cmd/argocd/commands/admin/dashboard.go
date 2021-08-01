package admin

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
)

func NewDashboardCommand() *cobra.Command {
	var (
		port int
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, args []string) {
			println(fmt.Sprintf("Argo CD UI is available at http://localhost:%d", port))
			<-context.Background().Done()
		},
	}
	clientOpts := &apiclient.ClientOptions{Core: true}
	headless.InitCommand(cmd, clientOpts, &port)
	cmd.Flags().IntVar(&port, "port", common.DefaultPortAPIServer, "Listen on given port")
	return cmd
}
