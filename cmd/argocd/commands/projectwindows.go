package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// NewProjectWindowsCommand returns a new instance of the `argocd proj windows` command
func NewProjectWindowsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	roleCommand := &cobra.Command{
		Use:   "windows",
		Short: "Manage a project's sync windows",
		Example: `
#Add a sync window to a project
argocd proj windows add my-project \
--schedule "0 0 * * 1-5" \
--duration 3600 \
--prune

#Delete a sync window from a project 
argocd proj windows delete <project-name> <window-id>

#List project sync windows
argocd proj windows list <project-name>`,
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

// NewProjectWindowsDisableManualSyncCommand returns a new instance of an `argocd proj windows disable-manual-sync` command
func NewProjectWindowsDisableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "disable-manual-sync PROJECT ID",
		Short: "Disable manual sync for a sync window",
		Long:  "Disable manual sync for a sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Example: `
#Disable manual sync for a sync window for the Project 
argocd proj windows disable-manual-sync PROJECT ID 

#Disabling manual sync for a windows set on the default project with Id 0
argocd proj windows disable-manual-sync default 0`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					window.ManualSync = false
				}
			}

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	return command
}

// NewProjectWindowsEnableManualSyncCommand returns a new instance of an `argocd proj windows enable-manual-sync` command
func NewProjectWindowsEnableManualSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "enable-manual-sync PROJECT ID",
		Short: "Enable manual sync for a sync window",
		Long:  "Enable manual sync for a sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Example: `
#Enabling manual sync for a general case
argocd proj windows enable-manual-sync PROJECT ID 

#Enabling manual sync for a windows set on the default project with Id 2
argocd proj windows enable-manual-sync default 2

#Enabling manual sync with a custom message
argocd proj windows enable-manual-sync my-app-project --message "Manual sync initiated by admin`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					window.ManualSync = true
				}
			}

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
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
		timeZone     string
	)
	command := &cobra.Command{
		Use:   "add PROJECT",
		Short: "Add a sync window to a project",
		Example: `
#Add a 1 hour allow sync window
argocd proj windows add PROJECT \
    --kind allow \
    --schedule "0 22 * * *" \
    --duration 1h \
    --applications "*"

#Add a deny sync window with the ability to manually sync.
argocd proj windows add PROJECT \
    --kind deny \
    --schedule "30 10 * * *" \
    --duration 30m \
    --applications "prod-\\*,website" \
    --namespaces "default,\\*-prod" \
    --clusters "prod,staging" \
    --manual-sync
	`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.AddWindow(kind, schedule, duration, applications, namespaces, clusters, manualSync, timeZone)
			errors.CheckError(err)

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
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
	command.Flags().StringVar(&timeZone, "time-zone", "UTC", "Time zone of the sync window")

	return command
}

// NewProjectWindowsDeleteCommand returns a new instance of an `argocd proj windows delete` command
func NewProjectWindowsDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete PROJECT ID",
		Short: "Delete a sync window from a project. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Example: `
#Delete a sync window from a project (default) with ID 0 
argocd proj windows delete default 0

#Delete a sync window from a project (new-project) with ID 1
argocd proj windows delete new-project 1`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			err = proj.Spec.DeleteWindow(id)
			errors.CheckError(err)

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
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
		timeZone     string
	)
	command := &cobra.Command{
		Use:   "update PROJECT ID",
		Short: "Update a project sync window",
		Long:  "Update a project sync window. Requires ID which can be found by running \"argocd proj windows list PROJECT\"",
		Example: `# Change a sync window's schedule
argocd proj windows update PROJECT ID \
    --schedule "0 20 * * *"
`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			projName := args[0]
			id, err := strconv.Atoi(args[1])
			errors.CheckError(err)

			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
			errors.CheckError(err)

			for i, window := range proj.Spec.SyncWindows {
				if id == i {
					err := window.Update(schedule, duration, applications, namespaces, clusters, timeZone)
					if err != nil {
						errors.CheckError(err)
					}
				}
			}

			_, err = projIf.Update(ctx, &projectpkg.ProjectUpdateRequest{Project: proj})
			errors.CheckError(err)
		},
	}
	command.Flags().StringVar(&schedule, "schedule", "", "Sync window schedule in cron format. (e.g. --schedule \"0 22 * * *\")")
	command.Flags().StringVar(&duration, "duration", "", "Sync window duration. (e.g. --duration 1h)")
	command.Flags().StringSliceVar(&applications, "applications", []string{}, "Applications that the schedule will be applied to. Comma separated, wildcards supported (e.g. --applications prod-\\*,website)")
	command.Flags().StringSliceVar(&namespaces, "namespaces", []string{}, "Namespaces that the schedule will be applied to. Comma separated, wildcards supported (e.g. --namespaces default,\\*-prod)")
	command.Flags().StringSliceVar(&clusters, "clusters", []string{}, "Clusters that the schedule will be applied to. Comma separated, wildcards supported (e.g. --clusters prod,staging)")
	command.Flags().StringVar(&timeZone, "time-zone", "UTC", "Time zone of the sync window. (e.g. --time-zone \"America/New_York\")")
	return command
}

// NewProjectWindowsListCommand returns a new instance of an `argocd proj windows list` command
func NewProjectWindowsListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "list PROJECT",
		Short: "List project sync windows",
		Example: `
#List project windows
argocd proj windows list PROJECT

#List project windows in yaml format
argocd proj windows list PROJECT -o yaml

#List project windows info for a project name (test-project)
argocd proj windows list test-project`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			projName := args[0]
			conn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer io.Close(conn)

			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: projName})
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
	headers := []interface{}{"ID", "STATUS", "KIND", "SCHEDULE", "DURATION", "APPLICATIONS", "NAMESPACES", "CLUSTERS", "MANUALSYNC", "TIMEZONE"}
	fmtStr = strings.Repeat("%s\t", len(headers)) + "\n"
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
				window.TimeZone,
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
