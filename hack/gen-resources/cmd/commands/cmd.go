package commands

import (
	"context"
	"log"

	"github.com/spf13/cobra"

	generator "github.com/argoproj/argo-cd/v2/hack/gen-resources/generators"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/hack/gen-resources/util"
	"github.com/argoproj/argo-cd/v2/util/cli"
)

const (
	cliName = "argocd-generator"
)

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	cli.SetLogFormat(cmdutil.LogFormat)
	cli.SetLogLevel(cmdutil.LogLevel)
}

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {

	var generateOpts util.GenerateOpts

	var command = &cobra.Command{
		Use:   cliName,
		Short: "Generator for argocd resources",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
	}

	command.AddCommand(NewGenerateCommand(&generateOpts))
	command.AddCommand(NewCleanCommand(&generateOpts))

	command.PersistentFlags().StringVar(&generateOpts.Namespace, "kube-namespace", "argocd", "Name of the namespace on which Argo agent should be installed [$KUBE_NAMESPACE]")
	return command
}

func NewGenerateCommand(opts *util.GenerateOpts) *cobra.Command {
	var file string
	var command = &cobra.Command{
		Use:   "generate [-f file]",
		Short: "Generate entities",
		Long:  "Generate entities",
		Run: func(c *cobra.Command, args []string) {
			log.Printf("Retrieve configuration from %s", file)
			err := util.Parse(opts, file)
			if err != nil {
				log.Fatalf("Failed to retrieve configuration, %v", err.Error())
			}
			argoClientSet := util.ConnectToK8sArgoClientSet()
			clientSet := util.ConnectToK8sClientSet()

			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, opts.Namespace)
			argoDB := db.NewDB(opts.Namespace, settingsMgr, clientSet)

			pg := generator.NewProjectGenerator(argoClientSet)
			ag := generator.NewApplicationGenerator(argoClientSet, clientSet, argoDB)
			rg := generator.NewRepoGenerator(util.ConnectToK8sClientSet())
			cg := generator.NewClusterGenerator(argoDB, util.ConnectToK8sClientSet(), util.ConnectToK8sConfig())

			err = pg.Generate(opts)
			if err != nil {
				log.Fatalf("Failed to generate projects, %v", err.Error())
			}
			err = rg.Generate(opts)
			if err != nil {
				log.Fatalf("Failed to generate repositories, %v", err.Error())
			}
			err = cg.Generate(opts)
			if err != nil {
				log.Fatalf("Failed to generate clusters, %v", err.Error())
			}
			err = ag.Generate(opts)
			if err != nil {
				log.Fatalf("Failed to generate applications, %v", err.Error())
			}
		},
	}
	command.Flags().StringVarP(&file, "file", "f", "", "")
	return command
}

func NewCleanCommand(opts *util.GenerateOpts) *cobra.Command {
	var command = &cobra.Command{
		Use:   "clean",
		Short: "Clean entities",
		Long:  "Clean entities",
		Run: func(c *cobra.Command, args []string) {
			argoClientSet := util.ConnectToK8sArgoClientSet()
			clientSet := util.ConnectToK8sClientSet()
			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, opts.Namespace)
			argoDB := db.NewDB(opts.Namespace, settingsMgr, clientSet)

			pg := generator.NewProjectGenerator(argoClientSet)
			ag := generator.NewApplicationGenerator(argoClientSet, clientSet, argoDB)
			cg := generator.NewClusterGenerator(argoDB, clientSet, util.ConnectToK8sConfig())
			rg := generator.NewRepoGenerator(clientSet)

			err := pg.Clean(opts)
			if err != nil {
				log.Fatalf("Failed to clean projects, %v", err.Error())
			}
			err = ag.Clean(opts)
			if err != nil {
				log.Fatalf("Failed to clean applications, %v", err.Error())
			}
			err = cg.Clean(opts)
			if err != nil {
				log.Fatalf("Failed to clean clusters, %v", err.Error())
			}
			err = rg.Clean(opts)
			if err != nil {
				log.Fatalf("Failed to clean repositores, %v", err.Error())
			}
		},
	}
	return command
}
