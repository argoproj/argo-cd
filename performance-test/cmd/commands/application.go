package commands

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/performance-test/generators"
)

func NewApplicationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "application",
		Short: "Manage applications",
		Long:  "Manage applications",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationGenerationCommand(opts))
	command.AddCommand(NewApplicationCleanCommand(opts))
	return command
}

func NewApplicationGenerationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "generate",
		Short: "Generate applications",
		Long:  "Generate applications",
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

func NewApplicationCleanCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean applications",
		Long:  "Clean applications",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewApplicationGenerator(generator.ConnectToK8s())
			err := pg.Clean()
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
