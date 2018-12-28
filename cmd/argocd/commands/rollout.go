package commands

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/controller/services"
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
)

// NewRolloutCommand returns a new instance of an `argocd rollout` command
func NewRolloutCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rollout",
		Short: "Manage rollout resource",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewRolloutProgressCommand(clientOpts))
	return command
}

// NewRolloutProgressCommand returns a new instance of an `argocd rollout progress` command
func NewRolloutProgressCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "progress APPNAME RESOURCENAME",
		Short: "Modifies rollout resource to allow it to progress past the verifying preview step",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			ctx := context.Background()
			resources, err := appIf.ManagedResources(ctx, &services.ResourcesQuery{ApplicationName: appName})
			errors.CheckError(err)
			verifyingPreviewPatch := "{ \"status\": { \"verifyingPreview\": false } }"
			resourceName := args[1]
			liveObjs, err := liveObjects(resources.Items)
			errors.CheckError(err)
			for _, obj := range liveObjs {
				if resourceName == obj.GetName() {
					gvk := obj.GroupVersionKind()
					_, err := appIf.PatchResource(ctx, &application.ApplicationResourcePatchRequest{
						Name:         &appName,
						Namespace:    obj.GetNamespace(),
						ResourceName: resourceName,
						Version:      gvk.Version,
						Group:        gvk.Group,
						Kind:         gvk.Kind,
						Patch:        verifyingPreviewPatch,
						PatchType:    string(types.MergePatchType),
					})
					errors.CheckError(err)
				}
			}
		},
	}
	return command
}
