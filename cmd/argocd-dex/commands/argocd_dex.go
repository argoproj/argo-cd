package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	"github.com/argoproj/argo-cd/v2/common"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/dex"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	cliName = "argocd-dex"
)

func NewCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:               cliName,
		Short:             "argocd-dex tools used by Argo CD",
		Long:              "argocd-dex has internal utility tools used by Argo CD",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewRunDexCommand())
	command.AddCommand(NewGenDexConfigCommand())

	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", "text", "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", "info", "Set the logging level. One of: debug|info|warn|error")
	return command
}

func NewRunDexCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = cobra.Command{
		Use:   "rundex",
		Short: "Runs dex generating a config using settings from the Argo CD configmap and secret",
		RunE: func(c *cobra.Command, args []string) error {
			_, err := exec.LookPath("dex")
			errors.CheckError(err)
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			vers := common.GetVersion()
			config.UserAgent = fmt.Sprintf("argocd-dex/%s (%s)", vers.Version, vers.Platform)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)

			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
			prevSettings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			updateCh := make(chan *settings.ArgoCDSettings, 1)
			settingsMgr.Subscribe(updateCh)

			for {
				var cmd *exec.Cmd
				dexCfgBytes, err := dex.GenerateDexConfigYAML(prevSettings)
				errors.CheckError(err)
				if len(dexCfgBytes) == 0 {
					log.Infof("dex is not configured")
				} else {
					err = ioutil.WriteFile("/tmp/dex.yaml", dexCfgBytes, 0644)
					errors.CheckError(err)
					log.Debug(redactor(string(dexCfgBytes)))
					cmd = exec.Command("dex", "serve", "/tmp/dex.yaml")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Start()
					errors.CheckError(err)
				}

				// loop until the dex config changes
				for {
					newSettings := <-updateCh
					newDexCfgBytes, err := dex.GenerateDexConfigYAML(newSettings)
					errors.CheckError(err)
					if string(newDexCfgBytes) != string(dexCfgBytes) {
						prevSettings = newSettings
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
		Short: "Generates a dex config from Argo CD settings",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			dexCfgBytes, err := dex.GenerateDexConfigYAML(settings)
			errors.CheckError(err)
			if len(dexCfgBytes) == 0 {
				log.Infof("dex is not configured")
				return nil
			}
			if out == "" {
				dexCfg := make(map[string]interface{})
				err := yaml.Unmarshal(dexCfgBytes, &dexCfg)
				errors.CheckError(err)
				if staticClientsInterface, ok := dexCfg["staticClients"]; ok {
					if staticClients, ok := staticClientsInterface.([]interface{}); ok {
						for i := range staticClients {
							staticClient := staticClients[i]
							if mappings, ok := staticClient.(map[string]interface{}); ok {
								for key := range mappings {
									if key == "secret" {
										mappings[key] = "******"
									}
								}
								staticClients[i] = mappings
							}
						}
						dexCfg["staticClients"] = staticClients
					}
				}
				errors.CheckError(err)
				maskedDexCfgBytes, err := yaml.Marshal(dexCfg)
				errors.CheckError(err)
				fmt.Print(string(maskedDexCfgBytes))
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

func iterateStringFields(obj interface{}, callback func(name string, val string) string) {
	if mapField, ok := obj.(map[string]interface{}); ok {
		for field, val := range mapField {
			if strVal, ok := val.(string); ok {
				mapField[field] = callback(field, strVal)
			} else {
				iterateStringFields(val, callback)
			}
		}
	} else if arrayField, ok := obj.([]interface{}); ok {
		for i := range arrayField {
			iterateStringFields(arrayField[i], callback)
		}
	}
}

func redactor(dirtyString string) string {
	config := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(dirtyString), &config)
	errors.CheckError(err)
	iterateStringFields(config, func(name string, val string) string {
		if name == "clientSecret" || name == "secret" || name == "bindPW" {
			return "********"
		} else {
			return val
		}
	})
	data, err := yaml.Marshal(config)
	errors.CheckError(err)
	return string(data)
}
