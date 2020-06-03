package commands

import (
	"fmt"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/util/localconfig"
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
				os.Exit(errors.ErrorCommandSpecific)
			}
			context := args[0]

			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)
			if localCfg == nil {
				errors.Fatalf("Nothing to logout from")
			}

			ok := localCfg.RemoveToken(context)
			if !ok {
				errors.Fatalf("Context %s does not exist", context)
			}

			err = localconfig.ValidateLocalConfig(*localCfg)
			if err != nil {
				errors.Fatalf("Error in logging out: %s", err)
			}
			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)

			fmt.Printf("Logged out from '%s'\n", context)
		},
	}
	return command
}
