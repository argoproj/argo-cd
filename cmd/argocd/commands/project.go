package commands

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"strings"

	"context"

	"fmt"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/project"
	"github.com/argoproj/argo-cd/util"
)

// NewProjectCommand returns a new instance of an `argocd proj` command
func NewProjectCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "proj",
		Short: "Manage projects",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectCreateCommand(clientOpts))
	command.AddCommand(NewProjectDeleteCommand(clientOpts))
	command.AddCommand(NewProjectListCommand(clientOpts))
	return command
}

func NewProjectCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		proj         v1alpha1.AppProject
		destinations []string
	)
	var command = &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Run: func(c *cobra.Command, args []string) {
			for _, destStr := range destinations {
				parts := strings.Split(destStr, ",")
				if len(parts) != 2 {
					log.Fatalf("Expected destination of the form: server;namespace. Received: %s", destStr)
				} else {
					proj.Spec.Destinations = append(proj.Spec.Destinations, v1alpha1.ApplicationDestination{
						Server:    parts[0],
						Namespace: parts[1],
					})
				}
			}
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err := projIf.Create(context.Background(), &project.ProjectCreateRequest{Project: &proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&proj.ObjectMeta.Name, "name", "n", "Project name")
	command.Flags().StringVarP(&proj.Spec.Description, "description", "", "desc", "Project description")
	command.Flags().StringArrayVarP(&destinations, "dest", "d", []string{},
		"Allowed deployment destination. Includes comma separated server url and namespace (e.g. https://192.168.99.100:8443,default")
	return command
}

func NewProjectDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm PROJECT",
		Short: "Remove project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			for _, name := range args {
				_, err := projIf.Delete(context.Background(), &project.ProjectQuery{Name: name})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewProjectListCommand returns a new instance of an `argocd proj list` command
func NewProjectListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Run: func(c *cobra.Command, args []string) {
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			projects, err := projIf.List(context.Background(), &project.ProjectQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tDESCRIPTION\tDESTINATIONS\n")
			for _, p := range projects.Items {
				fmt.Fprintf(w, "%s\t%s\t%v\n", p.Name, p.Spec.Description, p.Spec.Destinations)
			}
			_ = w.Flush()
		},
	}
	return command
}
