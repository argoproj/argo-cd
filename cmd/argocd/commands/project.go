package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/git"
)

type projectOpts struct {
	description  string
	destinations []string
	sources      []string
}

type policyOpts struct {
	action     string
	permission string
	object     string
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
	command.AddCommand(NewProjectRoleCommand(clientOpts))
	command.AddCommand(NewProjectCreateCommand(clientOpts))
	command.AddCommand(NewProjectGetCommand(clientOpts))
	command.AddCommand(NewProjectDeleteCommand(clientOpts))
	command.AddCommand(NewProjectListCommand(clientOpts))
	command.AddCommand(NewProjectSetCommand(clientOpts))
	command.AddCommand(NewProjectEditCommand(clientOpts))
	command.AddCommand(NewProjectAddDestinationCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationCommand(clientOpts))
	command.AddCommand(NewProjectAddSourceCommand(clientOpts))
	command.AddCommand(NewProjectRemoveSourceCommand(clientOpts))
	command.AddCommand(NewProjectAllowClusterResourceCommand(clientOpts))
	command.AddCommand(NewProjectDenyClusterResourceCommand(clientOpts))
	command.AddCommand(NewProjectAllowNamespaceResourceCommand(clientOpts))
	command.AddCommand(NewProjectDenyNamespaceResourceCommand(clientOpts))
	return command
}

func addProjFlags(command *cobra.Command, opts *projectOpts) {
	command.Flags().StringVarP(&opts.description, "description", "", "", "Project description")
	command.Flags().StringArrayVarP(&opts.destinations, "dest", "d", []string{},
		"Permitted destination server and namespace (e.g. https://192.168.99.100:8443,default)")
	command.Flags().StringArrayVarP(&opts.sources, "src", "s", []string{}, "Permitted git source repository URL")
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
		opts projectOpts
	)
	var command = &cobra.Command{
		Use:   "create PROJECT",
		Short: "Create a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			proj := v1alpha1.AppProject{
				ObjectMeta: v1.ObjectMeta{Name: projName},
				Spec: v1alpha1.AppProjectSpec{
					Description:  opts.description,
					Destinations: opts.GetDestinations(),
					SourceRepos:  opts.sources,
				},
			}
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err := projIf.Create(context.Background(), &projectpkg.ProjectCreateRequest{Project: &proj})
			errors.CheckError(err)
		},
	}
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			visited := 0
			c.Flags().Visit(func(f *pflag.Flag) {
				visited++
				switch f.Name {
				case "description":
					proj.Spec.Description = opts.description
				case "dest":
					proj.Spec.Destinations = opts.GetDestinations()
				case "src":
					proj.Spec.SourceRepos = opts.sources
				}
			})
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, dest := range proj.Spec.Destinations {
				if dest.Namespace == namespace && dest.Server == server {
					log.Fatal("Specified destination is already defined in project")
				}
			}
			proj.Spec.Destinations = append(proj.Spec.Destinations, v1alpha1.ApplicationDestination{Server: server, Namespace: namespace})
			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
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
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}

	return command
}

// NewProjectAddSourceCommand returns a new instance of an `argocd proj add-src` command
func NewProjectAddSourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "add-source PROJECT URL",
		Short: "Add project source repository",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			url := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
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
			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

func modifyProjectResourceCmd(cmdUse, cmdDesc string, clientOpts *argocdclient.ClientOptions, action func(proj *v1alpha1.AppProject, group string, kind string) bool) *cobra.Command {
	return &cobra.Command{
		Use:   cmdUse,
		Short: cmdDesc,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, group, kind := args[0], args[1], args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			if action(proj, group, kind) {
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
}

// NewProjectAllowNamespaceResourceCommand returns a new instance of an `deny-cluster-resources` command
func NewProjectAllowNamespaceResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "allow-namespace-resource PROJECT GROUP KIND"
	desc := "Removes a namespaced API resource from the blacklist"
	return modifyProjectResourceCmd(use, desc, clientOpts, func(proj *v1alpha1.AppProject, group string, kind string) bool {
		index := -1
		for i, item := range proj.Spec.NamespaceResourceBlacklist {
			if item.Group == group && item.Kind == kind {
				index = i
				break
			}
		}
		if index == -1 {
			fmt.Printf("Group '%s' and kind '%s' not in blacklisted namespaced resources\n", group, kind)
			return false
		}
		proj.Spec.NamespaceResourceBlacklist = append(proj.Spec.NamespaceResourceBlacklist[:index], proj.Spec.NamespaceResourceBlacklist[index+1:]...)
		return true
	})
}

// NewProjectDenyNamespaceResourceCommand returns a new instance of an `argocd proj deny-namespace-resource` command
func NewProjectDenyNamespaceResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "deny-namespace-resource PROJECT GROUP KIND"
	desc := "Adds a namespaced API resource to the blacklist"
	return modifyProjectResourceCmd(use, desc, clientOpts, func(proj *v1alpha1.AppProject, group string, kind string) bool {
		for _, item := range proj.Spec.NamespaceResourceBlacklist {
			if item.Group == group && item.Kind == kind {
				fmt.Printf("Group '%s' and kind '%s' already present in blacklisted namespaced resources\n", group, kind)
				return false
			}
		}
		proj.Spec.NamespaceResourceBlacklist = append(proj.Spec.NamespaceResourceBlacklist, v1.GroupKind{Group: group, Kind: kind})
		return true
	})
}

