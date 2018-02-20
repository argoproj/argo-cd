package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// NewApplicationCommand returns a new instance of an `argocd app` command
func NewApplicationCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "app",
		Short: fmt.Sprintf("%s app COMMAND", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}

	command.AddCommand(NewApplicationAddCommand())
	command.AddCommand(NewApplicationGetCommand())
	command.AddCommand(NewApplicationListCommand())
	command.AddCommand(NewApplicationRemoveCommand())
	return command
}

// NewApplicationAddCommand returns a new instance of an `argocd app add` command
func NewApplicationAddCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s app add APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	return command
}

// NewApplicationGetCommand returns a new instance of an `argocd app get` command
func NewApplicationGetCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "get",
		Short: fmt.Sprintf("%s app get APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := NewApplicationClient()
			defer util.Close(conn)
			for _, appName := range args {
				clst, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: appName})
				errors.CheckError(err)
				yamlBytes, err := yaml.Marshal(clst)
				errors.CheckError(err)
				fmt.Printf("%v\n", string(yamlBytes))
			}
		},
	}
	return command
}

// NewApplicationRemoveCommand returns a new instance of an `argocd app list` command
func NewApplicationRemoveCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm",
		Short: fmt.Sprintf("%s app rm APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := NewApplicationClient()
			defer util.Close(conn)
			for _, appName := range args {
				_, err := appIf.Delete(context.Background(), &application.ApplicationQuery{Name: appName})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewApplicationListCommand returns a new instance of an `argocd app rm` command
func NewApplicationListCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("%s app list", cliName),
		Run: func(c *cobra.Command, args []string) {
			conn, appIf := NewApplicationClient()
			defer util.Close(conn)
			apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
			errors.CheckError(err)
			for _, c := range apps.Items {
				fmt.Printf("%s\n", c.Name)
			}
		},
	}
	return command
}

func NewApplicationClient() (*grpc.ClientConn, application.ApplicationServiceClient) {
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
	appIf := application.NewApplicationServiceClient(conn)
	return conn, appIf
}
