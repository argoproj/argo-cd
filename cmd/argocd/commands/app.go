package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/ghodss/yaml"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/controller"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	settingspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/repository"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/templates"
	"github.com/argoproj/argo-cd/v2/util/text/label"
)

var (
	appExample = templates.Examples(`
	# List all the applications.
	argocd app list

	# Get the details of a application
	argocd app get my-app

	# Set an override parameter
	argocd app set my-app -p image.tag=v1.0.1`)
)

// NewApplicationCommand returns a new instance of an `argocd app` command
func NewApplicationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:     "app",
		Short:   "Manage applications",
		Example: appExample,
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationCreateCommand(clientOpts))
	command.AddCommand(NewApplicationGetCommand(clientOpts))
	command.AddCommand(NewApplicationDiffCommand(clientOpts))
	command.AddCommand(NewApplicationSetCommand(clientOpts))
	command.AddCommand(NewApplicationUnsetCommand(clientOpts))
	command.AddCommand(NewApplicationSyncCommand(clientOpts))
	command.AddCommand(NewApplicationHistoryCommand(clientOpts))
	command.AddCommand(NewApplicationRollbackCommand(clientOpts))
	command.AddCommand(NewApplicationListCommand(clientOpts))
	command.AddCommand(NewApplicationDeleteCommand(clientOpts))
	command.AddCommand(NewApplicationWaitCommand(clientOpts))
	command.AddCommand(NewApplicationManifestsCommand(clientOpts))
	command.AddCommand(NewApplicationTerminateOpCommand(clientOpts))
	command.AddCommand(NewApplicationEditCommand(clientOpts))
	command.AddCommand(NewApplicationPatchCommand(clientOpts))
	command.AddCommand(NewApplicationPatchResourceCommand(clientOpts))
	command.AddCommand(NewApplicationDeleteResourceCommand(clientOpts))
	command.AddCommand(NewApplicationResourceActionsCommand(clientOpts))
	command.AddCommand(NewApplicationListResourcesCommand(clientOpts))
	command.AddCommand(NewApplicationLogsCommand(clientOpts))
	return command
}

// NewApplicationCreateCommand returns a new instance of an `argocd app create` command
func NewApplicationCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts     cmdutil.AppOptions
		fileURL     string
		appName     string
		upsert      bool
		labels      []string
		annotations []string
	)
	var command = &cobra.Command{
		Use:   "create APPNAME",
		Short: "Create an application",
		Example: `
	# Create a directory app
	argocd app create guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --directory-recurse

	# Create a Jsonnet app
	argocd app create jsonnet-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path jsonnet-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --jsonnet-ext-str replicas=2

	# Create a Helm app
	argocd app create helm-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path helm-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --helm-set replicaCount=2

	# Create a Helm app from a Helm repo
	argocd app create nginx-ingress --repo https://charts.helm.sh/stable --helm-chart nginx-ingress --revision 1.24.3 --dest-namespace default --dest-server https://kubernetes.default.svc

	# Create a Kustomize app
	argocd app create kustomize-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path kustomize-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --kustomize-image gcr.io/heptio-images/ks-guestbook-demo:0.1

	# Create a app using a custom tool:
	argocd app create ksane --repo https://github.com/argoproj/argocd-example-apps.git --path plugins/kasane --dest-namespace default --dest-server https://kubernetes.default.svc --config-management-plugin kasane
`,
		Run: func(c *cobra.Command, args []string) {
			argocdClient := argocdclient.NewClientOrDie(clientOpts)

			app, err := cmdutil.ConstructApp(fileURL, appName, labels, annotations, args, appOpts, c.Flags())
			errors.CheckError(err)

			if app.Name == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			conn, appIf := argocdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appCreateRequest := applicationpkg.ApplicationCreateRequest{
				Application: *app,
				Upsert:      &upsert,
				Validate:    &appOpts.Validate,
			}
			created, err := appIf.Create(context.Background(), &appCreateRequest)
			errors.CheckError(err)
			fmt.Printf("application '%s' created\n", created.ObjectMeta.Name)
		},
	}
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override application with the same name even if supplied application spec is different from existing spec")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringArrayVarP(&labels, "label", "l", []string{}, "Labels to apply to the app")
	command.Flags().StringArrayVarP(&annotations, "annotations", "", []string{}, "Set metadata annotations (e.g. example=value)")
	// Only complete files with appropriate extension.
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	if err != nil {
		log.Fatal(err)
	}
	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

func getInfos(infos []string) []*argoappv1.Info {
	mapInfos, err := label.Parse(infos)
	errors.CheckError(err)
	sliceInfos := make([]*argoappv1.Info, len(mapInfos))
	i := 0
	for key, element := range mapInfos {
		sliceInfos[i] = &argoappv1.Info{Name: key, Value: element}
		i++
	}
	return sliceInfos
}

func getRefreshType(refresh bool, hardRefresh bool) *string {
	if hardRefresh {
		refreshType := string(argoappv1.RefreshTypeHard)
		return &refreshType
	}

	if refresh {
		refreshType := string(argoappv1.RefreshTypeNormal)
		return &refreshType
	}

	return nil
}

// NewApplicationGetCommand returns a new instance of an `argocd app get` command
func NewApplicationGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		refresh       bool
		hardRefresh   bool
		output        string
		showParams    bool
		showOperation bool
	)
	var command = &cobra.Command{
		Use:   "get APPNAME",
		Short: "Get application details",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: getRefreshType(refresh, hardRefresh)})
			errors.CheckError(err)

			pConn, projIf := argocdclient.NewClientOrDie(clientOpts).NewProjectClientOrDie()
			defer argoio.Close(pConn)
			proj, err := projIf.Get(context.Background(), &projectpkg.ProjectQuery{Name: app.Spec.Project})
			errors.CheckError(err)

			windows := proj.Spec.SyncWindows.Matches(app)

			switch output {
			case "yaml", "json":
				err := PrintResource(app, output)
				errors.CheckError(err)
			case "wide", "":
				aURL := appURL(acdClient, app.Name)
				printAppSummaryTable(app, aURL, windows)

				if len(app.Status.Conditions) > 0 {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					printAppConditions(w, app)
					_ = w.Flush()
					fmt.Println()
				}
				if showOperation && app.Status.OperationState != nil {
					fmt.Println()
					printOperationResult(app.Status.OperationState)
				}
				if showParams {
					printParams(app)
				}
				if len(app.Status.Resources) > 0 {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					printAppResources(w, app)
					_ = w.Flush()
				}
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: json|yaml|wide")
	command.Flags().BoolVar(&showOperation, "show-operation", false, "Show application operation")
	command.Flags().BoolVar(&showParams, "show-params", false, "Show application parameters and overrides")
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "Refresh application data as well as target manifests cache")
	return command
}

// NewApplicationLogsCommand returns logs of application pods
func NewApplicationLogsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		group        string
		kind         string
		namespace    string
		resourceName string
		follow       bool
		tail         int64
		sinceSeconds int64
		untilTime    string
		filter       string
		container    string
	)
	var command = &cobra.Command{
		Use:   "logs APPNAME",
		Short: "Get logs of application pods",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName := args[0]

			retry := true
			for retry {
				retry = false
				stream, err := appIf.PodLogs(context.Background(), &applicationpkg.ApplicationPodLogsQuery{
					Name:         &appName,
					Group:        &group,
					Namespace:    namespace,
					Kind:         &kind,
					ResourceName: &resourceName,
					Follow:       follow,
					TailLines:    tail,
					SinceSeconds: sinceSeconds,
					UntilTime:    &untilTime,
					Filter:       &filter,
					Container:    container,
				})
				if err != nil {
					log.Fatalf("failed to get pod logs: %v", err)
				}
				for {
					msg, err := stream.Recv()
					if err == io.EOF {
						return
					}
					if err != nil {
						st, ok := status.FromError(err)
						if !ok {
							log.Fatalf("stream read failed: %v", err)
						}
						if st.Code() == codes.Unavailable && follow {
							retry = true
							sinceSeconds = 1
							break
						}
						log.Fatalf("stream read failed: %v", err)
					}
					if !msg.Last {
						fmt.Println(msg.Content)
					} else {
						return
					}
				} //Done with receive message
			} //Done with retry
		},
	}

	command.Flags().StringVar(&group, "group", "", "Resource group")
	command.Flags().StringVar(&kind, "kind", "", "Resource kind")
	command.Flags().StringVar(&namespace, "namespace", "", "Resource namespace")
	command.Flags().StringVar(&resourceName, "name", "", "Resource name")
	command.Flags().BoolVar(&follow, "follow", false, "Specify if the logs should be streamed")
	command.Flags().Int64Var(&tail, "tail", 0, "The number of lines from the end of the logs to show")
	command.Flags().Int64Var(&sinceSeconds, "since-seconds", 0, "A relative time in seconds before the current time from which to show logs")
	command.Flags().StringVar(&untilTime, "until-time", "", "Show logs until this time")
	command.Flags().StringVar(&filter, "filter", "", "Show logs contain this string")
	command.Flags().StringVar(&container, "container", "", "Optional container name")

	return command
}

