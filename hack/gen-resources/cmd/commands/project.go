package commands

import (
	"log"
	"os"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/tools"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"

	"github.com/spf13/cobra"
)

func NewProjectCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "project",
		Short: "Manage applications",
		Long:  "Manage applications",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectGenerationCommand(opts))
	command.AddCommand(NewProjectCleanCommand(opts))
	return command
}

func NewProjectGenerationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "generate",
		Short: "Generate project",
		Long:  "Generate project",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewProjectGenerator(tools.ConnectToK8sArgoClientSet())
			err := pg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}

func NewProjectCleanCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean projects",
		Long:  "Clean projects",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewProjectGenerator(tools.ConnectToK8sArgoClientSet())
			err := pg.Clean()
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
