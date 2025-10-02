package commands

import (
	"github.com/njayp/ophis"
	"github.com/spf13/cobra"
)

func NewMCPCommand() *cobra.Command {
	return ophis.Command(&ophis.Config{
		Selectors: []ophis.Selector{
			{
				CmdSelector: ophis.AllowCmd(
					"argocd app get",
					"argocd app list",
					"argocd app diff",
					"argocd app manifests",
					"argocd app history",
					"argocd app resources",
					"argocd app logs",
					"argocd app sync",
					"argocd app wait",
					"argocd app rollback",
					"argocd cluster list",
					"argocd proj list",
					"argocd repo list",
				),

				// No inherited flags
				InheritedFlagSelector: ophis.NoFlags,
			},
		},
	})
}