func printAppSummaryTable(app *argoappv1.Application, appURL string, windows *argoappv1.SyncWindows) {
	fmt.Printf(printOpFmtStr, "Name:", app.Name)
	fmt.Printf(printOpFmtStr, "Project:", app.Spec.GetProject())
	fmt.Printf(printOpFmtStr, "Server:", app.Spec.Destination.Server)
	fmt.Printf(printOpFmtStr, "Namespace:", app.Spec.Destination.Namespace)
	fmt.Printf(printOpFmtStr, "URL:", appURL)
	fmt.Printf(printOpFmtStr, "Repo:", app.Spec.Source.RepoURL)
	fmt.Printf(printOpFmtStr, "Target:", app.Spec.Source.TargetRevision)
	fmt.Printf(printOpFmtStr, "Path:", app.Spec.Source.Path)
	printAppSourceDetails(&app.Spec.Source)
	var wds []string
	var status string
	var allow, deny, inactiveAllows bool
	if windows.HasWindows() {
		active := windows.Active()
		if active.HasWindows() {
			for _, w := range *active {
				if w.Kind == "deny" {
					deny = true
				} else {
					allow = true
				}
			}
		}
		if windows.InactiveAllows().HasWindows() {
			inactiveAllows = true
		}

		s := windows.CanSync(true)
		if deny || !deny && !allow && inactiveAllows {
			if s {
				status = "Manual Allowed"
			} else {
				status = "Sync Denied"

			}
		} else {
			status = "Sync Allowed"
		}
		for _, w := range *windows {
			s := w.Kind + ":" + w.Schedule + ":" + w.Duration
			wds = append(wds, s)
		}
	} else {
		status = "Sync Allowed"
	}
	fmt.Printf(printOpFmtStr, "SyncWindow:", status)
	if len(wds) > 0 {
		fmt.Printf(printOpFmtStr, "Assigned Windows:", strings.Join(wds, ","))
	}

	var syncPolicy string
	if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil {
		syncPolicy = "Automated"
		if app.Spec.SyncPolicy.Automated.Prune {
			syncPolicy += " (Prune)"
		}
	} else {
		syncPolicy = "<none>"
	}
	fmt.Printf(printOpFmtStr, "Sync Policy:", syncPolicy)
	syncStatusStr := string(app.Status.Sync.Status)
	switch app.Status.Sync.Status {
	case argoappv1.SyncStatusCodeSynced:
		syncStatusStr += fmt.Sprintf(" to %s", app.Spec.Source.TargetRevision)
	case argoappv1.SyncStatusCodeOutOfSync:
		syncStatusStr += fmt.Sprintf(" from %s", app.Spec.Source.TargetRevision)
	}
	if !git.IsCommitSHA(app.Spec.Source.TargetRevision) && !git.IsTruncatedCommitSHA(app.Spec.Source.TargetRevision) && len(app.Status.Sync.Revision) > 7 {
		syncStatusStr += fmt.Sprintf(" (%s)", app.Status.Sync.Revision[0:7])
	}
	fmt.Printf(printOpFmtStr, "Sync Status:", syncStatusStr)
	healthStr := string(app.Status.Health.Status)
	if app.Status.Health.Message != "" {
		healthStr = fmt.Sprintf("%s (%s)", app.Status.Health.Status, app.Status.Health.Message)
	}
	fmt.Printf(printOpFmtStr, "Health Status:", healthStr)
}

func printAppSourceDetails(appSrc *argoappv1.ApplicationSource) {
	if appSrc.Ksonnet != nil && appSrc.Ksonnet.Environment != "" {
		fmt.Printf(printOpFmtStr, "Environment:", appSrc.Ksonnet.Environment)
	}
	if appSrc.Helm != nil && len(appSrc.Helm.ValueFiles) > 0 {
		fmt.Printf(printOpFmtStr, "Helm Values:", strings.Join(appSrc.Helm.ValueFiles, ","))
	}
	if appSrc.Kustomize != nil && appSrc.Kustomize.NamePrefix != "" {
		fmt.Printf(printOpFmtStr, "Name Prefix:", appSrc.Kustomize.NamePrefix)
	}
}

func printAppConditions(w io.Writer, app *argoappv1.Application) {
	_, _ = fmt.Fprintf(w, "CONDITION\tMESSAGE\tLAST TRANSITION\n")
	for _, item := range app.Status.Conditions {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", item.Type, item.Message, item.LastTransitionTime)
	}
}

// appURLDefault returns the default URL of an application
func appURLDefault(acdClient argocdclient.Client, appName string) string {
	var scheme string
	opts := acdClient.ClientOptions()
	server := opts.ServerAddr
	if opts.PlainText {
		scheme = "http"
	} else {
		scheme = "https"
		if strings.HasSuffix(opts.ServerAddr, ":443") {
			server = server[0 : len(server)-4]
		}
	}
	return fmt.Sprintf("%s://%s/applications/%s", scheme, server, appName)
}

// appURL returns the URL of an application
func appURL(acdClient argocdclient.Client, appName string) string {
	conn, settingsIf := acdClient.NewSettingsClientOrDie()
	defer argoio.Close(conn)
	argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
	errors.CheckError(err)

	if argoSettings.URL != "" {
		return fmt.Sprintf("%s/applications/%s", argoSettings.URL, appName)
	}
	return appURLDefault(acdClient, appName)
}

func truncateString(str string, num int) string {
	bnoden := str
	if utf8.RuneCountInString(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = string([]rune(str)[0:num]) + "..."
	}
	return bnoden
}

// printParams prints parameters and overrides
func printParams(app *argoappv1.Application) {
	paramLenLimit := 80
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if app.Spec.Source.Ksonnet != nil {
		fmt.Println()
		_, _ = fmt.Fprintf(w, "COMPONENT\tNAME\tVALUE\n")
		for _, p := range app.Spec.Source.Ksonnet.Parameters {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.Component, p.Name, truncateString(p.Value, paramLenLimit))
		}
	} else if app.Spec.Source.Helm != nil {
		fmt.Println()
		_, _ = fmt.Fprintf(w, "NAME\tVALUE\n")
		for _, p := range app.Spec.Source.Helm.Parameters {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", p.Name, truncateString(p.Value, paramLenLimit))
		}
	}
	_ = w.Flush()
}

// NewApplicationSetCommand returns a new instance of an `argocd app set` command
func NewApplicationSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts cmdutil.AppOptions
	)
	var command = &cobra.Command{
		Use:   "set APPNAME",
		Short: "Set application parameters",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			ctx := context.Background()
			appName := args[0]
			argocdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := argocdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			visited := cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			setParameterOverrides(app, appOpts.Parameters)
			_, err = appIf.UpdateSpec(ctx, &applicationpkg.ApplicationUpdateSpecRequest{
				Name:     &app.Name,
				Spec:     app.Spec,
				Validate: &appOpts.Validate,
			})
			errors.CheckError(err)
		},
	}
	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

