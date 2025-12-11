package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	"github.com/argoproj/argo-cd/v3/cmd/util"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// NewApplicationGetResourceCommand returns a new instance of the `app get-resource` command
func NewApplicationGetResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		resourceName      string
		kind              string
		group             string
		project           string
		filteredFields    []string
		showManagedFields bool
		output            string
	)
	command := &cobra.Command{
		Use:   "get-resource APPNAME",
		Short: "Get details about the live Kubernetes manifests of a resource in an application. The filter-fields flag can be used to only display fields you want to see.",
		Example: `
  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod

  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in yaml format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod -o yaml

  # Get a specific resource, Pod my-app-pod, in 'my-app' by name in json format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod -o json

  # Get details about all Pods in the application
    argocd app get-resource my-app --kind Pod

  # Get a specific resource with managed fields, Pod my-app-pod, in 'my-app' by name in wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod --show-managed-fields

  # Get the details of a specific field in a resource in 'my-app' in the wide format
    argocd app get-resource my-app --kind Pod --filter-fields status.podIP

  # Get the details of multiple specific fields in a specific resource in 'my-app' in the wide format
    argocd app get-resource my-app --kind Pod --resource-name my-app-pod --filter-fields status.podIP,status.hostIP`,
	}

	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}

		appName, appNs := argo.ParseFromQualifiedName(args[0], "")

		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer utilio.Close(conn)

		tree, err := appIf.ResourceTree(ctx, &applicationpkg.ResourcesQuery{
			ApplicationName: &appName,
			AppNamespace:    &appNs,
		})
		errors.CheckError(err)

		// Get manifests of resources
		// If resource name is "" find all resources of that kind
		var resources []unstructured.Unstructured
		var fetchedStr string
		for _, r := range tree.Nodes {
			if (resourceName != "" && r.Name != resourceName) || (group != "" && r.Group != group) || r.Kind != kind {
				continue
			}
			resource, err := appIf.GetResource(ctx, &applicationpkg.ApplicationResourceRequest{
				Name:         &appName,
				AppNamespace: &appNs,
				Group:        &r.Group,
				Kind:         &r.Kind,
				Namespace:    &r.Namespace,
				Project:      &project,
				ResourceName: &r.Name,
				Version:      &r.Version,
			})
			errors.CheckError(err)
			manifest := resource.GetManifest()

			var obj *unstructured.Unstructured
			err = json.Unmarshal([]byte(manifest), &obj)
			errors.CheckError(err)

			if !showManagedFields {
				unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
			}

			if len(filteredFields) != 0 {
				obj = filterFieldsFromObject(obj, filteredFields)
			}

			fetchedStr += obj.GetName() + ", "
			resources = append(resources, *obj)
		}
		printManifests(&resources, len(filteredFields) > 0, resourceName == "", output)

		if fetchedStr != "" {
			fetchedStr = strings.TrimSuffix(fetchedStr, ", ")
		}
		log.Infof("Resources '%s' fetched", fetchedStr)
	}

	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource, if none is included will output details of all resources with specified kind")
	command.Flags().StringVar(&kind, "kind", "", "Kind of resource [REQUIRED]")
	err := command.MarkFlagRequired("kind")
	errors.CheckError(err)
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&project, "project", "", "Project of resource")
	command.Flags().StringSliceVar(&filteredFields, "filter-fields", nil, "A comma separated list of fields to display, if not provided will output the entire manifest")
	command.Flags().BoolVar(&showManagedFields, "show-managed-fields", false, "Show managed fields in the output manifest")
	command.Flags().StringVarP(&output, "output", "o", "wide", "Format of the output, wide, yaml, or json")
	return command
}

// filterFieldsFromObject creates a new unstructured object containing only the specified fields from the source object.
func filterFieldsFromObject(obj *unstructured.Unstructured, filteredFields []string) *unstructured.Unstructured {
	var filteredObj unstructured.Unstructured
	filteredObj.Object = make(map[string]any)

	for _, f := range filteredFields {
		fields := strings.Split(f, ".")

		value, exists, err := unstructured.NestedFieldCopy(obj.Object, fields...)
		if exists {
			errors.CheckError(err)
			err = unstructured.SetNestedField(filteredObj.Object, value, fields...)
			errors.CheckError(err)
		} else {
			// If doesn't exist assume its a nested inside a list of objects
			value := extractNestedItem(obj.Object, fields, 0)
			filteredObj.Object = value
		}
	}
	filteredObj.SetName(obj.GetName())
	return &filteredObj
}

