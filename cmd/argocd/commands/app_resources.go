package commands

import (
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/v2/cmd/util"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"

	"text/tabwriter"
)

func NewApplicationPatchResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var patch string
	var patchType string
	var resourceName string
	var namespace string
	var kind string
	var group string
	var all bool
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
	command.Flags().StringVar(&clientOpts.ServerName, "server-name", env.StringFromEnv(common.EnvServerName, common.DefaultServerName), "Server name")
	command.Flags().StringVar(&clientOpts.RedisHaHaProxyName, "redis-ha-haproxy-name", env.StringFromEnv(common.EnvRedisHaHaproxyName, common.DefaultRedisHaHaproxyName), "Redis HA HAProxy name")
	command.Flags().StringVar(&clientOpts.RedisName, "redis-name", env.StringFromEnv(common.EnvRedisName, common.DefaultRedisName), "Redis name")
	command.Flags().StringVar(&clientOpts.RepoServerName, "repo-server-name", env.StringFromEnv(common.EnvRepoServerName, common.DefaultRepoServerName), "Repo server name")
	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseAppQualifiedName(args[0], "")

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
				Namespace:    pointer.String(obj.GetNamespace()),
				ResourceName: pointer.String(obj.GetName()),
				Version:      pointer.String(gvk.Version),
				Group:        pointer.String(gvk.Group),
				Kind:         pointer.String(gvk.Kind),
				Patch:        pointer.String(patch),
				PatchType:    pointer.String(patchType),
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
	command.Flags().BoolVar(&force, "force", false, "Indicates whether to orphan the dependents of the deleted resource")
	command.Flags().BoolVar(&orphan, "orphan", false, "Indicates whether to force delete the resource")
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to patch multiple matching of resources")
	command.Flags().StringVar(&clientOpts.ServerName, "server-name", env.StringFromEnv(common.EnvServerName, common.DefaultServerName), "Server name")
	command.Flags().StringVar(&clientOpts.RedisHaHaProxyName, "redis-ha-haproxy-name", env.StringFromEnv(common.EnvRedisHaHaproxyName, common.DefaultRedisHaHaproxyName), "Redis HA HAProxy name")
	command.Flags().StringVar(&clientOpts.RedisName, "redis-name", env.StringFromEnv(common.EnvRedisName, common.DefaultRedisName), "Redis name")
	command.Flags().StringVar(&clientOpts.RepoServerName, "repo-server-name", env.StringFromEnv(common.EnvRepoServerName, common.DefaultRepoServerName), "Repo server name")
	command.Run = func(c *cobra.Command, args []string) {
		ctx := c.Context()

		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName, appNs := argo.ParseAppQualifiedName(args[0], "")

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
				Namespace:    pointer.String(obj.GetNamespace()),
				ResourceName: pointer.String(obj.GetName()),
				Version:      pointer.String(gvk.Version),
				Group:        pointer.String(gvk.Group),
				Kind:         pointer.String(gvk.Kind),
				Force:        &force,
				Orphan:       &orphan,
			})
			errors.CheckError(err)
			log.Infof("Resource '%s' deleted", obj.GetName())
		}
	}

	return command
}

func printResources(listAll bool, orphaned bool, appResourceTree *v1alpha1.ApplicationTree) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
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
	_ = w.Flush()
}

func NewApplicationListResourcesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var orphaned bool
	var command = &cobra.Command{
		Use:   "resources APPNAME",
		Short: "List resource of application",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			listAll := !c.Flag("orphaned").Changed
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appResourceTree, err := appIf.ResourceTree(ctx, &applicationpkg.ResourcesQuery{
				ApplicationName: &appName,
				AppNamespace:    &appNs,
			})
			errors.CheckError(err)
			printResources(listAll, orphaned, appResourceTree)
		},
	}
	command.Flags().BoolVar(&orphaned, "orphaned", false, "Lists only orphaned resources")
	command.Flags().StringVar(&clientOpts.ServerName, "server-name", env.StringFromEnv(common.EnvServerName, common.DefaultServerName), "Server name")
	command.Flags().StringVar(&clientOpts.RedisHaHaProxyName, "redis-ha-haproxy-name", env.StringFromEnv(common.EnvRedisHaHaproxyName, common.DefaultRedisHaHaproxyName), "Redis HA HAProxy name")
	command.Flags().StringVar(&clientOpts.RedisName, "redis-name", env.StringFromEnv(common.EnvRedisName, common.DefaultRedisName), "Redis name")
	command.Flags().StringVar(&clientOpts.RepoServerName, "repo-server-name", env.StringFromEnv(common.EnvRepoServerName, common.DefaultRepoServerName), "Repo server name")
	return command
}