// NewApplicationUnsetCommand returns a new instance of an `argocd app unset` command
func NewApplicationUnsetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		parameters       []string
		valuesLiteral    bool
		valuesFiles      []string
		nameSuffix       bool
		namePrefix       bool
		kustomizeVersion bool
		kustomizeImages  []string
		pluginEnvs       []string
		appOpts          cmdutil.AppOptions
	)
	var command = &cobra.Command{
		Use:   "unset APPNAME parameters",
		Short: "Unset application parameters",
		Example: `  # Unset kustomize override kustomize image
  argocd app unset my-app --kustomize-image=alpine

  # Unset kustomize override prefix
  argocd app unset my-app --namesuffix

  # Unset parameter override
  argocd app unset my-app -p COMPONENT=PARAM`,

		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)

			updated := false
			if app.Spec.Source.Kustomize != nil {
				if namePrefix {
					updated = true
					app.Spec.Source.Kustomize.NamePrefix = ""
				}

				if nameSuffix {
					updated = true
					app.Spec.Source.Kustomize.NameSuffix = ""
				}

				if kustomizeVersion {
					updated = true
					app.Spec.Source.Kustomize.Version = ""
				}

				for _, kustomizeImage := range kustomizeImages {
					for i, item := range app.Spec.Source.Kustomize.Images {
						if argoappv1.KustomizeImage(kustomizeImage).Match(item) {
							updated = true
							//remove i
							a := app.Spec.Source.Kustomize.Images
							copy(a[i:], a[i+1:]) // Shift a[i+1:] left one index.
							a[len(a)-1] = ""     // Erase last element (write zero value).
							a = a[:len(a)-1]     // Truncate slice.
							app.Spec.Source.Kustomize.Images = a
						}
					}
				}
			}
			if app.Spec.Source.Ksonnet != nil {
				if len(parameters) == 0 && len(valuesFiles) == 0 {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				for _, paramStr := range parameters {
					parts := strings.SplitN(paramStr, "=", 2)
					if len(parts) != 2 {
						log.Fatalf("Expected parameter of the form: component=param. Received: %s", paramStr)
					}
					overrides := app.Spec.Source.Ksonnet.Parameters
					for i, override := range overrides {
						if override.Component == parts[0] && override.Name == parts[1] {
							app.Spec.Source.Ksonnet.Parameters = append(overrides[0:i], overrides[i+1:]...)
							updated = true
							break
						}
					}
				}
			}
			if app.Spec.Source.Helm != nil {
				if len(parameters) == 0 && len(valuesFiles) == 0 && !valuesLiteral {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				for _, paramStr := range parameters {
					helmParams := app.Spec.Source.Helm.Parameters
					for i, p := range helmParams {
						if p.Name == paramStr {
							app.Spec.Source.Helm.Parameters = append(helmParams[0:i], helmParams[i+1:]...)
							updated = true
							break
						}
					}
				}
				if valuesLiteral {
					app.Spec.Source.Helm.Values = ""
					updated = true
				}
				for _, valuesFile := range valuesFiles {
					specValueFiles := app.Spec.Source.Helm.ValueFiles
					for i, vf := range specValueFiles {
						if vf == valuesFile {
							app.Spec.Source.Helm.ValueFiles = append(specValueFiles[0:i], specValueFiles[i+1:]...)
							updated = true
							break
						}
					}
				}
			}

			if app.Spec.Source.Plugin != nil {
				if len(pluginEnvs) == 0 {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				for _, env := range pluginEnvs {
					err = app.Spec.Source.Plugin.RemoveEnvEntry(env)
					if err == nil {
						updated = true
					}
				}
			}

			if !updated {
				return
			}

			cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
			_, err = appIf.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{
				Name:     &app.Name,
				Spec:     app.Spec,
				Validate: &appOpts.Validate,
			})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVarP(&parameters, "parameter", "p", []string{}, "Unset a parameter override (e.g. -p guestbook=image)")
	command.Flags().StringArrayVar(&valuesFiles, "values", []string{}, "Unset one or more Helm values files")
	command.Flags().BoolVar(&valuesLiteral, "values-literal", false, "Unset literal Helm values block")
	command.Flags().BoolVar(&nameSuffix, "namesuffix", false, "Kustomize namesuffix")
	command.Flags().BoolVar(&namePrefix, "nameprefix", false, "Kustomize nameprefix")
	command.Flags().BoolVar(&kustomizeVersion, "kustomize-version", false, "Kustomize version")
	command.Flags().StringArrayVar(&kustomizeImages, "kustomize-image", []string{}, "Kustomize images name (e.g. --kustomize-image node --kustomize-image mysql)")
	command.Flags().StringArrayVar(&pluginEnvs, "plugin-env", []string{}, "Unset plugin env variables (e.g --plugin-env name)")
	return command
}

// targetObjects deserializes the list of target states into unstructured objects
func targetObjects(resources []*argoappv1.ResourceDiff) ([]*unstructured.Unstructured, error) {
	objs := make([]*unstructured.Unstructured, len(resources))
	for i, resState := range resources {
		obj, err := resState.TargetObject()
		if err != nil {
			return nil, err
		}
		objs[i] = obj
	}
	return objs, nil
}

// liveObjects deserializes the list of live states into unstructured objects
func liveObjects(resources []*argoappv1.ResourceDiff) ([]*unstructured.Unstructured, error) {
	objs := make([]*unstructured.Unstructured, len(resources))
	for i, resState := range resources {
		obj, err := resState.LiveObject()
		if err != nil {
			return nil, err
		}
		objs[i] = obj
	}
	return objs, nil
}

func getLocalObjects(app *argoappv1.Application, local, localRepoRoot, appLabelKey, kubeVersion string, kustomizeOptions *argoappv1.KustomizeOptions,
	configManagementPlugins []*argoappv1.ConfigManagementPlugin, trackingMethod string) []*unstructured.Unstructured {
	manifestStrings := getLocalObjectsString(app, local, localRepoRoot, appLabelKey, kubeVersion, kustomizeOptions, configManagementPlugins, trackingMethod)
	objs := make([]*unstructured.Unstructured, len(manifestStrings))
	for i := range manifestStrings {
		obj := unstructured.Unstructured{}
		err := json.Unmarshal([]byte(manifestStrings[i]), &obj)
		errors.CheckError(err)
		objs[i] = &obj
	}
	return objs
}

func getLocalObjectsString(app *argoappv1.Application, local, localRepoRoot, appLabelKey, kubeVersion string, kustomizeOptions *argoappv1.KustomizeOptions,
	configManagementPlugins []*argoappv1.ConfigManagementPlugin, trackingMethod string) []string {

	res, err := repository.GenerateManifests(local, localRepoRoot, app.Spec.Source.TargetRevision, &repoapiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{Repo: app.Spec.Source.RepoURL},
		AppLabelKey:       appLabelKey,
		AppName:           app.Name,
		Namespace:         app.Spec.Destination.Namespace,
		ApplicationSource: &app.Spec.Source,
		KustomizeOptions:  kustomizeOptions,
		KubeVersion:       kubeVersion,
		Plugins:           configManagementPlugins,
		TrackingMethod:    trackingMethod,
	}, true)
	errors.CheckError(err)

	return res.Manifests
}

type resourceInfoProvider struct {
	namespacedByGk map[schema.GroupKind]bool
}

// Infer if obj is namespaced or not from corresponding live objects list. If corresponding live object has namespace then target object is also namespaced.
// If live object is missing then it does not matter if target is namespaced or not.
func (p *resourceInfoProvider) IsNamespaced(gk schema.GroupKind) (bool, error) {
	return p.namespacedByGk[gk], nil
}

func groupObjsByKey(localObs []*unstructured.Unstructured, liveObjs []*unstructured.Unstructured, appNamespace string) map[kube.ResourceKey]*unstructured.Unstructured {
	namespacedByGk := make(map[schema.GroupKind]bool)
	for i := range liveObjs {
		if liveObjs[i] != nil {
			key := kube.GetResourceKey(liveObjs[i])
			namespacedByGk[schema.GroupKind{Group: key.Group, Kind: key.Kind}] = key.Namespace != ""
		}
	}
	localObs, _, err := controller.DeduplicateTargetObjects(appNamespace, localObs, &resourceInfoProvider{namespacedByGk: namespacedByGk})
	errors.CheckError(err)
	objByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for i := range localObs {
		obj := localObs[i]
		if !(hook.IsHook(obj) || ignore.Ignore(obj)) {
			objByKey[kube.GetResourceKey(obj)] = obj
		}
	}
	return objByKey
}

type objKeyLiveTarget struct {
	key    kube.ResourceKey
	live   *unstructured.Unstructured
	target *unstructured.Unstructured
}

// NewApplicationDiffCommand returns a new instance of an `argocd app diff` command
func NewApplicationDiffCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		refresh       bool
		hardRefresh   bool
		exitCode      bool
		local         string
		revision      string
		localRepoRoot string
	)
	shortDesc := "Perform a diff against the target and live state."
	var command = &cobra.Command{
		Use:   "diff APPNAME",
		Short: shortDesc,
		Long:  shortDesc + "\nUses 'diff' to render the difference. KUBECTL_EXTERNAL_DIFF environment variable can be used to select your own diff tool.\nReturns the following exit codes: 2 on general errors, 1 when a diff is found, and 0 when no diff is found",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(2)
			}

			clientset := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := clientset.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: getRefreshType(refresh, hardRefresh)})
			errors.CheckError(err)
			resources, err := appIf.ManagedResources(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &appName})
			errors.CheckError(err)
			liveObjs, err := liveObjects(resources.Items)
			errors.CheckError(err)
			items := make([]objKeyLiveTarget, 0)

			conn, settingsIf := clientset.NewSettingsClientOrDie()
			defer argoio.Close(conn)
			argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
			errors.CheckError(err)

			if local != "" {
				conn, clusterIf := clientset.NewClusterClientOrDie()
				defer argoio.Close(conn)
				cluster, err := clusterIf.Get(context.Background(), &clusterpkg.ClusterQuery{Name: app.Spec.Destination.Name, Server: app.Spec.Destination.Server})
				errors.CheckError(err)
				localObjs := groupObjsByKey(getLocalObjects(app, local, localRepoRoot, argoSettings.AppLabelKey, cluster.ServerVersion, argoSettings.KustomizeOptions, argoSettings.ConfigManagementPlugins, argoSettings.TrackingMethod), liveObjs, app.Spec.Destination.Namespace)
				items = groupObjsForDiff(resources, localObjs, items, argoSettings, appName)
			} else if revision != "" {
				var unstructureds []*unstructured.Unstructured
				q := applicationpkg.ApplicationManifestQuery{
					Name:     &appName,
					Revision: revision,
				}
				res, err := appIf.GetManifests(context.Background(), &q)
				errors.CheckError(err)
				for _, mfst := range res.Manifests {
					obj, err := argoappv1.UnmarshalToUnstructured(mfst)
					errors.CheckError(err)
					unstructureds = append(unstructureds, obj)
				}
				groupedObjs := groupObjsByKey(unstructureds, liveObjs, app.Spec.Destination.Namespace)
				items = groupObjsForDiff(resources, groupedObjs, items, argoSettings, appName)
			} else {
				for i := range resources.Items {
					res := resources.Items[i]
					var live = &unstructured.Unstructured{}
					err := json.Unmarshal([]byte(res.NormalizedLiveState), &live)
					errors.CheckError(err)

					var target = &unstructured.Unstructured{}
					err = json.Unmarshal([]byte(res.TargetState), &target)
					errors.CheckError(err)

					items = append(items, objKeyLiveTarget{kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name), live, target})
				}
			}

			foundDiffs := false
			for _, item := range items {
				if item.target != nil && hook.IsHook(item.target) || item.live != nil && hook.IsHook(item.live) {
					continue
				}
				overrides := make(map[string]argoappv1.ResourceOverride)
				for k := range argoSettings.ResourceOverrides {
					val := argoSettings.ResourceOverrides[k]
					overrides[k] = *val
				}
				normalizer, err := argo.NewDiffNormalizer(app.Spec.IgnoreDifferences, overrides)
				errors.CheckError(err)

				diffRes, err := diff.Diff(item.target, item.live, diff.WithNormalizer(normalizer))
				errors.CheckError(err)

				if diffRes.Modified || item.target == nil || item.live == nil {
					fmt.Printf("===== %s/%s %s/%s ======\n", item.key.Group, item.key.Kind, item.key.Namespace, item.key.Name)
					var live *unstructured.Unstructured
					var target *unstructured.Unstructured
					if item.target != nil && item.live != nil {
						target = &unstructured.Unstructured{}
						live = item.live
						err = json.Unmarshal(diffRes.PredictedLive, target)
						errors.CheckError(err)
					} else {
						live = item.live
						target = item.target
					}

					foundDiffs = true
					_ = cli.PrintDiff(item.key.Name, live, target)
				}
			}
			if foundDiffs && exitCode {
				os.Exit(1)
			}

		},
	}
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "Refresh application data as well as target manifests cache")
	command.Flags().BoolVar(&exitCode, "exit-code", true, "Return non-zero exit code when there is a diff")
	command.Flags().StringVar(&local, "local", "", "Compare live app to a local manifests")
	command.Flags().StringVar(&revision, "revision", "", "Compare live app to a particular revision")
	command.Flags().StringVar(&localRepoRoot, "local-repo-root", "/", "Path to the repository root. Used together with --local allows setting the repository root")
	return command
}

