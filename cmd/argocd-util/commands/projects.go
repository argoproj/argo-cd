package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appclient "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/typed/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/errors"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func NewProjectsCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "projects",
		Short: "Utility commands operate on ArgoCD Projects",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}

	command.AddCommand(NewUpdatePolicyRuleCommand())
	command.AddCommand(NewProjectAllowListGenCommand())
	return command
}

func globMatch(pattern string, val string) bool {
	if pattern == "*" {
		return true
	}
	if ok, err := filepath.Match(pattern, val); ok && err == nil {
		return true
	}
	return false
}

func getModification(modification string, resource string, scope string, permission string) (func(string, string) string, error) {
	switch modification {
	case "set":
		if scope == "" {
			return nil, fmt.Errorf("Flag --group cannot be empty if permission should be set in role")
		}
		if permission == "" {
			return nil, fmt.Errorf("Flag --permission cannot be empty if permission should be set in role")
		}
		return func(proj string, action string) string {
			return fmt.Sprintf("%s, %s, %s/%s, %s", resource, action, proj, scope, permission)
		}, nil
	case "remove":
		return func(proj string, action string) string {
			return ""
		}, nil
	}
	return nil, fmt.Errorf("modification %s is not supported", modification)
}

func saveProject(updated v1alpha1.AppProject, orig v1alpha1.AppProject, projectsIf appclient.AppProjectInterface, dryRun bool) error {
	fmt.Printf("===== %s ======\n", updated.Name)
	target, err := kube.ToUnstructured(&updated)
	errors.CheckError(err)
	live, err := kube.ToUnstructured(&orig)
	if err != nil {
		return err
	}
	_ = cli.PrintDiff(updated.Name, target, live)
	if !dryRun {
		_, err = projectsIf.Update(context.Background(), &updated, v1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func formatPolicy(proj string, role string, permission string) string {
	return fmt.Sprintf("p, proj:%s:%s, %s", proj, role, permission)
}

func split(input string, delimiter string) []string {
	parts := strings.Split(input, delimiter)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func NewUpdatePolicyRuleCommand() *cobra.Command {
	var (
		clientConfig clientcmd.ClientConfig
		resource     string
		scope        string
		rolePattern  string
		permission   string
		dryRun       bool
	)
	var command = &cobra.Command{
		Use:   "update-role-policy PROJECT_GLOB MODIFICATION ACTION",
		Short: "Implement bulk project role update. Useful to back-fill existing project policies or remove obsolete actions.",
		Example: `  # Add policy that allows executing any action (action/*) to roles which name matches to *deployer* in all projects  
  argocd-util projects update-role-policy '*' set 'action/*' --role '*deployer*' --resource applications --scope '*' --permission allow

  # Remove policy that which manages running (action/*) from all roles which name matches *deployer* in all projects
  argocd-util projects update-role-policy '*' remove override --role '*deployer*'
`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 3 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projectGlob := args[0]
			modificationType := args[1]
			action := args[2]

			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			config.QPS = 100
			config.Burst = 50

			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)
			appclients := appclientset.NewForConfigOrDie(config)

			modification, err := getModification(modificationType, resource, scope, permission)
			errors.CheckError(err)
			projIf := appclients.ArgoprojV1alpha1().AppProjects(namespace)

			err = updateProjects(projIf, projectGlob, rolePattern, action, modification, dryRun)
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&resource, "resource", "", "Resource e.g. 'applications'")
	command.Flags().StringVar(&scope, "scope", "", "Resource scope e.g. '*'")
	command.Flags().StringVar(&rolePattern, "role", "*", "Role name pattern e.g. '*deployer*'")
	command.Flags().StringVar(&permission, "permission", "", "Action permission")
	command.Flags().BoolVar(&dryRun, "dry-run", true, "Dry run")
	clientConfig = cli.AddKubectlFlagsToCmd(command)
	return command
}

func updateProjects(projIf appclient.AppProjectInterface, projectGlob string, rolePattern string, action string, modification func(string, string) string, dryRun bool) error {
	projects, err := projIf.List(context.Background(), v1.ListOptions{})
	if err != nil {
		return err
	}
	for _, proj := range projects.Items {
		if !globMatch(projectGlob, proj.Name) {
			continue
		}
		origProj := proj.DeepCopy()
		updated := false
		for i, role := range proj.Spec.Roles {
			if !globMatch(rolePattern, role.Name) {
				continue
			}
			actionPolicyIndex := -1
			for i := range role.Policies {
				parts := split(role.Policies[i], ",")
				if len(parts) != 6 || parts[3] != action {
					continue
				}
				actionPolicyIndex = i
				break
			}
			policyPermission := modification(proj.Name, action)
			if actionPolicyIndex == -1 && policyPermission != "" {
				updated = true
				role.Policies = append(role.Policies, formatPolicy(proj.Name, role.Name, policyPermission))
			} else if actionPolicyIndex > -1 && policyPermission == "" {
				updated = true
				role.Policies = append(role.Policies[:actionPolicyIndex], role.Policies[actionPolicyIndex+1:]...)
			} else if actionPolicyIndex > -1 && policyPermission != "" {
				updated = true
				role.Policies[actionPolicyIndex] = formatPolicy(proj.Name, role.Name, policyPermission)
			}
			proj.Spec.Roles[i] = role
		}
		if updated {
			err = saveProject(proj, *origProj, projIf, dryRun)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
