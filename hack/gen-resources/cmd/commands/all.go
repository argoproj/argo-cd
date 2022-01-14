package commands

import (
	"log"
	"os"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"
	"github.com/argoproj/argo-cd/v2/hack/gen-resources/tools"

	"github.com/spf13/cobra"
)

func NewAllResourcesCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "all",
		Short: "Manage all resources",
		Long:  "Manage all resources",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewAllResourcesGenerationCommand(opts))
	command.AddCommand(NewAllResourcesCleanCommand(opts))
	return command
}

func NewAllResourcesGenerationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "generate",
		Short: "Generate all resources",
		Long:  "Generate all resources",
		Run: func(c *cobra.Command, args []string) {
			clientSet := tools.ConnectToK8sArgoClientSet()
			pg := generator.NewProjectGenerator(clientSet)
			ag := generator.NewApplicationGenerator(clientSet)
			rg := generator.NewRepoGenerator(tools.ConnectToK8sClientSet())
			err := pg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
			err = ag.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
			err = rg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	command.PersistentFlags().IntVar(&opts.Samples, "samples", 1, "Amount of samples")
	return command
}

func NewAllResourcesCleanCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean all resources",
		Long:  "Clean all resources",
		Run: func(c *cobra.Command, args []string) {
			clientSet := tools.ConnectToK8sArgoClientSet()
			pg := generator.NewProjectGenerator(clientSet)
			ag := generator.NewApplicationGenerator(clientSet)
			err := pg.Clean(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
			err = ag.Clean(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
