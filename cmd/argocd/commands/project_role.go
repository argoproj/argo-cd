package commands

import (
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	timeutil "github.com/argoproj/pkg/time"
	jwtgo "github.com/golang-jwt/jwt/v4"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/jwt"
	"github.com/argoproj/argo-cd/v2/util/templates"
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
	roleCommand.AddCommand(NewProjectRoleListTokensCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleDeleteTokenCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleAddPolicyCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleRemovePolicyCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleAddGroupCommand(clientOpts))
	roleCommand.AddCommand(NewProjectRoleRemoveGroupCommand(clientOpts))
	return roleCommand
}

// NewProjectRoleAddPolicyCommand returns a new instance of an `argocd proj role add-policy` command
func NewProjectRoleAddPolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var opts policyOpts
	command := &cobra.Command{
		Use:   "add-policy PROJECT ROLE-NAME",
		Short: "Add a policy to a project role",
		Example: `# Before adding new policy
$ argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
JWT Tokens:
ID          ISSUED-AT                                EXPIRES-AT
1696759698  2023-10-08T11:08:18+01:00 (3 hours ago)  <none>

# Add a new policy to allow update to the project
$ argocd proj role add-policy test-project test-role -a update -p allow -o project

# Policy should be updated
$  argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
p, proj:test-project:test-role, applications, update, test-project/project, allow
JWT Tokens:
ID          ISSUED-AT                                EXPIRES-AT
1696759698  2023-10-08T11:08:18+01:00 (3 hours ago)  <none>
`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			role, roleIndex, err := proj.GetRoleByName(roleName)
			errors.CheckError(err)

			policy := fmt.Sprintf(policyTemplate, proj.Name, role.Name, opts.action, proj.Name, opts.object, opts.permission)
			proj.Spec.Roles[roleIndex].Policies = append(role.Policies, policy)

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectRoleRemovePolicyCommand returns a new instance of an `argocd proj role remove-policy` command
func NewProjectRoleRemovePolicyCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var opts policyOpts
	command := &cobra.Command{
		Use:   "remove-policy PROJECT ROLE-NAME",
		Short: "Remove a policy from a role within a project",
		Example: `List the policy of the test-role before removing a policy
$ argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
p, proj:test-project:test-role, applications, update, test-project/project, allow
JWT Tokens:
ID          ISSUED-AT                                EXPIRES-AT
1696759698  2023-10-08T11:08:18+01:00 (3 hours ago)  <none>

# Remove the policy to allow update to objects
$ argocd proj role remove-policy test-project test-role -a update -p allow -o project

# The role should be removed now.
$ argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
JWT Tokens:
ID          ISSUED-AT                                EXPIRES-AT
1696759698  2023-10-08T11:08:18+01:00 (4 hours ago)  <none>
`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
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
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	addPolicyFlags(command, &opts)
	return command
}

// NewProjectRoleCreateCommand returns a new instance of an `argocd proj role create` command
func NewProjectRoleCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var description string
	command := &cobra.Command{
		Use:   "create PROJECT ROLE-NAME",
		Short: "Create a project role",
		Example: templates.Examples(`  
  # Create a project role in the "my-project" project with the name "my-role".
  argocd proj role create my-project my-role --description "My project role description"
  		`),

		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			_, _, err = proj.GetRoleByName(roleName)
			if err == nil {
				fmt.Printf("Role '%s' already exists\n", roleName)
				return
			}
			proj.Spec.Roles = append(proj.Spec.Roles, v1alpha1.ProjectRole{Name: roleName, Description: description})

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Role '%s' created\n", roleName)
		},
	}
	command.Flags().StringVarP(&description, "description", "", "", "Project description")
	return command
}

// NewProjectRoleDeleteCommand returns a new instance of an `argocd proj role delete` command
func NewProjectRoleDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:     "delete PROJECT ROLE-NAME",
		Short:   "Delete a project role",
		Example: `$ argocd proj role delete test-project test-role`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			_, index, err := proj.GetRoleByName(roleName)
			if err != nil {
				fmt.Printf("Role '%s' does not exist in project\n", roleName)
				return
			}
			proj.Spec.Roles[index] = proj.Spec.Roles[len(proj.Spec.Roles)-1]
			proj.Spec.Roles = proj.Spec.Roles[:len(proj.Spec.Roles)-1]

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Role '%s' deleted\n", roleName)
		},
	}
	return command
}

