package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
)

// NewConfigureCommand returns a new instance of an `argocd configure` command
func NewConfigureCommand(globalClientOpts *argocdclient.ClientOptions) *cobra.Command {
	var promptsEnabled bool

	command := &cobra.Command{
		Use:   "configure",
		Short: "Manage local configuration",
		Example: `# Enable optional interactive prompts
argocd configure --prompts-enabled
argocd configure --prompts-enabled=true

# Disable optional interactive prompts
argocd configure --prompts-enabled=false`,
		Run: func(c *cobra.Command, args []string) {
			localCfg, err := localconfig.ReadLocalConfig(globalClientOpts.ConfigPath)
			errors.CheckError(err)

			localCfg.PromptsEnabled = promptsEnabled

			err = localconfig.WriteLocalConfig(*localCfg, globalClientOpts.ConfigPath)
			errors.CheckError(err)

			fmt.Println("Successfully updated the following configuration settings:")
			fmt.Printf("prompts-enabled: %v\n", strconv.FormatBool(localCfg.PromptsEnabled))
		},
	}

	command.Flags().BoolVar(&promptsEnabled, "prompts-enabled", localconfig.GetPromptsEnabled(false), "Enable (or disable) optional interactive prompts")

	return command
}