// extractNestedItem recursively extracts an item that may be nested inside a list of objects.
func extractNestedItem(obj map[string]any, fields []string, depth int) map[string]any {
	if depth >= len(fields) {
		return nil
	}

	value, exists, _ := unstructured.NestedFieldCopy(obj, fields[:depth+1]...)
	list, ok := value.([]any)
	if !exists || !ok {
		return extractNestedItem(obj, fields, depth+1)
	}

	extractedItems := extractItemsFromList(list, fields[depth+1:])
	if len(extractedItems) == 0 {
		for _, e := range list {
			if o, ok := e.(map[string]any); ok {
				result := extractNestedItem(o, fields[depth+1:], 0)
				extractedItems = append(extractedItems, result)
			}
		}
	}

	filteredObj := reconstructObject(extractedItems, fields, depth)
	return filteredObj
}

// extractItemsFromList processes a list of objects and extracts specific fields from each item.
func extractItemsFromList(list []any, fields []string) []any {
	var extratedObjs []any
	for _, e := range list {
		extractedObj := make(map[string]any)
		if o, ok := e.(map[string]any); ok {
			value, exists, _ := unstructured.NestedFieldCopy(o, fields...)
			if !exists {
				continue
			}
			err := unstructured.SetNestedField(extractedObj, value, fields...)
			errors.CheckError(err)
			extratedObjs = append(extratedObjs, extractedObj)
		}
	}
	return extratedObjs
}

// reconstructObject rebuilds the original object structure by placing extracted items back into their proper nested location.
func reconstructObject(extracted []any, fields []string, depth int) map[string]any {
	obj := make(map[string]any)
	err := unstructured.SetNestedField(obj, extracted, fields[:depth+1]...)
	errors.CheckError(err)
	return obj
}

// printManifests outputs resource manifests in the specified format (wide, JSON, or YAML).
func printManifests(objs *[]unstructured.Unstructured, filteredFields bool, showName bool, output string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if showName {
		fmt.Fprintf(w, "FIELD\tRESOURCE NAME\tVALUE\n")
	} else {
		fmt.Fprintf(w, "FIELD\tVALUE\n")
	}

	for i, o := range *objs {
		if output == "json" || output == "yaml" {
			var formattedManifest []byte
			var err error
			if output == "json" {
				formattedManifest, err = json.MarshalIndent(o.Object, "", " ")
			} else {
				formattedManifest, err = yaml.Marshal(o.Object)
			}
			errors.CheckError(err)

			fmt.Println(string(formattedManifest))
			if len(*objs) > 1 && i != len(*objs)-1 {
				fmt.Println("---")
			}
		} else {
			name := o.GetName()
			if filteredFields {
				unstructured.RemoveNestedField(o.Object, "metadata", "name")
			}

			printManifestAsTable(w, name, showName, o.Object, "")
		}
	}

	if output != "json" && output != "yaml" {
		err := w.Flush()
		errors.CheckError(err)
	}
}

// printManifestAsTable recursively prints a manifest object as a tabular view with nested fields flattened.
func printManifestAsTable(w *tabwriter.Writer, name string, showName bool, obj map[string]any, parentField string) {
	for key, value := range obj {
		field := parentField + key
		switch v := value.(type) {
		case map[string]any:
			printManifestAsTable(w, name, showName, v, field+".")
		case []any:
			for i, e := range v {
				index := "[" + strconv.Itoa(i) + "]"

				if innerObj, ok := e.(map[string]any); ok {
					printManifestAsTable(w, name, showName, innerObj, field+index+".")
				} else {
					if showName {
						fmt.Fprintf(w, "%v\t%v\t%v\n", field+index, name, e)
					} else {
						fmt.Fprintf(w, "%v\t%v\n", field+index, e)
					}
				}
			}
		default:
			if showName {
				fmt.Fprintf(w, "%v\t%v\t%v\n", field, name, v)
			} else {
				fmt.Fprintf(w, "%v\t%v\n", field, v)
			}
		}
	}
}

func NewApplicationPatchResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		patch        string
		patchType    string
		resourceName string
		namespace    string
		kind         string
		group        string
		all          bool
		project      string
	)
	command := &cobra.Command{
		Use:   "patch-resource APPNAME",
		Short: "Patch resource in an application",
	}

	command.Flags().StringVar(&patch, "patch", "", "Patch")
	err := command.MarkFlagRequired("patch")
	errors.CheckError(err)
	command.Flags().StringVar(&patchType, "patch-type", string(types.MergePatchType), "Which Patching strategy to use: 'application/json-patch+json', 'application/merge-patch+json', or 'application/strategic-merge-patch+json'. Defaults to 'application/merge-patch+json'")
	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	err = command.MarkFlagRequired("kind")
	errors.CheckError(err)
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to patch multiple matching of resources")
	command.Flags().StringVar(&project, "project", "", `The name of the application's project - specifying this allows the command to report "not found" instead of "permission denied" if the app does not exist`)
	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseFromQualifiedName(args[0], "")

		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer utilio.Close(conn)
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
			ApplicationName: &appName,
			AppNamespace:    &appNs,
		})
		errors.CheckError(err)
		objectsToPatch, err := util.FilterResources(command.Flags().Changed("group"), resources.Items, group, kind, namespace, resourceName, all)
		errors.CheckError(err)
		for i := range objectsToPatch {
			obj := objectsToPatch[i]
			gvk := obj.GroupVersionKind()
			_, err = appIf.PatchResource(ctx, &applicationpkg.ApplicationResourcePatchRequest{
				Name:         &appName,
				AppNamespace: &appNs,
				Namespace:    ptr.To(obj.GetNamespace()),
				ResourceName: ptr.To(obj.GetName()),
				Version:      ptr.To(gvk.Version),
				Group:        ptr.To(gvk.Group),
				Kind:         ptr.To(gvk.Kind),
				Patch:        ptr.To(patch),
				PatchType:    ptr.To(patchType),
				Project:      ptr.To(project),
			})
			errors.CheckError(err)
			log.Infof("Resource '%s' patched", obj.GetName())
		}
	}

	return command
}

func NewApplicationDeleteResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		resourceName string
		namespace    string
		kind         string
		group        string
		force        bool
		orphan       bool
		all          bool
		project      string
	)
	command := &cobra.Command{
		Use:   "delete-resource APPNAME",
		Short: "Delete resource in an application",
	}

	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	err := command.MarkFlagRequired("kind")
	errors.CheckError(err)
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().BoolVar(&force, "force", false, "Indicates whether to force delete the resource")
	command.Flags().BoolVar(&orphan, "orphan", false, "Indicates whether to orphan the dependents of the deleted resource")
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to patch multiple matching of resources")
	command.Flags().StringVar(&project, "project", "", `The name of the application's project - specifying this allows the command to report "not found" instead of "permission denied" if the app does not exist`)
	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseFromQualifiedName(args[0], "")

		conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
		defer utilio.Close(conn)
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
			ApplicationName: &appName,
			AppNamespace:    &appNs,
		})
		errors.CheckError(err)
		objectsToDelete, err := util.FilterResources(command.Flags().Changed("group"), resources.Items, group, kind, namespace, resourceName, all)
		errors.CheckError(err)

		promptUtil := utils.NewPrompt(clientOpts.PromptsEnabled)

		for i := range objectsToDelete {
			obj := objectsToDelete[i]
			gvk := obj.GroupVersionKind()

			canDelete := promptUtil.Confirm(fmt.Sprintf("Are you sure you want to delete %s/%s %s/%s ? [y/n]", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName()))
			if canDelete {
				_, err = appIf.DeleteResource(ctx, &applicationpkg.ApplicationResourceDeleteRequest{
					Name:         &appName,
					AppNamespace: &appNs,
					Namespace:    ptr.To(obj.GetNamespace()),
					ResourceName: ptr.To(obj.GetName()),
					Version:      ptr.To(gvk.Version),
					Group:        ptr.To(gvk.Group),
					Kind:         ptr.To(gvk.Kind),
					Force:        &force,
					Orphan:       &orphan,
					Project:      ptr.To(project),
				})
				errors.CheckError(err)
				log.Infof("Resource '%s' deleted", obj.GetName())
			} else {
				fmt.Printf("The command to delete %s/%s %s/%s was cancelled.\n", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName())
			}
		}
	}

	return command
}

func parentChildInfo(nodes []v1alpha1.ResourceNode) (map[string]v1alpha1.ResourceNode, map[string][]string, map[string]struct{}) {
	mapUIDToNode := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})

	for _, node := range nodes {
		mapUIDToNode[node.UID] = node

		if len(node.ParentRefs) > 0 {
			_, ok := mapParentToChild[node.ParentRefs[0].UID]
			if !ok {
				var temp []string
				mapParentToChild[node.ParentRefs[0].UID] = temp
			}
			mapParentToChild[node.ParentRefs[0].UID] = append(mapParentToChild[node.ParentRefs[0].UID], node.UID)
		} else {
			parentNode[node.UID] = struct{}{}
		}
	}
	return mapUIDToNode, mapParentToChild, parentNode
}

