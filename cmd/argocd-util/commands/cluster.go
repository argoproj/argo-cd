package commands

import (
	"context"
	"os"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/settings"
)

func NewClusterCommand(pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: "Manage clusters configuration",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewClusterConfig())
	command.AddCommand(NewGenClusterConfigCommand(pathOpts))

	return command
}

// NewClusterConfig returns a new instance of `argocd-util kubeconfig` command
func NewClusterConfig() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
	)
	var command = &cobra.Command{
		Use:               "kubeconfig CLUSTER_URL OUTPUT_PATH",
		Short:             "Generates kubeconfig for the specified cluster",
		DisableAutoGenTag: true,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			serverUrl := args[0]
			output := args[1]
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			kubeclientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)

			cluster, err := db.NewDB(namespace, settings.NewSettingsManager(context.Background(), kubeclientset, namespace), kubeclientset).GetCluster(context.Background(), serverUrl)
			errors.CheckError(err)
			err = kube.WriteKubeConfig(cluster.RawRestConfig(), namespace, output)
			errors.CheckError(err)
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}

func NewGenClusterConfigCommand(pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		clusterOpts  cmdutil.ClusterOptions
		bearerToken  string
		outputFormat string
	)
	var command = &cobra.Command{
		Use:   "generate-spec CONTEXT",
		Short: "Generate declarative config for a cluster",
		Run: func(c *cobra.Command, args []string) {
			log.SetLevel(log.WarnLevel)
			var configAccess clientcmd.ConfigAccess = pathOpts
			if len(args) == 0 {
				log.Error("Choose a context name from:")
				cmdutil.PrintKubeContexts(configAccess)
				os.Exit(1)
			}
			cfgAccess, err := configAccess.GetStartingConfig()
			errors.CheckError(err)
			contextName := args[0]
			clstContext := cfgAccess.Contexts[contextName]
			if clstContext == nil {
				log.Fatalf("Context %s does not exist in kubeconfig", contextName)
			}

			overrides := clientcmd.ConfigOverrides{
				Context: *clstContext,
			}
			clientConfig := clientcmd.NewDefaultClientConfig(*cfgAccess, &overrides)
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			kubeClientset := fake.NewSimpleClientset()

			var awsAuthConf *argoappv1.AWSAuthConfig
			var execProviderConf *argoappv1.ExecProviderConfig
			if clusterOpts.AwsClusterName != "" {
				awsAuthConf = &argoappv1.AWSAuthConfig{
					ClusterName: clusterOpts.AwsClusterName,
					RoleARN:     clusterOpts.AwsRoleArn,
				}
			} else if clusterOpts.ExecProviderCommand != "" {
				execProviderConf = &argoappv1.ExecProviderConfig{
					Command:     clusterOpts.ExecProviderCommand,
					Args:        clusterOpts.ExecProviderArgs,
					Env:         clusterOpts.ExecProviderEnv,
					APIVersion:  clusterOpts.ExecProviderAPIVersion,
					InstallHint: clusterOpts.ExecProviderInstallHint,
				}
			} else if bearerToken == "" {
				bearerToken = "bearer-token"
			}
			if clusterOpts.Name != "" {
				contextName = clusterOpts.Name
			}
			clst := cmdutil.NewCluster(contextName, clusterOpts.Namespaces, conf, bearerToken, awsAuthConf, execProviderConf)
			if clusterOpts.InCluster {
				clst.Server = common.KubernetesInternalAPIServerAddr
			}
			if clusterOpts.Shard >= 0 {
				clst.Shard = &clusterOpts.Shard
			}

			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClientset, ArgoCDNamespace)
			argoDB := db.NewDB(ArgoCDNamespace, settingsMgr, kubeClientset)

			_, err = argoDB.CreateCluster(context.Background(), clst)
			errors.CheckError(err)

			secName, err := db.ServerToSecretName(clst.Server)
			errors.CheckError(err)

			secret, err := kubeClientset.CoreV1().Secrets(ArgoCDNamespace).Get(context.Background(), secName, v1.GetOptions{})
			errors.CheckError(err)

			cmdutil.ConvertSecretData(secret)
			var printResources []interface{}
			printResources = append(printResources, secret)
			errors.CheckError(cmdutil.PrintResources(printResources, outputFormat))
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().StringVar(&bearerToken, "bearer-token", "", "Authentication token that should be used to access K8S API server")
	command.Flags().StringVarP(&outputFormat, "output", "o", "yaml", "Output format. One of: json|yaml")
	cmdutil.AddClusterFlags(command, &clusterOpts)
	return command
}
