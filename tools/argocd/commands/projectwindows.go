package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/io"
)

// NewProjectWindowsCommand returns a new instance of the `argocd proj windows` command
func NewProjectWindowsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "windows",
		Short: "Manage a project's sync windows",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	roleCommand.AddCommand(NewProjectWindowsDisableManualSyncCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsEnableManualSyncCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsAddWindowCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsDeleteCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsListCommand(clientOpts))
	roleCommand.AddCommand(NewProjectWindowsUpdateCommand(clientOpts))
	return roleCommand
}

// NewProjectSyncWindowsDisableManualSyncCommand returns a new instance of an `argocd proj windows disable-manual-sync` command
func NewProjectWindowsDisableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "disable-manual-sync PROJECT ID",
		Short: "Disable manual sync for a sync window",
		Long:  "Disable manual sync for a sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					window.ManualSync = false
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsEnableManualSyncCommand returns a new instance of an `argocd proj windows enable-manual-sync` command
func NewProjectWindowsEnableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "enable-manual-sync PROJECT ID",
		Short: "Enable manual sync for a sync window",
		Long:  "Enable manual sync for a sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					window.ManualSync = true
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsAddWindowCommand returns a new instance of an `argocd proj windows add` command
func NewProjectWindowsAddWindowCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		kind         string
		schedule     string
		duration     string
		applications []string
		namespaces   []string
		clusters     []string
		manualSync   bool
	)
	var command = &cobra.Command{
		Use:   "add PROJECT",
		Short: "Add a sync window to a project",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.AddWindow(kind, schedule, duration, applications, namespaces, clusters, manualSync)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&kind, "kind", "k", "", "Sync window kind, either allow or deny")
	command.Flags().StringVar(&schedule, "schedule", "", "Sync window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVar(&duration, "duration", "", "Sync window duration. (e.g. --duration 1h)")
	command.Flags().StringSliceVar(&applications, "applications", []string{}, "Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\\*,website)")
	command.Flags().StringSliceVar(&namespaces, "namespaces", []string{}, "Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\\*-prod)")
	command.Flags().StringSliceVar(&clusters, "clusters", []string{}, "Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)")
	command.Flags().BoolVar(&manualSync, "manual-sync", false, "Allow manual syncs for both deny and allow windows")

	return command
}

// NewProjectWindowsAddWindowCommand returns a new instance of an `argocd proj windows delete` command
func NewProjectWindowsDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "delete PROJECT ID",
		Short: "Delete a sync window from a project. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.DeleteWindow(id)
			errors.CheckError(err)

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsUpdateCommand returns a new instance of an `argocd proj windows update` command
func NewProjectWindowsUpdateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		schedule     string
		duration     string
		applications []string
		namespaces   []string
		clusters     []string
	)
	var command = &cobra.Command{
		Use:   "update PROJECT ID",
		Short: "Update a project sync window",
		Long:  "Update a project sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					err := window.Update(schedule, duration, applications, namespaces, clusters)
					if err != nil {
						errors.CheckError(err)
					}
				}
			}

			_, err = projIf.Update(context.Background(), &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&schedule, "schedule", "", "Sync window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVar(&duration, "duration", "", "Sync window duration. (e.g. --duration 1h)")
	command.Flags().StringSliceVar(&applications, "applications", []string{}, "Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\\*,website)")
	command.Flags().StringSliceVar(&namespaces, "namespaces", []string{}, "Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\\*-prod)")
	command.Flags().StringSliceVar(&clusters, "clusters", []string{}, "Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)")
	return command
}

// NewProjectWindowsListCommand returns a new instance of an `argocd proj windows list` command
func NewProjectWindowsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list PROJECT",
		Short: "List project sync windows",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)
			switch output {
			case "yaml", "json":
				err := PrintResourceList(proj.Spec.SyncWindows, output, false)
				errors.CheckError(err)
			case "wide", "":
				printSyncWindows(proj)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// Print table of sync window data
func printSyncWindows(proj *v1alpha1.AppProject) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS", "MANUALSYNC"}
	fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	fmt.Fprintf(w, fmtStr, headers...)
	if proj.Spec.SyncWindows.HasWindows() {
		for i, window := range proj.Spec.SyncWindows {
			vals := []interface{}{
				strconv.Itoa(i),
				formatBoolOutput(window.Active()),
				window.Kind,
				window.Schedule,
				window.Duration,
				formatListOutput(window.Applications),
				formatListOutput(window.Namespaces),
				formatListOutput(window.Clusters),
				formatManualOutput(window.ManualSync),
			}
			fmt.Fprintf(w, fmtStr, vals...)
		}
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
func formatBoolOutput(active bool) string {
	var o string
	if active {
		o = "Active"
	} else {
		o = "Inactive"
	}
	return o
}
func formatManualOutput(active bool) string {
	var o string
	if active {
		o = "Enabled"
	} else {
		o = "Disabled"
	}
	return o
}
