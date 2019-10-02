package commands

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
)

// NewProjectRoleCommand returns a new instance of the `argocd proj role` command
func NewProjectMaintenanceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "maintenance",
		Short: "Manage a project's maintenance",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	roleCommand.AddCommand(NewProjectMaintenanceEnableCommand(clientOpts))
	roleCommand.AddCommand(NewProjectMaintenanceDisableCommand(clientOpts))
	roleCommand.AddCommand(NewProjectMaintenanceAddWindowCommand(clientOpts))
	roleCommand.AddCommand(NewProjectMaintenanceDeleteWindowCommand(clientOpts))
	roleCommand.AddCommand(NewProjectMaintenanceListWindowsCommand(clientOpts))
	roleCommand.AddCommand(NewProjectMaintenanceUpdateWindowCommand(clientOpts))
	return roleCommand
}

// NewProjectDisableMaintenance returns a new instance of an `argocd proj maintenance disable` command
func NewProjectMaintenanceDisableCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "disable PROJECT",
		Short: "Disable project maintenance",
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

			if proj.Spec.Maintenance.IsEnabled() {
				proj.Spec.Maintenance.Enabled = false
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewProjectEnableMaintenance returns a new instance of an `argocd proj maintenance enable` command
func NewProjectMaintenanceEnableCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "enable PROJECT",
		Short: "Enable project maintenance",
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

			if !proj.Spec.Maintenance.IsEnabled() {
				if proj.Spec.HasMaintenance() {
					proj.Spec.Maintenance.Enabled = true
				} else {
					proj.Spec.Maintenance = &v1alpha1.ProjectMaintenance{Enabled: true}
				}
				_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewProjectMaintenanceAddWindowCommand returns a new instance of an `argocd proj maintenance add-window` command
func NewProjectMaintenanceAddWindowCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		schedule     string
		duration     string
		applications []string
		namespaces   []string
		clusters     []string
	)
	var command = &cobra.Command{
		Use:   "add-window PROJECT",
		Short: "Add a maintenance window to a project",
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

			if !proj.Spec.HasMaintenance() {
				proj.Spec.AddMaintenance()
			}

			proj.Spec.Maintenance.AddWindow(schedule, duration, applications, namespaces, clusters)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&schedule, "schedule", "s", "", "Maintenance window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVarP(&duration, "duration", "d", "", "Maintenance window duration. (e.g. --duration 1h")
	command.Flags().StringSliceVar(&applications, "applications", []string{}, "Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\\*,website)")
	command.Flags().StringSliceVar(&namespaces, "namespaces", []string{}, "Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\\*-prod)")
	command.Flags().StringSliceVar(&clusters, "clusters", []string{}, "Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)")
	return command
}

// NewProjectMaintenanceAddWindowCommand returns a new instance of an `argocd proj maintenance add-window` command
func NewProjectMaintenanceDeleteWindowCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		schedule string
		duration string
	)
	var command = &cobra.Command{
		Use:   "delete-window PROJECT",
		Short: "Delete a maintenance window from a project",
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

			err = proj.Spec.Maintenance.DeleteWindow(schedule, duration)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&schedule, "schedule", "s", "", "Maintenance window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVarP(&duration, "duration", "d", "", "Maintenance window duration. (e.g. --duration 1h")
	return command
}

// NewProjectMaintenanceAddWindowCommand returns a new instance of an `argocd proj maintenance add-window` command
func NewProjectMaintenanceUpdateWindowCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		schedule     string
		duration     string
		applications []string
		namespaces   []string
		clusters     []string
	)
	var command = &cobra.Command{
		Use:   "update-window PROJECT",
		Short: "Update a project maintenance window",
		Long:  "Update a project maintenance window. Requires --schedule and --duration to lookup the window",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if schedule == "" || duration == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer util.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for _, window := range proj.Spec.Maintenance.Windows {
				if window.Matches(schedule, duration) {
					err := window.Update(applications, namespaces, clusters)
					if err != nil {
						errors.CheckError(err)
					}
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&schedule, "schedule", "s", "", "Maintenance window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVarP(&duration, "duration", "d", "", "Maintenance window duration. (e.g. --duration 1h")
	command.Flags().StringSliceVar(&applications, "applications", []string{}, "Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\\*,website)")
	command.Flags().StringSliceVar(&namespaces, "namespaces", []string{}, "Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\\*-prod)")
	command.Flags().StringSliceVar(&clusters, "clusters", []string{}, "Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)")
	return command
}

// NewProjectEnableMaintenance returns a new instance of an `argocd proj maintenance enable` command
func NewProjectMaintenanceListWindowsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list-windows PROJECT",
		Short: "List project maintenance windows",
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

			printMaintenanceWindows(proj)
		},
	}
	return command
}

// Print table of maintenance window data
func printMaintenanceWindows(proj *v1alpha1.AppProject) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"STATUS", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS"}
	fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, fmtStr, headers...)
	var m bool
	if proj.Spec.Maintenance.Enabled {
		m = true
	}
	for _, window := range proj.Spec.Maintenance.Windows {
		vals := []interface{}{
			formatBoolOutput(window.Active(), m),
			window.Schedule,
			window.Duration,
			formatListOutput(window.Applications),
			formatListOutput(window.Namespaces),
			formatListOutput(window.Clusters),
		}
		fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
}

func formatListOutput(list []string) string {
	var o string
	if len(list) == 0 {
		o = "-"
	} else {
		o = strings.Join(list, ",")
	}
	return o
}
func formatBoolOutput(active bool, enabled bool) string {
	var o string
	if active && enabled {
		o = "Active"
	} else {
		o = "Inactive"
	}
	return o
}
