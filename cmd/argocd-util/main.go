package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/dex"
	"github.com/argoproj/argo-cd/util/settings"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/util/password"
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
	command.AddCommand(NewResetAdminPasswordCommand())

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
			dexPath, err := exec.LookPath("dex")
			errors.CheckError(err)
			dexCfgBytes, err := genDexConfig(clientConfig)
			errors.CheckError(err)
			if len(dexCfgBytes) == 0 {
				log.Infof("dex is not configured")
				// need to sleep forever since we run as a sidecar and kubernetes does not permit
				// containers in a deployment to have restartPolicy anything other than Always.
				// TODO: we should watch for a change in the dex.config key in the config-map
				// to restart dex when there is a change (e.g. clientID and clientSecretKey changed)
				select {}
			}
			err = ioutil.WriteFile("/tmp/dex.yaml", dexCfgBytes, 0644)
			errors.CheckError(err)
			log.Info(string(dexCfgBytes))
			return syscall.Exec(dexPath, []string{"dex", "serve", "/tmp/dex.yaml"}, []string{})
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
			dexCfgBytes, err := genDexConfig(clientConfig)
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

func NewResetAdminPasswordCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		passwordRaw  string
	)
	var command = cobra.Command{
		Use:   "resetadmin",
		Short: "Reset admin password",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			kubeclientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			settingsMgr := settings.NewSettingsManager(kubeclientset, namespace)
			argoCDSettings, err := settingsMgr.GetSettings()

			if apierr.IsNotFound(err) {
				argoCDSettings = &settings.ArgoCDSettings{}
			} else {
				log.Fatal(err)
			}

			if argoCDSettings.LocalUsers == nil {
				argoCDSettings.LocalUsers = make(map[string]string)
			}

			if passwordRaw == "" {
				passwordRaw = password.ReadAndConfirmAdminPassword()
			}
			hashedPassword, err := password.HashPassword(passwordRaw)
			errors.CheckError(err)
			argoCDSettings.LocalUsers = map[string]string{
				common.ArgoCDAdminUsername: hashedPassword,
			}
			err = settingsMgr.SaveSettings(argoCDSettings)
			errors.CheckError(err)
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVar(&passwordRaw, "password", "", "admin password")
	return &command
}

func genDexConfig(clientConfig clientcmd.ClientConfig) ([]byte, error) {
	config, err := clientConfig.ClientConfig()
	errors.CheckError(err)
	namespace, _, err := clientConfig.Namespace()
	errors.CheckError(err)

	kubeClient := kubernetes.NewForConfigOrDie(config)
	return dex.GenerateDexConfigYAML(kubeClient, namespace)
}

func main() {
	if err := NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
