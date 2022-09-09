package commands

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"text/tabwriter"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	arogappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/grpc"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/templates"
)

var (
	appSetExample = templates.Examples(`
	# Get an ApplicationSet.
	argocd appset get APPSETNAME

	# List all the ApplicationSets
	argocd appset list

	# Create an ApplicationSet from a YAML stored in a file or at given URL
	argocd appset create <filename or URL> (<filename or URL>...)

	# Delete an ApplicationSet
	argocd appset delete APPSETNAME (APPSETNAME...)
	`)
)

// NewAppSetCommand returns a new instance of an `argocd appset` command
func NewAppSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "appset",
		Short:   "Manage ApplicationSets",
		Example: appSetExample,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationSetGetCommand(clientOpts))
	command.AddCommand(NewApplicationSetCreateCommand(clientOpts))
	command.AddCommand(NewApplicationSetListCommand(clientOpts))
	command.AddCommand(NewApplicationSetDeleteCommand(clientOpts))
	return command
}

// NewApplicationSetGetCommand returns a new instance of an `argocd appset get` command
func NewApplicationSetGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output     string
		showParams bool
	)
	var command = &cobra.Command{
		Use:   "get APPSETNAME",
		Short: "Get ApplicationSet details",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			appSetName := args[0]
			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName})
			errors.CheckError(err)

			switch output {
			case "yaml", "json":
				err := PrintResource(appSet, output)
				errors.CheckError(err)
			case "wide", "":
				printAppSetSummaryTable(appSet)

				if len(appSet.Status.Conditions) > 0 {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					printAppSetConditions(w, appSet)
					_ = w.Flush()
					fmt.Println()
				}
				if showParams {
					printHelmParams(appSet.Spec.Template.Spec.Source.Helm)
				}
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().BoolVar(&showParams, "show-params", false, "Show ApplicationSet parameters and overrides")
	return command
}