func groupObjsForDiff(resources *application.ManagedResourcesResponse, objs map[kube.ResourceKey]*unstructured.Unstructured, items []objKeyLiveTarget, argoSettings *settings.Settings, appName string) []objKeyLiveTarget {
	resourceTracking := argo.NewResourceTracking()
	for _, res := range resources.Items {
		var live = &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(res.NormalizedLiveState), &live)
		errors.CheckError(err)

		key := kube.ResourceKey{Name: res.Name, Namespace: res.Namespace, Group: res.Group, Kind: res.Kind}
		if key.Kind == kube.SecretKind && key.Group == "" {
			// Don't bother comparing secrets, argo-cd doesn't have access to k8s secret data
			delete(objs, key)
			continue
		}
		if local, ok := objs[key]; ok || live != nil {
			if local != nil && !kube.IsCRD(local) {
				err = resourceTracking.SetAppInstance(local, argoSettings.AppLabelKey, appName, "", argoappv1.TrackingMethod(argoSettings.GetTrackingMethod()))
				errors.CheckError(err)
			}

			items = append(items, objKeyLiveTarget{key, live, local})
			delete(objs, key)
		}
	}
	for key, local := range objs {
		if key.Kind == kube.SecretKind && key.Group == "" {
			// Don't bother comparing secrets, argo-cd doesn't have access to k8s secret data
			delete(objs, key)
			continue
		}
		items = append(items, objKeyLiveTarget{key, nil, local})
	}
	return items
}

// NewApplicationDeleteCommand returns a new instance of an `argocd app delete` command
func NewApplicationDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		cascade           bool
		noPrompt          bool
		propagationPolicy string
	)
	var command = &cobra.Command{
		Use:   "delete APPNAME",
		Short: "Delete an application",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			var isTerminal bool = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			var isConfirmAll bool = false
			var numOfApps = len(args)
			var promptFlag = c.Flag("yes")
			if promptFlag.Changed && promptFlag.Value.String() == "true" {
				noPrompt = true
			}
			for _, appName := range args {
				appDeleteReq := applicationpkg.ApplicationDeleteRequest{
					Name: &appName,
				}
				if c.Flag("cascade").Changed {
					appDeleteReq.Cascade = &cascade
				}
				if c.Flag("propagation-policy").Changed {
					appDeleteReq.PropagationPolicy = &propagationPolicy
				}
				if cascade && isTerminal && !noPrompt {
					var confirmAnswer string = "n"
					var lowercaseAnswer string
					if numOfApps == 1 {
						fmt.Println("Are you sure you want to delete '" + appName + "' and all its resources? [y/n]")
						fmt.Scan(&confirmAnswer)
						lowercaseAnswer = strings.ToLower(confirmAnswer)
					} else {
						if !isConfirmAll {
							fmt.Println("Are you sure you want to delete '" + appName + "' and all its resources? [y/n/A] where 'A' is to delete all specified apps and their resources without prompting")
							fmt.Scan(&confirmAnswer)
							lowercaseAnswer = strings.ToLower(confirmAnswer)
							if lowercaseAnswer == "a" || lowercaseAnswer == "all" {
								lowercaseAnswer = "y"
								isConfirmAll = true
							}
						} else {
							lowercaseAnswer = "y"
						}
					}
					if lowercaseAnswer == "y" || lowercaseAnswer == "yes" {
						_, err := appIf.Delete(context.Background(), &appDeleteReq)
						errors.CheckError(err)
					} else {
						fmt.Println("The command to delete '" + appName + "' was cancelled.")
					}
				} else {
					_, err := appIf.Delete(context.Background(), &appDeleteReq)
					errors.CheckError(err)
				}
			}
		},
	}
	command.Flags().BoolVar(&cascade, "cascade", true, "Perform a cascaded deletion of all application resources")
	command.Flags().StringVarP(&propagationPolicy, "propagation-policy", "p", "foreground", "Specify propagation policy for deletion of application's resources. One of: foreground|background")
	command.Flags().BoolVarP(&noPrompt, "yes", "y", false, "Turn off prompting to confirm cascaded deletion of application resources")
	return command
}

// Print simple list of application names
func printApplicationNames(apps []argoappv1.Application) {
	for _, app := range apps {
		fmt.Println(app.Name)
	}
}

// Print table of application data
func printApplicationTable(apps []argoappv1.Application, output *string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	var fmtStr string
	headers := []interface{}{"NAME", "CLUSTER", "NAMESPACE", "PROJECT", "STATUS", "HEALTH", "SYNCPOLICY", "CONDITIONS"}
	if *output == "wide" {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
		headers = append(headers, "REPO", "PATH", "TARGET")
	} else {
		fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
	}
	_, _ = fmt.Fprintf(w, fmtStr, headers...)
	for _, app := range apps {
		vals := []interface{}{
			app.Name,
			app.Spec.Destination.Server,
			app.Spec.Destination.Namespace,
			app.Spec.GetProject(),
			app.Status.Sync.Status,
			app.Status.Health.Status,
			formatSyncPolicy(app),
			formatConditionsSummary(app),
		}
		if *output == "wide" {
			vals = append(vals, app.Spec.Source.RepoURL, app.Spec.Source.Path, app.Spec.Source.TargetRevision)
		}
		_, _ = fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
}

// NewApplicationListCommand returns a new instance of an `argocd app list` command
func NewApplicationListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output   string
		selector string
		projects []string
		repo     string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List applications",
		Example: `  # List all apps
  argocd app list

  # List apps by label, in this example we listing apps that are children of another app (aka app-of-apps)
  argocd app list -l app.kubernetes.io/instance=my-app`,
		Run: func(c *cobra.Command, args []string) {
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			apps, err := appIf.List(context.Background(), &applicationpkg.ApplicationQuery{Selector: selector})
			errors.CheckError(err)
			appList := apps.Items
			if len(projects) != 0 {
				appList = argo.FilterByProjects(appList, projects)
			}
			if repo != "" {
				appList = argo.FilterByRepo(appList, repo)
			}
			switch output {
			case "yaml", "json":
				err := PrintResourceList(appList, output, false)
				errors.CheckError(err)
			case "name":
				printApplicationNames(appList)
			case "wide", "":
				printApplicationTable(appList, &output)
			default:
				errors.CheckError(fmt.Errorf("unknown output format: %s", output))
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name|json|yaml")
	command.Flags().StringVarP(&selector, "selector", "l", "", "List apps by label")
	command.Flags().StringArrayVarP(&projects, "project", "p", []string{}, "Filter by project name")
	command.Flags().StringVarP(&repo, "repo", "r", "", "List apps by source repo URL")
	return command
}

func formatSyncPolicy(app argoappv1.Application) string {
	if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
		return "<none>"
	}
	policy := "Auto"
	if app.Spec.SyncPolicy.Automated.Prune {
		policy = policy + "-Prune"
	}
	return policy
}

func formatConditionsSummary(app argoappv1.Application) string {
	typeToCnt := make(map[string]int)
	for i := range app.Status.Conditions {
		condition := app.Status.Conditions[i]
		if cnt, ok := typeToCnt[condition.Type]; ok {
			typeToCnt[condition.Type] = cnt + 1
		} else {
			typeToCnt[condition.Type] = 1
		}
	}
	items := make([]string, 0)
	for cndType, cnt := range typeToCnt {
		if cnt > 1 {
			items = append(items, fmt.Sprintf("%s(%d)", cndType, cnt))
		} else {
			items = append(items, cndType)
		}
	}
	summary := "<none>"
	if len(items) > 0 {
		summary = strings.Join(items, ",")
	}
	return summary
}

const (
	resourceFieldDelimiter              = ":"
	resourceFieldCount                  = 3
	resourceFieldNamespaceDelimiter     = "/"
	resourceFieldNameWithNamespaceCount = 2
)

func parseSelectedResources(resources []string) []argoappv1.SyncOperationResource {
	var selectedResources []argoappv1.SyncOperationResource
	if resources != nil {
		selectedResources = []argoappv1.SyncOperationResource{}
		for _, r := range resources {
			fields := strings.Split(r, resourceFieldDelimiter)
			if len(fields) != resourceFieldCount {
				log.Fatalf("Resource should have GROUP%sKIND%sNAME, but instead got: %s", resourceFieldDelimiter, resourceFieldDelimiter, r)
			}
			name := fields[2]
			namespace := ""
			if strings.Contains(fields[2], resourceFieldNamespaceDelimiter) {
				nameFields := strings.Split(fields[2], resourceFieldNamespaceDelimiter)
				if len(nameFields) != resourceFieldNameWithNamespaceCount {
					log.Fatalf("Resource with namespace should have GROUP%sKIND%sNAMESPACE%sNAME, but instead got: %s", resourceFieldDelimiter, resourceFieldDelimiter, resourceFieldNamespaceDelimiter, r)
				}
				namespace = nameFields[0]
				name = nameFields[1]
			}
			rsrc := argoappv1.SyncOperationResource{
				Group:     fields[0],
				Kind:      fields[1],
				Name:      name,
				Namespace: namespace,
			}
			selectedResources = append(selectedResources, rsrc)
		}
	}
	return selectedResources
}

// NewApplicationWaitCommand returns a new instance of an `argocd app wait` command
func NewApplicationWaitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		watchSync       bool
		watchHealth     bool
		watchSuspended  bool
		watchOperations bool
		timeout         uint
		selector        string
		resources       []string
	)
	var command = &cobra.Command{
		Use:   "wait [APPNAME.. | -l selector]",
		Short: "Wait for an application to reach a synced and healthy state",
		Example: `  # Wait for an app
  argocd app wait my-app

  # Wait for multiple apps
  argocd app wait my-app other-app

  # Wait for apps by label, in this example we waiting for apps that are children of another app (aka app-of-apps)
  argocd app wait -l app.kubernetes.io/instance=apps`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 && selector == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if !watchSync && !watchHealth && !watchOperations && !watchSuspended {
				watchSync = true
				watchHealth = true
				watchOperations = true
				watchSuspended = false
			}
			selectedResources := parseSelectedResources(resources)
			appNames := args
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			closer, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(closer)
			if selector != "" {
				list, err := appIf.List(context.Background(), &applicationpkg.ApplicationQuery{Selector: selector})
				errors.CheckError(err)
				for _, i := range list.Items {
					appNames = append(appNames, i.Name)
				}
			}
			for _, appName := range appNames {
				_, err := waitOnApplicationStatus(acdClient, appName, timeout, watchSync, watchHealth, watchOperations, watchSuspended, selectedResources)
				errors.CheckError(err)
			}
		},
	}
	command.Flags().BoolVar(&watchSync, "sync", false, "Wait for sync")
	command.Flags().BoolVar(&watchHealth, "health", false, "Wait for health")
	command.Flags().BoolVar(&watchSuspended, "suspended", false, "Wait for suspended")
	command.Flags().StringVarP(&selector, "selector", "l", "", "Wait for apps by label")
	command.Flags().StringArrayVar(&resources, "resource", []string{}, fmt.Sprintf("Sync only specific resources as GROUP%sKIND%sNAME. Fields may be blank. This option may be specified repeatedly", resourceFieldDelimiter, resourceFieldDelimiter))
	command.Flags().BoolVar(&watchOperations, "operation", false, "Wait for pending operations")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	return command
}

