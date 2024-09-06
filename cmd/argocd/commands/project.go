package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	humanize "github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/gpg"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/templates"
)

type policyOpts struct {
	action     string
	permission string
	object     string
}

// NewProjectCommand returns a new instance of an `argocd proj` command
func NewProjectCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "proj",
		Short: "Manage projects",
		Example: templates.Examples(`
			# List all available projects
			argocd proj list

			# Create a new project with name PROJECT
			argocd proj create PROJECT

			# Delete the project with name PROJECT
			argocd proj delete PROJECT

			# Edit the information on project with name PROJECT
			argocd proj edit PROJECT
		`),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewProjectRoleCommand(clientOpts))
	command.AddCommand(NewProjectCreateCommand(clientOpts))
	command.AddCommand(NewProjectGetCommand(clientOpts))
	command.AddCommand(NewProjectDeleteCommand(clientOpts))
	command.AddCommand(NewProjectListCommand(clientOpts))
	command.AddCommand(NewProjectSetCommand(clientOpts))
	command.AddCommand(NewProjectEditCommand(clientOpts))
	command.AddCommand(NewProjectAddSignatureKeyCommand(clientOpts))
	command.AddCommand(NewProjectRemoveSignatureKeyCommand(clientOpts))
	command.AddCommand(NewProjectAddDestinationCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationCommand(clientOpts))
	command.AddCommand(NewProjectAddSourceCommand(clientOpts))
	command.AddCommand(NewProjectRemoveSourceCommand(clientOpts))
	command.AddCommand(NewProjectAllowClusterResourceCommand(clientOpts))
	command.AddCommand(NewProjectDenyClusterResourceCommand(clientOpts))
	command.AddCommand(NewProjectAllowNamespaceResourceCommand(clientOpts))
	command.AddCommand(NewProjectDenyNamespaceResourceCommand(clientOpts))
	command.AddCommand(NewProjectWindowsCommand(clientOpts))
	command.AddCommand(NewProjectAddOrphanedIgnoreCommand(clientOpts))
	command.AddCommand(NewProjectRemoveOrphanedIgnoreCommand(clientOpts))
	command.AddCommand(NewProjectAddSourceNamespace(clientOpts))
	command.AddCommand(NewProjectRemoveSourceNamespace(clientOpts))
	command.AddCommand(NewProjectAddDestinationServiceAccountCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationServiceAccountCommand(clientOpts))
	return command
}

func addPolicyFlags(command *cobra.Command, opts *policyOpts) {
	command.Flags().StringVarP(&opts.action, "action", "a", "", "Action to grant/deny permission on (e.g. get, create, list, update, delete)")
	command.Flags().StringVarP(&opts.permission, "permission", "p", "allow", "Whether to allow or deny access to object with the action.  This can only be 'allow' or 'deny'")
	command.Flags().StringVarP(&opts.object, "object", "o", "", "Object within the project to grant/deny access.  Use '*' for a wildcard. Will want access to '<project>/<object>'")
}

func humanizeTimestamp(epoch int64) string {
	ts := time.Unix(epoch, 0)
	return fmt.Sprintf("%s (%s)", ts.Format(time.RFC3339), humanize.Time(ts))
}

