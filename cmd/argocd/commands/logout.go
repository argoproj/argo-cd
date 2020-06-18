package commands

import (
	"fmt"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
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
				errors.CheckErrorWithCode(fmt.Errorf("Nothing to logout from"), errors.ErrorCommandSpecific)
			}

			ok := localCfg.RemoveToken(context)
			if !ok {
				errors.CheckErrorWithCode(fmt.Errorf("Context %s does not exist", context), errors.ErrorCommandSpecific)
			}

			err = localconfig.ValidateLocalConfig(*localCfg)
			if err != nil {
				errors.CheckErrorWithCode(fmt.Errorf("Error in logging out: %s", err), errors.ErrorCommandSpecific)
			}
			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckErrorWithCode(err, errors.ErrorCommandSpecific)

			fmt.Printf("Logged out from '%s'\n", context)
		},
	}
	return command
}