func printDetailedTreeViewAppResourcesNotOrphaned(nodeMapping map[string]v1alpha1.ResourceNode, parentChildMapping map[string][]string, parentNodes map[string]struct{}, w *tabwriter.Writer) {
	for uid := range parentNodes {
		detailedTreeViewAppResourcesNotOrphaned("", nodeMapping, parentChildMapping, nodeMapping[uid], w)
	}
}

func printDetailedTreeViewAppResourcesOrphaned(nodeMapping map[string]v1alpha1.ResourceNode, parentChildMapping map[string][]string, parentNodes map[string]struct{}, w *tabwriter.Writer) {
	for uid := range parentNodes {
		detailedTreeViewAppResourcesOrphaned("", nodeMapping, parentChildMapping, nodeMapping[uid], w)
	}
}

func printTreeViewAppResourcesNotOrphaned(nodeMapping map[string]v1alpha1.ResourceNode, parentChildMapping map[string][]string, parentNodes map[string]struct{}, w *tabwriter.Writer) {
	for uid := range parentNodes {
		treeViewAppResourcesNotOrphaned("", nodeMapping, parentChildMapping, nodeMapping[uid], w)
	}
}

func printTreeViewAppResourcesOrphaned(nodeMapping map[string]v1alpha1.ResourceNode, parentChildMapping map[string][]string, parentNodes map[string]struct{}, w *tabwriter.Writer) {
	for uid := range parentNodes {
		treeViewAppResourcesOrphaned("", nodeMapping, parentChildMapping, nodeMapping[uid], w)
	}
}

func printResources(listAll bool, orphaned bool, appResourceTree *v1alpha1.ApplicationTree, output string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	switch output {
	case "tree=detailed":
		fmt.Fprintf(w, "GROUP\tKIND\tNAMESPACE\tNAME\tORPHANED\tAGE\tHEALTH\tREASON\n")

		if !orphaned || listAll {
			mapUIDToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.Nodes)
			printDetailedTreeViewAppResourcesNotOrphaned(mapUIDToNode, mapParentToChild, parentNode, w)
		}

		if orphaned || listAll {
			mapUIDToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.OrphanedNodes)
			printDetailedTreeViewAppResourcesOrphaned(mapUIDToNode, mapParentToChild, parentNode, w)
		}
	case "tree":
		fmt.Fprintf(w, "GROUP\tKIND\tNAMESPACE\tNAME\tORPHANED\n")

		if !orphaned || listAll {
			mapUIDToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.Nodes)
			printTreeViewAppResourcesNotOrphaned(mapUIDToNode, mapParentToChild, parentNode, w)
		}

		if orphaned || listAll {
			mapUIDToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.OrphanedNodes)
			printTreeViewAppResourcesOrphaned(mapUIDToNode, mapParentToChild, parentNode, w)
		}
	default:
		headers := []any{"GROUP", "KIND", "NAMESPACE", "NAME", "ORPHANED"}
		fmtStr := "%s\t%s\t%s\t%s\t%s\n"
		_, _ = fmt.Fprintf(w, fmtStr, headers...)
		if !orphaned || listAll {
			for _, res := range appResourceTree.Nodes {
				if len(res.ParentRefs) == 0 {
					_, _ = fmt.Fprintf(w, fmtStr, res.Group, res.Kind, res.Namespace, res.Name, "No")
				}
			}
		}
		if orphaned || listAll {
			for _, res := range appResourceTree.OrphanedNodes {
				_, _ = fmt.Fprintf(w, fmtStr, res.Group, res.Kind, res.Namespace, res.Name, "Yes")
			}
		}
	}
	err := w.Flush()
	errors.CheckError(err)
}

func NewApplicationListResourcesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		orphaned bool
		output   string
		project  string
	)
	command := &cobra.Command{
		Use:   "resources APPNAME",
		Short: "List resource of application",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			listAll := !c.Flag("orphaned").Changed
			appName, appNs := argo.ParseFromQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer utilio.Close(conn)
			appResourceTree, err := appIf.ResourceTree(ctx, &applicationpkg.ResourcesQuery{
				ApplicationName: &appName,
				AppNamespace:    &appNs,
				Project:         &project,
			})
			errors.CheckError(err)
			printResources(listAll, orphaned, appResourceTree, output)
		},
	}
	command.Flags().BoolVar(&orphaned, "orphaned", false, "Lists only orphaned resources")
	command.Flags().StringVar(&output, "output", "", "Provides the tree view of the resources")
	command.Flags().StringVar(&project, "project", "", `The name of the application's project - specifying this allows the command to report "not found" instead of "permission denied" if the app does not exist`)
	return command
}
