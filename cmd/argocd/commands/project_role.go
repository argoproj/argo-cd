package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	timeutil "github.com/argoproj/pkg/time"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
)

const (
	policyTemplate = "p, proj:%s:%s, applications, %s, %s/%s, %s"
)

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
	roleCommand.AddCommand(NewProjectRoleAddGroupCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleRemoveGroupCommand(clientOpts))
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
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			role, roleIndex, err := proj.GetRoleByName(roleName)
			errors.CheckError(err)

			policy := fmt.Sprintf(policyTemplate, proj.Name, role.Name, opts.action, proj.Name, opts.object, opts.permission)
			proj.Spec.Roles[roleIndex].Policies = append(role.Policies, policy)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
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
			projName := args[0]
			roleName := args[1]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			role, roleIndex, err := proj.GetRoleByName(roleName)
			errors.CheckError(err)

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
			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			_, _, err = proj.GetRoleByName(roleName)
			if err == nil {
				fmt.Printf("Role '%s' already exists\n", roleName)
				return
			}
			proj.Spec.Roles = append(proj.Spec.Roles, v1alpha1.ProjectRole{Name: roleName, Description: description})

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Role '%s' created\n", roleName)
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			_, index, err := proj.GetRoleByName(roleName)
			if err != nil {
				fmt.Printf("Role '%s' does not exist in project\n", roleName)
				return
			}
			proj.Spec.Roles[index] = proj.Spec.Roles[len(proj.Spec.Roles)-1]
			proj.Spec.Roles = proj.Spec.Roles[:len(proj.Spec.Roles)-1]

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Role '%s' deleted\n", roleName)
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
			token, err := projIf.CreateToken(context.Background(), &projectpkg.ProjectTokenCreateRequest{Project: projName, Role: roleName, ExpiresIn: int64(duration.Seconds())})
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
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			_, err = projIf.DeleteToken(context.Background(), &projectpkg.ProjectTokenDeleteRequest{Project: projName, Role: roleName, Iat: issuedAt})
			errors.CheckError(err)
		},
	}
	return command
}

// Print list of project role names
func printProjectRoleListName(roles []v1alpha1.ProjectRole) {
	for _, role := range roles {
		fmt.Println(role.Name)
	}
}

// Print table of project roles
func printProjectRoleListTable(roles []v1alpha1.ProjectRole) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ROLE-NAME\tDESCRIPTION\n")
	for _, role := range roles {
		fmt.Fprintf(w, "%s\t%s\n", role.Name, role.Description)
	}
	_ = w.Flush()
}

// NewProjectRoleListCommand returns a new instance of an `argocd proj roles list` command
func NewProjectRoleListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
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

			project, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			if output == "name" {
				printProjectRoleListName(project.Spec.Roles)
			} else {
				printProjectRoleListTable(project.Spec.Roles)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name")
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

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			role, _, err := proj.GetRoleByName(roleName)
			errors.CheckError(err)

			printRoleFmtStr := "%-15s%s\n"
			fmt.Printf(printRoleFmtStr, "Role Name:", roleName)
			fmt.Printf(printRoleFmtStr, "Description:", role.Description)
			fmt.Printf("Policies:\n")
			fmt.Printf("%s\n", proj.ProjectPoliciesString())
			fmt.Printf("JWT Tokens:\n")
			// TODO(jessesuen): print groups
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

// NewProjectRoleAddGroupCommand returns a new instance of an `argocd proj role add-group` command
func NewProjectRoleAddGroupCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "add-group PROJECT ROLE-NAME GROUP-CLAIM",
		Short: "Add a group claim to a project role",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, roleName, groupName := args[0], args[1], args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			updated, err := proj.AddGroupToRole(roleName, groupName)
			errors.CheckError(err)
			if !updated {
				fmt.Printf("Group '%s' already present in role '%s'\n", groupName, roleName)
				return
			}
			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Group '%s' added to role '%s'\n", groupName, roleName)
		},
	}
	return command
}

// NewProjectRoleRemoveGroupCommand returns a new instance of an `argocd proj role remove-group` command
func NewProjectRoleRemoveGroupCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "remove-group PROJECT ROLE-NAME GROUP-CLAIM",
		Short: "Remove a group claim from a role within a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, roleName, groupName := args[0], args[1], args[2]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)
			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			updated, err := proj.RemoveGroupFromRole(roleName, groupName)
			errors.CheckError(err)
			if !updated {
				fmt.Printf("Group '%s' not present in role '%s'\n", groupName, roleName)
				return
			}
			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Group '%s' removed from role '%s'\n", groupName, roleName)
		},
	}
	return command
}