func tokenTimeToString(t int64) string {
	tokenTimeToString := "Never"
	if t > 0 {
		tokenTimeToString = time.Unix(t, 0).Format(time.RFC3339)
	}
	return tokenTimeToString
}

// NewProjectRoleCreateTokenCommand returns a new instance of an `argocd proj role create-token` command
func NewProjectRoleCreateTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		expiresIn       string
		outputTokenOnly bool
		tokenID         string
	)
	command := &cobra.Command{
		Use:   "create-token PROJECT ROLE-NAME",
		Short: "Create a project token",
		Example: `$ argocd proj role create-token test-project test-role
Create token succeeded for proj:test-project:test-role.
  ID: f316c466-40bd-4cfd-8a8c-1392e92255d4
  Issued At: 2023-10-08T15:21:40+01:00
  Expires At: Never
  Token: xxx
`,
		Aliases: []string{"token-create"},
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)
			if expiresIn == "" {
				expiresIn = "0s"
			}
			duration, err := timeutil.ParseDuration(expiresIn)
			errors.CheckError(err)
			tokenResponse, err := projIf.CreateToken(ctx, &projectpkg.ProjectTokenCreateRequest{
				Project:   projName,
				Role:      roleName,
				ExpiresIn: int64(duration.Seconds()),
				Id:        tokenID,
			})
			errors.CheckError(err)

			token, err := jwtgo.Parse(tokenResponse.Token, nil)
			if token == nil {
				err = fmt.Errorf("received malformed token %w", err)
				errors.CheckError(err)
				return
			}

			claims := token.Claims.(jwtgo.MapClaims)
			issuedAt, _ := jwt.IssuedAt(claims)
			expiresAt := int64(jwt.Float64Field(claims, "exp"))
			id := jwt.StringField(claims, "jti")
			subject := jwt.StringField(claims, "sub")

			if !outputTokenOnly {
				fmt.Printf("Create token succeeded for %s.\n", subject)
				fmt.Printf("  ID: %s\n  Issued At: %s\n  Expires At: %s\n",
					id, tokenTimeToString(issuedAt), tokenTimeToString(expiresAt),
				)
				fmt.Println("  Token: " + tokenResponse.Token)
			} else {
				fmt.Println(tokenResponse.Token)
			}
		},
	}
	command.Flags().StringVarP(&expiresIn, "expires-in", "e", "",
		"Duration before the token will expire, e.g. \"12h\", \"7d\". (Default: No expiration)",
	)
	command.Flags().StringVarP(&tokenID, "id", "i", "", "Token unique identifier. (Default: Random UUID)")
	command.Flags().BoolVarP(&outputTokenOnly, "token-only", "t", false, "Output token only - for use in scripts.")

	return command
}

func NewProjectRoleListTokensCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var useUnixTime bool
	command := &cobra.Command{
		Use:   "list-tokens PROJECT ROLE-NAME",
		Short: "List tokens for a given role.",
		Example: `$ argocd proj role list-tokens test-project test-role
ID                                      ISSUED AT                    EXPIRES AT
f316c466-40bd-4cfd-8a8c-1392e92255d4    2023-10-08T15:21:40+01:00    Never
fa9d3517-c52d-434c-9bff-215b38508842    2023-10-08T11:08:18+01:00    Never
`,
		Aliases: []string{"list-token", "token-list"},
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			role, _, err := proj.GetRoleByName(roleName)
			errors.CheckError(err)

			if len(role.JWTTokens) == 0 {
				fmt.Printf("No tokens for %s.%s\n", projName, roleName)
				return
			}

			writer := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
			_, err = fmt.Fprintf(writer, "ID\tISSUED AT\tEXPIRES AT\n")
			errors.CheckError(err)

			tokenRowFormat := "%s\t%v\t%v\n"
			for _, token := range role.JWTTokens {
				if useUnixTime {
					_, _ = fmt.Fprintf(writer, tokenRowFormat, token.ID, token.IssuedAt, token.ExpiresAt)
				} else {
					_, _ = fmt.Fprintf(writer, tokenRowFormat, token.ID, tokenTimeToString(token.IssuedAt), tokenTimeToString(token.ExpiresAt))
				}
			}
			err = writer.Flush()
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVarP(&useUnixTime, "unixtime", "u", false,
		"Print timestamps as Unix time instead of converting. Useful for piping into delete-token.",
	)
	return command
}