// printAppResources prints the resources of an application in a tabwriter table
func printAppResources(w io.Writer, app *argoappv1.Application) {
	_, _ = fmt.Fprintf(w, "GROUP\tKIND\tNAMESPACE\tNAME\tSTATUS\tHEALTH\tHOOK\tMESSAGE\n")
	for _, res := range getResourceStates(app, nil) {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", res.Group, res.Kind, res.Namespace, res.Name, res.Status, res.Health, res.Hook, res.Message)
	}
}

// NewApplicationSyncCommand returns a new instance of an `argocd app sync` command
func NewApplicationSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		revision                string
		resources               []string
		labels                  []string
		selector                string
		prune                   bool
		dryRun                  bool
		timeout                 uint
		strategy                string
		force                   bool
		replace                 bool
		async                   bool
		retryLimit              int64
		retryBackoffDuration    time.Duration
		retryBackoffMaxDuration time.Duration
		retryBackoffFactor      int64
		local                   string
		localRepoRoot           string
		infos                   []string
	)
	var command = &cobra.Command{
		Use:   "sync [APPNAME... | -l selector]",
		Short: "Sync an application to its target state",
		Example: `  # Sync an app
  argocd app sync my-app

  # Sync multiples apps
  argocd app sync my-app other-app

  # Sync apps by label, in this example we sync apps that are children of another app (aka app-of-apps)
  argocd app sync -l app.kubernetes.io/instance=my-app

  # Sync a specific resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME
  argocd app sync my-app --resource :Service:my-service
  argocd app sync my-app --resource argoproj.io:Rollout:my-rollout
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app sync my-app --resource argoproj.io:Rollout:my-namespace/my-rollout`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 && selector == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)

			selectedLabels, err := label.Parse(labels)
			errors.CheckError(err)

			appNames := args
			if selector != "" {
				list, err := appIf.List(context.Background(), &applicationpkg.ApplicationQuery{Selector: selector})
				errors.CheckError(err)
				// unlike list, we'd want to fail if nothing was found
				if len(list.Items) == 0 {
					log.Fatalf("no apps match selector %v", selector)
				}
				for _, i := range list.Items {
					appNames = append(appNames, i.Name)
				}
			}

			for _, appName := range appNames {

				if len(selectedLabels) > 0 {
					ctx := context.Background()

					q := applicationpkg.ApplicationManifestQuery{
						Name:     &appName,
						Revision: revision,
					}

					res, err := appIf.GetManifests(ctx, &q)
					if err != nil {
						log.Fatal(err)
					}

					for _, mfst := range res.Manifests {
						obj, err := argoappv1.UnmarshalToUnstructured(mfst)
						errors.CheckError(err)
						for key, selectedValue := range selectedLabels {
							if objectValue, ok := obj.GetLabels()[key]; ok && selectedValue == objectValue {
								gvk := obj.GroupVersionKind()
								resources = append(resources, fmt.Sprintf("%s:%s:%s", gvk.Group, gvk.Kind, obj.GetName()))
							}
						}
					}

					// If labels are provided and none are found return error only if specific resources were also not
					// specified.
					if len(resources) == 0 {
						log.Fatalf("No matching resources found for labels: %v", labels)
						return
					}
				}

				selectedResources := parseSelectedResources(resources)

				var localObjsStrings []string
				if local != "" {
					app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
					errors.CheckError(err)
					if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil && !dryRun {
						log.Fatal("Cannot use local sync when Automatic Sync Policy is enabled except with --dry-run")
					}

					errors.CheckError(err)
					conn, settingsIf := acdClient.NewSettingsClientOrDie()
					argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
					errors.CheckError(err)
					argoio.Close(conn)

					conn, clusterIf := acdClient.NewClusterClientOrDie()
					defer argoio.Close(conn)
					cluster, err := clusterIf.Get(context.Background(), &clusterpkg.ClusterQuery{Name: app.Spec.Destination.Name, Server: app.Spec.Destination.Server})
					errors.CheckError(err)
					argoio.Close(conn)
					localObjsStrings = getLocalObjectsString(app, local, localRepoRoot, argoSettings.AppLabelKey, cluster.ServerVersion, argoSettings.KustomizeOptions, argoSettings.ConfigManagementPlugins, argoSettings.TrackingMethod)
				}

				syncOptionsFactory := func() *applicationpkg.SyncOptions {
					syncOptions := applicationpkg.SyncOptions{}
					items := make([]string, 0)
					if replace {
						items = append(items, common.SyncOptionReplace)
					}

					if len(items) == 0 {
						// for prevent send even empty array if not need
						return nil
					}
					syncOptions.Items = items
					return &syncOptions
				}

				syncReq := applicationpkg.ApplicationSyncRequest{
					Name:        &appName,
					DryRun:      dryRun,
					Revision:    revision,
					Resources:   selectedResources,
					Prune:       prune,
					Manifests:   localObjsStrings,
					Infos:       getInfos(infos),
					SyncOptions: syncOptionsFactory(),
				}

				switch strategy {
				case "apply":
					syncReq.Strategy = &argoappv1.SyncStrategy{Apply: &argoappv1.SyncStrategyApply{}}
					syncReq.Strategy.Apply.Force = force
				case "", "hook":
					syncReq.Strategy = &argoappv1.SyncStrategy{Hook: &argoappv1.SyncStrategyHook{}}
					syncReq.Strategy.Hook.Force = force
				default:
					log.Fatalf("Unknown sync strategy: '%s'", strategy)
				}
				if retryLimit > 0 {
					syncReq.RetryStrategy = &argoappv1.RetryStrategy{
						Limit: retryLimit,
						Backoff: &argoappv1.Backoff{
							Duration:    retryBackoffDuration.String(),
							MaxDuration: retryBackoffMaxDuration.String(),
							Factor:      pointer.Int64Ptr(retryBackoffFactor),
						},
					}
				}
				ctx := context.Background()
				_, err := appIf.Sync(ctx, &syncReq)
				errors.CheckError(err)

				if !async {
					app, err := waitOnApplicationStatus(acdClient, appName, timeout, false, false, true, false, selectedResources)
					errors.CheckError(err)

					if !dryRun {
						if !app.Status.OperationState.Phase.Successful() {
							log.Fatalf("Operation has completed with phase: %s", app.Status.OperationState.Phase)
						} else if len(selectedResources) == 0 && app.Status.Sync.Status != argoappv1.SyncStatusCodeSynced {
							// Only get resources to be pruned if sync was application-wide and final status is not synced
							pruningRequired := app.Status.OperationState.SyncResult.Resources.PruningRequired()
							if pruningRequired > 0 {
								log.Fatalf("%d resources require pruning", pruningRequired)
							}
						}
					}
				}
			}
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview apply without affecting cluster")
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	command.Flags().StringVar(&revision, "revision", "", "Sync to a specific revision. Preserves parameter overrides")
	command.Flags().StringArrayVar(&resources, "resource", []string{}, fmt.Sprintf("Sync only specific resources as GROUP%sKIND%sNAME. Fields may be blank. This option may be specified repeatedly", resourceFieldDelimiter, resourceFieldDelimiter))
	command.Flags().StringVarP(&selector, "selector", "l", "", "Sync apps that match this label")
	command.Flags().StringArrayVar(&labels, "label", []string{}, "Sync only specific resources with a label. This option may be specified repeatedly.")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	command.Flags().Int64Var(&retryLimit, "retry-limit", 0, "Max number of allowed sync retries")
	command.Flags().DurationVar(&retryBackoffDuration, "retry-backoff-duration", argoappv1.DefaultSyncRetryDuration, "Retry backoff base duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().DurationVar(&retryBackoffMaxDuration, "retry-backoff-max-duration", argoappv1.DefaultSyncRetryMaxDuration, "Max retry backoff duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().Int64Var(&retryBackoffFactor, "retry-backoff-factor", argoappv1.DefaultSyncRetryFactor, "Factor multiplies the base duration after each failed retry")
	command.Flags().StringVar(&strategy, "strategy", "", "Sync strategy (one of: apply|hook)")
	command.Flags().BoolVar(&force, "force", false, "Use a force apply")
	command.Flags().BoolVar(&replace, "replace", false, "Use a kubectl create/replace instead apply")
	command.Flags().BoolVar(&async, "async", false, "Do not wait for application to sync before continuing")
	command.Flags().StringVar(&local, "local", "", "Path to a local directory. When this flag is present no git queries will be made")
	command.Flags().StringVar(&localRepoRoot, "local-repo-root", "/", "Path to the repository root. Used together with --local allows setting the repository root")
	command.Flags().StringArrayVar(&infos, "info", []string{}, "A list of key-value pairs during sync process. These infos will be persisted in app.")
	return command
}

