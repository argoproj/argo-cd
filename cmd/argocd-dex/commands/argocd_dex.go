package commands

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/argoproj/argo-cd/v2/common"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/dex"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/argo-cd/v2/util/tls"
)

const (
	cliName = "argocd-dex"
)

func NewCommand() *cobra.Command {
	command := &cobra.Command{
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
	return command
}

func NewRunDexCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		disableTLS   bool
	)
	command := cobra.Command{
		Use:   "rundex",
		Short: "Runs dex generating a config using settings from the Argo CD configmap and secret",
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			vers := common.GetVersion()
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			vers.LogStartupInfo(
				"ArgoCD Dex Server",
				map[string]any{
					"namespace": namespace,
				},
			)

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)
			_, err = exec.LookPath("dex")
			errors.CheckError(err)
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			config.UserAgent = fmt.Sprintf("argocd-dex/%s (%s)", vers.Version, vers.Platform)
			kubeClientset := kubernetes.NewForConfigOrDie(config)

			if !disableTLS {
				config, err := tls.CreateServerTLSConfig("/tls/tls.crt", "/tls/tls.key", []string{"localhost", "dexserver"})
				if err != nil {
					log.Fatalf("could not create TLS config: %v", err)
				}
				certPem, keyPem := tls.EncodeX509KeyPair(config.Certificates[0])
				err = os.WriteFile("/tmp/tls.crt", certPem, 0o600)
				if err != nil {
					log.Fatalf("could not write TLS certificate: %v", err)
				}
				err = os.WriteFile("/tmp/tls.key", keyPem, 0o600)
				if err != nil {
					log.Fatalf("could not write TLS key: %v", err)
				}
			}

			settingsMgr := settings.NewSettingsManager(ctx, kubeClientset, namespace)
			prevSettings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			updateCh := make(chan *settings.ArgoCDSettings, 1)
			settingsMgr.Subscribe(updateCh)

			for {
				var cmd *exec.Cmd
				dexCfgBytes, err := dex.GenerateDexConfigYAML(prevSettings, disableTLS)
				errors.CheckError(err)
				if len(dexCfgBytes) == 0 {
					log.Infof("dex is not configured")
				} else {
					err = os.WriteFile("/tmp/dex.yaml", dexCfgBytes, 0o644)
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
					newDexCfgBytes, err := dex.GenerateDexConfigYAML(newSettings, disableTLS)
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
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_DEX_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_DEX_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().BoolVar(&disableTLS, "disable-tls", env.ParseBoolFromEnv("ARGOCD_DEX_SERVER_DISABLE_TLS", false), "Disable TLS on the HTTP endpoint")
	return &command
}

func NewGenDexConfigCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
		disableTLS   bool
	)
	command := cobra.Command{
		Use:   "gendexcfg",
		Short: "Generates a dex config from Argo CD settings",
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			cli.SetLogFormat(cmdutil.LogFormat)
			cli.SetLogLevel(cmdutil.LogLevel)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)
			settingsMgr := settings.NewSettingsManager(ctx, kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			dexCfgBytes, err := dex.GenerateDexConfigYAML(settings, disableTLS)
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
				err = os.WriteFile(out, dexCfgBytes, 0o644)
				errors.CheckError(err)
			}
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVar(&cmdutil.LogFormat, "logformat", env.StringFromEnv("ARGOCD_DEX_SERVER_LOGFORMAT", "text"), "Set the logging format. One of: text|json")
	command.Flags().StringVar(&cmdutil.LogLevel, "loglevel", env.StringFromEnv("ARGOCD_DEX_SERVER_LOGLEVEL", "info"), "Set the logging level. One of: debug|info|warn|error")
	command.Flags().StringVarP(&out, "out", "o", "", "Output to the specified file instead of stdout")
	command.Flags().BoolVar(&disableTLS, "disable-tls", env.ParseBoolFromEnv("ARGOCD_DEX_SERVER_DISABLE_TLS", false), "Disable TLS on the HTTP endpoint")
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
