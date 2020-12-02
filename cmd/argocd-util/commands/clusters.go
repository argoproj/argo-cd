package commands

import (
	"context"
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
		clusterOpts  cmdutil.ClusterOptions
		outputFormat string
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
			} else {
				// Install RBAC resources for managing the cluster
				if clusterOpts.ServiceAccount != "" {
					managerBearerToken, err = clusterauth.GetServiceAccountBearerToken(clientset, clusterOpts.SystemNamespace, clusterOpts.ServiceAccount)
				} else {
					rbacResources, err = clusterauth.GenerateClusterManagerRBAC(clientset, clusterOpts.SystemNamespace, clusterOpts.Namespaces)
				}
				errors.CheckError(err)
			}
			if clusterOpts.Name != "" {
				contextName = clusterOpts.Name
			}
			clst := cmdutil.NewCluster(contextName, clusterOpts.Namespaces, conf, managerBearerToken, awsAuthConf, execProviderConf)
			if clusterOpts.InCluster {
				clst.Server = common.KubernetesInternalAPIServerAddr
			}
			if clusterOpts.Shard >= 0 {
				clst.Shard = &clusterOpts.Shard
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
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")
	cmdutil.AddClusterFlags(command, &clusterOpts)
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
