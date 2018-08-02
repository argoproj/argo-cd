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
	"github.com/argoproj/argo-cd/util/git"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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

//Default expiration time to 3 months
const defaultSecondsBeforeExpiry = 60 * 60 * 24 * 3

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
	var tokenCommand = &cobra.Command{
		Use:   "token",
		Short: "Manage a project's token",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	tokenCommand.AddCommand(NewProjectListTokenCommand(clientOpts))
	tokenCommand.AddCommand(NewProjectCreateTokenCommand(clientOpts))
	tokenCommand.AddCommand(NewProjectDeleteTokenCommand(clientOpts))
	tokenCommand.AddCommand(NewProjectAddTokenPolicyCommand(clientOpts))
	tokenCommand.AddCommand(NewProjectRemoveTokenPolicyCommand(clientOpts))
	command.AddCommand(tokenCommand)
	command.AddCommand(NewProjectCreateCommand(clientOpts))
	command.AddCommand(NewProjectDeleteCommand(clientOpts))
	command.AddCommand(NewProjectListCommand(clientOpts))
	command.AddCommand(NewProjectSetCommand(clientOpts))
	command.AddCommand(NewProjectAddDestinationCommand(clientOpts))
	command.AddCommand(NewProjectRemoveDestinationCommand(clientOpts))
	command.AddCommand(NewProjectAddSourceCommand(clientOpts))
	command.AddCommand(NewProjectRemoveSourceCommand(clientOpts))
	return command
}

func addProjFlags(command *cobra.Command, opts *projectOpts) {
	command.Flags().StringVarP(&opts.description, "description", "", "desc", "Project description")
	command.Flags().StringArrayVarP(&opts.destinations, "dest", "d", []string{},
		"Allowed deployment destination. Includes comma separated server url and namespace (e.g. https://192.168.99.100:8443,default")
	command.Flags().StringArrayVarP(&opts.sources, "src", "s", []string{}, "Allowed deployment source repository URL.")
}

func addPolicyFlags(command *cobra.Command, opts *policyOpts) {
	command.Flags().StringVarP(&opts.action, "action", "a", "", "Action to grant/deny permission on")
	command.Flags().StringVarP(&opts.permission, "permission", "p", "allow", "Whether to allow or deny access to object with the action.  This can only be 'allow' or 'deny'")
	command.Flags().StringVarP(&opts.object, "object", "o", "", "Object within the project to grant/deny access.  Use '*' for a wildcard. Will want access to '<project>/<object>'")
}

// NewProjectAddTokenPolicyCommand returns a new instance of an `argocd proj token add-policy` command
func NewProjectAddTokenPolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts policyOpts
	)
	var command = &cobra.Command{
		Use:   "add-policy PROJECT TOKEN-NAME",
		Short: "Add a policy to a project token",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if opts.permission != "allow" && opts.permission != "deny" {
				log.Fatal("Permission flag can only have the values 'allow' or 'deny'")
			}

			projName := args[0]
			tokenName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			tokenIndex, err := proj.GetTokenIndex(tokenName)
			if err != nil {
				log.Fatal(err)
			}
			token := proj.Spec.Tokens[tokenIndex]

			policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"
			policy := fmt.Sprintf(policyTemplate, proj.Name, token.Name, opts.action, proj.Name, opts.object)
			proj.Spec.Tokens[tokenIndex].Policies = append(token.Policies, policy)

			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectRemoveTokenPolicyCommand returns a new instance of an `argocd proj token remove-policy` command
func NewProjectRemoveTokenPolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		opts policyOpts
	)
	var command = &cobra.Command{
		Use:   "remove-policy PROJECT TOKEN-NAME",
		Short: "Remove a policy from a token within a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if opts.permission != "allow" && opts.permission != "deny" {
				log.Fatal("Permission flag can only have the values 'allow' or 'deny'")
			}

			projName := args[0]
			tokenName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &project.ProjectQuery{Name: projName})
			errors.CheckError(err)

			tokenIndex, err := proj.GetTokenIndex(tokenName)
			if err != nil {
				log.Fatal(err)
			}
			token := proj.Spec.Tokens[tokenIndex]

			policyTemplate := "p, proj:%s:%s, projects, %s, %s/%s"
			policyToRemove := fmt.Sprintf(policyTemplate, proj.Name, token.Name, opts.action, proj.Name, opts.object)
			duplicateIndex := -1
			for i, policy := range token.Policies {
				if policy == policyToRemove {
					duplicateIndex = i
					break
				}
			}
			if duplicateIndex < 0 {
				log.Fatal("Policy does not exist in token.")
			}
			token.Policies[duplicateIndex] = token.Policies[len(token.Policies)-1]
			proj.Spec.Tokens[tokenIndex].Policies = token.Policies[:len(token.Policies)-1]
			_, err = projIf.Update(context.Background(), &project.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectCreateTokenCommand returns a new instance of an `argocd proj token create` command
func NewProjectCreateTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		secondsBeforeExpiry int64
	)
	var command = &cobra.Command{
		Use:   "create PROJECT TOKEN-NAME [--seconds seconds]",
		Short: "Create a project token",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			tokenName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			token, err := projIf.CreateToken(context.Background(), &project.ProjectTokenCreateRequest{Project: projName, Token: tokenName, SecondsBeforeExpiry: secondsBeforeExpiry})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "New token for %s-%s:\n'%s'\n", projName, tokenName, token)
			fmt.Fprintf(w, "Make sure to save token as it is not stored.")
			_ = w.Flush()
		},
	}
	command.Flags().Int64VarP(&secondsBeforeExpiry, "secondsBeforeExpiry", "s", defaultSecondsBeforeExpiry, "Number of seconds before the token will expire (Default: 3 months)")

	return command
}

// NewProjectListTokenCommand returns a new instance of an `argocd proj token list` command
func NewProjectListTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list PROJECT",
		Short: "List all the tokens in a project",
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
			fmt.Fprintf(w, "TOKEN-NAME\tCREATED-AT\tPOLICIES\n")
			for _, token := range project.Spec.Tokens {
				fmt.Fprintf(w, "%s\t%d\t\n", token.Name, token.CreatedAt)
				for _, policy := range token.Policies {
					fmt.Fprintf(w, "%s\t%d\t%s\n", token.Name, token.CreatedAt, policy)
				}
			}
			_ = w.Flush()
		},
	}
	return command
}

// NewProjectDeleteTokenCommand returns a new instance of an `argocd proj token delete` command
func NewProjectDeleteTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT TOKEN-NAME",
		Short: "Delete a project token",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			tokenName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err := projIf.DeleteToken(context.Background(), &project.ProjectTokenDeleteRequest{Project: projName, Token: tokenName})
			errors.CheckError(err)
		},
	}
	return command
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
				if item == git.NormalizeGitURL(url) {
					log.Fatal("Specified source repository is already defined in project")
				}
			}
			proj.Spec.SourceRepos = append(proj.Spec.SourceRepos, url)
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
				if item == git.NormalizeGitURL(url) {
					index = i
					break
				}
			}
			if index == -1 {
				log.Fatal("Specified source repository does not exist in project")
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
			fmt.Fprintf(w, "NAME\tDESCRIPTION\tDESTINATIONS\n")
			for _, p := range projects.Items {
				fmt.Fprintf(w, "%s\t%s\t%v\n", p.Name, p.Spec.Description, p.Spec.Destinations)
			}
			_ = w.Flush()
		},
	}
	return command
}
