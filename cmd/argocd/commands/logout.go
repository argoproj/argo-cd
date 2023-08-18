package commands

import (
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			context := args[0]

			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)
			if localCfg == nil {
				log.Fatalf("Nothing to logout from")
			}

			token := localCfg.GetToken(context)
			if token == "" {
				log.Fatalf("Error in getting token from context")
			}

			prefix := "http"
			if !globalClientOpts.Insecure {
				prefix += "s"
			}

			logoutURL := fmt.Sprintf("%s://%s%s", prefix, context, common.LogoutEndpoint)
			req, err := http.NewRequest("POST", logoutURL, nil)
			errors.CheckError(err)
			cookie := &http.Cookie{
				Name:  common.AuthCookieName,
				Value: token,
			}
			req.AddCookie(cookie)

			client := &http.Client{}
			_, err = client.Do(req)
			errors.CheckError(err)

			ok := localCfg.RemoveToken(context)
			if !ok {
				log.Fatalf("Context %s does not exist", context)
			}

			err = localconfig.ValidateLocalConfig(*localCfg)
			if err != nil {
				log.Fatalf("Error in logging out: %s", err)
			}

			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)

			fmt.Printf("Logged out from '%s'\n", context)
		},
	}
	return command
}