// NewApplicationSetCreateCommand returns a new instance of an `argocd appset create` command
func NewApplicationSetCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var upsert bool
	var command = &cobra.Command{
		Use:   "create",
		Short: "Create one or more ApplicationSets",
		Example: templates.Examples(`
	# Create ApplicationSets
	argocd appset create <filename or URL> (<filename or URL>...)
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			argocdClient := headless.NewClientOrDie(clientOpts, c)
			fileUrl := args[0]
			appsets, err := cmdutil.ConstructApplicationSet(fileUrl)
			errors.CheckError(err)

			if len(appsets) == 0 {
				fmt.Printf("No ApplicationSets found while parsing the input file")
				os.Exit(1)
			}

			for _, appset := range appsets {
				if appset.Name == "" {
					err := fmt.Errorf("Error creating ApplicationSet %s. ApplicationSet does not have Name field set", appset)
					errors.CheckError(err)
				}

				conn, appIf := argocdClient.NewApplicationSetClientOrDie()
				defer argoio.Close(conn)

				// Get app before creating to see if it is being updated or no change
				existing, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appset.Name})
				if grpc.UnwrapGRPCStatus(err).Code() != codes.NotFound {
					errors.CheckError(err)
				}

				appSetCreateRequest := applicationset.ApplicationSetCreateRequest{
					Applicationset: appset,
					Upsert:         upsert,
				}
				created, err := appIf.Create(ctx, &appSetCreateRequest)
				errors.CheckError(err)

				var action string
				if existing == nil {
					action = "created"
				} else if !hasAppSetChanged(existing, created, upsert) {
					action = "unchanged"
				} else {
					action = "updated"
				}

				fmt.Printf("ApplicationSet '%s' %s\n", created.ObjectMeta.Name, action)
			}
		},
	}
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override ApplicationSet with the same name even if supplied ApplicationSet spec is different from existing spec")
	return command
}

// NewApplicationSetListCommand returns a new instance of an `argocd appset list` command
func NewApplicationSetListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output   string
		selector string
		projects []string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List ApplicationSets",
		Example: templates.Examples(`  
	# List all ApplicationSets
	argocd appset list
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			appsets, err := appIf.List(ctx, &applicationset.ApplicationSetListQuery{Selector: selector, Projects: projects})
			errors.CheckError(err)

			appsetList := appsets.Items

			switch output {
			case "yaml", "json":
				err := PrintResourceList(appsetList, output, false)
				errors.CheckError(err)
			case "name":
				printApplicationSetNames(appsetList)
			case "wide", "":
				printApplicationSetTable(appsetList, &output)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name|json|yaml")
	command.Flags().StringVarP(&selector, "selector", "l", "", "List applicationsets by label")
	command.Flags().StringArrayVarP(&projects, "project", "p", []string{}, "Filter by project name")

	return command
}

// NewApplicationSetDeleteCommand returns a new instance of an `argocd appset delete` command
func NewApplicationSetDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		noPrompt bool
	)
	var command = &cobra.Command{
		Use:   "delete",
		Short: "Delete one or more ApplicationSets",
		Example: templates.Examples(`  
	# Delete an applicationset
	argocd appset delete APPSETNAME (APPSETNAME...)
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer argoio.Close(conn)
			var isTerminal bool = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			var isConfirmAll bool = false
			var numOfApps = len(args)
			var promptFlag = c.Flag("yes")
			if promptFlag.Changed && promptFlag.Value.String() == "true" {
				noPrompt = true
			}
			for _, appsetName := range args {
				appsetDeleteReq := applicationset.ApplicationSetDeleteRequest{
					Name: appsetName,
				}

				if isTerminal && !noPrompt {
					var lowercaseAnswer string
					if numOfApps == 1 {
						lowercaseAnswer = cli.AskToProceedS("Are you sure you want to delete '" + appsetName + "' and all its Applications? [y/n] ")
					} else {
						if !isConfirmAll {
							lowercaseAnswer = cli.AskToProceedS("Are you sure you want to delete '" + appsetName + "' and all its Applications? [y/n/A] where 'A' is to delete all specified ApplicationSets and their Applications without prompting")
							if lowercaseAnswer == "a" || lowercaseAnswer == "all" {
								lowercaseAnswer = "y"
								isConfirmAll = true
							}
						} else {
							lowercaseAnswer = "y"
						}
					}
					if lowercaseAnswer == "y" || lowercaseAnswer == "yes" {
						_, err := appIf.Delete(ctx, &appsetDeleteReq)
						errors.CheckError(err)
						fmt.Printf("applicationset '%s' deleted\n", appsetName)
					} else {
						fmt.Println("The command to delete '" + appsetName + "' was cancelled.")
					}
				} else {
					_, err := appIf.Delete(ctx, &appsetDeleteReq)
					errors.CheckError(err)
				}
			}
		},
	}
	command.Flags().BoolVarP(&noPrompt, "yes", "y", false, "Turn off prompting to confirm cascaded deletion of Application resources")
	return command
}

// Print simple list of application names
func printApplicationSetNames(apps []arogappsetv1.ApplicationSet) {
	for _, app := range apps {
		fmt.Println(app.Name)
	}
}

// Print table of application data
func printApplicationSetTable(apps []arogappsetv1.ApplicationSet, output *string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"NAME", "NAMESPACE", "PROJECT", "SYNCPOLICY", "CONDITIONS"}
	if *output == "wide" {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		headers = append(headers, "REPO", "PATH", "TARGET")
	} else {
		fmtStr = "%s\t%s\t%s\t%s\t%s\n"
	}
	_, _ = fmt.Fprintf(w, fmtStr, headers...)
	for _, app := range apps {
		conditions := make([]arogappsetv1.ApplicationSetCondition, 0)
		for _, condition := range app.Status.Conditions {
			if condition.Status == arogappsetv1.ApplicationSetConditionStatusTrue {
				conditions = append(conditions, condition)
			}
		}
		vals := []interface{}{
			app.ObjectMeta.Name,
			app.ObjectMeta.Namespace,
			app.Spec.Template.Spec.Project,
			app.Spec.SyncPolicy,
			conditions,
		}
		if *output == "wide" {
			vals = append(vals, app.Spec.Template.Spec.Source.RepoURL, app.Spec.Template.Spec.Source.Path, app.Spec.Template.Spec.Source.TargetRevision)
		}
		_, _ = fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
}

func getServerForAppSet(appSet *arogappsetv1.ApplicationSet) string {
	if appSet.Spec.Template.Spec.Destination.Server == "" {
		return appSet.Spec.Template.Spec.Destination.Name
	}

	return appSet.Spec.Template.Spec.Destination.Server
}

func printAppSetSummaryTable(appSet *arogappsetv1.ApplicationSet) {
	fmt.Printf(printOpFmtStr, "Name:", appSet.Name)
	fmt.Printf(printOpFmtStr, "Project:", appSet.Spec.Template.Spec.GetProject())
	fmt.Printf(printOpFmtStr, "Server:", getServerForAppSet(appSet))
	fmt.Printf(printOpFmtStr, "Namespace:", appSet.Spec.Template.Spec.Destination.Namespace)
	fmt.Printf(printOpFmtStr, "Repo:", appSet.Spec.Template.Spec.Source.RepoURL)
	fmt.Printf(printOpFmtStr, "Target:", appSet.Spec.Template.Spec.Source.TargetRevision)
	fmt.Printf(printOpFmtStr, "Path:", appSet.Spec.Template.Spec.Source.Path)
	printAppSourceDetails(&appSet.Spec.Template.Spec.Source)

	var syncPolicy string
	if appSet.Spec.SyncPolicy != nil && appSet.Spec.Template.Spec.SyncPolicy.Automated != nil {
		syncPolicy = "Automated"
		if appSet.Spec.Template.Spec.SyncPolicy.Automated.Prune {
			syncPolicy += " (Prune)"
		}
	} else {
		syncPolicy = "<none>"
	}
	fmt.Printf(printOpFmtStr, "SyncPolicy:", syncPolicy)

}

func printAppSetConditions(w io.Writer, appSet *arogappsetv1.ApplicationSet) {
	_, _ = fmt.Fprintf(w, "CONDITION\tSTATUS\tMESSAGE\tLAST TRANSITION\n")
	for _, item := range appSet.Status.Conditions {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Type, item.Status, item.Message, item.LastTransitionTime)
	}
}

func hasAppSetChanged(appReq, appRes *arogappsetv1.ApplicationSet, upsert bool) bool {
	// upsert==false, no change occurred from create command
	if !upsert {
		return false
	}

	// Server will return nils for empty labels, annotations, finalizers
	if len(appReq.Labels) == 0 {
		appReq.Labels = nil
	}
	if len(appReq.Annotations) == 0 {
		appReq.Annotations = nil
	}
	if len(appReq.Finalizers) == 0 {
		appReq.Finalizers = nil
	}

	if reflect.DeepEqual(appRes.Spec, appReq.Spec) &&
		reflect.DeepEqual(appRes.Labels, appReq.Labels) &&
		reflect.DeepEqual(appRes.ObjectMeta.Annotations, appReq.Annotations) &&
		reflect.DeepEqual(appRes.Finalizers, appReq.Finalizers) {
		return false
	}

	return true
}
