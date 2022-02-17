package admin

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

func NewDashboardCommand() *cobra.Command {
	var (
		port    int
		address string
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, args []string) {
			errors.CheckError(headless.StartLocalServer(&argocdclient.ClientOptions{Core: true}, initialize.RetrieveContextIfChanged(cmd.Flag("context")), &port, &address))
			println(fmt.Sprintf("Argo CD UI is available at http://%s:%d", address, port))
			<-context.Background().Done()
		},
	}
	initialize.InitCommand(cmd)
	cmd.Flags().IntVar(&port, "port", common.DefaultPortAPIServer, "Listen on given port")
	cmd.Flags().StringVar(&address, "address", common.DefaultAddressAPIServer, "Listen on given address")
	return cmd
}
