package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/common"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	clusterpkg "github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
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
		Example: `  # List all known clusters in JSON format:
  argocd cluster list -o json

  # Add a target cluster configuration to ArgoCD. The context must exist in your kubectl config:
  argocd cluster add example-cluster

  # Get specific details about a cluster in plain text (wide) format:
  argocd cluster get example-cluster -o wide

  #	Remove a target cluster context from ArgoCD
  argocd cluster rm example-cluster
`,
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
		serviceAccount  string
		awsRoleArn      string
		awsClusterName  string
		systemNamespace string
		namespaces      []string
		name            string
	)
	var command = &cobra.Command{
		Use:   "add CONTEXT",
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
			contextName := args[0]
			clstContext := config.Contexts[contextName]
			if clstContext == nil {
				log.Fatalf("Context %s does not exist in kubeconfig", contextName)
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
				if serviceAccount != "" {
					managerBearerToken, err = clusterauth.GetServiceAccountBearerToken(clientset, systemNamespace, serviceAccount)
				} else {
					managerBearerToken, err = clusterauth.InstallClusterManagerRBAC(clientset, systemNamespace, namespaces)
				}
				errors.CheckError(err)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer io.Close(conn)
			if name != "" {
				contextName = name
			}
			clst := newCluster(contextName, namespaces, conf, managerBearerToken, awsAuthConf)
			if inCluster {
				clst.Server = common.KubernetesInternalAPIServerAddr
			}
			clstCreateReq := clusterpkg.ClusterCreateRequest{
				Cluster: clst,
				Upsert:  upsert,
			}
			_, err = clusterIf.Create(context.Background(), &clstCreateReq)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' added\n", clst.Server)
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

func newCluster(name string, namespaces []string, conf *rest.Config, managerBearerToken string, awsAuthConf *argoappv1.AWSAuthConfig) *argoappv1.Cluster {
	tlsClientConfig := argoappv1.TLSClientConfig{
		Insecure:   conf.TLSClientConfig.Insecure,
		ServerName: conf.TLSClientConfig.ServerName,
		CAData:     conf.TLSClientConfig.CAData,
		CertData:   conf.TLSClientConfig.CertData,
		KeyData:    conf.TLSClientConfig.KeyData,
	}
	if len(conf.TLSClientConfig.CAData) == 0 && conf.TLSClientConfig.CAFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CAFile)
		errors.CheckError(err)
		tlsClientConfig.CAData = data
	}
	if len(conf.TLSClientConfig.CertData) == 0 && conf.TLSClientConfig.CertFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CertFile)
		errors.CheckError(err)
		tlsClientConfig.CertData = data
	}
	if len(conf.TLSClientConfig.KeyData) == 0 && conf.TLSClientConfig.KeyFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.KeyFile)
		errors.CheckError(err)
		tlsClientConfig.KeyData = data
	}

	clst := argoappv1.Cluster{
		Server:     conf.Host,
		Name:       name,
		Namespaces: namespaces,
		Config: argoappv1.ClusterConfig{
			TLSClientConfig: tlsClientConfig,
			AWSAuthConfig:   awsAuthConf,
		},
	}

	// Bearer token will preferentially be used for auth if present,
	// Even in presence of key/cert credentials
	// So set bearer token only if the key/cert data is absent
	if len(tlsClientConfig.CertData) == 0 || len(tlsClientConfig.KeyData) == 0 {
		clst.Config.BearerToken = managerBearerToken
	}

	return &clst
}

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:     "get SERVER",
		Short:   "Get cluster information",
		Example: `argocd cluster get https://12.34.567.89`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer io.Close(conn)
			clusters := make([]argoappv1.Cluster, 0)
			for _, clusterName := range args {
				clst, err := clusterIf.Get(context.Background(), &clusterpkg.ClusterQuery{Server: clusterName})
				errors.CheckError(err)
				clusters = append(clusters, *clst)
			}
			switch output {
			case "yaml", "json":
				err := PrintResourceList(clusters, output, true)
				errors.CheckError(err)
			case "wide", "":
				printClusterDetails(clusters)
			case "server":
				printClusterServers(clusters)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	// we have yaml as default to not break backwards-compatibility
	command.Flags().StringVarP(&output, "output", "o", "yaml", "Output format. One of: json|yaml|wide|server")
	return command
}

func strWithDefault(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func formatNamespaces(cluster argoappv1.Cluster) string {
	if len(cluster.Namespaces) == 0 {
		return "all namespaces"
	}
	return strings.Join(cluster.Namespaces, ", ")
}

func printClusterDetails(clusters []argoappv1.Cluster) {
	for _, cluster := range clusters {
		fmt.Printf("Cluster information\n\n")
		fmt.Printf("  Server URL:            %s\n", cluster.Server)
		fmt.Printf("  Server Name:           %s\n", strWithDefault(cluster.Name, "-"))
		fmt.Printf("  Server Version:        %s\n", cluster.ServerVersion)
		fmt.Printf("  Namespaces:        	 %s\n", formatNamespaces(cluster))
		fmt.Printf("\nTLS configuration\n\n")
		fmt.Printf("  Client cert:           %v\n", string(cluster.Config.TLSClientConfig.CertData) != "")
		fmt.Printf("  Cert validation:       %v\n", !cluster.Config.TLSClientConfig.Insecure)
		fmt.Printf("\nAuthentication\n\n")
		fmt.Printf("  Basic authentication:  %v\n", cluster.Config.Username != "")
		fmt.Printf("  oAuth authentication:  %v\n", cluster.Config.BearerToken != "")
		fmt.Printf("  AWS authentication:    %v\n", cluster.Config.AWSAuthConfig != nil)
		fmt.Println()
	}
}

// NewClusterRemoveCommand returns a new instance of an `argocd cluster list` command
func NewClusterRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "rm SERVER",
		Short:   "Remove cluster credentials",
		Example: `argocd cluster rm https://12.34.567.89`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer io.Close(conn)

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
	_, _ = fmt.Fprintf(w, "SERVER\tNAME\tVERSION\tSTATUS\tMESSAGE\n")
	for _, c := range clusters {
		server := c.Server
		if len(c.Namespaces) > 0 {
			server = fmt.Sprintf("%s (%d namespaces)", c.Server, len(c.Namespaces))
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", server, c.Name, c.ServerVersion, c.ConnectionState.Status, c.ConnectionState.Message)
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
			defer io.Close(conn)
			clusters, err := clusterIf.List(context.Background(), &clusterpkg.ClusterQuery{})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(clusters.Items, output, false)
				errors.CheckError(err)
			case "server":
				printClusterServers(clusters.Items)
			case "wide", "":
				printClusterTable(clusters.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|server")
	return command
}

// NewClusterRotateAuthCommand returns a new instance of an `argocd cluster rotate-auth` command
func NewClusterRotateAuthCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "rotate-auth SERVER",
		Short:   fmt.Sprintf("%s cluster rotate-auth SERVER", cliName),
		Example: fmt.Sprintf("%s cluster rotate-auth https://12.34.567.89", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer io.Close(conn)
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
