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

func NewApplicationCommand(opts *util.GenerateOpts) *cobra.Command {
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

func NewApplicationGenerationCommand(opts *util.GenerateOpts) *cobra.Command {
	var file string
	var command = &cobra.Command{
		Use:   "generate [-f file]",
		Short: "Generate applications",
		Long:  "Generate applications",
		Run: func(c *cobra.Command, args []string) {
			err := util.Parse(opts, file)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
			argoClientSet := util.ConnectToK8sArgoClientSet()
			clientSet := util.ConnectToK8sClientSet()

			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, opts.Namespace)
			argoDB := db.NewDB(opts.Namespace, settingsMgr, clientSet)

			pg := generator.NewApplicationGenerator(argoClientSet, clientSet, argoDB)
			err = pg.Generate(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	command.Flags().StringVarP(&file, "file", "f", "", "")
	return command
}

func NewApplicationCleanCommand(opts *util.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean applications",
		Long:  "Clean applications",
		Run: func(c *cobra.Command, args []string) {
			argoClientSet := util.ConnectToK8sArgoClientSet()
			clientSet := util.ConnectToK8sClientSet()

			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, opts.Namespace)
			argoDB := db.NewDB(opts.Namespace, settingsMgr, clientSet)

			pg := generator.NewApplicationGenerator(argoClientSet, clientSet, argoDB)
			err := pg.Clean(opts)
			if err != nil {
				log.Fatalf("Something went wrong, %v", err.Error())
			}
		},
	}
	return command
}
