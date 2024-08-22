package commands

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
)

func NewApplicationPatchResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var patch string
	var patchType string
	var resourceName string
	var namespace string
	var kind string
	var group string
	var all bool
	var project string
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
		defer argoio.Close(conn)
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
	var resourceName string
	var namespace string
	var kind string
	var group string
	var force bool
	var orphan bool
	var all bool
	var project string
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
		defer argoio.Close(conn)
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
			ApplicationName: &appName,
			AppNamespace:    &appNs,
		})
		errors.CheckError(err)
		objectsToDelete, err := util.FilterResources(command.Flags().Changed("group"), resources.Items, group, kind, namespace, resourceName, all)
		errors.CheckError(err)
		for i := range objectsToDelete {
			obj := objectsToDelete[i]
			gvk := obj.GroupVersionKind()
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
		}
	}

	return command
}

func parentChildInfo(nodes []v1alpha1.ResourceNode) (map[string]v1alpha1.ResourceNode, map[string][]string, map[string]struct{}) {
	mapUidToNode := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})

	for _, node := range nodes {
		mapUidToNode[node.UID] = node

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
	return mapUidToNode, mapParentToChild, parentNode
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
	if output == "tree=detailed" {
		fmt.Fprintf(w, "GROUP\tKIND\tNAMESPACE\tNAME\tORPHANED\tAGE\tHEALTH\tREASON\n")

		if !orphaned || listAll {
			mapUidToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.Nodes)
			printDetailedTreeViewAppResourcesNotOrphaned(mapUidToNode, mapParentToChild, parentNode, w)
		}

		if orphaned || listAll {
			mapUidToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.OrphanedNodes)
			printDetailedTreeViewAppResourcesOrphaned(mapUidToNode, mapParentToChild, parentNode, w)
		}
	} else if output == "tree" {
		fmt.Fprintf(w, "GROUP\tKIND\tNAMESPACE\tNAME\tORPHANED\n")

		if !orphaned || listAll {
			mapUidToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.Nodes)
			printTreeViewAppResourcesNotOrphaned(mapUidToNode, mapParentToChild, parentNode, w)
		}

		if orphaned || listAll {
			mapUidToNode, mapParentToChild, parentNode := parentChildInfo(appResourceTree.OrphanedNodes)
			printTreeViewAppResourcesOrphaned(mapUidToNode, mapParentToChild, parentNode, w)
		}
	} else {
		headers := []interface{}{"GROUP", "KIND", "NAMESPACE", "NAME", "ORPHANED"}
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
	_ = w.Flush()
}

func NewApplicationListResourcesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var orphaned bool
	var output string
	var project string
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
			defer argoio.Close(conn)
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
