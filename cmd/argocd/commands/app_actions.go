package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
)

type DisplayedAction struct {
	Group    string
	Kind     string
	Name     string
	Action   string
	Disabled bool
}

// NewApplicationResourceActionsCommand returns a new instance of an `argocd app actions` command
func NewApplicationResourceActionsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "actions",
		Short: "Manage Resource actions",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationResourceActionsListCommand(clientOpts))
	command.AddCommand(NewApplicationResourceActionsRunCommand(clientOpts))
	return command
}

// NewApplicationResourceActionsListCommand returns a new instance of an `argocd app actions list` command
func NewApplicationResourceActionsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var namespace string
	var kind string
	var group string
	var resourceName string
	var output string
	var command = &cobra.Command{
		Use:   "list APPNAME",
		Short: "Lists available actions on a resource",
	}
	command.Run = func(c *cobra.Command, args []string) {
		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName := args[0]
		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer io.Close(conn)
		ctx := context.Background()
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{ApplicationName: &appName})
		errors.CheckError(err)
		filteredObjects := filterResources(command, resources.Items, group, kind, namespace, resourceName, true)
		var availableActions []DisplayedAction
		for i := range filteredObjects {
			obj := filteredObjects[i]
			gvk := obj.GroupVersionKind()
			availActionsForResource, err := appIf.ListResourceActions(ctx, &applicationpkg.ApplicationResourceRequest{
				Name:         &appName,
				Namespace:    obj.GetNamespace(),
				ResourceName: obj.GetName(),
				Group:        gvk.Group,
				Kind:         gvk.Kind,
			})
			errors.CheckError(err)
			for _, action := range availActionsForResource.Actions {
				displayAction := DisplayedAction{
					Group:    gvk.Group,
					Kind:     gvk.Kind,
					Name:     obj.GetName(),
					Action:   action.Name,
					Disabled: action.Disabled,
				}
				availableActions = append(availableActions, displayAction)
			}
		}

		switch output {
		case "yaml":
			yamlBytes, err := yaml.Marshal(availableActions)
			errors.CheckError(err)
			fmt.Println(string(yamlBytes))
		case "json":
			jsonBytes, err := json.MarshalIndent(availableActions, "", "  ")
			errors.CheckError(err)
			fmt.Println(string(jsonBytes))
		case "":
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "GROUP\tKIND\tNAME\tACTION\tDISABLED\n")
			for _, action := range availableActions {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", action.Group, action.Kind, action.Name, action.Action, strconv.FormatBool(action.Disabled))
			}
			_ = w.Flush()
		}
	}
	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().StringVarP(&output, "out", "o", "", "Output format. One of: yaml, json")

	return command
}

// NewApplicationResourceActionsRunCommand returns a new instance of an `argocd app actions run` command
func NewApplicationResourceActionsRunCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var namespace string
	var resourceName string
	var kind string
	var group string
	var all bool
	var command = &cobra.Command{
		Use:   "run APPNAME ACTION",
		Short: "Runs an available action on resource(s)",
	}

	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	command.Flags().StringVar(&group, "group", "", "Group")
	errors.CheckError(command.MarkFlagRequired("kind"))
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to run the action on multiple matching resources")

	command.Run = func(c *cobra.Command, args []string) {
		if len(args) != 2 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName := args[0]
		actionName := args[1]

		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer io.Close(conn)
		ctx := context.Background()
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{ApplicationName: &appName})
		errors.CheckError(err)
		filteredObjects := filterResources(command, resources.Items, group, kind, namespace, resourceName, all)
		var resGroup = filteredObjects[0].GroupVersionKind().Group
		for i := range filteredObjects[1:] {
			if filteredObjects[i].GroupVersionKind().Group != resGroup {
				log.Fatal("Ambiguous resource group. Use flag --group to specify resource group explicitly.")
			}
		}

		for i := range filteredObjects {
			obj := filteredObjects[i]
			gvk := obj.GroupVersionKind()
			objResourceName := obj.GetName()
			_, err := appIf.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{
				Name:         &appName,
				Namespace:    obj.GetNamespace(),
				ResourceName: objResourceName,
				Group:        gvk.Group,
				Kind:         gvk.Kind,
				Action:       actionName,
			})
			errors.CheckError(err)
		}
	}
	return command
}