// ResourceDiff tracks the state of a resource when waiting on an application status.
type resourceState struct {
	Group     string
	Kind      string
	Namespace string
	Name      string
	Status    string
	Health    string
	Hook      string
	Message   string
}

// Key returns a unique-ish key for the resource.
func (rs *resourceState) Key() string {
	return fmt.Sprintf("%s/%s/%s/%s", rs.Group, rs.Kind, rs.Namespace, rs.Name)
}

func (rs *resourceState) FormatItems() []interface{} {
	timeStr := time.Now().Format("2006-01-02T15:04:05-07:00")
	return []interface{}{timeStr, rs.Group, rs.Kind, rs.Namespace, rs.Name, rs.Status, rs.Health, rs.Hook, rs.Message}
}

// Merge merges the new state with any different contents from another resourceState.
// Blank fields in the receiver state will be updated to non-blank.
// Non-blank fields in the receiver state will never be updated to blank.
// Returns whether or not any keys were updated.
func (rs *resourceState) Merge(newState *resourceState) bool {
	updated := false
	for _, field := range []string{"Status", "Health", "Hook", "Message"} {
		v := reflect.ValueOf(rs).Elem().FieldByName(field)
		currVal := v.String()
		newVal := reflect.ValueOf(newState).Elem().FieldByName(field).String()
		if newVal != "" && currVal != newVal {
			v.SetString(newVal)
			updated = true
		}
	}
	return updated
}

func getResourceStates(app *argoappv1.Application, selectedResources []argoappv1.SyncOperationResource) []*resourceState {
	var states []*resourceState
	resourceByKey := make(map[kube.ResourceKey]argoappv1.ResourceStatus)
	for i := range app.Status.Resources {
		res := app.Status.Resources[i]
		resourceByKey[kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name)] = res
	}

	// print most resources info along with most recent operation results
	if app.Status.OperationState != nil && app.Status.OperationState.SyncResult != nil {
		for _, res := range app.Status.OperationState.SyncResult.Resources {
			sync := string(res.HookPhase)
			health := string(res.Status)
			key := kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name)
			if resource, ok := resourceByKey[key]; ok && res.HookType == "" {
				health = ""
				if resource.Health != nil {
					health = string(resource.Health.Status)
				}
				sync = string(resource.Status)
			}
			states = append(states, &resourceState{
				Group: res.Group, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name, Status: sync, Health: health, Hook: string(res.HookType), Message: res.Message})
			delete(resourceByKey, kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name))
		}
	}
	resKeys := make([]kube.ResourceKey, 0)
	for k := range resourceByKey {
		resKeys = append(resKeys, k)
	}
	sort.Slice(resKeys, func(i, j int) bool {
		return resKeys[i].String() < resKeys[j].String()
	})
	// print rest of resources which were not part of most recent operation
	for _, resKey := range resKeys {
		res := resourceByKey[resKey]
		health := ""
		if res.Health != nil {
			health = string(res.Health.Status)
		}
		states = append(states, &resourceState{
			Group: res.Group, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name, Status: string(res.Status), Health: health, Hook: "", Message: ""})
	}
	// filter out not selected resources
	if len(selectedResources) > 0 {
		for i := len(states) - 1; i >= 0; i-- {
			res := states[i]
			if !argo.ContainsSyncResource(res.Name, res.Namespace, schema.GroupVersionKind{Group: res.Group, Kind: res.Kind}, selectedResources) {
				states = append(states[:i], states[i+1:]...)
			}
		}
	}
	return states
}

func groupResourceStates(app *argoappv1.Application, selectedResources []argoappv1.SyncOperationResource) map[string]*resourceState {
	resStates := make(map[string]*resourceState)
	for _, result := range getResourceStates(app, selectedResources) {
		key := result.Key()
		if prev, ok := resStates[key]; ok {
			prev.Merge(result)
		} else {
			resStates[key] = result
		}
	}
	return resStates
}

func checkResourceStatus(watchSync bool, watchHealth bool, watchOperation bool, watchSuspended bool, healthStatus string, syncStatus string, operationStatus *argoappv1.Operation) bool {
	healthCheckPassed := true
	if watchSuspended && watchHealth {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy) ||
			healthStatus == string(health.HealthStatusSuspended)
	} else if watchSuspended {
		healthCheckPassed = healthStatus == string(health.HealthStatusSuspended)
	} else if watchHealth {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy)
	}

	synced := !watchSync || syncStatus == string(argoappv1.SyncStatusCodeSynced)
	operational := !watchOperation || operationStatus == nil
	return synced && healthCheckPassed && operational
}

const waitFormatString = "%s\t%5s\t%10s\t%10s\t%20s\t%8s\t%7s\t%10s\t%s\n"

func waitOnApplicationStatus(acdClient apiclient.Client, appName string, timeout uint, watchSync bool, watchHealth bool, watchOperation bool, watchSuspended bool, selectedResources []argoappv1.SyncOperationResource) (*argoappv1.Application, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// refresh controls whether or not we refresh the app before printing the final status.
	// We only want to do this when an operation is in progress, since operations are the only
	// time when the sync status lags behind when an operation completes
	refresh := false

	printFinalStatus := func(app *argoappv1.Application) *argoappv1.Application {
		var err error
		if refresh {
			conn, appClient := acdClient.NewApplicationClientOrDie()
			refreshType := string(argoappv1.RefreshTypeNormal)
			app, err = appClient.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: &refreshType})
			errors.CheckError(err)
			_ = conn.Close()
		}

		fmt.Println()
		printAppSummaryTable(app, appURL(acdClient, appName), nil)
		fmt.Println()
		if watchOperation {
			printOperationResult(app.Status.OperationState)
		}

		if len(app.Status.Resources) > 0 {
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 5, 0, 2, ' ', 0)
			printAppResources(w, app)
			_ = w.Flush()
		}
		return app
	}

	if timeout != 0 {
		time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			cancel()
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, waitFormatString, "TIMESTAMP", "GROUP", "KIND", "NAMESPACE", "NAME", "STATUS", "HEALTH", "HOOK", "MESSAGE")

	prevStates := make(map[string]*resourceState)
	conn, appClient := acdClient.NewApplicationClientOrDie()
	defer argoio.Close(conn)
	app, err := appClient.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
	errors.CheckError(err)
	appEventCh := acdClient.WatchApplicationWithRetry(ctx, appName, app.ResourceVersion)
	for appEvent := range appEventCh {
		app = &appEvent.Application

		operationInProgress := false
		// consider the operation is in progress
		if app.Operation != nil {
			// if it just got requested
			operationInProgress = true
			if !app.Operation.DryRun() {
				refresh = true
			}
		} else if app.Status.OperationState != nil {
			if app.Status.OperationState.FinishedAt == nil {
				// if it is not finished yet
				operationInProgress = true
			} else if !app.Status.OperationState.Operation.DryRun() && (app.Status.ReconciledAt == nil || app.Status.ReconciledAt.Before(app.Status.OperationState.FinishedAt)) {
				// if it is just finished and we need to wait for controller to reconcile app once after syncing
				operationInProgress = true
			}
		}

		var selectedResourcesAreReady bool

		// If selected resources are included, wait only on those resources, otherwise wait on the application as a whole.
		if len(selectedResources) > 0 {
			selectedResourcesAreReady = true
			for _, state := range getResourceStates(app, selectedResources) {
				resourceIsReady := checkResourceStatus(watchSync, watchHealth, watchOperation, watchSuspended, state.Health, state.Status, appEvent.Application.Operation)
				if !resourceIsReady {
					selectedResourcesAreReady = false
					break
				}
			}
		} else {
			// Wait on the application as a whole
			selectedResourcesAreReady = checkResourceStatus(watchSync, watchHealth, watchOperation, watchSuspended, string(app.Status.Health.Status), string(app.Status.Sync.Status), appEvent.Application.Operation)
		}

		if selectedResourcesAreReady && (!operationInProgress || !watchOperation) {
			app = printFinalStatus(app)
			return app, nil
		}

		newStates := groupResourceStates(app, selectedResources)
		for _, newState := range newStates {
			var doPrint bool
			stateKey := newState.Key()
			if prevState, found := prevStates[stateKey]; found {
				if watchHealth && prevState.Health != string(health.HealthStatusUnknown) && prevState.Health != string(health.HealthStatusDegraded) && newState.Health == string(health.HealthStatusDegraded) {
					_ = printFinalStatus(app)
					return nil, fmt.Errorf("application '%s' health state has transitioned from %s to %s", appName, prevState.Health, newState.Health)
				}
				doPrint = prevState.Merge(newState)
			} else {
				prevStates[stateKey] = newState
				doPrint = true
			}
			if doPrint {
				_, _ = fmt.Fprintf(w, waitFormatString, prevStates[stateKey].FormatItems()...)
			}
		}
		_ = w.Flush()
	}
	_ = printFinalStatus(app)
	return nil, fmt.Errorf("timed out (%ds) waiting for app %q match desired state", timeout, appName)
}