// NewProjectCreateCommand returns a new instance of an `argocd proj create` command
func NewProjectCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts    cmdutil.ProjectOpts
		fileURL string
		upsert  bool
	)
	command := &cobra.Command{
		Use:   "create PROJECT",
		Short: "Create a project",
		Example: templates.Examples(`
			# Create a new project with name PROJECT
			argocd proj create PROJECT

			# Create a new project with name PROJECT from a file or URL to a Kubernetes manifest
			argocd proj create PROJECT -f FILE|URL
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			proj, err := cmdutil.ConstructAppProj(fileURL, args, opts, c)
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)
			_, err = projIf.Create(ctx, &projectpkg.ProjectCreateRequest{Project: proj, Upsert: upsert})
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override a project with the same name even if supplied project spec is different from existing spec")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the project")
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	if err != nil {
		log.Fatal(err)
	}
	cmdutil.AddProjFlags(command, &opts)
	return command
}

// NewProjectSetCommand returns a new instance of an `argocd proj set` command
func NewProjectSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var opts cmdutil.ProjectOpts
	command := &cobra.Command{
		Use:   "set PROJECT",
		Short: "Set project parameters",
		Example: templates.Examples(`
			# Set project parameters with some allowed cluster resources [RES1,RES2,...] for project with name PROJECT
			argocd proj set PROJECT --allow-cluster-resource [RES1,RES2,...]

			# Set project parameters with some denied namespaced resources [RES1,RES2,...] for project with name PROJECT
			argocd proj set PROJECT ---deny-namespaced-resource [RES1,RES2,...]
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if visited := cmdutil.SetProjSpecOptions(c.Flags(), &proj.Spec, &opts); visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	cmdutil.AddProjFlags(command, &opts)
	return command
}

// NewProjectAddSignatureKeyCommand returns a new instance of an `argocd proj add-signature-key` command
func NewProjectAddSignatureKeyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "add-signature-key PROJECT KEY-ID",
		Short: "Add GnuPG signature key to project",
		Example: templates.Examples(`
			# Add GnuPG signature key KEY-ID to project PROJECT
			argocd proj add-signature-key PROJECT KEY-ID
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			signatureKey := args[1]

			if !gpg.IsShortKeyID(signatureKey) && !gpg.IsLongKeyID(signatureKey) {
				log.Fatalf("%s is not a valid GnuPG key ID", signatureKey)
			}

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, key := range proj.Spec.SignatureKeys {
				if key.KeyID == signatureKey {
					log.Fatal("Specified signature key is already defined in project")
				}
			}
			proj.Spec.SignatureKeys = append(proj.Spec.SignatureKeys, v1alpha1.SignatureKey{KeyID: signatureKey})
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRemoveSignatureKeyCommand returns a new instance of an `argocd proj remove-signature-key` command
func NewProjectRemoveSignatureKeyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-signature-key PROJECT KEY-ID",
		Short: "Remove GnuPG signature key from project",
		Example: templates.Examples(`
			# Remove GnuPG signature key KEY-ID from project PROJECT
			argocd proj remove-signature-key PROJECT KEY-ID
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			signatureKey := args[1]

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, key := range proj.Spec.SignatureKeys {
				if key.KeyID == signatureKey {
					index = i
					break
				}
			}
			if index == -1 {
				log.Fatal("Specified signature key is not configured for project")
			} else {
				proj.Spec.SignatureKeys = append(proj.Spec.SignatureKeys[:index], proj.Spec.SignatureKeys[index+1:]...)
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}

	return command
}

// NewProjectAddDestinationCommand returns a new instance of an `argocd proj add-destination` command
func NewProjectAddDestinationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var nameInsteadServer bool

	buildApplicationDestination := func(destination string, namespace string, nameInsteadServer bool) v1alpha1.ApplicationDestination {
		if nameInsteadServer {
			return v1alpha1.ApplicationDestination{Name: destination, Namespace: namespace}
		}
		return v1alpha1.ApplicationDestination{Server: destination, Namespace: namespace}
	}

	command := &cobra.Command{
		Use:   "add-destination PROJECT SERVER/NAME NAMESPACE",
		Short: "Add project destination",
		Example: templates.Examples(`
			# Add project destination using a server URL (SERVER) in the specified namespace (NAMESPACE) on the project with name PROJECT
			argocd proj add-destination PROJECT SERVER NAMESPACE

			# Add project destination using a server name (NAME) in the specified namespace (NAMESPACE) on the project with name PROJECT
			argocd proj add-destination PROJECT NAME NAMESPACE --name
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			namespace := args[2]
			destination := buildApplicationDestination(args[1], namespace, nameInsteadServer)
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, dest := range proj.Spec.Destinations {
				dstServerExist := destination.Server != "" && dest.Server == destination.Server
				dstNameExist := destination.Name != "" && dest.Name == destination.Name
				if dest.Namespace == namespace && (dstServerExist || dstNameExist) {
					log.Fatal("Specified destination is already defined in project")
				}
			}
			proj.Spec.Destinations = append(proj.Spec.Destinations, destination)
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVar(&nameInsteadServer, "name", false, "Use name as destination instead server")
	return command
}

// NewProjectRemoveDestinationCommand returns a new instance of an `argocd proj remove-destination` command
func NewProjectRemoveDestinationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-destination PROJECT SERVER NAMESPACE",
		Short: "Remove project destination",
		Example: templates.Examples(`
			# Remove the destination (SERVER) from the specified namespace (NAMESPACE) on the project with name PROJECT
			argocd proj remove-destination PROJECT SERVER NAMESPACE
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			server := args[1]
			namespace := args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
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
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}

	return command
}

// NewProjectAddOrphanedIgnoreCommand returns a new instance of an `argocd proj add-orphaned-ignore` command
func NewProjectAddOrphanedIgnoreCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var name string
	command := &cobra.Command{
		Use:   "add-orphaned-ignore PROJECT GROUP KIND",
		Short: "Add a resource to orphaned ignore list",
		Example: templates.Examples(`
		# Add a resource of the specified GROUP and KIND to orphaned ignore list on the project with name PROJECT
		argocd proj add-orphaned-ignore PROJECT GROUP KIND

		# Add resources of the specified GROUP and KIND using a NAME pattern to orphaned ignore list on the project with name PROJECT
		argocd proj add-orphaned-ignore PROJECT GROUP KIND --name NAME
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			group := args[1]
			kind := args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if proj.Spec.OrphanedResources == nil {
				settings := v1alpha1.OrphanedResourcesMonitorSettings{}
				settings.Ignore = []v1alpha1.OrphanedResourceKey{{Group: group, Kind: kind, Name: name}}
				proj.Spec.OrphanedResources = &settings
			} else {
				for _, ignore := range proj.Spec.OrphanedResources.Ignore {
					if ignore.Group == group && ignore.Kind == kind && ignore.Name == name {
						log.Fatal("Specified resource is already defined in the orphaned ignore list of project")
						return
					}
				}
				proj.Spec.OrphanedResources.Ignore = append(proj.Spec.OrphanedResources.Ignore, v1alpha1.OrphanedResourceKey{Group: group, Kind: kind, Name: name})
			}
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&name, "name", "", "Resource name pattern")
	return command
}

// NewProjectRemoveOrphanedIgnoreCommand returns a new instance of an `argocd proj remove-orphaned-ignore` command
func NewProjectRemoveOrphanedIgnoreCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var name string
	command := &cobra.Command{
		Use:   "remove-orphaned-ignore PROJECT GROUP KIND",
		Short: "Remove a resource from orphaned ignore list",
		Example: templates.Examples(`
		# Remove a resource of the specified GROUP and KIND from orphaned ignore list on the project with name PROJECT
		argocd proj remove-orphaned-ignore PROJECT GROUP KIND

		# Remove resources of the specified GROUP and KIND using a NAME pattern from orphaned ignore list on the project with name PROJECT
		argocd proj remove-orphaned-ignore PROJECT GROUP KIND --name NAME
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			group := args[1]
			kind := args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if proj.Spec.OrphanedResources == nil {
				log.Fatal("Specified resource does not exist in the orphaned ignore list of project")
				return
			}

			index := -1
			for i, ignore := range proj.Spec.OrphanedResources.Ignore {
				if ignore.Group == group && ignore.Kind == kind && ignore.Name == name {
					index = i
					break
				}
			}
			if index == -1 {
				log.Fatal("Specified resource does not exist in the orphaned ignore of project")
			} else {
				proj.Spec.OrphanedResources.Ignore = append(proj.Spec.OrphanedResources.Ignore[:index], proj.Spec.OrphanedResources.Ignore[index+1:]...)
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	command.Flags().StringVar(&name, "name", "", "Resource name pattern")
	return command
}

// NewProjectAddSourceCommand returns a new instance of an `argocd proj add-src` command
func NewProjectAddSourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "add-source PROJECT URL",
		Short: "Add project source repository",
		Example: templates.Examples(`
			# Add a source repository (URL) to the project with name PROJECT
			argocd proj add-source PROJECT URL
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			url := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, item := range proj.Spec.SourceRepos {
				if item == "*" && item == url {
					fmt.Printf("Source repository '*' already allowed in project\n")
					return
				}
				if git.SameURL(item, url) {
					fmt.Printf("Source repository '%s' already allowed in project\n", item)
					return
				}
			}
			proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, url)
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectAddSourceNamespace returns a new instance of an `argocd proj add-source-namespace` command
func NewProjectAddSourceNamespace(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "add-source-namespace PROJECT NAMESPACE",
		Short: "Add source namespace to the AppProject",
		Example: templates.Examples(`
			# Add Kubernetes namespace as source namespace to the AppProject where application resources are allowed to be created in.
			argocd proj add-source-namespace PROJECT NAMESPACE
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			srcNamespace := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, item := range proj.Spec.SourceNamespaces {
				if item == "*" || item == srcNamespace {
					fmt.Printf("Source namespace '*' already allowed in project\n")
					return
				}
			}
			proj.Spec.SourceNamespaces = append(proj.Spec.SourceNamespaces, srcNamespace)
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRemoveSourceNamespace returns a new instance of an `argocd proj remove-source-namespace` command
func NewProjectRemoveSourceNamespace(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-source-namespace PROJECT NAMESPACE",
		Short: "Removes the source namespace from the AppProject",
		Example: templates.Examples(`
			# Remove source NAMESPACE in PROJECT 
			argocd proj remove-source-namespace PROJECT NAMESPACE
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			srcNamespace := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, item := range proj.Spec.SourceNamespaces {
				if item == srcNamespace && item != "*" {
					index = i
					break
				}
			}
			if index == -1 {
				fmt.Printf("Source namespace '%s' does not exist in project or cannot be removed\n", srcNamespace)
			} else {
				proj.Spec.SourceNamespaces = append(proj.Spec.SourceNamespaces[:index], proj.Spec.SourceNamespaces[index+1:]...)
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}

	return command
}

func modifyResourcesList(list *[]metav1.GroupKind, add bool, listDesc string, group string, kind string) bool {
	if add {
		for _, item := range *list {
			if item.Group == group && item.Kind == kind {
				fmt.Printf("Group '%s' and kind '%s' already present in %s resources\n", group, kind, listDesc)
				return false
			}
		}
		fmt.Printf("Group '%s' and kind '%s' is added to %s resources\n", group, kind, listDesc)
		*list = append(*list, metav1.GroupKind{Group: group, Kind: kind})
		return true
	} else {
		index := -1
		for i, item := range *list {
			if item.Group == group && item.Kind == kind {
				index = i
				break
			}
		}
		if index == -1 {
			fmt.Printf("Group '%s' and kind '%s' not in %s resources\n", group, kind, listDesc)
			return false
		}
		*list = append((*list)[:index], (*list)[index+1:]...)
		fmt.Printf("Group '%s' and kind '%s' is removed from %s resources\n", group, kind, listDesc)
		return true
	}
}

func modifyResourceListCmd(cmdUse, cmdDesc, examples string, clientOpts *argocdclient.ClientOptions, allow bool, namespacedList bool) *cobra.Command {
	var (
		listType    string
		defaultList string
	)
	if namespacedList {
		defaultList = "deny"
	} else {
		defaultList = "allow"
	}
	command := &cobra.Command{
		Use:     cmdUse,
		Short:   cmdDesc,
		Example: templates.Examples(examples),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, group, kind := args[0], args[1], args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			var list, allowList, denyList *[]metav1.GroupKind
			var listAction, listDesc string
			var add bool
			if namespacedList {
				allowList, denyList = &proj.Spec.NamespaceResourceWhitelist, &proj.Spec.NamespaceResourceBlacklist
				listDesc = "namespaced"
			} else {
				allowList, denyList = &proj.Spec.ClusterResourceWhitelist, &proj.Spec.ClusterResourceBlacklist
				listDesc = "cluster"
			}

			if (listType == "allow") || (listType == "white") {
				list = allowList
				listAction = "allowed"
				add = allow
			} else {
				list = denyList
				listAction = "denied"
				add = !allow
			}

			if modifyResourcesList(list, add, listAction+" "+listDesc, group, kind) {
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	command.Flags().StringVarP(&listType, "list", "l", defaultList, "Use deny list or allow list. This can only be 'allow' or 'deny'")
	return command
}

// NewProjectAllowNamespaceResourceCommand returns a new instance of an `deny-cluster-resources` command
func NewProjectAllowNamespaceResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "allow-namespace-resource PROJECT GROUP KIND"
	desc := "Removes a namespaced API resource from the deny list or add a namespaced API resource to the allow list"
	examples := `
	# Removes a namespaced API resource with specified GROUP and KIND from the deny list or add a namespaced API resource to the allow list for project PROJECT
	argocd proj allow-namespace-resource PROJECT GROUP KIND
	`
	return modifyResourceListCmd(use, desc, examples, clientOpts, true, true)
}

// NewProjectDenyNamespaceResourceCommand returns a new instance of an `argocd proj deny-namespace-resource` command
func NewProjectDenyNamespaceResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "deny-namespace-resource PROJECT GROUP KIND"
	desc := "Adds a namespaced API resource to the deny list or removes a namespaced API resource from the allow list"
	examples := `
	# Adds a namespaced API resource with specified GROUP and KIND from the deny list or removes a namespaced API resource from the allow list for project PROJECT
	argocd proj deny-namespace-resource PROJECT GROUP KIND
	`
	return modifyResourceListCmd(use, desc, examples, clientOpts, false, true)
}

// NewProjectDenyClusterResourceCommand returns a new instance of an `deny-cluster-resource` command
func NewProjectDenyClusterResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "deny-cluster-resource PROJECT GROUP KIND"
	desc := "Removes a cluster-scoped API resource from the allow list and adds it to deny list"
	examples := `
	# Removes a cluster-scoped API resource with specified GROUP and KIND from the allow list and adds it to deny list for project PROJECT
	argocd proj deny-cluster-resource PROJECT GROUP KIND
	`
	return modifyResourceListCmd(use, desc, examples, clientOpts, false, false)
}

// NewProjectAllowClusterResourceCommand returns a new instance of an `argocd proj allow-cluster-resource` command
func NewProjectAllowClusterResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "allow-cluster-resource PROJECT GROUP KIND"
	desc := "Adds a cluster-scoped API resource to the allow list and removes it from deny list"
	examples := `
	# Adds a cluster-scoped API resource with specified GROUP and KIND to the allow list and removes it from deny list for project PROJECT
	argocd proj allow-cluster-resource PROJECT GROUP KIND
	`
	return modifyResourceListCmd(use, desc, examples, clientOpts, true, false)
}

// NewProjectRemoveSourceCommand returns a new instance of an `argocd proj remove-src` command
func NewProjectRemoveSourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-source PROJECT URL",
		Short: "Remove project source repository",
		Example: templates.Examples(`
			# Remove URL source repository to project PROJECT
			argocd proj remove-source PROJECT URL
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			url := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, item := range proj.Spec.SourceRepos {
				if item == url {
					index = i
					break
				}
			}
			if index == -1 {
				fmt.Printf("Source repository '%s' does not exist in project\n", url)
			} else {
				proj.Spec.SourceRepos = append(proj.Spec.SourceRepos[:index], proj.Spec.SourceRepos[index+1:]...)
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}

	return command
}

// NewProjectDeleteCommand returns a new instance of an `argocd proj delete` command
func NewProjectDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete PROJECT",
		Short: "Delete project",
		Example: templates.Examples(`
			# Delete the project with name PROJECT
			argocd proj delete PROJECT
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)
			for _, name := range args {
				_, err := projIf.Delete(ctx, &projectpkg.ProjectQuery{Name: name})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// Print list of project names
func printProjectNames(projects []v1alpha1.AppProject) {
	for _, p := range projects {
		fmt.Println(p.Name)
	}
}

// Print table of project info
func printProjectTable(projects []v1alpha1.AppProject) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tDESTINATIONS\tSOURCES\tCLUSTER-RESOURCE-WHITELIST\tNAMESPACE-RESOURCE-BLACKLIST\tSIGNATURE-KEYS\tORPHANED-RESOURCES\tDESTINATION-SERVICE-ACCOUNTS\n")
	for _, p := range projects {
		printProjectLine(w, &p)
	}
	_ = w.Flush()
}

// NewProjectListCommand returns a new instance of an `argocd proj list` command
func NewProjectListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Example: templates.Examples(`
			# List all available projects
			argocd proj list

			# List all available projects in yaml format
			argocd proj list -o yaml
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)
			projects, err := projIf.List(ctx, &projectpkg.ProjectQuery{})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(projects.Items, output, false)
				errors.CheckError(err)
			case "name":
				printProjectNames(projects.Items)
			case "wide", "":
				printProjectTable(projects.Items)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|name")
	return command
}

func formatOrphanedResources(p *v1alpha1.AppProject) string {
	if p.Spec.OrphanedResources == nil {
		return "disabled"
	}
	details := fmt.Sprintf("warn=%v", p.Spec.OrphanedResources.IsWarn())
	if len(p.Spec.OrphanedResources.Ignore) > 0 {
		details = fmt.Sprintf("%s, ignored %d", details, len(p.Spec.OrphanedResources.Ignore))
	}
	return fmt.Sprintf("enabled (%s)", details)
}

func printProjectLine(w io.Writer, p *v1alpha1.AppProject) {
	var destinations, destinationServiceAccounts, sourceRepos, clusterWhitelist, namespaceBlacklist, signatureKeys string
	switch len(p.Spec.Destinations) {
	case 0:
		destinations = "<none>"
	case 1:
		destinations = fmt.Sprintf("%s,%s", p.Spec.Destinations[0].Server, p.Spec.Destinations[0].Namespace)
	default:
		destinations = fmt.Sprintf("%d destinations", len(p.Spec.Destinations))
	}
	switch len(p.Spec.DestinationServiceAccounts) {
	case 0:
		destinationServiceAccounts = "<none>"
	case 1:
		destinationServiceAccounts = fmt.Sprintf("%s,%s,%s", p.Spec.DestinationServiceAccounts[0].Server, p.Spec.DestinationServiceAccounts[0].Namespace, p.Spec.DestinationServiceAccounts[0].DefaultServiceAccount)
	default:
		destinationServiceAccounts = fmt.Sprintf("%d destinationServiceAccounts", len(p.Spec.DestinationServiceAccounts))
	}
	switch len(p.Spec.SourceRepos) {
	case 0:
		sourceRepos = "<none>"
	case 1:
		sourceRepos = p.Spec.SourceRepos[0]
	default:
		sourceRepos = fmt.Sprintf("%d repos", len(p.Spec.SourceRepos))
	}
	switch len(p.Spec.ClusterResourceWhitelist) {
	case 0:
		clusterWhitelist = "<none>"
	case 1:
		clusterWhitelist = fmt.Sprintf("%s/%s", p.Spec.ClusterResourceWhitelist[0].Group, p.Spec.ClusterResourceWhitelist[0].Kind)
	default:
		clusterWhitelist = fmt.Sprintf("%d resources", len(p.Spec.ClusterResourceWhitelist))
	}
	switch len(p.Spec.NamespaceResourceBlacklist) {
	case 0:
		namespaceBlacklist = "<none>"
	default:
		namespaceBlacklist = fmt.Sprintf("%d resources", len(p.Spec.NamespaceResourceBlacklist))
	}
	switch len(p.Spec.SignatureKeys) {
	case 0:
		signatureKeys = "<none>"
	default:
		signatureKeys = fmt.Sprintf("%d key(s)", len(p.Spec.SignatureKeys))
	}
	fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n", p.Name, p.Spec.Description, destinations, sourceRepos, clusterWhitelist, namespaceBlacklist, signatureKeys, formatOrphanedResources(p), destinationServiceAccounts)
}

func printProject(p *v1alpha1.AppProject, scopedRepositories []*v1alpha1.Repository, scopedClusters []*v1alpha1.Cluster) {
	const printProjFmtStr = "%-29s%s\n"

	fmt.Printf(printProjFmtStr, "Name:", p.Name)
	fmt.Printf(printProjFmtStr, "Description:", p.Spec.Description)

	// Print destinations
	dest0 := "<none>"
	if len(p.Spec.Destinations) > 0 {
		dest0 = fmt.Sprintf("%s,%s", p.Spec.Destinations[0].Server, p.Spec.Destinations[0].Namespace)
	}
	fmt.Printf(printProjFmtStr, "Destinations:", dest0)
	for i := 1; i < len(p.Spec.Destinations); i++ {
		fmt.Printf(printProjFmtStr, "", fmt.Sprintf("%s,%s", p.Spec.Destinations[i].Server, p.Spec.Destinations[i].Namespace))
	}

	// Print sources
	src0 := "<none>"
	if len(p.Spec.SourceRepos) > 0 {
		src0 = p.Spec.SourceRepos[0]
	}
	fmt.Printf(printProjFmtStr, "Repositories:", src0)
	for i := 1; i < len(p.Spec.SourceRepos); i++ {
		fmt.Printf(printProjFmtStr, "", p.Spec.SourceRepos[i])
	}

	// Print source namespaces
	ns0 := "<none>"
	if len(p.Spec.SourceNamespaces) > 0 {
		ns0 = p.Spec.SourceNamespaces[0]
	}
	fmt.Printf(printProjFmtStr, "Source Namespaces:", ns0)
	for i := 1; i < len(p.Spec.SourceNamespaces); i++ {
		fmt.Printf(printProjFmtStr, "", p.Spec.SourceNamespaces[i])
	}

	// Print scoped repositories
	scr0 := "<none>"
	if len(scopedRepositories) > 0 {
		scr0 = scopedRepositories[0].Repo
	}
	fmt.Printf(printProjFmtStr, "Scoped Repositories:", scr0)
	for i := 1; i < len(scopedRepositories); i++ {
		fmt.Printf(printProjFmtStr, "", scopedRepositories[i].Repo)
	}

	// Print allowed cluster resources
	cwl0 := "<none>"
	if len(p.Spec.ClusterResourceWhitelist) > 0 {
		cwl0 = fmt.Sprintf("%s/%s", p.Spec.ClusterResourceWhitelist[0].Group, p.Spec.ClusterResourceWhitelist[0].Kind)
	}
	fmt.Printf(printProjFmtStr, "Allowed Cluster Resources:", cwl0)
	for i := 1; i < len(p.Spec.ClusterResourceWhitelist); i++ {
		fmt.Printf(printProjFmtStr, "", fmt.Sprintf("%s/%s", p.Spec.ClusterResourceWhitelist[i].Group, p.Spec.ClusterResourceWhitelist[i].Kind))
	}

	// Print scoped clusters
	scc0 := "<none>"
	if len(scopedClusters) > 0 {
		scc0 = scopedClusters[0].Server
	}
	fmt.Printf(printProjFmtStr, "Scoped Clusters:", scc0)
	for i := 1; i < len(scopedClusters); i++ {
		fmt.Printf(printProjFmtStr, "", scopedClusters[i].Server)
	}

	// Print denied namespaced resources
	rbl0 := "<none>"
	if len(p.Spec.NamespaceResourceBlacklist) > 0 {
		rbl0 = fmt.Sprintf("%s/%s", p.Spec.NamespaceResourceBlacklist[0].Group, p.Spec.NamespaceResourceBlacklist[0].Kind)
	}
	fmt.Printf(printProjFmtStr, "Denied Namespaced Resources:", rbl0)
	for i := 1; i < len(p.Spec.NamespaceResourceBlacklist); i++ {
		fmt.Printf(printProjFmtStr, "", fmt.Sprintf("%s/%s", p.Spec.NamespaceResourceBlacklist[i].Group, p.Spec.NamespaceResourceBlacklist[i].Kind))
	}

	// Print required signature keys
	signatureKeysStr := "<none>"
	if len(p.Spec.SignatureKeys) > 0 {
		kids := make([]string, 0)
		for _, key := range p.Spec.SignatureKeys {
			kids = append(kids, key.KeyID)
		}
		signatureKeysStr = strings.Join(kids, ", ")
	}
	fmt.Printf(printProjFmtStr, "Signature keys:", signatureKeysStr)

	fmt.Printf(printProjFmtStr, "Orphaned Resources:", formatOrphanedResources(p))
}

// NewProjectGetCommand returns a new instance of an `argocd proj get` command
func NewProjectGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "get PROJECT",
		Short: "Get project details",
		Example: templates.Examples(`
			# Get details from project PROJECT
			argocd proj get PROJECT

			# Get details from project PROJECT in yaml format
			argocd proj get PROJECT -o yaml

		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			detailedProject := getProject(c, clientOpts, ctx, projName)

			switch output {
			case "yaml", "json":
				err := PrintResource(detailedProject.Project, output)
				errors.CheckError(err)
			case "wide", "":
				printProject(detailedProject.Project, detailedProject.Repositories, detailedProject.Clusters)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

func getProject(c *cobra.Command, clientOpts *argocdclient.ClientOptions, ctx context.Context, projName string) *projectpkg.DetailedProjectsResponse {
	conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
	defer argoio.Close(conn)
	detailedProject, err := projIf.GetDetailedProject(ctx, &projectpkg.ProjectQuery{Name: projName})
	errors.CheckError(err)
	return detailedProject
}

func NewProjectEditCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "edit PROJECT",
		Short: "Edit project",
		Example: templates.Examples(`
			# Edit the information on project with name PROJECT
			argocd proj edit PROJECT
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)
			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			projData, err := json.Marshal(proj.Spec)
			errors.CheckError(err)
			projData, err = yaml.JSONToYAML(projData)
			errors.CheckError(err)

			cli.InteractiveEdit(fmt.Sprintf("%s-*-edit.yaml", projName), projData, func(input []byte) error {
				input, err = yaml.YAMLToJSON(input)
				if err != nil {
					return fmt.Errorf("error converting YAML to JSON: %w", err)
				}
				updatedSpec := v1alpha1.AppProjectSpec{}
				err = json.Unmarshal(input, &updatedSpec)
				if err != nil {
					return fmt.Errorf("error unmarshaling input into application spec: %w", err)
				}
				proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
				if err != nil {
					return fmt.Errorf("could not get project by project name: %w", err)
				}
				proj.Spec = updatedSpec
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				if err != nil {
					return fmt.Errorf("failed to update project:\n%w", err)
				}
				return nil
			})
		},
	}
	return command
}

// NewProjectAddDestinationServiceAccountCommand returns a new instance of an `argocd proj add-destination-service-account` command
func NewProjectAddDestinationServiceAccountCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var serviceAccountNamespace string

	buildApplicationDestinationServiceAccount := func(destination string, namespace string, serviceAccount string, serviceAccountNamespace string) v1alpha1.ApplicationDestinationServiceAccount {
		if serviceAccountNamespace != "" {
			return v1alpha1.ApplicationDestinationServiceAccount{
				Server:                destination,
				Namespace:             namespace,
				DefaultServiceAccount: fmt.Sprintf("%s:%s", serviceAccountNamespace, serviceAccount),
			}
		} else {
			return v1alpha1.ApplicationDestinationServiceAccount{
				Server:                destination,
				Namespace:             namespace,
				DefaultServiceAccount: serviceAccount,
			}
		}
	}

	command := &cobra.Command{
		Use:   "add-destination-service-account PROJECT SERVER NAMESPACE SERVICE_ACCOUNT",
		Short: "Add project destination's default service account",
		Example: templates.Examples(`
			# Add project destination service account (SERVICE_ACCOUNT) for a server URL (SERVER) in the specified namespace (NAMESPACE) on the project with name PROJECT
			argocd proj add-destination-service-account PROJECT SERVER NAMESPACE SERVICE_ACCOUNT

			# Add project destination service account (SERVICE_ACCOUNT) from a different namespace
			argocd proj add-destination PROJECT SERVER NAMESPACE SERVICE_ACCOUNT --service-account-namespace <service_account_namespace>
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 4 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			server := args[1]
			namespace := args[2]
			serviceAccount := args[3]

			if strings.Contains(serviceAccountNamespace, "*") {
				log.Fatal("service-account-namespace for DestinationServiceAccount must not contain wildcards")
			}

			if strings.Contains(serviceAccount, "*") {
				log.Fatal("ServiceAccount for DestinationServiceAccount must not contain wildcards")
			}

			destinationServiceAccount := buildApplicationDestinationServiceAccount(server, namespace, serviceAccount, serviceAccountNamespace)
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, dest := range proj.Spec.DestinationServiceAccounts {
				dstServerExist := destinationServiceAccount.Server != "" && dest.Server == destinationServiceAccount.Server
				dstServiceAccountExist := destinationServiceAccount.DefaultServiceAccount != "" && dest.DefaultServiceAccount == destinationServiceAccount.DefaultServiceAccount
				if dest.Namespace == destinationServiceAccount.Namespace && dstServerExist && dstServiceAccountExist {
					log.Fatal("Specified destination service account is already defined in project")
				}
			}
			proj.Spec.DestinationServiceAccounts = append(proj.Spec.DestinationServiceAccounts, destinationServiceAccount)
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&serviceAccountNamespace, "service-account-namespace", "", "Use service-account-namespace as namespace where the service account is present")
	return command
}

// NewProjectRemoveDestinationCommand returns a new instance of an `argocd proj remove-destination-service-account` command
func NewProjectRemoveDestinationServiceAccountCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-destination-service-account PROJECT SERVER NAMESPACE SERVICE_ACCOUNT",
		Short: "Remove default destination service account from the project",
		Example: templates.Examples(`
			# Remove the destination service account (SERVICE_ACCOUNT) from the specified destination (SERVER and NAMESPACE combination) on the project with name PROJECT
			argocd proj remove-destination-service-account PROJECT SERVER NAMESPACE SERVICE_ACCOUNT
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 4 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			server := args[1]
			namespace := args[2]
			serviceAccount := args[3]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			originalLength := len(proj.Spec.DestinationServiceAccounts)
			proj.Spec.DestinationServiceAccounts = slices.DeleteFunc(proj.Spec.DestinationServiceAccounts,
				func(destServiceAccount v1alpha1.ApplicationDestinationServiceAccount) bool {
					return destServiceAccount.Namespace == namespace &&
						destServiceAccount.Server == server &&
						destServiceAccount.DefaultServiceAccount == serviceAccount
				},
			)
			if originalLength != len(proj.Spec.DestinationServiceAccounts) {
				_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			} else {
				log.Fatal("Specified destination service account does not exist in project")
			}
		},
	}

	return command
}
