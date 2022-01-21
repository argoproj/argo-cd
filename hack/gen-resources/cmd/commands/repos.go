package commands

import (
	"log"
	"os"

	"github.com/argoproj/argo-cd/v2/hack/gen-resources/tools"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"

	"github.com/spf13/cobra"
)

func NewReposCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "repos",
		Short: "Manage repos",
		Long:  "Manage repos",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewReposGenerationCommand(opts))
	command.AddCommand(NewReposCleanCommand(opts))
	return command
}

func NewReposGenerationCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "generate",
		Short: "Generate repos",
		Long:  "Generate repos",
		Run: func(c *cobra.Command, args []string) {
			rg := generator.NewRepoGenerator(tools.ConnectToK8sClientSet())
			err := rg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	command.PersistentFlags().StringVar(&opts.GithubToken, "token", "", "Github token")
	command.PersistentFlags().IntVar(&opts.Samples, "samples", 1, "Amount of samples")
	return command
}

func NewReposCleanCommand(opts *generator.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean repos",
		Long:  "Clean repos",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewRepoGenerator(tools.ConnectToK8sClientSet())
			err := pg.Clean(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