// NewProjectDenyClusterResourceCommand returns a new instance of an `deny-cluster-resource` command
func NewProjectDenyClusterResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "deny-cluster-resource PROJECT GROUP KIND"
	desc := "Removes a cluster-scoped API resource from the whitelist"
	return modifyProjectResourceCmd(use, desc, clientOpts, func(proj *v1alpha1.AppProject, group string, kind string) bool {
		index := -1
		for i, item := range proj.Spec.ClusterResourceWhitelist {
			if item.Group == group && item.Kind == kind {
				index = i
				break
			}
		}
		if index == -1 {
			fmt.Printf("Group '%s' and kind '%s' not in whitelisted cluster resources\n", group, kind)
			return false
		}
		proj.Spec.ClusterResourceWhitelist = append(proj.Spec.ClusterResourceWhitelist[:index], proj.Spec.ClusterResourceWhitelist[index+1:]...)
		return true
	})
}

// NewProjectAllowClusterResourceCommand returns a new instance of an `argocd proj allow-cluster-resource` command
func NewProjectAllowClusterResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	use := "allow-cluster-resource PROJECT GROUP KIND"
	desc := "Adds a cluster-scoped API resource to the whitelist"
	return modifyProjectResourceCmd(use, desc, clientOpts, func(proj *v1alpha1.AppProject, group string, kind string) bool {
		for _, item := range proj.Spec.ClusterResourceWhitelist {
			if item.Group == group && item.Kind == kind {
				fmt.Printf("Group '%s' and kind '%s' already present in whitelisted cluster resources\n", group, kind)
				return false
			}
		}
		proj.Spec.ClusterResourceWhitelist = append(proj.Spec.ClusterResourceWhitelist, v1.GroupKind{Group: group, Kind: kind})
		return true
	})
}

// NewProjectRemoveSourceCommand returns a new instance of an `argocd proj remove-src` command
func NewProjectRemoveSourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "remove-source PROJECT URL",
		Short: "Remove project source repository",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			url := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
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
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
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
				_, err := projIf.Delete(context.Background(), &projectpkg.ProjectQuery{Name: name})
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
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tDESTINATIONS\tSOURCES\tCLUSTER-RESOURCE-WHITELIST\tNAMESPACE-RESOURCE-BLACKLIST\n")
	for _, p := range projects {
		printProjectLine(w, &p)
	}
	_ = w.Flush()
}

// NewProjectListCommand returns a new instance of an `argocd proj list` command
func NewProjectListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List projects",
		Run: func(c *cobra.Command, args []string) {
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			projects, err := projIf.List(context.Background(), &projectpkg.ProjectQuery{})
			errors.CheckError(err)
			if output == "name" {
				printProjectNames(projects.Items)
			} else {
				printProjectTable(projects.Items)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name")
	return command
}

func printProjectLine(w io.Writer, p *v1alpha1.AppProject) {
	var destinations, sourceRepos, clusterWhitelist, namespaceBlacklist string
	switch len(p.Spec.Destinations) {
	case 0:
		destinations = "<none>"
	case 1:
		destinations = fmt.Sprintf("%s,%s", p.Spec.Destinations[0].Server, p.Spec.Destinations[0].Namespace)
	default:
		destinations = fmt.Sprintf("%d destinations", len(p.Spec.Destinations))
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
	fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%v\t%v\n", p.Name, p.Spec.Description, destinations, sourceRepos, clusterWhitelist, namespaceBlacklist)
}

// NewProjectGetCommand returns a new instance of an `argocd proj get` command
func NewProjectGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	const printProjFmtStr = "%-34s%s\n"
	var command = &cobra.Command{
		Use:   "get PROJECT",
		Short: "Get project details",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			p, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
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

			// Print whitelisted cluster resources
			cwl0 := "<none>"
			if len(p.Spec.ClusterResourceWhitelist) > 0 {
				cwl0 = fmt.Sprintf("%s/%s", p.Spec.ClusterResourceWhitelist[0].Group, p.Spec.ClusterResourceWhitelist[0].Kind)
			}
			fmt.Printf(printProjFmtStr, "Whitelisted Cluster Resources:", cwl0)
			for i := 1; i < len(p.Spec.ClusterResourceWhitelist); i++ {
				fmt.Printf(printProjFmtStr, "", fmt.Sprintf("%s/%s", p.Spec.ClusterResourceWhitelist[i].Group, p.Spec.ClusterResourceWhitelist[i].Kind))
			}

			// Print blacklisted namespaced resources
			rbl0 := "<none>"
			if len(p.Spec.NamespaceResourceBlacklist) > 0 {
				rbl0 = fmt.Sprintf("%s/%s", p.Spec.NamespaceResourceBlacklist[0].Group, p.Spec.NamespaceResourceBlacklist[0].Kind)
			}
			fmt.Printf(printProjFmtStr, "Blacklisted Namespaced Resources:", rbl0)
			for i := 1; i < len(p.Spec.NamespaceResourceBlacklist); i++ {
				fmt.Printf(printProjFmtStr, "", fmt.Sprintf("%s/%s", p.Spec.NamespaceResourceBlacklist[i].Group, p.Spec.NamespaceResourceBlacklist[i].Kind))
			}
		},
	}
	return command
}

func NewProjectEditCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "edit PROJECT",
		Short: "Edit project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			projData, err := json.Marshal(proj.Spec)
			errors.CheckError(err)
			projData, err = yaml.JSONToYAML(projData)
			errors.CheckError(err)

			cli.InteractiveEdit(fmt.Sprintf("%s-*-edit.yaml", projName), projData, func(input []byte) error {
				input, err = yaml.YAMLToJSON(input)
				if err != nil {
					return err
				}
				updatedSpec := v1alpha1.AppProjectSpec{}
				err = json.Unmarshal(input, &updatedSpec)
				if err != nil {
					return err
				}
				proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
				if err != nil {
					return err
				}
				proj.Spec = updatedSpec
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
				if err != nil {
					return fmt.Errorf("Failed to update project:\n%v", err)
				}
				return err
			})
		},
	}
	return command
}
