package commands

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/utils"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Example: `# To log out of argocd
$ argocd logout
# This can be helpful for security reasons or when you want to switch between different Argo CD contexts or accounts.
`,
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

			promptUtil := utils.NewPrompt(globalClientOpts.PromptsEnabled)

			canLogout := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to log out from '%s'?", context))
			if canLogout {
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
			} else {
				log.Infof("Logout from '%s' cancelled", context)
			}
		},
	}
	return command
}
