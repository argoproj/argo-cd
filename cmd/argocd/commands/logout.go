package commands

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

// NewLogoutCommand returns a new instance of `argocd logout` command
func NewLogoutCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var allContexts bool
	command := &cobra.Command{
		Use:   "logout CONTEXT",
		Short: "Log out from Argo CD",
		Long:  "Log out from Argo CD",
		Example: `# Logout from the active Argo CD context
# This can be helpful for security reasons or when you want to switch between different Argo CD contexts or accounts.
argocd logout CONTEXT

# Logout from a specific context named 'cd.argoproj.io'
argocd logout cd.argoproj.io
`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				if !allContexts {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				err := logoutAllContext(globalClientOpts)
				if err != nil {
					log.Fatalf("error while logging out from all contexts")
				}
			} else {
				// TODO: What if we have multiple arguments, not only one?
				context := args
				err := logoutContext(globalClientOpts, context)
				if err != nil {
					log.Fatalf("error while logging out from context '%s'. %s'", context, err)
				}
			}
		},
	}
	command.Flags().BoolVar(&allContexts, "all", false, "To log out from all Argo CD Contexts")
	return command
}

func logoutAllContext(globalClientOpts *argocdclient.ClientOptions) error {
	localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
	errors.CheckError(err)
	if localCfg == nil {
		return fmt.Errorf("nothing to logout from")
	}

	for _, contextRef := range localCfg.Contexts {
		context, err := localCfg.ResolveContext(contextRef.Name)
		if err != nil {
			return fmt.Errorf("context '%s' had error: %v", contextRef.Name, err)
		}

		ok := localCfg.RemoveToken(context.Server.Server)
		if !ok {
			return fmt.Errorf("context '%s' does not exist", context.Server.Server)
		}
		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return fmt.Errorf("error in logging out: %s", err)
		}
		err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
		errors.CheckError(err)

		fmt.Printf("Logged out from '%s'\n", context.Name)
	}
	return nil
}

func logoutContext(globalClientOpts *argocdclient.ClientOptions, contexts []string) error {
	localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
	errors.CheckError(err)
	if localCfg == nil {
		return fmt.Errorf("nothing to logout from")
	}

	for _, c := range contexts {
		ctxRef, err := localCfg.GetContext(c)
		if err != nil {
			return fmt.Errorf("context '%s' undefined", c)
		}
		localCfg.UpsertContext(*ctxRef)

		context, err := localCfg.ResolveContext(ctxRef.Name)
		if err != nil {
			return fmt.Errorf("couldn't resolve context '%s'", ctxRef.Name)
		}
		//if canLogout {
		ok := localCfg.RemoveToken(context.Server.Server)
		if !ok {
			return fmt.Errorf("context %s does not exist", context.Server.Server)
		}

		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return fmt.Errorf("error in logging out: %s", err)
		}
		err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
		errors.CheckError(err)

		fmt.Printf("Logged out from '%s'\n", context.Name)
	}
	return nil
}