// setParameterOverrides updates an existing or appends a new parameter override in the application
// If the app is a ksonnet app, then parameters are expected to be in the form: component=param=value
// Otherwise, the app is assumed to be a helm app and is expected to be in the form:
// param=value
func setParameterOverrides(app *argoappv1.Application, parameters []string) {
	if len(parameters) == 0 {
		return
	}
	var sourceType argoappv1.ApplicationSourceType
	if st, _ := app.Spec.Source.ExplicitType(); st != nil {
		sourceType = *st
	} else if app.Status.SourceType != "" {
		sourceType = app.Status.SourceType
	} else {
		// HACK: we don't know the source type, so make an educated guess based on the supplied
		// parameter string. This code handles the corner case where app doesn't exist yet, and the
		// command is something like: `argocd app create MYAPP -p foo=bar`
		// This logic is not foolproof, but when ksonnet is deprecated, this will no longer matter
		// since helm will remain as the only source type which has parameters.
		if len(strings.SplitN(parameters[0], "=", 3)) == 3 {
			sourceType = argoappv1.ApplicationSourceTypeKsonnet
		} else if len(strings.SplitN(parameters[0], "=", 2)) == 2 {
			sourceType = argoappv1.ApplicationSourceTypeHelm
		}
	}

	switch sourceType {
	case argoappv1.ApplicationSourceTypeKsonnet:
		if app.Spec.Source.Ksonnet == nil {
			app.Spec.Source.Ksonnet = &argoappv1.ApplicationSourceKsonnet{}
		}
		for _, paramStr := range parameters {
			parts := strings.SplitN(paramStr, "=", 3)
			if len(parts) != 3 {
				log.Fatalf("Expected ksonnet parameter of the form: component=param=value. Received: %s", paramStr)
			}
			newParam := argoappv1.KsonnetParameter{
				Component: parts[0],
				Name:      parts[1],
				Value:     parts[2],
			}
			found := false
			for i, cp := range app.Spec.Source.Ksonnet.Parameters {
				if cp.Component == newParam.Component && cp.Name == newParam.Name {
					found = true
					app.Spec.Source.Ksonnet.Parameters[i] = newParam
					break
				}
			}
			if !found {
				app.Spec.Source.Ksonnet.Parameters = append(app.Spec.Source.Ksonnet.Parameters, newParam)
			}
		}
	case argoappv1.ApplicationSourceTypeHelm:
		if app.Spec.Source.Helm == nil {
			app.Spec.Source.Helm = &argoappv1.ApplicationSourceHelm{}
		}
		for _, p := range parameters {
			newParam, err := argoappv1.NewHelmParameter(p, false)
			if err != nil {
				log.Error(err)
				continue
			}
			app.Spec.Source.Helm.AddParameter(*newParam)
		}
	default:
		log.Fatalf("Parameters can only be set against Ksonnet or Helm applications")
	}
}

// Print list of history ID's for an application.
func printApplicationHistoryIds(revHistory []argoappv1.RevisionHistory) {
	for _, depInfo := range revHistory {
		fmt.Println(depInfo.ID)
	}
}

// Print a history table for an application.
func printApplicationHistoryTable(revHistory []argoappv1.RevisionHistory) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, "ID\tDATE\tREVISION\n")
	for _, depInfo := range revHistory {
		rev := depInfo.Source.TargetRevision
		if len(depInfo.Revision) >= 7 {
			rev = fmt.Sprintf("%s (%s)", rev, depInfo.Revision[0:7])
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\n", depInfo.ID, depInfo.DeployedAt, rev)
	}
	_ = w.Flush()
}

// NewApplicationHistoryCommand returns a new instance of an `argocd app history` command
func NewApplicationHistoryCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "history APPNAME",
		Short: "Show application deployment history",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			if output == "id" {
				printApplicationHistoryIds(app.Status.History)
			} else {
				printApplicationHistoryTable(app.Status.History)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|id")
	return command
}

func findRevisionHistory(application *argoappv1.Application, historyId int64) (*argoappv1.RevisionHistory, error) {
	// in case if history id not passed and need fetch previous history revision
	if historyId == -1 {
		l := len(application.Status.History)
		if l < 2 {
			return nil, fmt.Errorf("Application '%s' should have at least two successful deployments", application.ObjectMeta.Name)
		}
		return &application.Status.History[l-2], nil
	}
	for _, di := range application.Status.History {
		if di.ID == historyId {
			return &di, nil
		}
	}
	return nil, fmt.Errorf("Application '%s' does not have deployment id '%d' in history\n", application.ObjectMeta.Name, historyId)
}

// NewApplicationRollbackCommand returns a new instance of an `argocd app rollback` command
func NewApplicationRollbackCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		prune   bool
		timeout uint
	)
	var command = &cobra.Command{
		Use:   "rollback APPNAME [ID]",
		Short: "Rollback application to a previous deployed version by History ID, omitted will Rollback to the previous version",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			var err error
			depID := -1
			if len(args) > 1 {
				depID, err = strconv.Atoi(args[1])
				errors.CheckError(err)
			}
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			ctx := context.Background()
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)

			depInfo, err := findRevisionHistory(app, int64(depID))
			errors.CheckError(err)

			_, err = appIf.Rollback(ctx, &applicationpkg.ApplicationRollbackRequest{
				Name:  &appName,
				ID:    depInfo.ID,
				Prune: prune,
			})
			errors.CheckError(err)

			_, err = waitOnApplicationStatus(acdClient, appName, timeout, false, false, true, false, nil)
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	return command
}

const printOpFmtStr = "%-20s%s\n"
const defaultCheckTimeoutSeconds = 0

func printOperationResult(opState *argoappv1.OperationState) {
	if opState == nil {
		return
	}
	if opState.SyncResult != nil {
		fmt.Printf(printOpFmtStr, "Operation:", "Sync")
		fmt.Printf(printOpFmtStr, "Sync Revision:", opState.SyncResult.Revision)
	}
	fmt.Printf(printOpFmtStr, "Phase:", opState.Phase)
	fmt.Printf(printOpFmtStr, "Start:", opState.StartedAt)
	fmt.Printf(printOpFmtStr, "Finished:", opState.FinishedAt)
	var duration time.Duration
	if !opState.FinishedAt.IsZero() {
		duration = time.Second * time.Duration(opState.FinishedAt.Unix()-opState.StartedAt.Unix())
	} else {
		duration = time.Second * time.Duration(time.Now().UTC().Unix()-opState.StartedAt.Unix())
	}
	fmt.Printf(printOpFmtStr, "Duration:", duration)
	if opState.Message != "" {
		fmt.Printf(printOpFmtStr, "Message:", opState.Message)
	}
}

