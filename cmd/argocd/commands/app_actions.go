package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/v2/util/templates"

	"github.com/argoproj/argo-cd/v2/cmd/util"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
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

var appActionExample = templates.Examples(`
	# List all the available actions for an application
	argocd app actions list APPNAME

	# Run an available action for an application
	argocd app actions run APPNAME ACTION --kind KIND [--resource-name RESOURCE] [--namespace NAMESPACE] [--group GROUP]
	`)

// NewApplicationResourceActionsCommand returns a new instance of an `argocd app actions` command
func NewApplicationResourceActionsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "actions",
		Short:   "Manage Resource actions",
		Example: appActionExample,
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
	command := &cobra.Command{
		Use:   "list APPNAME",
		Short: "Lists available actions on a resource",
		Example: templates.Examples(`
	# List all the available actions for an application
	argocd app actions list APPNAME
	`),
	}
	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseFromQualifiedName(args[0], "")
		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer io.Close(conn)
		resources, err := getActionableResourcesForApplication(appIf, ctx, &appNs, &appName)
		errors.CheckError(err)
		filteredObjects, err := util.FilterResources(command.Flags().Changed("group"), resources, group, kind, namespace, resourceName, true)
		errors.CheckError(err)
		var availableActions []DisplayedAction
		for i := range filteredObjects {
			obj := filteredObjects[i]
			gvk := obj.GroupVersionKind()
			availActionsForResource, err := appIf.ListResourceActions(ctx, &applicationpkg.ApplicationResourceRequest{
				Name:         &appName,
				AppNamespace: &appNs,
				Namespace:    ptr.To(obj.GetNamespace()),
				ResourceName: ptr.To(obj.GetName()),
				Group:        ptr.To(gvk.Group),
				Kind:         ptr.To(gvk.Kind),
				Version:      ptr.To(gvk.Version),
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
	command := &cobra.Command{
		Use:   "run APPNAME ACTION",
		Short: "Runs an available action on resource(s)",
		Example: templates.Examples(`
	# Run an available action for an application
	argocd app actions run APPNAME ACTION --kind KIND [--resource-name RESOURCE] [--namespace NAMESPACE] [--group GROUP]
	`),
	}

	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	command.Flags().StringVar(&group, "group", "", "Group")
	errors.CheckError(command.MarkFlagRequired("kind"))
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to run the action on multiple matching resources")

	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 2 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseFromQualifiedName(args[0], "")
		actionName := args[1]

		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer io.Close(conn)
		resources, err := getActionableResourcesForApplication(appIf, ctx, &appNs, &appName)
		errors.CheckError(err)
		filteredObjects, err := util.FilterResources(command.Flags().Changed("group"), resources, group, kind, namespace, resourceName, all)
		errors.CheckError(err)
		resGroup := filteredObjects[0].GroupVersionKind().Group
		for i := range filteredObjects[1:] {
			if filteredObjects[i].GroupVersionKind().Group != resGroup {
				log.Fatal("Ambiguous resource group. Use flag --group to specify resource group explicitly.")
			}
		}

		for i := range filteredObjects {
			obj := filteredObjects[i]
			gvk := obj.GroupVersionKind()
			objResourceName := obj.GetName()
			_, err := appIf.RunResourceAction(ctx, &applicationpkg.ResourceActionRunRequest{
				Name:         &appName,
				AppNamespace: &appNs,
				Namespace:    ptr.To(obj.GetNamespace()),
				ResourceName: ptr.To(objResourceName),
				Group:        ptr.To(gvk.Group),
				Kind:         ptr.To(gvk.Kind),
				Version:      ptr.To(gvk.GroupVersion().Version),
				Action:       ptr.To(actionName),
			})
			errors.CheckError(err)
		}
	}
	return command
}

func getActionableResourcesForApplication(appIf applicationpkg.ApplicationServiceClient, ctx context.Context, appNs *string, appName *string) ([]*v1alpha1.ResourceDiff, error) {
	resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
		ApplicationName: appName,
		AppNamespace:    appNs,
	})
	if err != nil {
		return nil, err
	}
	app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
		Name:         appName,
		AppNamespace: appNs,
	})
	if err != nil {
		return nil, err
	}
	app.Kind = application.ApplicationKind
	app.APIVersion = "argoproj.io/v1alpha1"
	appManifest, err := json.Marshal(app)
	if err != nil {
		return nil, err
	}
	appGVK := app.GroupVersionKind()
	return append(resources.Items, &v1alpha1.ResourceDiff{
		Group:     appGVK.Group,
		Kind:      appGVK.Kind,
		Namespace: app.Namespace,
		Name:      *appName,
		LiveState: string(appManifest),
	}), nil
}
