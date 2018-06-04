package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/ghodss/yaml"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/dex"
	"github.com/argoproj/argo-cd/util/settings"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const (
	// CLIName is the name of the CLI
	cliName = "argocd-util"
)

// NewCommand returns a new instance of an argocd command
func NewCommand() *cobra.Command {
	var (
		logLevel string
	)

	var command = &cobra.Command{
		Use:   cliName,
		Short: "argocd-util has internal tools used by ArgoCD",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(cli.NewVersionCmd(cliName))
	command.AddCommand(NewRunDexCommand())
	command.AddCommand(NewGenDexConfigCommand())
	command.AddCommand(NewExportCommand())

	command.Flags().StringVar(&logLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}

func NewRunDexCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = cobra.Command{
		Use:   "rundex",
		Short: "Runs dex generating a config using settings from the ArgoCD configmap and secret",
		RunE: func(c *cobra.Command, args []string) error {
			_, err := exec.LookPath("dex")
			errors.CheckError(err)
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			ctx := context.Background()
			settingsMgr.StartNotifier(ctx, settings)
			updateCh := make(chan struct{}, 1)
			settingsMgr.Subscribe(updateCh)

			for {
				var cmd *exec.Cmd
				dexCfgBytes, err := dex.GenerateDexConfigYAML(settings)
				errors.CheckError(err)
				if len(dexCfgBytes) == 0 {
					log.Infof("dex is not configured")
				} else {
					err = ioutil.WriteFile("/tmp/dex.yaml", dexCfgBytes, 0644)
					errors.CheckError(err)
					log.Info(string(dexCfgBytes))
					cmd = exec.Command("dex", "serve", "/tmp/dex.yaml")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Start()
					errors.CheckError(err)
				}

				// loop until the dex config changes
				for {
					<-updateCh
					newDexCfgBytes, err := dex.GenerateDexConfigYAML(settings)
					errors.CheckError(err)
					if string(newDexCfgBytes) != string(dexCfgBytes) {
						log.Infof("dex config modified. restarting dex")
						if cmd != nil && cmd.Process != nil {
							err = cmd.Process.Signal(syscall.SIGTERM)
							errors.CheckError(err)
							_, err = cmd.Process.Wait()
							errors.CheckError(err)
						}
						break
					} else {
						log.Infof("dex config unmodified")
					}
				}
			}
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	return &command
}

func NewGenDexConfigCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	var command = cobra.Command{
		Use:   "gendexcfg",
		Short: "Generates a dex config from ArgoCD settings",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			dexCfgBytes, err := dex.GenerateDexConfigYAML(settings)
			errors.CheckError(err)
			if len(dexCfgBytes) == 0 {
				log.Infof("dex is not configured")
				return nil
			}
			if out == "" {
				fmt.Printf(string(dexCfgBytes))
			} else {
				err = ioutil.WriteFile(out, dexCfgBytes, 0644)
				errors.CheckError(err)
			}
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "", "Output to the specified file instead of stdout")
	return &command
}

// NewExportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewExportCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
		clientOpts   argocdclient.ClientOptions
	)
	var command = cobra.Command{
		Use:   "export",
		Short: "Export all Argo CD data",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)

			settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			settingsData, err := yaml.Marshal(settings)
			errors.CheckError(err)

			db := db.NewDB(namespace, kubeClientset)
			clusters, err := db.ListClusters(context.Background())
			errors.CheckError(err)
			clusterData, err := yaml.Marshal(clusters)
			errors.CheckError(err)

			repos, err := db.ListRepositories(context.Background())
			errors.CheckError(err)
			repoData, err := yaml.Marshal(repos)
			errors.CheckError(err)

			conn, appIf := argocdclient.NewClientOrDie(&clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
			errors.CheckError(err)
			appsData, err := yaml.Marshal(apps)
			errors.CheckError(err)

			outputStrings := []string{
				string(settingsData),
				string(repoData),
				string(clusterData),
				string(appsData),
			}
			output := fmt.Sprint(strings.Join(outputStrings, "\n---\n"))

			if out == "" {
				fmt.Println(output)
			} else {
				err = ioutil.WriteFile(out, []byte(output), 0644)
				errors.CheckError(err)
			}
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "", "Output to the specified file instead of stdout")
	command.Flags().StringVar(&clientOpts.ConfigPath, "config", "", "Path to ArgoCD config")
	command.Flags().StringVar(&clientOpts.ServerAddr, "server", "", "ArgoCD server address")
	command.Flags().BoolVar(&clientOpts.PlainText, "plaintext", false, "Disable TLS")
	command.Flags().BoolVar(&clientOpts.Insecure, "insecure", false, "Skip server certificate and domain verification")
	command.Flags().StringVar(&clientOpts.CertFile, "server-crt", "", "Server certificate file")
	command.Flags().StringVar(&clientOpts.AuthToken, "auth-token", "", "Authentication token")

	return &command
}

func main() {
	if err := NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
