package commands

import (
	"context"
	"log"
	"os"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"
	"github.com/argoproj/argo-cd/v2/hack/gen-resources/util"

	"github.com/spf13/cobra"
)

func NewClusterCommand(opts *util.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters",
		Long:  "Manage clusters",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewClusterGenerationCommand(opts))
	command.AddCommand(NewClusterCleanCommand(opts))
	return command
}

func NewClusterGenerationCommand(opts *util.GenerateOpts) *cobra.Command {
	var file string
	var command = &cobra.Command{
		Use:   "generate",
		Short: "Generate cluster",
		Long:  "Generate cluster",
		Run: func(c *cobra.Command, args []string) {
			err := util.Parse(opts, file)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}

			clientSet := util.ConnectToK8sClientSet()

			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, opts.Namespace)
			argoDB := db.NewDB(opts.Namespace, settingsMgr, clientSet)

			cg := generator.NewClusterGenerator(argoDB, clientSet, util.ConnectToK8sConfig())
			err = cg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	command.Flags().StringVarP(&file, "file", "f", "", "")
	return command
}

func NewClusterCleanCommand(opts *util.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean clusters",
		Long:  "Clean clusters",
		Run: func(c *cobra.Command, args []string) {
			pg := generator.NewProjectGenerator(util.ConnectToK8sArgoClientSet())
			err := pg.Clean(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
