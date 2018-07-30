package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
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
	return command
}

// DefaultKubeConfigDir returns the local configuration path for settings such as cached authentication tokens.
func DefaultKubeConfigDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	return path.Join(usr.HomeDir, ".kube"), nil
}

// DefaultKubeConfigPath returns the local configuration path for settings such as cached authentication tokens.
func DefaultKubeConfigPath() (string, error) {
	dir, err := DefaultKubeConfigDir()
	if err != nil {
		return "", err
	}
	return path.Join(dir, "config"), nil
}

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var (
		inCluster bool
		upsert    bool
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

			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)

			p, err := DefaultKubeConfigPath()
			errors.CheckError(err)
			kubeconfig, err := ioutil.ReadFile(p)
			errors.CheckError(err)

			clstCreateReq := cluster.ClusterCreateFromKubeConfigRequest{
				Kubeconfig: string(kubeconfig),
				Context:    args[0],
				Upsert:     upsert,
				InCluster:  inCluster,
			}
			clst, err := clusterIf.CreateFromKubeConfig(context.Background(), &clstCreateReq)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' added\n", clst.Name)
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	command.Flags().BoolVar(&inCluster, "in-cluster", false, "Indicates ArgoCD resides inside this cluster and should connect using the internal k8s hostname (kubernetes.default.svc)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Override an existing cluster with the same name even if the spec differs")
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

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "get",
		Short: "Get cluster information",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			for _, clusterName := range args {
				clst, err := clusterIf.Get(context.Background(), &cluster.ClusterQuery{Server: clusterName})
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
		Use:   "rm",
		Short: "Remove cluster credentials",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			for _, clusterName := range args {
				// TODO(jessesuen): find the right context and remove manager RBAC artifacts
				// common.UninstallClusterManagerRBAC(conf)
				_, err := clusterIf.Delete(context.Background(), &cluster.ClusterQuery{Server: clusterName})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewClusterListCommand returns a new instance of an `argocd cluster rm` command
func NewClusterListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: "List configured clusters",
		Run: func(c *cobra.Command, args []string) {
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clusters, err := clusterIf.List(context.Background(), &cluster.ClusterQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "SERVER\tNAME\tSTATUS\tMESSAGE\n")
			for _, c := range clusters.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.Server, c.Name, c.ConnectionState.Status, c.ConnectionState.Message)
			}
			_ = w.Flush()
		},
	}
	return command
}
