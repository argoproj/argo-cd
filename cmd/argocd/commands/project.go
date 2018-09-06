package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	timeutil "github.com/argoproj/pkg/time"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/project"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/git"
	projectutil "github.com/argoproj/argo-cd/util/project"
)

const (
	policyTemplate = "p, proj:%s:%s, applications, %s, %s/%s, %s"
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
	command.AddCommand(NewProjectDeleteCommand(clientOpts))
	command.AddCommand(NewProjectListCommand(clientOpts))
	command.AddCommand(NewProjectSetCommand(clientOpts))
	command.AddCommand(NewProjectAddDestinationCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationCommand(clientOpts))
	command.AddCommand(NewProjectAddSourceCommand(clientOpts))
	command.AddCommand(NewProjectRemoveSourceCommand(clientOpts))
	command.AddCommand(NewProjectAddClusterResCommand(clientOpts))
	command.AddCommand(NewProjectRemoveClusterResCommand(clientOpts))
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

// NewProjectRoleCommand returns a new instance of the `argocd proj role` command
func NewProjectRoleCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "role",
		Short: "Manage a project's roles",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	roleCommand.AddCommand(NewProjectRoleListCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleGetCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleCreateCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleDeleteCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleCreateTokenCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleDeleteTokenCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleAddPolicyCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleRemovePolicyCommand(clientOpts))
	return roleCommand
}

// NewProjectRoleAddPolicyCommand returns a new instance of an `argocd proj role add-policy` command
func NewProjectRoleAddPolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts policyOpts
	)
	var command = &cobra.Command{
		Use:   "add-policy PROJECT ROLE-NAME",
		Short: "Add a policy to a project role",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if len(opts.action) <= 0 {
				log.Fatal("Action needs to longer than 0 characters")
			}
			if len(opts.object) <= 0 {
				log.Fatal("Objects needs to longer than 0 characters")

			}

			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			roleIndex, err := projectutil.GetRoleIndexByName(proj, roleName)
			if err != nil {
				log.Fatal(err)
			}
			role := proj.Spec.Roles[roleIndex]

			policy := fmt.Sprintf(policyTemplate, proj.Name, role.Name, opts.action, proj.Name, opts.object, opts.permission)
			proj.Spec.Roles[roleIndex].Policies = append(role.Policies, policy)

			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectRoleRemovePolicyCommand returns a new instance of an `argocd proj role remove-policy` command
func NewProjectRoleRemovePolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts policyOpts
	)
	var command = &cobra.Command{
		Use:   "remove-policy PROJECT ROLE-NAME",
		Short: "Remove a policy from a role within a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if opts.permission != "allow" && opts.permission != "deny" {
				log.Fatal("Permission flag can only have the values 'allow' or 'deny'")
			}

			if len(opts.action) <= 0 {
				log.Fatal("Action needs to longer than 0 characters")
			}
			if len(opts.object) <= 0 {
				log.Fatal("Objects needs to longer than 0 characters")

			}

			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			roleIndex, err := projectutil.GetRoleIndexByName(proj, roleName)
			if err != nil {
				log.Fatal(err)
			}
			role := proj.Spec.Roles[roleIndex]

			policyToRemove := fmt.Sprintf(policyTemplate, proj.Name, role.Name, opts.action, proj.Name, opts.object, opts.permission)
			duplicateIndex := -1
			for i, policy := range role.Policies {
				if policy == policyToRemove {
					duplicateIndex = i
					break
				}
			}
			if duplicateIndex < 0 {
				return
			}
			role.Policies[duplicateIndex] = role.Policies[len(role.Policies)-1]
			proj.Spec.Roles[roleIndex].Policies = role.Policies[:len(role.Policies)-1]
			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectRoleCreateCommand returns a new instance of an `argocd proj role create` command
func NewProjectRoleCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		description string
	)
	var command = &cobra.Command{
		Use:   "create PROJECT ROLE-NAME",
		Short: "Create a project role",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			_, err = projectutil.GetRoleIndexByName(proj, roleName)
			if err == nil {
				return
			}
			proj.Spec.Roles = append(proj.Spec.Roles, v1alpha1.ProjectRole{Name: roleName, Description: description})

			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&description, "description", "", "", "Project description")
	return command
}

// NewProjectRoleDeleteCommand returns a new instance of an `argocd proj role delete` command
func NewProjectRoleDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT ROLE-NAME",
		Short: "Delete a project role",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index, err := projectutil.GetRoleIndexByName(proj, roleName)
			if err != nil {
				return
			}
			proj.Spec.Roles[index] = proj.Spec.Roles[len(proj.Spec.Roles)-1]
			proj.Spec.Roles = proj.Spec.Roles[:len(proj.Spec.Roles)-1]

			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRoleCreateTokenCommand returns a new instance of an `argocd proj role create-token` command
func NewProjectRoleCreateTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		expiresIn string
	)
	var command = &cobra.Command{
		Use:   "create-token PROJECT ROLE-NAME",
		Short: "Create a project token",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			duration, err := timeutil.ParseDuration(expiresIn)
			errors.CheckError(err)
			token, err := projIf.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projName, Role: roleName, ExpiresIn: int64(duration.Seconds())})
			errors.CheckError(err)
			fmt.Println(token.Token)
		},
	}
	command.Flags().StringVarP(&expiresIn, "expires-in", "e", "0s", "Duration before the token will expire. (Default: No expiration)")

	return command
}

// NewProjectRoleDeleteTokenCommand returns a new instance of an `argocd proj role delete-token` command
func NewProjectRoleDeleteTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete-token PROJECT ROLE-NAME ISSUED-AT",
		Short: "Delete a project token",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			issuedAt, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				log.Fatal(err)
			}

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err = projIf.DeleteToken(context.Background(), &project.ProjectTokenDeleteRequest{Project: projName, Role: roleName, Iat: issuedAt})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRoleListCommand returns a new instance of an `argocd proj roles list` command
func NewProjectRoleListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list PROJECT",
		Short: "List all the roles in a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			project, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ROLE-NAME\tDESCRIPTION\n")
			for _, role := range project.Spec.Roles {
				fmt.Fprintf(w, "%s\t%s\n", role.Name, role.Description)
			}
			_ = w.Flush()
		},
	}
	return command
}

// NewProjectRoleGetCommand returns a new instance of an `argocd proj roles get` command
func NewProjectRoleGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "get PROJECT ROLE-NAME",
		Short: "Get the details of a specific role",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			project, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index, err := projectutil.GetRoleIndexByName(project, roleName)
			errors.CheckError(err)
			role := project.Spec.Roles[index]

			printRoleFmtStr := "%-15s%s\n"
			fmt.Printf(printRoleFmtStr, "Role Name:", roleName)
			fmt.Printf(printRoleFmtStr, "Description:", role.Description)
			fmt.Printf("Policies:\n")
			fmt.Printf("%s\n", project.ProjectPoliciesString())
			fmt.Printf("JWT Tokens:\n")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tISSUED-AT\tEXPIRES-AT\n")
			for _, token := range role.JWTTokens {
				expiresAt := "<none>"
				if token.ExpiresAt > 0 {
					expiresAt = humanizeTimestamp(token.ExpiresAt)
				}
				fmt.Fprintf(w, "%d\t%s\t%s\n", token.IssuedAt, humanizeTimestamp(token.IssuedAt), expiresAt)
			}
			_ = w.Flush()
		},
	}
	return command
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

			_, err := projIf.Create(context.Background(), &project.ProjectCreateRequest{Project: &proj})
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
				case "src":
					proj.Spec.SourceRepos = opts.sources
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

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, item := range proj.Spec.SourceRepos {
				if item == "*" && item == url {
					log.Info("Wildcard source repository is already defined in project")
					return
				}
				if item == git.NormalizeGitURL(url) {
					log.Info("Specified source repository is already defined in project")
					return
				}
			}
			proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, url)
			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectRemoveClusterResCommand returns a new instance of an `argocd proj remove-res` command
func NewProjectRemoveClusterResCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "remove-res PROJECT group kind",
		Short: "Add cluster resource",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			group := args[1]
			kind := args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, item := range proj.Spec.ClusterResources {
				if item.Group == group && item.Kind == kind {
					index = i
					break
				}
			}
			if index == -1 {
				log.Info("Specified cluster resource does not exist in project")
			} else {
				proj.Spec.ClusterResources = append(proj.Spec.ClusterResources[:index], proj.Spec.ClusterResources[index+1:]...)
				_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewProjectAddClusterResCommand returns a new instance of an `argocd proj add-res` command
func NewProjectAddClusterResCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "add-res PROJECT group kind",
		Short: "Add cluster resource",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			group := args[1]
			kind := args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, item := range proj.Spec.ClusterResources {
				if item.Group == group && item.Kind == kind {
					log.Fatalf("Group '%s' and kind '%s' are already defined in project", item.Group, item.Kind)
				}
			}
			proj.Spec.ClusterResources = append(proj.Spec.ClusterResources, v1.GroupKind{Group: group, Kind: kind})
			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
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

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			index := -1
			for i, item := range proj.Spec.SourceRepos {
				if item == "*" && item == url {
					index = i
					break
				}
				if item == git.NormalizeGitURL(url) {
					index = i
					break
				}
			}
			if index == -1 {
				log.Info("Specified source repository does not exist in project")
			} else {
				proj.Spec.SourceRepos = append(proj.Spec.SourceRepos[:index], proj.Spec.SourceRepos[index+1:]...)
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
			fmt.Fprintf(w, "NAME\tDESCRIPTION\tDESTINATIONS\tSOURCES\tCLUSTER RESOURCES\n")
			for _, p := range projects.Items {
				fmt.Fprintf(w, "%s\t%s\t%v\t%v\t%v\n", p.Name, p.Spec.Description, p.Spec.Destinations, p.Spec.SourceRepos, p.Spec.ClusterResources)
			}
			_ = w.Flush()
		},
	}
	return command
}
