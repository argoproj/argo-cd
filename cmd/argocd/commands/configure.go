package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/localconfig"
)

// NewConfigureCommand returns a new instance of an `argocd configure` command
func NewConfigureCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var promptsEnabled bool

	command := &cobra.Command{
		Use:   "configure",
		Short: "Manage local configuration",
		Example: `# Enable optional interactive prompts
argocd configure --prompts-enabled
argocd configure --prompts-enabled=true

# Disable optional interactive prompts
argocd configure --prompts-enabled=false`,
		RunE: func(c *cobra.Command, _ []string) error {
			localCfg, err := localconfig.ReadLocalConfig(clientOpts.ConfigPath)
			if err != nil {
				return util.NewExitError(errors.ErrorGeneric, err)
			}

			if localCfg == nil {
				fmt.Fprintln(c.OutOrStdout(), "No local configuration found")
				return util.NewExitError(1, nil)
			}

			localCfg.PromptsEnabled = promptsEnabled

			err = localconfig.WriteLocalConfig(*localCfg, clientOpts.ConfigPath)
			if err != nil {
				return util.NewExitError(errors.ErrorGeneric, err)
			}

			fmt.Fprintln(c.OutOrStdout(), "Successfully updated the following configuration settings:")
			fmt.Fprintf(c.OutOrStdout(), "prompts-enabled: %v\n", strconv.FormatBool(localCfg.PromptsEnabled))

			return nil
		},
	}

	command.Flags().BoolVar(&promptsEnabled, "prompts-enabled", localconfig.GetPromptsEnabled(false), "Enable (or disable) optional interactive prompts")

	return command
}
