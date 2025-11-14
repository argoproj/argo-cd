package commands

import (
	stderrors "errors"
	"fmt"

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

# Logout from multiple named contexts 'localhost:8080' and 'cd.argoproj.io'
argocd logout localhost:8080 cd.argoproj.io

# Logout from all contexts
argocd logout --all
`,
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) == 0 {
				if !allContexts {
					c.HelpFunc()(c, args)
					return stderrors.New("context name is required or use --all flag")
				}
				err := LogoutAllContext(globalClientOpts)
				if err != nil {
					return fmt.Errorf("error while logging out from all contexts. %w", err)
				}
			} else {
				context := args
				err := LogoutContext(globalClientOpts, context)
				if err != nil {
					return fmt.Errorf("error while logging out from context '%s': %w", context, err)
				}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&allContexts, "all", false, "To log out from all Argo CD Contexts")
	return command
}

func LogoutAllContext(globalClientOpts *argocdclient.ClientOptions) error {
	localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
	errors.CheckError(err)
	if localCfg == nil {
		return stderrors.New("nothing to logout from")
	}

	for _, contextRef := range localCfg.Contexts {
		context, err := localCfg.ResolveContext(contextRef.Name)
		if err != nil {
			return fmt.Errorf("context '%s' had error: %w", contextRef.Name, err)
		}

		ok := localCfg.RemoveToken(context.Server.Server)
		if !ok {
			return fmt.Errorf("context '%s' does not exist", context.Server.Server)
		}
		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return fmt.Errorf("error in logging out: %w", err)
		}
		err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
		errors.CheckError(err)

		fmt.Printf("Logged out from '%s'\n", context.Name)
	}
	return nil
}

func LogoutContext(globalClientOpts *argocdclient.ClientOptions, contexts []string) error {
	localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
	errors.CheckError(err)
	if localCfg == nil {
		return stderrors.New("nothing to logout from")
	}

	for _, c := range contexts {
		context, err := localCfg.ResolveContext(c)
		if err != nil {
			return fmt.Errorf("context '%s' undefined", c)
		}
		ok := localCfg.RemoveToken(context.Server.Server)
		if !ok {
			return fmt.Errorf("context %s does not exist", context.Server.Server)
		}

		err = localconfig.ValidateLocalConfig(*localCfg)
		if err != nil {
			return fmt.Errorf("error in logging out: %w", err)
		}
		err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
		errors.CheckError(err)

		fmt.Printf("Logged out from '%s'\n", context.Name)
	}
	return nil
}