// NewApplicationManifestsCommand returns a new instance of an `argocd app manifests` command
func NewApplicationManifestsCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		source   string
		revision string
	)
	var command = &cobra.Command{
		Use:   "manifests APPNAME",
		Short: "Print manifests of an application",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			ctx := context.Background()
			resources, err := appIf.ManagedResources(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &appName})
			errors.CheckError(err)

			var unstructureds []*unstructured.Unstructured
			switch source {
			case "git":
				if revision != "" {
					q := applicationpkg.ApplicationManifestQuery{
						Name:     &appName,
						Revision: revision,
					}
					res, err := appIf.GetManifests(ctx, &q)
					errors.CheckError(err)
					for _, mfst := range res.Manifests {
						obj, err := argoappv1.UnmarshalToUnstructured(mfst)
						errors.CheckError(err)
						unstructureds = append(unstructureds, obj)
					}
				} else {
					targetObjs, err := targetObjects(resources.Items)
					errors.CheckError(err)
					unstructureds = targetObjs
				}
			case "live":
				liveObjs, err := liveObjects(resources.Items)
				errors.CheckError(err)
				unstructureds = liveObjs
			default:
				log.Fatalf("Unknown source type '%s'", source)
			}

			for _, obj := range unstructureds {
				fmt.Println("---")
				yamlBytes, err := yaml.Marshal(obj)
				errors.CheckError(err)
				fmt.Printf("%s\n", yamlBytes)
			}
		},
	}
	command.Flags().StringVar(&source, "source", "git", "Source of manifests. One of: live|git")
	command.Flags().StringVar(&revision, "revision", "", "Show manifests at a specific revision")
	return command
}

// NewApplicationTerminateOpCommand returns a new instance of an `argocd app terminate-op` command
func NewApplicationTerminateOpCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "terminate-op APPNAME",
		Short: "Terminate running operation of an application",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			ctx := context.Background()
			_, err := appIf.TerminateOperation(ctx, &applicationpkg.OperationTerminateRequest{Name: &appName})
			errors.CheckError(err)
			fmt.Printf("Application '%s' operation terminating\n", appName)
		},
	}
	return command
}

func NewApplicationEditCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "edit APPNAME",
		Short: "Edit application",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			appData, err := json.Marshal(app.Spec)
			errors.CheckError(err)
			appData, err = yaml.JSONToYAML(appData)
			errors.CheckError(err)

			cli.InteractiveEdit(fmt.Sprintf("%s-*-edit.yaml", appName), appData, func(input []byte) error {
				input, err = yaml.YAMLToJSON(input)
				if err != nil {
					return err
				}
				updatedSpec := argoappv1.ApplicationSpec{}
				err = json.Unmarshal(input, &updatedSpec)
				if err != nil {
					return err
				}

				var appOpts cmdutil.AppOptions
				cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
				_, err = appIf.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: updatedSpec, Validate: &appOpts.Validate})
				if err != nil {
					return fmt.Errorf("Failed to update application spec:\n%v", err)
				}
				return err
			})
		},
	}
	return command
}

func NewApplicationListResourcesCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var orphaned bool
	var command = &cobra.Command{
		Use:   "resources APPNAME",
		Short: "List resource of application",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			listAll := !c.Flag("orphaned").Changed
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appResourceTree, err := appIf.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &appName})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			headers := []interface{}{"GROUP", "KIND", "NAMESPACE", "NAME", "ORPHANED"}
			fmtStr := "%s\t%s\t%s\t%s\t%s\n"
			_, _ = fmt.Fprintf(w, fmtStr, headers...)
			if !orphaned || listAll {
				for _, res := range appResourceTree.Nodes {
					if len(res.ParentRefs) == 0 {
						_, _ = fmt.Fprintf(w, fmtStr, res.Group, res.Kind, res.Namespace, res.Name, "No")
					}
				}
			}
			if orphaned || listAll {
				for _, res := range appResourceTree.OrphanedNodes {
					_, _ = fmt.Fprintf(w, fmtStr, res.Group, res.Kind, res.Namespace, res.Name, "Yes")
				}
			}
			_ = w.Flush()
		},
	}
	command.Flags().BoolVar(&orphaned, "orphaned", false, "Lists only orphaned resources")
	return command
}

func NewApplicationPatchCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var patch string
	var patchType string

	command := cobra.Command{
		Use:   "patch APPNAME",
		Short: "Patch application",
		Long: `Examples:
	# Update an application's source path using json patch
	argocd app patch myapplication --patch='[{"op": "replace", "path": "/spec/source/path", "value": "newPath"}]' --type json

	# Update an application's repository target revision using merge patch
	argocd app patch myapplication --patch '{"spec": { "source": { "targetRevision": "master" } }}' --type merge`,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer argoio.Close(conn)

			patchedApp, err := appIf.Patch(context.Background(), &applicationpkg.ApplicationPatchRequest{
				Name:      &appName,
				Patch:     patch,
				PatchType: patchType,
			})
			errors.CheckError(err)

			yamlBytes, err := yaml.Marshal(patchedApp)
			errors.CheckError(err)

			fmt.Println(string(yamlBytes))
		},
	}

	command.Flags().StringVar(&patch, "patch", "", "Patch body")
	command.Flags().StringVar(&patchType, "type", "json", "The type of patch being provided; one of [json merge]")
	return &command
}

func filterResources(command *cobra.Command, resources []*argoappv1.ResourceDiff, group, kind, namespace, resourceName string, all bool) []*unstructured.Unstructured {
	liveObjs, err := liveObjects(resources)
	errors.CheckError(err)
	filteredObjects := make([]*unstructured.Unstructured, 0)
	for i := range liveObjs {
		obj := liveObjs[i]
		if obj == nil {
			continue
		}
		gvk := obj.GroupVersionKind()
		if command.Flags().Changed("group") && group != gvk.Group {
			continue
		}
		if namespace != "" && namespace != obj.GetNamespace() {
			continue
		}
		if resourceName != "" && resourceName != obj.GetName() {
			continue
		}
		if kind != "" && kind != gvk.Kind {
			continue
		}
		deepCopy := obj.DeepCopy()
		filteredObjects = append(filteredObjects, deepCopy)
	}
	if len(filteredObjects) == 0 {
		log.Fatal("No matching resource found")
	}
	if len(filteredObjects) > 1 && !all {
		log.Fatal("Multiple resources match inputs. Use the --all flag to patch multiple resources")
	}
	return filteredObjects
}

func NewApplicationPatchResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var patch string
	var patchType string
	var resourceName string
	var namespace string
	var kind string
	var group string
	var all bool
	command := &cobra.Command{
		Use:   "patch-resource APPNAME",
		Short: "Patch resource in an application",
	}

	command.Flags().StringVar(&patch, "patch", "", "Patch")
	err := command.MarkFlagRequired("patch")
	errors.CheckError(err)
	command.Flags().StringVar(&patchType, "patch-type", string(types.MergePatchType), "Which Patching strategy to use: 'application/json-patch+json', 'application/merge-patch+json', or 'application/strategic-merge-patch+json'. Defaults to 'application/merge-patch+json'")
	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	err = command.MarkFlagRequired("kind")
	errors.CheckError(err)
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to patch multiple matching of resources")
	command.Run = func(c *cobra.Command, args []string) {
		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName := args[0]

		conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
		defer argoio.Close(conn)
		ctx := context.Background()
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{ApplicationName: &appName})
		errors.CheckError(err)
		objectsToPatch := filterResources(command, resources.Items, group, kind, namespace, resourceName, all)
		for i := range objectsToPatch {
			obj := objectsToPatch[i]
			gvk := obj.GroupVersionKind()
			_, err = appIf.PatchResource(ctx, &applicationpkg.ApplicationResourcePatchRequest{
				Name:         &appName,
				Namespace:    obj.GetNamespace(),
				ResourceName: obj.GetName(),
				Version:      gvk.Version,
				Group:        gvk.Group,
				Kind:         gvk.Kind,
				Patch:        patch,
				PatchType:    patchType,
			})
			errors.CheckError(err)
			log.Infof("Resource '%s' patched", obj.GetName())
		}
	}

	return command
}

func NewApplicationDeleteResourceCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var resourceName string
	var namespace string
	var kind string
	var group string
	var force bool
	var orphan bool
	var all bool
	command := &cobra.Command{
		Use:   "delete-resource APPNAME",
		Short: "Delete resource in an application",
	}

	command.Flags().StringVar(&resourceName, "resource-name", "", "Name of resource")
	command.Flags().StringVar(&kind, "kind", "", "Kind")
	err := command.MarkFlagRequired("kind")
	errors.CheckError(err)
	command.Flags().StringVar(&group, "group", "", "Group")
	command.Flags().StringVar(&namespace, "namespace", "", "Namespace")
	command.Flags().BoolVar(&force, "force", false, "Indicates whether to orphan the dependents of the deleted resource")
	command.Flags().BoolVar(&orphan, "orphan", false, "Indicates whether to force delete the resource")
	command.Flags().BoolVar(&all, "all", false, "Indicates whether to patch multiple matching of resources")
	command.Run = func(c *cobra.Command, args []string) {
		if len(args) != 1 {
			c.HelpFunc()(c, args)
			os.Exit(1)
		}
		appName := args[0]

		conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
		defer argoio.Close(conn)
		ctx := context.Background()
		resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{ApplicationName: &appName})
		errors.CheckError(err)
		objectsToDelete := filterResources(command, resources.Items, group, kind, namespace, resourceName, all)
		for i := range objectsToDelete {
			obj := objectsToDelete[i]
			gvk := obj.GroupVersionKind()
			_, err = appIf.DeleteResource(ctx, &applicationpkg.ApplicationResourceDeleteRequest{
				Name:         &appName,
				Namespace:    obj.GetNamespace(),
				ResourceName: obj.GetName(),
				Version:      gvk.Version,
				Group:        gvk.Group,
				Kind:         gvk.Kind,
				Force:        &force,
				Orphan:       &orphan,
			})
			errors.CheckError(err)
			log.Infof("Resource '%s' deleted", obj.GetName())
		}
	}

	return command
}
