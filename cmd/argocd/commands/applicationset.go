package commands

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"

	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/admin"
	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/headless"
	"github.com/argoproj/argo-cd/v3/cmd/argocd/commands/utils"
	cmdutil "github.com/argoproj/argo-cd/v3/cmd/util"
	argocdclient "github.com/argoproj/argo-cd/v3/pkg/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/applicationset"
	arogappsetv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/grpc"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/templates"
)

var appSetExample = templates.Examples(`
	# Get an ApplicationSet.
	argocd appset get APPSETNAME

	# List all the ApplicationSets
	argocd appset list

	# Create an ApplicationSet from a YAML stored in a file or at given URL
	argocd appset create <filename or URL> (<filename or URL>...)

	# Delete an ApplicationSet
	argocd appset delete APPSETNAME (APPSETNAME...)

	# Show phase deployment status
	argocd appset phase status APPSETNAME

	# Advance to next phase
	argocd appset phase advance APPSETNAME

	# Rollback to previous phase
	argocd appset phase rollback APPSETNAME
	`)

// NewAppSetCommand returns a new instance of an `argocd appset` command
func NewAppSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
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
	command.AddCommand(NewApplicationSetGenerateCommand(clientOpts))
	command.AddCommand(NewApplicationSetPhaseCommand(clientOpts))
	return command
}

