package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/dex"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// YamlSeparator separates sections of a YAML file
	yamlSeparator = "\n---\n"
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
	command.AddCommand(NewImportCommand())
	command.AddCommand(NewExportCommand())
	command.AddCommand(NewSettingsCommand())

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

// NewImportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewImportCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = cobra.Command{
		Use:   "import SOURCE",
		Short: "Import Argo CD data from stdin (specify `-') or a file",
		RunE: func(c *cobra.Command, args []string) error {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			var (
				input       []byte
				err         error
				newSettings *settings.ArgoCDSettings
				newRepos    []*v1alpha1.Repository
				newClusters []*v1alpha1.Cluster
				newApps     []*v1alpha1.Application
				newRBACCM   *apiv1.ConfigMap
			)

			if in := args[0]; in == "-" {
				input, err = ioutil.ReadAll(os.Stdin)
				errors.CheckError(err)
			} else {
				input, err = ioutil.ReadFile(in)
				errors.CheckError(err)
			}
			inputStrings := strings.Split(string(input), yamlSeparator)

			err = yaml.Unmarshal([]byte(inputStrings[0]), &newSettings)
			errors.CheckError(err)

			err = yaml.Unmarshal([]byte(inputStrings[1]), &newRepos)
			errors.CheckError(err)

			err = yaml.Unmarshal([]byte(inputStrings[2]), &newClusters)
			errors.CheckError(err)

			err = yaml.Unmarshal([]byte(inputStrings[3]), &newApps)
			errors.CheckError(err)

			err = yaml.Unmarshal([]byte(inputStrings[4]), &newRBACCM)
			errors.CheckError(err)

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)

			settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
			err = settingsMgr.SaveSettings(newSettings)
			errors.CheckError(err)
			db := db.NewDB(namespace, kubeClientset)

			_, err = kubeClientset.CoreV1().ConfigMaps(namespace).Create(newRBACCM)
			errors.CheckError(err)

			for _, repo := range newRepos {
				_, err := db.CreateRepository(context.Background(), repo)
				if err != nil {
					log.Warn(err)
				}
			}

			for _, cluster := range newClusters {
				_, err := db.CreateCluster(context.Background(), cluster)
				if err != nil {
					log.Warn(err)
				}
			}

			appClientset := appclientset.NewForConfigOrDie(config)
			for _, app := range newApps {
				out, err := appClientset.ArgoprojV1alpha1().Applications(namespace).Create(app)
				errors.CheckError(err)
				log.Println(out)
			}

			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)

	return &command
}

// NewExportCommand defines a new command for exporting Kubernetes and Argo CD resources.
func NewExportCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		out          string
	)
	var command = cobra.Command{
		Use:   "export",
		Short: "Export all Argo CD data to stdout (default) or a file",
		RunE: func(c *cobra.Command, args []string) error {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeClientset := kubernetes.NewForConfigOrDie(config)

			settingsMgr := settings.NewSettingsManager(kubeClientset, namespace)
			settings, err := settingsMgr.GetSettings()
			errors.CheckError(err)
			// certificate data is included in secrets that are exported alongside
			settings.Certificate = nil

			db := db.NewDB(namespace, kubeClientset)
			clusters, err := db.ListClusters(context.Background())
			errors.CheckError(err)

			repos, err := db.ListRepositories(context.Background())
			errors.CheckError(err)

			appClientset := appclientset.NewForConfigOrDie(config)
			apps, err := appClientset.ArgoprojV1alpha1().Applications(namespace).List(metav1.ListOptions{})
			errors.CheckError(err)

			rbacCM, err := kubeClientset.CoreV1().ConfigMaps(namespace).Get(common.ArgoCDRBACConfigMapName, metav1.GetOptions{})
			errors.CheckError(err)

			// remove extraneous cruft from output
			rbacCM.ObjectMeta = metav1.ObjectMeta{
				Name: rbacCM.ObjectMeta.Name,
			}

			// remove extraneous cruft from output
			for idx, app := range apps.Items {
				apps.Items[idx].ObjectMeta = metav1.ObjectMeta{
					Name:       app.ObjectMeta.Name,
					Finalizers: app.ObjectMeta.Finalizers,
				}
				apps.Items[idx].Status = v1alpha1.ApplicationStatus{
					History: app.Status.History,
				}
				apps.Items[idx].Operation = nil
			}

			// take a list of exportable objects, marshal them to YAML,
			// and return a string joined by a delimiter
			output := func(delimiter string, oo ...interface{}) string {
				out := make([]string, 0)
				for _, o := range oo {
					data, err := yaml.Marshal(o)
					errors.CheckError(err)
					out = append(out, string(data))
				}
				return strings.Join(out, delimiter)
			}(yamlSeparator, settings, repos.Items, clusters.Items, apps.Items, rbacCM)

			if out == "-" {
				fmt.Println(output)
			} else {
				err = ioutil.WriteFile(out, []byte(output), 0644)
				errors.CheckError(err)
			}
			return nil
		},
	}

	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.Flags().StringVarP(&out, "out", "o", "-", "Output to the specified file instead of stdout")

	return &command
}

// NewSettingsCommand returns a new instance of `argocd-util settings` command
func NewSettingsCommand() *cobra.Command {
	var (
		clientConfig      clientcmd.ClientConfig
		updateSuperuser   bool
		superuserPassword string
		updateSignature   bool
	)
	var command = &cobra.Command{
		Use:   "settings",
		Short: "Creates or updates ArgoCD settings",
		Long:  "Creates or updates ArgoCD settings",
		Run: func(c *cobra.Command, args []string) {
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, wasSpecified, err := clientConfig.Namespace()
			errors.CheckError(err)
			if !(wasSpecified) {
				namespace = "argocd"
			}

			kubeclientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)
			settingsMgr := settings.NewSettingsManager(kubeclientset, namespace)

			_ = settings.UpdateSettings(superuserPassword, settingsMgr, updateSignature, updateSuperuser, namespace)
		},
	}
	command.Flags().BoolVar(&updateSuperuser, "update-superuser", false, "force updating the  superuser password")
	command.Flags().StringVar(&superuserPassword, "superuser-password", "", "password for super user")
	command.Flags().BoolVar(&updateSignature, "update-signature", false, "force updating the server-side token signing signature")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}

func main() {
	if err := NewCommand().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
