package commands

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewClusterCommand returns a new instance of an `argocd cluster` command
func NewClusterCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: fmt.Sprintf("%s cluster COMMAND", cliName),
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

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand(clientOpts *argocdclient.ClientOptions, pathOpts *clientcmd.PathOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s cluster add CONTEXT", cliName),
		Run: func(c *cobra.Command, args []string) {
			var configAccess clientcmd.ConfigAccess = pathOpts
			if len(args) == 0 {
				log.Error("Choose a context name from:")
				printContexts(configAccess)
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

			// Install RBAC resources for managing the cluster
			conf.BearerToken = common.InstallClusterManagerRBAC(conf)

			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clst := NewCluster(args[0], conf)
			clst, err = clusterIf.Create(context.Background(), clst)
			errors.CheckError(err)
			fmt.Printf("Cluster '%s' added\n", clst.Name)
		},
	}
	command.PersistentFlags().StringVar(&pathOpts.LoadingRules.ExplicitPath, pathOpts.ExplicitFileFlag, pathOpts.LoadingRules.ExplicitPath, "use a particular kubeconfig file")
	return command
}

func printContexts(ca clientcmd.ConfigAccess) {
	config, err := ca.GetStartingConfig()
	errors.CheckError(err)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() { _ = w.Flush() }()
	columnNames := []string{"CURRENT", "NAME", "CLUSTER", "AUTHINFO", "NAMESPACE"}
	_, err = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, "\t"))
	errors.CheckError(err)

	// sort names so output is deterministic
	contextNames := make([]string, 0)
	for name := range config.Contexts {
		contextNames = append(contextNames, name)
	}
	sort.Strings(contextNames)

	for _, name := range contextNames {
		context := config.Contexts[name]
		prefix := " "
		if config.CurrentContext == name {
			prefix = "*"
		}
		_, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", prefix, name, context.Cluster, context.AuthInfo, context.Namespace)
		errors.CheckError(err)
	}
}

func NewCluster(name string, conf *rest.Config) *argoappv1.Cluster {
	tlsClientConfig := argoappv1.TLSClientConfig{
		Insecure:   conf.TLSClientConfig.Insecure,
		ServerName: conf.TLSClientConfig.ServerName,
		CertData:   conf.TLSClientConfig.CertData,
		KeyData:    conf.TLSClientConfig.KeyData,
		CAData:     conf.TLSClientConfig.CAData,
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
	if len(conf.TLSClientConfig.CAData) == 0 && conf.TLSClientConfig.CAFile != "" {
		data, err := ioutil.ReadFile(conf.TLSClientConfig.CAFile)
		errors.CheckError(err)
		tlsClientConfig.CAData = data
	}
	clst := argoappv1.Cluster{
		Server: conf.Host,
		Name:   name,
		Config: argoappv1.ClusterConfig{
			Username:        conf.Username,
			Password:        conf.Password,
			BearerToken:     conf.BearerToken,
			TLSClientConfig: tlsClientConfig,
		},
	}
	return &clst
}

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "get",
		Short: fmt.Sprintf("%s cluster get SERVER", cliName),
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
		Short: fmt.Sprintf("%s cluster rm SERVER", cliName),
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
		Short: fmt.Sprintf("%s cluster list", cliName),
		Run: func(c *cobra.Command, args []string) {
			conn, clusterIf := argocdclient.NewClientOrDie(clientOpts).NewClusterClientOrDie()
			defer util.Close(conn)
			clusters, err := clusterIf.List(context.Background(), &cluster.ClusterQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "SERVER\tNAME\n")
			for _, c := range clusters.Items {
				fmt.Fprintf(w, "%s\t%s\n", c.Server, c.Name)
			}
			_ = w.Flush()
		},
	}
	return command
}
