package commands

import (
	"log"

	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/performance-test/generators"
)

func NewApplicationGenerationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "application",
		Short: "Generate application",
		Long:  "Generate application",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewApplicationGenerator(generator.ConnectToK8s())
			err := pg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
