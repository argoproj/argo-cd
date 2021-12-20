package commands

import (
	"log"

	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/performance-test/generators"
)

// NewLogoutCommand returns a new instance of `argocd-generator project` command
func NewProjectGenerationCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "project",
		Short: "Generate project",
		Long:  "Generate project",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewProjectGenerator(generator.ConnectToK8s())
			err := pg.Generate()
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
