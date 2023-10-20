package commands

import (
	"os"
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/hack/benchmark/benchmark"
	"github.com/argoproj/argo-cd/v2/hack/benchmark/env"
	"github.com/argoproj/argo-cd/v2/hack/benchmark/util"

	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/cli"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
)

const (
	cliName = "argocd-benchmark"
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
	var command = &cobra.Command{
		Use:   cliName,
		Short: "Benchmark for argocd",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
		DisableAutoGenTag: true,
	}

	command.AddCommand(NewBenchmarkCommand())
	command.AddCommand(NewBuildEnvCommand())

	return command
}

func NewBenchmarkCommand() *cobra.Command {
	var namespace string
	var testtype string
	var command = &cobra.Command{
		Use:   "benchmark",
		Short: "Run a benchmark",
		Long:  "Run a benchmark",
		Run: func(c *cobra.Command, args []string) {
			clientSet := util.ConnectToK8sClientSet()
			argoClientSet := util.ConnectToK8sArgoClientSet()
			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, namespace)
			argoDB := db.NewDB(namespace, settingsMgr, clientSet)
			clusters,_ := util.GetClusterList(argoDB)
			
			if testtype == "synctest" {
				_,err := benchmark.SyncTest(clientSet, argoClientSet, argoDB, namespace, clusters)
				if err != nil {
					log.Printf("%v", err)
				}
			}	else {
				log.Print("Invalid parameters.")
			}
			os.Exit(0)
		},
	}
	command.Flags().StringVar(&namespace, "namespace", "argocd", "")
	command.Flags().StringVar(&testtype, "testtype", "synctest", "")
	return command
}

func NewBuildEnvCommand() *cobra.Command {
	var numapps int
	var appdist string
	var namespace string
	var command = &cobra.Command{
		Use:   "buildenv",
		Short: "Build benchmark environment",
		Long:  "Build benchmark environment",
		Run: func(c *cobra.Command, args []string) {
			clientSet := util.ConnectToK8sClientSet()
			argoClientSet := util.ConnectToK8sArgoClientSet()
			settingsMgr := settings.NewSettingsManager(context.TODO(), clientSet, namespace)
			argoDB := db.NewDB(namespace, settingsMgr, clientSet)
			clusters,_ := util.GetClusterList(argoDB)
			
			if numapps != 0 {
				_,err := env.BuildEnv(clientSet, argoClientSet, numapps, appdist, namespace, clusters)
				if err != nil {
					log.Printf("%v", err)
				}
			} else {
				log.Print("Missing required parameters for building environment.")
			}
			os.Exit(0)
		},
	}
	command.Flags().IntVar(&numapps, "numapps", 0, "")
	command.Flags().StringVar(&appdist, "appdist", "equal", "")
	command.Flags().StringVar(&namespace, "namespace", "argocd", "")
	return command
}