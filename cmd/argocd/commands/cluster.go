package commands

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/text/label"
)

const (
	// type of the cluster ID is 'name'
	clusterIdTypeName = "name"
	// cluster field is 'name'
	clusterFieldName = "name"
	// cluster field is 'namespaces'
	clusterFieldNamespaces = "namespaces"
	// indicates managing all namespaces
	allNamespaces = "*"
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

  # Remove a target cluster context from ArgoCD
  argocd cluster rm example-cluster

  # Set a target cluster context from ArgoCD
  argocd cluster set CLUSTER_NAME --name new-cluster-name --namespace '*'
  argocd cluster set CLUSTER_NAME --name new-cluster-name --namespace namespace-one --namespace namespace-two`,
	}

	command.AddCommand(NewClusterAddCommand(clientOpts, pathOpts))
	command.AddCommand(NewClusterGetCommand(clientOpts))
	command.AddCommand(NewClusterListCommand(clientOpts))
	command.AddCommand(NewClusterRemoveCommand(clientOpts, pathOpts))
	command.AddCommand(NewClusterRotateAuthCommand(clientOpts))
	command.AddCommand(NewClusterSetCommand(clientOpts))
	return command
}

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		clusterOpts      cmdutil.ClusterOptions
		skipConfirmation bool
		labels           []string
		annotations      []string
	)
	var command = &cobra.Command{
		Use:   "add CONTEXT",
		Short: fmt.Sprintf("%s cluster add CONTEXT", cliName),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			var configAccess clientcmd.ConfigAccess = pathOpts
			if len(args) == 0 {
				log.Error("Choose a context name from:")
				cmdutil.PrintKubeContexts(configAccess)
				os.Exit(1)
			}

			if clusterOpts.InCluster && clusterOpts.ClusterEndpoint != "" {
				log.Fatal("Can only use one of --in-cluster or --cluster-endpoint")
				return
			}

			contextName := args[0]
			conf, err := getRestConfig(pathOpts, contextName)
			errors.CheckError(err)
			clientset, err := kubernetes.NewForConfig(conf)
			errors.CheckError(err)
			managerBearerToken := ""
			var awsAuthConf *argoappv1.AWSAuthConfig
			var execProviderConf *argoappv1.ExecProviderConfig
			if clusterOpts.AwsClusterName != "" {
				awsAuthConf = &argoappv1.AWSAuthConfig{
					ClusterName: clusterOpts.AwsClusterName,
					RoleARN:     clusterOpts.AwsRoleArn,
					Profile:     clusterOpts.AwsProfile,
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
					managerBearerToken, err = clusterauth.GetServiceAccountBearerToken(clientset, clusterOpts.SystemNamespace, clusterOpts.ServiceAccount, common.BearerTokenTimeout)
				} else {
					isTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
					if isTerminal && !skipConfirmation {
						accessLevel := "cluster"
						if len(clusterOpts.Namespaces) > 0 {
							accessLevel = "namespace"
						}
						message := fmt.Sprintf("WARNING: This will create a service account `argocd-manager` on the cluster referenced by context `%s` with full %s level privileges. Do you want to continue [y/N]? ", contextName, accessLevel)
						if !cli.AskToProceed(message) {
							os.Exit(1)
						}
					}
					managerBearerToken, err = clusterauth.InstallClusterManagerRBAC(clientset, clusterOpts.SystemNamespace, clusterOpts.Namespaces, common.BearerTokenTimeout)
				}
				errors.CheckError(err)
			}

			labelsMap, err := label.Parse(labels)
			errors.CheckError(err)
			annotationsMap, err := label.Parse(annotations)
			errors.CheckError(err)

			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)
			if clusterOpts.Name != "" {
				contextName = clusterOpts.Name
			}
			clst := cmdutil.NewCluster(contextName, clusterOpts.Namespaces, clusterOpts.ClusterResources, conf, managerBearerToken, awsAuthConf, execProviderConf, labelsMap, annotationsMap)
			if clusterOpts.InClusterEndpoint() {
				clst.Server = argoappv1.KubernetesInternalAPIServerAddr
			} else if clusterOpts.ClusterEndpoint == string(cmdutil.KubePublicEndpoint) {
				endpoint, err := cmdutil.GetKubePublicEndpoint(clientset)
				if err != nil || len(endpoint) == 0 {
					log.Warnf("Failed to find the cluster endpoint from kube-public data: %v", err)
					log.Infof("Falling back to the endpoint '%s' as listed in the kubeconfig context", clst.Server)
					endpoint = clst.Server
				}
				clst.Server = endpoint
			}

			if clusterOpts.Shard >= 0 {
				clst.Shard = &clusterOpts.Shard
			}
			if clusterOpts.Project != "" {
				clst.Project = clusterOpts.Project
			}
			clstCreateReq := clusterpkg.ClusterCreateRequest{
				Cluster: clst,
				Upsert:  clusterOpts.Upsert,
			}
			_, err = clusterIf.Create(ctx, &clstCreateReq)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' added\n", clst.Server)
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().BoolVar(&clusterOpts.Upsert, "upsert", false, "Override an existing cluster with the same name even if the spec differs")
	command.Flags().StringVar(&clusterOpts.ServiceAccount, "service-account", "", fmt.Sprintf("System namespace service account to use for kubernetes resource management. If not set then default \"%s\" SA will be created", clusterauth.ArgoCDManagerServiceAccount))
	command.Flags().StringVar(&clusterOpts.SystemNamespace, "system-namespace", common.DefaultSystemNamespace, "Use different system namespace")
	command.Flags().BoolVarP(&skipConfirmation, "yes", "y", false, "Skip explicit confirmation")
	command.Flags().StringArrayVar(&labels, "label", nil, "Set metadata labels (e.g. --label key=value)")
	command.Flags().StringArrayVar(&annotations, "annotation", nil, "Set metadata annotations (e.g. --annotation key=value)")
	cmdutil.AddClusterFlags(command, &clusterOpts)
	return command
}

func getRestConfig(pathOpts *clientcmd.PathOptions, ctxName string) (*rest.Config, error) {
	config, err := pathOpts.GetStartingConfig()
	if err != nil {
		return nil, err
	}

	clstContext := config.Contexts[ctxName]
	if clstContext == nil {
		return nil, fmt.Errorf("Context %s does not exist in kubeconfig", ctxName)
	}

	overrides := clientcmd.ConfigOverrides{
		Context: *clstContext,
	}

	clientConfig := clientcmd.NewDefaultClientConfig(*config, &overrides)
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	return conf, nil
}

// NewClusterSetCommand returns a new instance of an `argocd cluster set` command
func NewClusterSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		clusterOptions cmdutil.ClusterOptions
		clusterName    string
	)
	var command = &cobra.Command{
		Use:   "set NAME",
		Short: "Set cluster information",
		Example: `  # Set cluster information
  argocd cluster set CLUSTER_NAME --name new-cluster-name --namespace '*'
  argocd cluster set CLUSTER_NAME --name new-cluster-name --namespace namespace-one --namespace namespace-two`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			// name of the cluster whose fields have to be updated.
			clusterName = args[0]
			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)
			// checks the fields that needs to be updated
			updatedFields := checkFieldsToUpdate(clusterOptions)
			namespaces := clusterOptions.Namespaces
			// check if all namespaces have to be considered
			if len(namespaces) == 1 && strings.EqualFold(namespaces[0], allNamespaces) {
				namespaces[0] = ""
			}
			if updatedFields != nil {
				clusterUpdateRequest := clusterpkg.ClusterUpdateRequest{
					Cluster: &argoappv1.Cluster{
						Name:       clusterOptions.Name,
						Namespaces: namespaces,
					},
					UpdatedFields: updatedFields,
					Id: &clusterpkg.ClusterID{
						Type:  clusterIdTypeName,
						Value: clusterName,
					},
				}
				_, err := clusterIf.Update(ctx, &clusterUpdateRequest)
				errors.CheckError(err)
				fmt.Printf("Cluster '%s' updated.\n", clusterName)
			} else {
				fmt.Print("Specify the cluster field to be updated.\n")
			}
		},
	}
	command.Flags().StringVar(&clusterOptions.Name, "name", "", "Overwrite the cluster name")
	command.Flags().StringArrayVar(&clusterOptions.Namespaces, "namespace", nil, "List of namespaces which are allowed to manage. Specify '*' to manage all namespaces")
	return command
}

// checkFieldsToUpdate returns the fields that needs to be updated
func checkFieldsToUpdate(clusterOptions cmdutil.ClusterOptions) []string {
	var updatedFields []string
	if clusterOptions.Name != "" {
		updatedFields = append(updatedFields, clusterFieldName)
	}
	if clusterOptions.Namespaces != nil {
		updatedFields = append(updatedFields, clusterFieldNamespaces)
	}
	return updatedFields
}

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "get SERVER/NAME",
		Short: "Get cluster information",
		Example: `argocd cluster get https://12.34.567.89
argocd cluster get in-cluster`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)
			clusters := make([]argoappv1.Cluster, 0)
			for _, clusterSelector := range args {
				clst, err := clusterIf.Get(ctx, getQueryBySelector(clusterSelector))
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

// NewClusterRemoveCommand returns a new instance of an `argocd cluster rm` command
func NewClusterRemoveCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var noPrompt bool
	var command = &cobra.Command{
		Use:   "rm SERVER/NAME",
		Short: "Remove cluster credentials",
		Example: `argocd cluster rm https://12.34.567.89
argocd cluster rm cluster-name`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)
			var numOfClusters = len(args)
			var isConfirmAll bool = false

			for _, clusterSelector := range args {
				clusterQuery := getQueryBySelector(clusterSelector)
				var lowercaseAnswer string
				if !noPrompt {
					if numOfClusters == 1 {
						lowercaseAnswer = cli.AskToProceedS("Are you sure you want to remove '" + clusterSelector + "'? Any Apps deploying to this cluster will go to health status Unknown.[y/n] ")
					} else {
						if !isConfirmAll {
							lowercaseAnswer = cli.AskToProceedS("Are you sure you want to remove '" + clusterSelector + "'? Any Apps deploying to this cluster will go to health status Unknown.[y/n/A] where 'A' is to remove all specified clusters without prompting. Any Apps deploying to these clusters will go to health status Unknown. ")
							if lowercaseAnswer == "a" {
								lowercaseAnswer = "y"
								isConfirmAll = true
							}
						} else {
							lowercaseAnswer = "y"
						}
					}
				} else {
					lowercaseAnswer = "y"
				}

				if lowercaseAnswer == "y" {
					// get the cluster name to use as context to delete RBAC on cluster
					clst, err := clusterIf.Get(ctx, clusterQuery)
					errors.CheckError(err)

					// remove cluster
					_, err = clusterIf.Delete(ctx, clusterQuery)
					errors.CheckError(err)
					fmt.Printf("Cluster '%s' removed\n", clusterSelector)

					// remove RBAC from cluster
					conf, err := getRestConfig(pathOpts, clst.Name)
					errors.CheckError(err)

					clientset, err := kubernetes.NewForConfig(conf)
					errors.CheckError(err)

					err = clusterauth.UninstallClusterManagerRBAC(clientset)
					errors.CheckError(err)
				} else {
					fmt.Println("The command to remove '" + clusterSelector + "' was cancelled.")
				}
			}
		},
	}
	command.Flags().BoolVarP(&noPrompt, "yes", "y", false, "Turn off prompting to confirm remove of cluster resources")
	return command
}

// Print table of cluster information
func printClusterTable(clusters []argoappv1.Cluster) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "SERVER\tNAME\tVERSION\tSTATUS\tMESSAGE\tPROJECT\n")
	for _, c := range clusters {
		server := c.Server
		if len(c.Namespaces) > 0 {
			server = fmt.Sprintf("%s (%d namespaces)", c.Server, len(c.Namespaces))
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", server, c.Name, c.ServerVersion, c.ConnectionState.Status, c.ConnectionState.Message, c.Project)
	}
	_ = w.Flush()
}

// Returns cluster query for getting cluster depending on the cluster selector
func getQueryBySelector(clusterSelector string) *clusterpkg.ClusterQuery {
	var query clusterpkg.ClusterQuery
	isServer, err := regexp.MatchString(`^https?://`, clusterSelector)
	if isServer || err != nil {
		query.Server = clusterSelector
	} else {
		query.Name = clusterSelector
	}
	return &query
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
			ctx := c.Context()

			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)
			clusters, err := clusterIf.List(ctx, &clusterpkg.ClusterQuery{})
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
		Example: `
# List Clusters in Default "Wide" Format
argocd cluster list

# List Cluster via specifing the server
argocd cluster list --server <ARGOCD_SERVER_ADDRESS>

# List Clusters in JSON Format
argocd cluster list -o json --server <ARGOCD_SERVER_ADDRESS>

# List Clusters in YAML Format
argocd cluster list -o yaml --server <ARGOCD_SERVER_ADDRESS>

# List Clusters that have been added to your Argo CD 
argocd cluster list -o server <ARGOCD_SERVER_ADDRESS>

`,
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|server")
	return command
}

// NewClusterRotateAuthCommand returns a new instance of an `argocd cluster rotate-auth` command
func NewClusterRotateAuthCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rotate-auth SERVER/NAME",
		Short: fmt.Sprintf("%s cluster rotate-auth SERVER/NAME", cliName),
		Example: `argocd cluster rotate-auth https://12.34.567.89
argocd cluster rotate-auth cluster-name`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := headless.NewClientOrDie(clientOpts, c).NewClusterClientOrDie()
			defer io.Close(conn)

			cluster := args[0]
			clusterQuery := getQueryBySelector(cluster)
			_, err := clusterIf.RotateAuth(ctx, clusterQuery)
			errors.CheckError(err)

			fmt.Printf("Cluster '%s' rotated auth\n", cluster)
		},
	}
	return command
}