// NewProjectRoleDeleteTokenCommand returns a new instance of an `argocd proj role delete-token` command
func NewProjectRoleDeleteTokenCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete-token PROJECT ROLE-NAME ISSUED-AT",
		Short: "Delete a project token",
		Example: `#Create project test-project
$ argocd proj create test-project

# Create a role associated with test-project
$ argocd proj role create test-project test-role
Role 'test-role' created

# Create test-role associated with test-project
$ argocd proj role create-token test-project test-role
Create token succeeded for proj:test-project:test-role.
  ID: c312450e-12e1-4e0d-9f65-fac9cb027b32
  Issued At: 2023-10-08T13:58:57+01:00
  Expires At: Never
  Token: xxx

# Get test-role id to input into the delete-token command below
$ argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
JWT Tokens:
ID          ISSUED-AT                                  EXPIRES-AT
1696769937  2023-10-08T13:58:57+01:00 (6 minutes ago)  <none>

$ argocd proj role delete-token test-project test-role 1696769937
`,
		Aliases: []string{"token-delete", "remove-token"},
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			issuedAt, err := strconv.ParseInt(args[2], 10, 64)
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			_, err = projIf.DeleteToken(ctx, &projectpkg.ProjectTokenDeleteRequest{Project: projName, Role: roleName, Iat: issuedAt})
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
	var output string
	command := &cobra.Command{
		Use:   "list PROJECT",
		Short: "List all the roles in a project",
		Example: templates.Examples(`  
  # This command will list all the roles in argocd-project in a default table format.
  argocd proj role list PROJECT

  # List the roles in the project in formats like json, yaml, wide, or name.
  argocd proj role list PROJECT --output json

  		`),

		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			project, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			switch output {
			case "json", "yaml":
				err := PrintResourceList(project.Spec.Roles, output, false)
				errors.CheckError(err)
			case "name":
				printProjectRoleListName(project.Spec.Roles)
			case "wide", "":
				printProjectRoleListTable(project.Spec.Roles)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide|name")
	return command
}

// NewProjectRoleGetCommand returns a new instance of an `argocd proj roles get` command
func NewProjectRoleGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "get PROJECT ROLE-NAME",
		Short: "Get the details of a specific role",
		Example: `$ argocd proj role get test-project test-role
Role Name:     test-role
Description:
Policies:
p, proj:test-project:test-role, projects, get, test-project, allow
JWT Tokens:
ID          ISSUED-AT                                  EXPIRES-AT
1696774900  2023-10-08T15:21:40+01:00 (4 minutes ago)  <none>
1696759698  2023-10-08T11:08:18+01:00 (4 hours ago)    <none>
`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			roleName := args[1]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
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
			for _, token := range proj.Status.JWTTokensByRole[roleName].Items {
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
	command := &cobra.Command{
		Use:   "add-group PROJECT ROLE-NAME GROUP-CLAIM",
		Short: "Add a group claim to a project role",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, roleName, groupName := args[0], args[1], args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)
			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			updated, err := proj.AddGroupToRole(roleName, groupName)
			errors.CheckError(err)
			if !updated {
				fmt.Printf("Group '%s' already present in role '%s'\n", groupName, roleName)
				return
			}
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Group '%s' added to role '%s'\n", groupName, roleName)
		},
	}
	return command
}

// NewProjectRoleRemoveGroupCommand returns a new instance of an `argocd proj role remove-group` command
func NewProjectRoleRemoveGroupCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "remove-group PROJECT ROLE-NAME GROUP-CLAIM",
		Short: "Remove a group claim from a role within a project",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName, roleName, groupName := args[0], args[1], args[2]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)
			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			updated, err := proj.RemoveGroupFromRole(roleName, groupName)
			errors.CheckError(err)
			if !updated {
				fmt.Printf("Group '%s' not present in role '%s'\n", groupName, roleName)
				return
			}
			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
			fmt.Printf("Group '%s' removed from role '%s'\n", groupName, roleName)
		},
	}
	return command
}
