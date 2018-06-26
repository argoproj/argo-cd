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
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectOpts struct {
	description  string
	destinations []string
}

func (opts *projectOpts) GetDestinations() []v1alpha1.ApplicationDestination {
	destinations := make([]v1alpha1.ApplicationDestination, 0)
	for _, destStr := range opts.destinations {
		parts := strings.Split(destStr, ",")
		if len(parts) != 2 {
			log.Fatalf("Expected destination of the form: server,namespace. Received: %s", destStr)
		} else {
			destinations = append(destinations, v1alpha1.ApplicationDestination{
				Server:    parts[0],
				Namespace: parts[1],
			})
		}
	}
	return destinations
}

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
	command.AddCommand(NewProjectSetCommand(clientOpts))
	command.AddCommand(NewProjectAddDestinationCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationCommand(clientOpts))
	return command
}

func addProjFlags(command *cobra.Command, opts *projectOpts) {
	command.Flags().StringVarP(&opts.description, "description", "", "desc", "Project description")
	command.Flags().StringArrayVarP(&opts.destinations, "dest", "d", []string{},
		"Allowed deployment destination. Includes comma separated server url and namespace (e.g. https://192.168.99.100:8443,default")
}

// NewProjectCreateCommand returns a new instance of an `argocd proj create` command
func NewProjectCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		name string
		opts projectOpts
	)
	var command = &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		Run: func(c *cobra.Command, args []string) {
			proj := v1alpha1.AppProject{
				ObjectMeta: v1.ObjectMeta{Name: name},
				Spec: v1alpha1.AppProjectSpec{
					Description:  opts.description,
					Destinations: opts.GetDestinations(),
				},
			}
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err := projIf.Create(context.Background(), &project.ProjectCreateRequest{Project: &proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&name, "name", "n", "Project name")
	addProjFlags(command, &opts)
	return command
}

// NewProjectSetCommand returns a new instance of an `argocd proj set` command
func NewProjectSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts projectOpts
	)
	var command = &cobra.Command{
		Use:   "set PROJECT",
		Short: "Set project parameters",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			visited := 0
			c.Flags().Visit(func(f *pflag.Flag) {
				visited++
				switch f.Name {
				case "description":
					proj.Spec.Description = opts.description
				case "dest":
					proj.Spec.Destinations = opts.GetDestinations()
				}
			})
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addProjFlags(command, &opts)
	return command
}

// NewProjectAddDestinationCommand returns a new instance of an `argocd proj add-destination` command
func NewProjectAddDestinationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "add-destination PROJECT SERVER NAMESPACE",
		Short: "Add project destination",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			server := args[1]
			namespace := args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, dest := range proj.Spec.Destinations {
				if dest.Namespace == namespace && dest.Server == server {
					log.Fatal("Specified destination is already defined in project")
				}
			}
			proj.Spec.Destinations = append(proj.Spec.Destinations, v1alpha1.ApplicationDestination{Server: server, Namespace: namespace})
			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRemoveDestinationCommand returns a new instance of an `argocd proj remove-destination` command
func NewProjectRemoveDestinationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "remove-destination PROJECT SERVER NAMESPACE",
		Short: "Remove project destination",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			server := args[1]
			namespace := args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, dest := range proj.Spec.Destinations {
				if dest.Namespace == namespace && dest.Server == server {
					index = i
					break
				}
			}
			if index == -1 {
				log.Fatal("Specified destination does not exist in project")
			} else {
				proj.Spec.Destinations = append(proj.Spec.Destinations[:index], proj.Spec.Destinations[index+1:]...)
				_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewProjectDeleteCommand returns a new instance of an `argocd proj delete` command
func NewProjectDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT",
		Short: "Delete project",
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
