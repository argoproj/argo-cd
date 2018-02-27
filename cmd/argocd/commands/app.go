package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/errors"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	var (
		repoURL string
		appPath string
		env     string
	)
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s app add APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			app := argoappv1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name: args[0],
				},
				Spec: argoappv1.ApplicationSpec{
					Source: argoappv1.ApplicationSource{
						RepoURL:     repoURL,
						Path:        appPath,
						Environment: env,
					},
				},
			}
			conn, appIf := NewApplicationClient()
			defer util.Close(conn)
			_, err := appIf.Create(context.Background(), &app)
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&repoURL, "repo", "", "Repository URL")
	command.Flags().StringVar(&appPath, "path", "", "Path in repository to the ksonnet app directory")
	command.Flags().StringVar(&env, "env", "", "Application environment to monitor")

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
