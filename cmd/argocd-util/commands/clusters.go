package commands

import (
	"context"
	"fmt"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/clusterauth"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
)

func NewGenClusterConfigCommand(pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		inCluster               bool
		upsert                  bool
		serviceAccount          string
		awsRoleArn              string
		awsClusterName          string
		systemNamespace         string
		namespaces              []string
		name                    string
		shard                   int64
		execProviderCommand     string
		execProviderArgs        []string
		execProviderEnv         map[string]string
		execProviderAPIVersion  string
		execProviderInstallHint string
		outputFormat            string
	)
	var command = &cobra.Command{
		Use:   "cluster-add CONTEXT",
		Short: "Generate declarative config for a project",
		Run: func(c *cobra.Command, args []string) {
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
			clientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)

			managerBearerToken := ""
			var awsAuthConf *argoappv1.AWSAuthConfig
			var execProviderConf *argoappv1.ExecProviderConfig
			var rbacResources *clusterauth.RBACResources
			if awsClusterName != "" {
				awsAuthConf = &argoappv1.AWSAuthConfig{
					ClusterName: awsClusterName,
					RoleARN:     awsRoleArn,
				}
			} else if execProviderCommand != "" {
				execProviderConf = &argoappv1.ExecProviderConfig{
					Command:     execProviderCommand,
					Args:        execProviderArgs,
					Env:         execProviderEnv,
					APIVersion:  execProviderAPIVersion,
					InstallHint: execProviderInstallHint,
				}
			} else {
				// Install RBAC resources for managing the cluster
				if serviceAccount != "" {
					managerBearerToken, err = clusterauth.GetServiceAccountBearerToken(clientset, systemNamespace, serviceAccount)
				} else {
					managerBearerToken, rbacResources, err = clusterauth.InstallClusterManagerRBAC(clientset, systemNamespace, namespaces)
				}
				errors.CheckError(err)
			}
			if name != "" {
				contextName = name
			}
			clst := cmdutil.NewCluster(contextName, namespaces, conf, managerBearerToken, awsAuthConf, execProviderConf)
			if inCluster {
				clst.Server = common.KubernetesInternalAPIServerAddr
			}
			if shard >= 0 {
				clst.Shard = &shard
			}

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			secret, err := genClusterSecret(clst, namespace, clientset)
			errors.CheckError(err)
			secret.Kind = kube.SecretKind
			secret.APIVersion = "v1"

			err = cmdutil.PrintResource(secret, outputFormat)
			errors.CheckError(err)
			if rbacResources != nil && rbacResources.ServiceAccount != nil {
				err = cmdutil.PrintResource(rbacResources.ServiceAccount, outputFormat)
				errors.CheckError(err)
			}
			if rbacResources != nil && rbacResources.ClusterRole != nil {
				err = cmdutil.PrintResource(rbacResources.ClusterRole, outputFormat)
				errors.CheckError(err)
			}
			if rbacResources != nil && rbacResources.ClusterRoleBinding != nil {
				err = cmdutil.PrintResource(rbacResources.ClusterRoleBinding, outputFormat)
				errors.CheckError(err)
			}
			if rbacResources != nil && rbacResources.Role != nil {
				err = cmdutil.PrintResource(rbacResources.Role, outputFormat)
				errors.CheckError(err)
			}
			if rbacResources != nil && rbacResources.RoleBinding != nil {
				err = cmdutil.PrintResource(rbacResources.RoleBinding, outputFormat)
				errors.CheckError(err)
			}
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().BoolVar(&inCluster, "in-cluster", false, "Indicates Argo CD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing cluster with the same name even if the spec differs")
	command.Flags().StringVar(&serviceAccount, "service-account", "", fmt.Sprintf("System namespace service account to use for kubernetes resource management. If not set then default \"%s\" SA will be created", clusterauth.ArgoCDManagerServiceAccount))
	command.Flags().StringVar(&awsClusterName, "aws-cluster-name", "", "AWS Cluster name if set then aws cli eks token command will be used to access cluster")
	command.Flags().StringVar(&awsRoleArn, "aws-role-arn", "", "Optional AWS role arn. If set then AWS IAM Authenticator assume a role to perform cluster operations instead of the default AWS credential provider chain.")
	command.Flags().StringVar(&systemNamespace, "system-namespace", common.DefaultSystemNamespace, "Use different system namespace")
	command.Flags().StringArrayVar(&namespaces, "namespace", nil, "List of namespaces which are allowed to manage")
	command.Flags().StringVar(&name, "name", "", "Overwrite the cluster name")
	command.Flags().Int64Var(&shard, "shard", -1, "Cluster shard number; inferred from hostname if not set")
	command.Flags().StringVar(&execProviderCommand, "exec-command", "", "Command to run to provide client credentials to the cluster. You may need to build a custom ArgoCD image to ensure the command is available at runtime.")
	command.Flags().StringArrayVar(&execProviderArgs, "exec-command-args", nil, "Arguments to supply to the --exec-command command")
	command.Flags().StringToStringVar(&execProviderEnv, "exec-command-env", nil, "Environment vars to set when running the --exec-command command")
	command.Flags().StringVar(&execProviderAPIVersion, "exec-command-api-version", "", "Preferred input version of the ExecInfo for the --exec-command")
	command.Flags().StringVar(&execProviderInstallHint, "exec-command-install-hint", "", "Text shown to the user when the --exec-command executable doesn't seem to be present")
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")
	return command
}

func genClusterSecret(c *argoappv1.Cluster, namespace string, kubeClientset *kubernetes.Clientset) (*apiv1.Secret, error) {
	secName, err := db.ServerToSecretName(c.Server)
	if err != nil {
		return nil, err
	}
	clusterSecret := &apiv1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: secName,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
	}

	if err = db.ClusterToSecret(c, clusterSecret); err != nil {
		return nil, err
	}

	clusterSecret, err = kubeClientset.CoreV1().Secrets(namespace).Create(context.Background(), clusterSecret, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
	if err != nil {
		return nil, err
	}

	return clusterSecret, nil
}
