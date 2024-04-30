package admin

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/cache"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

func NewDashboardCommand() *cobra.Command {
	var (
		port           int
		address        string
		compressionStr string
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			compression, err := cache.CompressionTypeFromString(compressionStr)
			errors.CheckError(err)
			errors.CheckError(headless.StartLocalServer(ctx, &argocdclient.ClientOptions{Core: true}, initialize.RetrieveContextIfChanged(cmd.Flag("context")), &port, &address, compression))
			println(fmt.Sprintf("Argo CD UI is available at http://%s:%d", address, port))
			<-ctx.Done()
		},
	}
	initialize.InitCommand(cmd)
	cmd.Flags().IntVar(&port, "port", common.DefaultPortAPIServer, "Listen on given port")
	cmd.Flags().StringVar(&address, "address", common.DefaultAddressAdminDashboard, "Listen on given address")
	cmd.Flags().StringVar(&compressionStr, "redis-compress", env.StringFromEnv("REDIS_COMPRESSION", string(cache.RedisCompressionGZip)), "Enable this if the application controller is configured with redis compression enabled. (possible values: gzip, none)")
	return cmd
}
