package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	clusterpkg "github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/clusterauth"
)

// NewClusterCommand returns a new instance of an `argocd cluster` command
func NewClusterCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: "Manage cluster credentials",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewClusterAddCommand(clientOpts, pathOpts))
	command.AddCommand(NewClusterGetCommand(clientOpts))
	command.AddCommand(NewClusterListCommand(clientOpts))
	command.AddCommand(NewClusterRemoveCommand(clientOpts))
	command.AddCommand(NewClusterRotateAuthCommand(clientOpts))
	return command
}

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		inCluster       bool
		upsert          bool
		awsRoleArn      string
		awsClusterName  string
		systemNamespace string
	)
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s cluster add CONTEXT", cliName),
		Run: func(c *cobra.Command, args []string) {
			var configAccess clientcmd.ConfigAccess = pathOpts
			if len(args) == 0 {
				log.Error("Choose a context name from:")
				printKubeContexts(configAccess)
				os.Exit(1)
			}
			config, err := configAccess.GetStartingConfig()
			errors.CheckError(err)
			clstContext := config.Contexts[args[0]]
			if clstContext == nil {
				log.Fatalf("Context %s does not exist in kubeconfig", args[0])
			}

			overrides := clientcmd.ConfigOverrides{
				Context: *clstContext,
			}
			clientConfig := clientcmd.NewDefaultClientConfig(*config, &overrides)
			conf, err := clientConfig.ClientConfig()
			errors.CheckError(err)

			managerBearerToken := ""
			var awsAuthConf *argoappv1.AWSAuthConfig
			if awsClusterName != "" {
				awsAuthConf = &argoappv1.AWSAuthConfig{
					ClusterName: awsClusterName,
					RoleARN:     awsRoleArn,
				}
			} else {
				// Install RBAC resources for managing the cluster
				clientset, err := kubernetes.NewForConfig(conf)
				errors.CheckError(err)
				managerBearerToken, err = clusterauth.InstallClusterManagerRBAC(clientset, systemNamespace)
				errors.CheckError(err)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clst := NewCluster(args[0], conf, managerBearerToken, awsAuthConf)
			if inCluster {
				clst.Server = common.KubernetesInternalAPIServerAddr
			}
			clstCreateReq := clusterpkg.ClusterCreateRequest{
				Cluster: clst,
				Upsert:  upsert,
			}
			clst, err = clusterIf.Create(context.Background(), &clstCreateReq)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' added\n", clst.Name)
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().BoolVar(&inCluster, "in-cluster", false, "Indicates Argo CD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing cluster with the same name even if the spec differs")
	command.Flags().StringVar(&awsClusterName, "aws-cluster-name", "", "AWS Cluster name if set then aws-iam-authenticator will be used to access cluster")
	command.Flags().StringVar(&awsRoleArn, "aws-role-arn", "", "Optional AWS role arn. If set then AWS IAM Authenticator assume a role to perform cluster operations instead of the default AWS credential provider chain.")
	command.Flags().StringVar(&systemNamespace, "system-namespace", common.DefaultSystemNamespace, "Use different system namespace")
	return command
}

func printKubeContexts(ca clientcmd.ConfigAccess) {
	config, err := ca.GetStartingConfig()
	errors.CheckError(err)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	columnNames := []string{"CURRENT", "NAME", "CLUSTER", "SERVER"}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, "\t"))
	errors.CheckError(err)

	// sort names so output is deterministic
	contextNames := make([]string, 0)
	for name := range config.Contexts {
		contextNames = append(contextNames, name)
	}
	sort.Strings(contextNames)

	if config.Clusters == nil {
		return
	}

	for _, name := range contextNames {
		// ignore malformed kube config entries
		context := config.Contexts[name]
		if context == nil {
			continue
		}
		cluster := config.Clusters[context.Cluster]
		if cluster == nil {
			continue
		}
		prefix := " "
		if config.CurrentContext == name {
			prefix = "*"
		}
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", prefix, name, context.Cluster, cluster.Server)
		errors.CheckError(err)
	}
}

func NewCluster(name string, conf *rest.Config, managerBearerToken string, awsAuthConf *argoappv1.AWSAuthConfig) *argoappv1.Cluster {
	tlsClientConfig := argoappv1.TLSClientConfig{
		Insecure:   conf.TLSClientConfig.Insecure,
		ServerName: conf.TLSClientConfig.ServerName,
		CAData:     conf.TLSClientConfig.CAData,
	}
	if len(conf.TLSClientConfig.CAData) == 0 && conf.TLSClientConfig.CAFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CAFile)
		errors.CheckError(err)
		tlsClientConfig.CAData = data
	}
	clst := argoappv1.Cluster{
		Server: conf.Host,
		Name:   name,
		Config: argoappv1.ClusterConfig{
			BearerToken:     managerBearerToken,
			TLSClientConfig: tlsClientConfig,
			AWSAuthConfig:   awsAuthConf,
		},
	}
	return &clst
}

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "get CLUSTER",
		Short: "Get cluster information",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			for _, clusterName := range args {
				clst, err := clusterIf.Get(context.Background(), &clusterpkg.ClusterQuery{Server: clusterName})
				errors.CheckError(err)
				yamlBytes, err := yaml.Marshal(clst)
				errors.CheckError(err)
				fmt.Printf("%v", string(yamlBytes))
			}
		},
	}
	return command
}

// NewClusterRemoveCommand returns a new instance of an `argocd cluster list` command
func NewClusterRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm CLUSTER",
		Short: "Remove cluster credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)

			// clientset, err := kubernetes.NewForConfig(conf)
			// errors.CheckError(err)

			for _, clusterName := range args {
				// TODO(jessesuen): find the right context and remove manager RBAC artifacts
				// err := clusterauth.UninstallClusterManagerRBAC(clientset)
				// errors.CheckError(err)
				_, err := clusterIf.Delete(context.Background(), &clusterpkg.ClusterQuery{Server: clusterName})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// Print table of cluster information
func printClusterTable(clusters []argoappv1.Cluster) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVER\tNAME\tSTATUS\tMESSAGE\n")
	for _, c := range clusters {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Server, c.Name, c.ConnectionState.Status, c.ConnectionState.Message)
	}
	_ = w.Flush()
}

// Print list of cluster servers
func printClusterServers(clusters []argoappv1.Cluster) {
	for _, c := range clusters {
		fmt.Println(c.Server)
	}
}

// NewClusterListCommand returns a new instance of an `argocd cluster rm` command
func NewClusterListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured clusters",
		Run: func(c *cobra.Command, args []string) {
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clusters, err := clusterIf.List(context.Background(), &clusterpkg.ClusterQuery{})
			errors.CheckError(err)
			if output == "server" {
				printClusterServers(clusters.Items)
			} else {
				printClusterTable(clusters.Items)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|server")
	return command
}

// NewClusterRotateAuthCommand returns a new instance of an `argocd cluster rotate-auth` command
func NewClusterRotateAuthCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rotate-auth CLUSTER",
		Short: fmt.Sprintf("%s cluster rotate-auth CLUSTER", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clusterQuery := clusterpkg.ClusterQuery{
				Server: args[0],
			}
			_, err := clusterIf.RotateAuth(context.Background(), &clusterQuery)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' rotated auth\n", clusterQuery.Server)
		},
	}
	return command
}