// NewApplicationSetGetCommand returns a new instance of an `argocd appset get` command
func NewApplicationSetGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output     string
		showParams bool
		showPhase  bool
	)
	command := &cobra.Command{
		Use:   "get APPSETNAME",
		Short: "Get ApplicationSet details",
		Example: templates.Examples(`
	# Get ApplicationSets
	argocd appset get APPSETNAME
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			appSetName, appSetNs := argo.ParseFromQualifiedName(args[0], "")

			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName, AppsetNamespace: appSetNs})
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
					printHelmParams(appSet.Spec.Template.Spec.GetSource().Helm)
				}
				if showPhase || hasPhaseDeployment(appSet) {
					fmt.Println()
					printPhaseStatusTable(appSet)
				}
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().BoolVar(&showParams, "show-params", false, "Show ApplicationSet parameters and overrides")
	command.Flags().BoolVar(&showPhase, "show-phase", false, "Show phase deployment status")
	return command
}

// NewApplicationSetCreateCommand returns a new instance of an `argocd appset create` command
func NewApplicationSetCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	var upsert, dryRun bool
	command := &cobra.Command{
		Use:   "create",
		Short: "Create one or more ApplicationSets",
		Example: templates.Examples(`
	# Create ApplicationSets
	argocd appset create <filename or URL> (<filename or URL>...)

	# Dry-run AppSet creation to see what applications would be managed
	argocd appset create --dry-run <filename or URL> -o json | jq -r '.status.resources[].name' 
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			argocdClient := headless.NewClientOrDie(clientOpts, c)
			fileURL := args[0]
			appsets, err := cmdutil.ConstructApplicationSet(fileURL)
			errors.CheckError(err)

			if len(appsets) == 0 {
				fmt.Printf("No ApplicationSets found while parsing the input file")
				os.Exit(1)
			}

			for _, appset := range appsets {
				if appset.Name == "" {
					errors.Fatal(errors.ErrorGeneric, fmt.Sprintf("Error creating ApplicationSet %s. ApplicationSet does not have Name field set", appset))
				}

				conn, appIf := argocdClient.NewApplicationSetClientOrDie()
				defer utilio.Close(conn)

				// Get app before creating to see if it is being updated or no change
				existing, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appset.Name, AppsetNamespace: appset.Namespace})
				if grpc.UnwrapGRPCStatus(err).Code() != codes.NotFound {
					errors.CheckError(err)
				}

				appSetCreateRequest := applicationset.ApplicationSetCreateRequest{
					Applicationset: appset,
					Upsert:         upsert,
					DryRun:         dryRun,
				}
				created, err := appIf.Create(ctx, &appSetCreateRequest)
				errors.CheckError(err)

				dryRunMsg := ""
				if dryRun {
					dryRunMsg = " (dry-run)"
				}

				var action string
				switch {
				case existing == nil:
					action = "created"
				case !hasAppSetChanged(existing, created, upsert):
					action = "unchanged"
				default:
					action = "updated"
				}

				c.PrintErrf("ApplicationSet '%s' %s%s\n", created.Name, action, dryRunMsg)

				switch output {
				case "yaml", "json":
					err := PrintResource(created, output)
					errors.CheckError(err)
				case "wide", "":
					printAppSetSummaryTable(created)

					if len(created.Status.Conditions) > 0 {
						fmt.Println()
						w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
						printAppSetConditions(w, created)
						_ = w.Flush()
						fmt.Println()
					}
				default:
					errors.CheckError(fmt.Errorf("unknown output format: %s", output))
				}
			}
		},
	}
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override ApplicationSet with the same name even if supplied ApplicationSet spec is different from existing spec")
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Allows to evaluate the ApplicationSet template on the server to get a preview of the applications that would be created")
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// NewApplicationSetGenerateCommand returns a new instance of an `argocd appset generate` command
func NewApplicationSetGenerateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "generate",
		Short: "Generate apps of ApplicationSet rendered templates",
		Example: templates.Examples(`
	# Generate apps of ApplicationSet rendered templates
	argocd appset generate <filename or URL> (<filename or URL>...)
`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			argocdClient := headless.NewClientOrDie(clientOpts, c)
			fileURL := args[0]
			appsets, err := cmdutil.ConstructApplicationSet(fileURL)
			errors.CheckError(err)

			if len(appsets) != 1 {
				fmt.Printf("Input file must contain one ApplicationSet")
				os.Exit(1)
			}
			appset := appsets[0]
			if appset.Name == "" {
				errors.Fatal(errors.ErrorGeneric, fmt.Sprintf("Error generating apps for ApplicationSet %s. ApplicationSet does not have Name field set", appset))
			}

			conn, appIf := argocdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			req := applicationset.ApplicationSetGenerateRequest{
				ApplicationSet: appset,
			}
			resp, err := appIf.Generate(ctx, &req)
			errors.CheckError(err)

			var appsList []arogappsetv1.Application
			for i := range resp.Applications {
				appsList = append(appsList, *resp.Applications[i])
			}

			switch output {
			case "yaml", "json":
				var resources []any
				for i := range appsList {
					app := appsList[i]
					// backfill api version and kind because k8s client always return empty values for these fields
					app.APIVersion = arogappsetv1.ApplicationSchemaGroupVersionKind.GroupVersion().String()
					app.Kind = arogappsetv1.ApplicationSchemaGroupVersionKind.Kind
					resources = append(resources, app)
				}

				cobra.CheckErr(admin.PrintResources(output, os.Stdout, resources...))
			case "wide", "":
				printApplicationTable(appsList, &output)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// NewApplicationSetListCommand returns a new instance of an `argocd appset list` command
func NewApplicationSetListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output          string
		selector        string
		projects        []string
		appSetNamespace string
	)
	command := &cobra.Command{
		Use:   "list",
		Short: "List ApplicationSets",
		Example: templates.Examples(`
	# List all ApplicationSets
	argocd appset list
		`),
		Run: func(c *cobra.Command, _ []string) {
			ctx := c.Context()

			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationSetClientOrDie()
			defer utilio.Close(conn)
			appsets, err := appIf.List(ctx, &applicationset.ApplicationSetListQuery{Selector: selector, Projects: projects, AppsetNamespace: appSetNamespace})
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
	command.Flags().StringVarP(&appSetNamespace, "appset-namespace", "N", "", "Only list applicationsets in namespace")

	return command
}

// NewApplicationSetDeleteCommand returns a new instance of an `argocd appset delete` command
func NewApplicationSetDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var noPrompt bool
	command := &cobra.Command{
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
			defer utilio.Close(conn)
			isTerminal := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			numOfApps := len(args)
			promptFlag := c.Flag("yes")
			if promptFlag.Changed && promptFlag.Value.String() == "true" {
				noPrompt = true
			}

			var (
				confirmAll = false
				confirm    = false
			)

			// This is for backward compatibility,
			// before we showed the prompts only when condition isTerminal && !noPrompt is true
			promptUtil := utils.NewPrompt(isTerminal && !noPrompt)

			for _, appSetQualifiedName := range args {
				appSetName, appSetNs := argo.ParseFromQualifiedName(appSetQualifiedName, "")

				appsetDeleteReq := applicationset.ApplicationSetDeleteRequest{
					Name:            appSetName,
					AppsetNamespace: appSetNs,
				}
				messageForSingle := "Are you sure you want to delete '" + appSetQualifiedName + "' and all its Applications? [y/n] "
				messageForAll := "Are you sure you want to delete '" + appSetQualifiedName + "' and all its Applications? [y/n/a] where 'a' is to delete all specified ApplicationSets and their Applications without prompting"
				if !confirmAll {
					confirm, confirmAll = promptUtil.ConfirmBaseOnCount(messageForSingle, messageForAll, numOfApps)
				}
				if confirm || confirmAll {
					_, err := appIf.Delete(ctx, &appsetDeleteReq)
					errors.CheckError(err)
					fmt.Printf("applicationset '%s' deleted\n", appSetQualifiedName)
				} else {
					fmt.Println("The command to delete '" + appSetQualifiedName + "' was cancelled.")
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
		fmt.Println(app.QualifiedName())
	}
}

// Print table of application data
func printApplicationSetTable(apps []arogappsetv1.ApplicationSet, output *string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []any{"NAME", "PROJECT", "SYNCPOLICY", "CONDITIONS"}
	if *output == "wide" {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		headers = append(headers, "REPO", "PATH", "TARGET", "PHASE")
	} else {
		fmtStr = "%s\t%s\t%s\t%s\n"
	}
	_, _ = fmt.Fprintf(w, fmtStr, headers...)
	for _, app := range apps {
		conditions := make([]arogappsetv1.ApplicationSetCondition, 0)
		for _, condition := range app.Status.Conditions {
			if condition.Status == arogappsetv1.ApplicationSetConditionStatusTrue {
				conditions = append(conditions, condition)
			}
		}
		vals := []any{
			app.QualifiedName(),
			app.Spec.Template.Spec.Project,
			app.Spec.SyncPolicy,
			conditions,
		}
		if *output == "wide" {
			phaseStatus := "-"
			if hasPhaseDeployment(&app) {
				currentPhase, totalPhases := getCurrentPhaseInfo(&app)
				phaseStatus = strconv.Itoa(currentPhase) + "/" + strconv.Itoa(totalPhases)
			}
			vals = append(vals,
				app.Spec.Template.Spec.GetSource().RepoURL,
				app.Spec.Template.Spec.GetSource().Path,
				app.Spec.Template.Spec.GetSource().TargetRevision,
				phaseStatus)
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
	fmt.Printf(printOpFmtStr, "Name:", appSet.QualifiedName())
	fmt.Printf(printOpFmtStr, "Project:", appSet.Spec.Template.Spec.GetProject())
	fmt.Printf(printOpFmtStr, "Server:", getServerForAppSet(appSet))
	fmt.Printf(printOpFmtStr, "Namespace:", appSet.Spec.Template.Spec.Destination.Namespace)
	if !appSet.Spec.Template.Spec.HasMultipleSources() {
		fmt.Println("Source:")
	} else {
		fmt.Println("Sources:")
	}

	// if no source has been defined, print the default value for a source
	if len(appSet.Spec.Template.Spec.GetSources()) == 0 {
		src := appSet.Spec.Template.Spec.GetSource()
		printAppSourceDetails(&src)
	} else {
		// otherwise range over the sources and print each source details
		for _, source := range appSet.Spec.Template.Spec.GetSources() {
			printAppSourceDetails(&source)
		}
	}

	var (
		syncPolicyStr string
		syncPolicy    = appSet.Spec.Template.Spec.SyncPolicy
	)
	if syncPolicy != nil && syncPolicy.IsAutomatedSyncEnabled() {
		syncPolicyStr = "Automated"
		if syncPolicy.Automated.Prune {
			syncPolicyStr += " (Prune)"
		}
	} else {
		syncPolicyStr = "<none>"
	}
	fmt.Printf(printOpFmtStr, "SyncPolicy:", syncPolicyStr)
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
		reflect.DeepEqual(appRes.Annotations, appReq.Annotations) &&
		reflect.DeepEqual(appRes.Finalizers, appReq.Finalizers) {
		return false
	}

	return true
}

// NewApplicationSetPhaseCommand returns a new instance of an `argocd appset phase` command
func NewApplicationSetPhaseCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "phase",
		Short: "Manage ApplicationSet phase deployments",
		Example: templates.Examples(`
	# Show phase deployment status
	argocd appset phase status my-appset

	# Advance to next phase
	argocd appset phase advance my-appset

	# Rollback current phase
	argocd appset phase rollback my-appset

	# Show phase history
	argocd appset phase history my-appset
		`),
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationSetPhaseStatusCommand(clientOpts))
	command.AddCommand(NewApplicationSetPhaseAdvanceCommand(clientOpts))
	command.AddCommand(NewApplicationSetPhaseRollbackCommand(clientOpts))
	command.AddCommand(NewApplicationSetPhaseHistoryCommand(clientOpts))
	return command
}

// NewApplicationSetPhaseStatusCommand returns a new instance of an `argocd appset phase status` command
func NewApplicationSetPhaseStatusCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "status APPSETNAME",
		Short: "Show phase deployment status",
		Example: templates.Examples(`
	# Show phase status
	argocd appset phase status my-appset

	# Show phase status in JSON format
	argocd appset phase status my-appset -o json
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			appSetName, appSetNs := argo.ParseFromQualifiedName(args[0], "")

			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName, AppsetNamespace: appSetNs})
			errors.CheckError(err)

			switch output {
			case "json", "yaml":
				phaseStatus := extractPhaseStatus(appSet)
				err := PrintResource(phaseStatus, output)
				errors.CheckError(err)
			case "wide", "":
				printPhaseStatusTable(appSet)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// NewApplicationSetPhaseAdvanceCommand returns a new instance of an `argocd appset phase advance` command
func NewApplicationSetPhaseAdvanceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "advance APPSETNAME",
		Short: "Advance to the next phase",
		Example: templates.Examples(`
	# Advance to next phase
	argocd appset phase advance my-appset

	# Dry run advance (show what would happen)
	argocd appset phase advance my-appset --dry-run
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			appSetName, appSetNs := argo.ParseFromQualifiedName(args[0], "")

			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName, AppsetNamespace: appSetNs})
			errors.CheckError(err)

			currentPhase, totalPhases := getCurrentPhaseInfo(appSet)

			if currentPhase >= totalPhases {
				fmt.Printf("ApplicationSet '%s' has already completed all phases (%d/%d)\n", args[0], currentPhase, totalPhases)
				return
			}

			nextPhase := currentPhase + 1
			if dryRun {
				fmt.Printf("Would advance ApplicationSet '%s' from phase %d to phase %d (dry-run)\n",
					args[0], currentPhase, nextPhase)
				return
			}

			// Update the phase annotation
			if appSet.Annotations == nil {
				appSet.Annotations = make(map[string]string)
			}
			appSet.Annotations["applicationset.argoproj.io/phase-"+getGeneratorTypeFromAppSet(appSet)] = strconv.Itoa(nextPhase)

			appSetCreateRequest := applicationset.ApplicationSetCreateRequest{
				Applicationset: appSet,
				Upsert:         true,
			}
			_, err = appIf.Create(ctx, &appSetCreateRequest)
			errors.CheckError(err)

			fmt.Printf("ApplicationSet '%s' advanced to phase %d/%d\n", args[0], nextPhase, totalPhases)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the phase advance without making changes")
	return command
}

// NewApplicationSetPhaseRollbackCommand returns a new instance of an `argocd appset phase rollback` command
func NewApplicationSetPhaseRollbackCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var dryRun bool
	command := &cobra.Command{
		Use:   "rollback APPSETNAME",
		Short: "Rollback to the previous phase",
		Example: templates.Examples(`
	# Rollback to previous phase
	argocd appset phase rollback my-appset

	# Dry run rollback (show what would happen)
	argocd appset phase rollback my-appset --dry-run
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			appSetName, appSetNs := argo.ParseFromQualifiedName(args[0], "")

			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName, AppsetNamespace: appSetNs})
			errors.CheckError(err)

			currentPhase, _ := getCurrentPhaseInfo(appSet)

			if currentPhase <= 0 {
				fmt.Printf("ApplicationSet '%s' is already at the initial phase (0)\n", args[0])
				return
			}

			previousPhase := currentPhase - 1
			if dryRun {
				fmt.Printf("Would rollback ApplicationSet '%s' from phase %d to phase %d (dry-run)\n",
					args[0], currentPhase, previousPhase)
				return
			}

			// Update the phase annotation and add rollback annotation
			if appSet.Annotations == nil {
				appSet.Annotations = make(map[string]string)
			}
			generatorType := getGeneratorTypeFromAppSet(appSet)
			appSet.Annotations["applicationset.argoproj.io/phase-"+generatorType] = strconv.Itoa(previousPhase)
			appSet.Annotations["applicationset.argoproj.io/rollback-phase-"+generatorType] = strconv.Itoa(currentPhase)

			appSetCreateRequest := applicationset.ApplicationSetCreateRequest{
				Applicationset: appSet,
				Upsert:         true,
			}
			_, err = appIf.Create(ctx, &appSetCreateRequest)
			errors.CheckError(err)

			fmt.Printf("ApplicationSet '%s' rolled back to phase %d (from phase %d)\n", args[0], previousPhase, currentPhase)
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the phase rollback without making changes")
	return command
}

// NewApplicationSetPhaseHistoryCommand returns a new instance of an `argocd appset phase history` command
func NewApplicationSetPhaseHistoryCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var output string
	command := &cobra.Command{
		Use:   "history APPSETNAME",
		Short: "Show phase deployment history",
		Example: templates.Examples(`
	# Show phase history
	argocd appset phase history my-appset

	# Show phase history in JSON format
	argocd appset phase history my-appset -o json
		`),
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationSetClientOrDie()
			defer utilio.Close(conn)

			appSetName, appSetNs := argo.ParseFromQualifiedName(args[0], "")

			appSet, err := appIf.Get(ctx, &applicationset.ApplicationSetGetQuery{Name: appSetName, AppsetNamespace: appSetNs})
			errors.CheckError(err)

			switch output {
			case "json", "yaml":
				history := extractPhaseHistory(appSet)
				err := PrintResource(history, output)
				errors.CheckError(err)
			case "wide", "":
				printPhaseHistoryTable(appSet)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	return command
}

// Helper functions for phase deployment CLI

func getCurrentPhaseInfo(appSet *arogappsetv1.ApplicationSet) (int, int) {
	if appSet.Annotations == nil {
		return 0, getTotalPhases(appSet)
	}

	generatorType := getGeneratorTypeFromAppSet(appSet)
	phaseKey := "applicationset.argoproj.io/phase-" + generatorType
	phaseStr, exists := appSet.Annotations[phaseKey]

	currentPhase := 0
	if exists {
		if phase, err := fmt.Sscanf(phaseStr, "%d", &currentPhase); phase != 1 || err != nil {
			currentPhase = 0
		}
	}

	return currentPhase, getTotalPhases(appSet)
}

func getTotalPhases(appSet *arogappsetv1.ApplicationSet) int {
	for _, generator := range appSet.Spec.Generators {
		if generator.DeploymentStrategy != nil &&
			generator.DeploymentStrategy.Type == "phaseDeployment" {
			return len(generator.DeploymentStrategy.Phases)
		}
	}
	return 0
}

func hasPhaseDeployment(appSet *arogappsetv1.ApplicationSet) bool {
	for _, generator := range appSet.Spec.Generators {
		if generator.DeploymentStrategy != nil &&
			generator.DeploymentStrategy.Type == "phaseDeployment" {
			return true
		}
	}
	return false
}

func getGeneratorTypeFromAppSet(appSet *arogappsetv1.ApplicationSet) string {
	if len(appSet.Spec.Generators) == 0 {
		return "unknown"
	}

	generator := appSet.Spec.Generators[0]
	switch {
	case generator.List != nil:
		return "list"
	case generator.Clusters != nil:
		return "clusters"
	case generator.Git != nil:
		return "git"
	case generator.SCMProvider != nil:
		return "scm-provider"
	case generator.ClusterDecisionResource != nil:
		return "cluster-decision-resource"
	case generator.PullRequest != nil:
		return "pull-request"
	case generator.Matrix != nil:
		return "matrix"
	case generator.Merge != nil:
		return "merge"
	case generator.Plugin != nil:
		return "plugin"
	default:
		return "unknown"
	}
}

func printPhaseStatusTable(appSet *arogappsetv1.ApplicationSet) {
	currentPhase, totalPhases := getCurrentPhaseInfo(appSet)

	fmt.Printf("ApplicationSet: %s\n", appSet.QualifiedName())
	fmt.Printf("Phase Status: %d/%d", currentPhase, totalPhases)

	if totalPhases > 0 {
		percentage := (float64(currentPhase) / float64(totalPhases)) * 100
		fmt.Printf(" (%.1f%% complete)", percentage)
	}
	fmt.Println()

	if totalPhases == 0 {
		fmt.Println("No phase deployment strategy configured")
		return
	}

	// Find the deployment strategy
	var phases []arogappsetv1.GeneratorDeploymentPhase
	for _, generator := range appSet.Spec.Generators {
		if generator.DeploymentStrategy != nil &&
			generator.DeploymentStrategy.Type == "phaseDeployment" {
			phases = generator.DeploymentStrategy.Phases
			break
		}
	}

	if len(phases) == 0 {
		return
	}

	fmt.Println("\nPhases:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "PHASE\tNAME\tSTATUS\tTARGETS\tCHECKS\n")

	for i, phase := range phases {
		status := "Pending"
		if i < currentPhase {
			status = "Completed"
		} else if i == currentPhase {
			status = "Current"
		}

		targets := strconv.Itoa(len(phase.Targets))
		if phase.Percentage != nil {
			targets = strconv.Itoa(*phase.Percentage) + "% + " + targets + " targets"
		}

		checks := strconv.Itoa(len(phase.Checks))

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", i, phase.Name, status, targets, checks)
	}
	w.Flush()

	// Show rollback information if available
	generatorType := getGeneratorTypeFromAppSet(appSet)
	rollbackKey := "applicationset.argoproj.io/rollback-phase-" + generatorType
	if appSet.Annotations != nil {
		if rollbackPhase, exists := appSet.Annotations[rollbackKey]; exists {
			fmt.Printf("\nRollback: Phase %s marked for rollback\n", rollbackPhase)
		}
	}
}

func printPhaseHistoryTable(appSet *arogappsetv1.ApplicationSet) {
	fmt.Printf("ApplicationSet: %s\n", appSet.QualifiedName())

	if appSet.Annotations == nil {
		fmt.Println("No phase history available")
		return
	}

	generatorType := getGeneratorTypeFromAppSet(appSet)
	phaseKey := "applicationset.argoproj.io/phase-" + generatorType
	rollbackKey := "applicationset.argoproj.io/rollback-phase-" + generatorType

	fmt.Println("\nPhase History:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TYPE\tVALUE\tTIMESTAMP\n")

	if phaseStr, exists := appSet.Annotations[phaseKey]; exists {
		// Use creation timestamp as approximation for phase changes
		timestamp := appSet.CreationTimestamp.Format("2006-01-02 15:04:05")
		fmt.Fprintf(w, "CURRENT\t%s\t%s\n", phaseStr, timestamp)
	}

	if rollbackPhase, exists := appSet.Annotations[rollbackKey]; exists {
		// Parse the rollback timestamp if it's in the expected format
		timestamp := rollbackPhase
		if len(rollbackPhase) > 1 {
			timestamp = "Unknown"
		}
		fmt.Fprintf(w, "ROLLBACK\t%s\t%s\n", rollbackPhase, timestamp)
	}

	w.Flush()
}

type PhaseStatus struct {
	ApplicationSetName string      `json:"applicationSetName"`
	CurrentPhase       int         `json:"currentPhase"`
	TotalPhases        int         `json:"totalPhases"`
	Completed          bool        `json:"completed"`
	Percentage         float64     `json:"percentage"`
	Phases             []PhaseInfo `json:"phases,omitempty"`
}

type PhaseInfo struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Targets    int    `json:"targets"`
	Percentage *int64 `json:"percentage,omitempty"`
	Checks     int    `json:"checks"`
}

func extractPhaseStatus(appSet *arogappsetv1.ApplicationSet) PhaseStatus {
	currentPhase, totalPhases := getCurrentPhaseInfo(appSet)

	status := PhaseStatus{
		ApplicationSetName: appSet.QualifiedName(),
		CurrentPhase:       currentPhase,
		TotalPhases:        totalPhases,
		Completed:          currentPhase >= totalPhases,
	}

	if totalPhases > 0 {
		status.Percentage = (float64(currentPhase) / float64(totalPhases)) * 100
	}

	// Extract phase details
	for _, generator := range appSet.Spec.Generators {
		if generator.DeploymentStrategy != nil &&
			generator.DeploymentStrategy.Type == "phaseDeployment" {
			for i, phase := range generator.DeploymentStrategy.Phases {
				phaseStatus := "pending"
				if i < currentPhase {
					phaseStatus = "completed"
				} else if i == currentPhase {
					phaseStatus = "current"
				}

				phaseInfo := PhaseInfo{
					Index:      i,
					Name:       phase.Name,
					Status:     phaseStatus,
					Targets:    len(phase.Targets),
					Percentage: phase.Percentage,
					Checks:     len(phase.Checks),
				}
				status.Phases = append(status.Phases, phaseInfo)
			}
			break
		}
	}

	return status
}

type PhaseHistory struct {
	ApplicationSetName string            `json:"applicationSetName"`
	Events             []PhaseEvent      `json:"events"`
	Annotations        map[string]string `json:"annotations,omitempty"`
}

type PhaseEvent struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Timestamp string `json:"timestamp"`
}

func extractPhaseHistory(appSet *arogappsetv1.ApplicationSet) PhaseHistory {
	history := PhaseHistory{
		ApplicationSetName: appSet.QualifiedName(),
		Events:             []PhaseEvent{},
	}

	if appSet.Annotations == nil {
		return history
	}

	// Filter phase-related annotations
	phaseAnnotations := make(map[string]string)
	for key, value := range appSet.Annotations {
		if strings.HasPrefix(key, "applicationset.argoproj.io/phase-") ||
			strings.HasPrefix(key, "applicationset.argoproj.io/rollback-") {
			phaseAnnotations[key] = value
		}
	}
	history.Annotations = phaseAnnotations

	generatorType := getGeneratorTypeFromAppSet(appSet)
	phaseKey := "applicationset.argoproj.io/phase-" + generatorType
	rollbackKey := "applicationset.argoproj.io/rollback-phase-" + generatorType

	// Add current phase event
	if phaseStr, exists := appSet.Annotations[phaseKey]; exists {
		history.Events = append(history.Events, PhaseEvent{
			Type:      "current",
			Value:     phaseStr,
			Timestamp: appSet.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
		})
	}

	// Add rollback event
	if rollbackValue, exists := appSet.Annotations[rollbackKey]; exists {
		history.Events = append(history.Events, PhaseEvent{
			Type:      "rollback",
			Value:     rollbackValue,
			Timestamp: rollbackValue, // Rollback annotation contains timestamp
		})
	}

	return history
}
