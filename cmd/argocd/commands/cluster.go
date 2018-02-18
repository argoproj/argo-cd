package commands

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/core"
	"github.com/argoproj/argo-cd/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// NewClusterCommand returns a new instance of an `argocd cluster` command
func NewClusterCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "cluster",
		Short: fmt.Sprintf("%s cluster COMMAND", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewClusterAddCommand())
	command.AddCommand(NewClusterGetCommand())
	command.AddCommand(NewClusterListCommand())
	command.AddCommand(NewClusterRemoveCommand())
	return command
}

// NewClusterAddCommand returns a new instance of an `argocd cluster add` command
func NewClusterAddCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s cluster add CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	return command
}

// NewClusterGetCommand returns a new instance of an `argocd cluster get` command
func NewClusterGetCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "get",
		Short: fmt.Sprintf("%s cluster get CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
			}
			conn, clusterIf := NewClusterClient()
			defer util.Close(conn)
			for _, clusterName := range args {
				clst, err := clusterIf.Get(context.Background(), &core.NameMessage{Name: clusterName})
				errors.CheckError(err)
				fmt.Printf("%v\n", clst)
			}
		},
	}
	return command
}

// NewClusterRemoveCommand returns a new instance of an `argocd cluster list` command
func NewClusterRemoveCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm",
		Short: fmt.Sprintf("%s cluster rm CLUSTERNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
			}
			conn, clusterIf := NewClusterClient()
			defer util.Close(conn)
			for _, clusterName := range args {
				_, err := clusterIf.Delete(context.Background(), &core.NameMessage{Name: clusterName})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewClusterListCommand returns a new instance of an `argocd cluster rm` command
func NewClusterListCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("%s cluster list", cliName),
		Run: func(c *cobra.Command, args []string) {
			conn, clusterIf := NewClusterClient()
			defer util.Close(conn)
			clusters, err := clusterIf.List(context.Background(), &cluster.ClusterQuery{})
			errors.CheckError(err)
			for _, c := range clusters.Items {
				fmt.Printf("%s\n", c.Name)
			}
		},
	}
	return command
}

func NewClusterClient() (*grpc.ClientConn, cluster.ClusterServiceClient) {
	// TODO: get this from a config or command line flag
	serverAddr := "localhost:8080"
	var dialOpts []grpc.DialOption
	// TODO: add insecure config option and --insecure global flag
	if true {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	} // else if opts.Credentials != nil {
	//	dialOpts = append(dialOpts, grpc.WithTransportCredentials(opts.Credentials))
	//}
	conn, err := grpc.Dial(serverAddr, dialOpts...)
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", serverAddr, err)
	}
	clusterIf := cluster.NewClusterServiceClient(conn)
	return conn, clusterIf
}
