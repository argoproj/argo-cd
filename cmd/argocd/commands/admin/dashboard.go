package admin

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/initialize"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

func NewDashboardCommand() *cobra.Command {
	var (
		port               int
		address            string
		repoServerName     string
		redisHaHaProxyName string
		redisName          string
		serverName         string
	)
	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Starts Argo CD Web UI locally",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			errors.CheckError(headless.StartLocalServer(ctx, &argocdclient.ClientOptions{
				Core:               true,
				ServerName:         serverName,
				RedisHaHaProxyName: redisHaHaProxyName,
				RedisName:          redisName,
				RepoServerName:     repoServerName},
				initialize.RetrieveContextIfChanged(cmd.Flag("context")), &port, &address))
			println(fmt.Sprintf("Argo CD UI is available at http://%s:%d", address, port))
			<-ctx.Done()
		},
	}
	initialize.InitCommand(cmd)
	cmd.Flags().IntVar(&port, "port", common.DefaultPortAPIServer, "Listen on given port")
	cmd.Flags().StringVar(&address, "address", common.DefaultAddressAPIServer, "Listen on given address")
	cmd.Flags().StringVar(&serverName, "server-name", env.StringFromEnv(common.EnvServerName, common.DefaultServerName), "Server name")
	cmd.Flags().StringVar(&redisHaHaProxyName, "redis-ha-haproxy-name", env.StringFromEnv(common.EnvRedisHaHaproxyName, common.DefaultRedisHaHaproxyName), "Redis HA HAProxy name")
	cmd.Flags().StringVar(&redisName, "redis-name", env.StringFromEnv(common.EnvRedisName, common.DefaultRedisName), "Redis name")
	cmd.Flags().StringVar(&repoServerName, "repo-server-name", env.StringFromEnv(common.EnvRepoServerName, common.DefaultRepoServerName), "Repo server name")
	return cmd
}
