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

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/gitops-engine/pkg/sync/ignore"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/cmd/argocd/commands/headless"
	cmdutil "github.com/argoproj/argo-cd/v2/cmd/util"
	"github.com/argoproj/argo-cd/v2/controller"
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
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/grpc"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/manifeststream"
	"github.com/argoproj/argo-cd/v2/util/text/label"
)

// NewApplicationCommand returns a new instance of an `argocd app` command
func NewApplicationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "app",
		Short: "Manage applications",
		Example: `  # List all the applications.
  argocd app list

  # Get the details of a application
  argocd app get my-app

  # Set an override parameter
  argocd app set my-app -p image.tag=v1.0.1`,
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

type watchOpts struct {
	sync      bool
	health    bool
	operation bool
	suspended bool
	degraded  bool
}

// NewApplicationCreateCommand returns a new instance of an `argocd app create` command
func NewApplicationCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts      cmdutil.AppOptions
		fileURL      string
		appName      string
		upsert       bool
		labels       []string
		annotations  []string
		setFinalizer bool
		appNamespace string
	)
	var command = &cobra.Command{
		Use:   "create APPNAME",
		Short: "Create an application",
		Example: `  # Create a directory app
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
  argocd app create kasane --repo https://github.com/argoproj/argocd-example-apps.git --path plugins/kasane --dest-namespace default --dest-server https://kubernetes.default.svc --config-management-plugin kasane`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			argocdClient := headless.NewClientOrDie(clientOpts, c)

			apps, err := cmdutil.ConstructApps(fileURL, appName, labels, annotations, args, appOpts, c.Flags())
			errors.CheckError(err)

			for _, app := range apps {
				if app.Name == "" {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				if appNamespace != "" {
					app.Namespace = appNamespace
				}
				if setFinalizer {
					app.Finalizers = append(app.Finalizers, "resources-finalizer.argocd.argoproj.io")
				}
				conn, appIf := argocdClient.NewApplicationClientOrDie()
				defer argoio.Close(conn)
				appCreateRequest := applicationpkg.ApplicationCreateRequest{
					Application: app,
					Upsert:      &upsert,
					Validate:    &appOpts.Validate,
				}

				// Get app before creating to see if it is being updated or no change
				existing, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &app.Name})
				unwrappedError := grpc.UnwrapGRPCStatus(err).Code()
				// As part of the fix for CVE-2022-41354, the API will return Permission Denied when an app does not exist.
				if unwrappedError != codes.NotFound && unwrappedError != codes.PermissionDenied {
					errors.CheckError(err)
				}

				created, err := appIf.Create(ctx, &appCreateRequest)
				errors.CheckError(err)

				var action string
				if existing == nil {
					action = "created"
				} else if !hasAppChanged(existing, created, upsert) {
					action = "unchanged"
				} else {
					action = "updated"
				}

				fmt.Printf("application '%s' %s\n", created.ObjectMeta.Name, action)
			}
		},
	}
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override application with the same name even if supplied application spec is different from existing spec")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringArrayVarP(&labels, "label", "l", []string{}, "Labels to apply to the app")
	command.Flags().StringArrayVarP(&annotations, "annotations", "", []string{}, "Set metadata annotations (e.g. example=value)")
	command.Flags().BoolVar(&setFinalizer, "set-finalizer", false, "Sets deletion finalizer on the application, application resources will be cascaded on deletion")
	// Only complete files with appropriate extension.
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	if err != nil {
		log.Fatal(err)
	}
	command.Flags().StringVarP(&appNamespace, "app-namespace", "N", "", "Namespace where the application will be created in")
	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

// getInfos converts a list of string key=value pairs to a list of Info objects.
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

func hasAppChanged(appReq, appRes *argoappv1.Application, upsert bool) bool {
	// upsert==false, no change occurred from create command
	if !upsert {
		return false
	}

	// If no project, assume default project
	if appReq.Spec.Project == "" {
		appReq.Spec.Project = "default"
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
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)

			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appName,
				Refresh:      getRefreshType(refresh, hardRefresh),
				AppNamespace: &appNs,
			})
			errors.CheckError(err)

			pConn, projIf := headless.NewClientOrDie(clientOpts, c).NewProjectClientOrDie()
			defer argoio.Close(pConn)
			proj, err := projIf.Get(ctx, &projectpkg.ProjectQuery{Name: app.Spec.Project})
			errors.CheckError(err)

			windows := proj.Spec.SyncWindows.Matches(app)

			switch output {
			case "yaml", "json":
				err := PrintResource(app, output)
				errors.CheckError(err)
			case "wide", "":
				aURL := appURL(ctx, acdClient, app.Name)
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
		previous     bool
	)
	var command = &cobra.Command{
		Use:   "logs APPNAME",
		Short: "Get logs of application pods",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")

			retry := true
			for retry {
				retry = false
				stream, err := appIf.PodLogs(ctx, &applicationpkg.ApplicationPodLogsQuery{
					Name:         &appName,
					Group:        &group,
					Namespace:    pointer.String(namespace),
					Kind:         &kind,
					ResourceName: &resourceName,
					Follow:       pointer.Bool(follow),
					TailLines:    pointer.Int64(tail),
					SinceSeconds: pointer.Int64(sinceSeconds),
					UntilTime:    &untilTime,
					Filter:       &filter,
					Container:    pointer.String(container),
					Previous:     pointer.Bool(previous),
					AppNamespace: &appNs,
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
					if !msg.GetLast() {
						fmt.Println(msg.GetContent())
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
	command.Flags().BoolVarP(&previous, "previous", "p", false, "Specify if the previously terminated container logs should be returned")

	return command
}

func printAppSummaryTable(app *argoappv1.Application, appURL string, windows *argoappv1.SyncWindows) {
	source := app.Spec.GetSource()
	fmt.Printf(printOpFmtStr, "Name:", app.QualifiedName())
	fmt.Printf(printOpFmtStr, "Project:", app.Spec.GetProject())
	fmt.Printf(printOpFmtStr, "Server:", getServer(app))
	fmt.Printf(printOpFmtStr, "Namespace:", app.Spec.Destination.Namespace)
	fmt.Printf(printOpFmtStr, "URL:", appURL)
	fmt.Printf(printOpFmtStr, "Repo:", source.RepoURL)
	fmt.Printf(printOpFmtStr, "Target:", source.TargetRevision)
	fmt.Printf(printOpFmtStr, "Path:", source.Path)
	printAppSourceDetails(&source)
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
		syncStatusStr += fmt.Sprintf(" to %s", app.Spec.GetSource().TargetRevision)
	case argoappv1.SyncStatusCodeOutOfSync:
		syncStatusStr += fmt.Sprintf(" from %s", app.Spec.GetSource().TargetRevision)
	}
	if !git.IsCommitSHA(app.Spec.GetSource().TargetRevision) && !git.IsTruncatedCommitSHA(app.Spec.GetSource().TargetRevision) && len(app.Status.Sync.Revision) > 7 {
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
func appURL(ctx context.Context, acdClient argocdclient.Client, appName string) string {
	conn, settingsIf := acdClient.NewSettingsClientOrDie()
	defer argoio.Close(conn)
	argoSettings, err := settingsIf.Get(ctx, &settingspkg.SettingsQuery{})
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
	if app.Spec.GetSource().Helm != nil {
		printHelmParams(app.Spec.GetSource().Helm)
	}
}

func printHelmParams(helm *argoappv1.ApplicationSourceHelm) {
	paramLenLimit := 80
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if helm != nil {
		fmt.Println()
		_, _ = fmt.Fprintf(w, "NAME\tVALUE\n")
		for _, p := range helm.Parameters {
			_, _ = fmt.Fprintf(w, "%s\t%s\n", p.Name, truncateString(p.Value, paramLenLimit))
		}
	}
	_ = w.Flush()
}

func getServer(app *argoappv1.Application) string {
	if app.Spec.Destination.Server == "" {
		return app.Spec.Destination.Name
	}

	return app.Spec.Destination.Server
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
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			argocdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := argocdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName, AppNamespace: &appNs})
			errors.CheckError(err)

			visited := cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			setParameterOverrides(app, appOpts.Parameters)
			_, err = appIf.UpdateSpec(ctx, &applicationpkg.ApplicationUpdateSpecRequest{
				Name:         &app.Name,
				Spec:         &app.Spec,
				Validate:     &appOpts.Validate,
				AppNamespace: &appNs,
			})
			errors.CheckError(err)
		},
	}
	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

// unsetOpts describe what to unset in an Application.
type unsetOpts struct {
	namePrefix              bool
	nameSuffix              bool
	kustomizeVersion        bool
	kustomizeNamespace      bool
	kustomizeImages         []string
	kustomizeReplicas       []string
	parameters              []string
	valuesFiles             []string
	valuesLiteral           bool
	ignoreMissingValueFiles bool
	pluginEnvs              []string
	passCredentials         bool
}

// IsZero returns true when the Application options for kustomize are considered empty
func (o *unsetOpts) KustomizeIsZero() bool {
	return o == nil ||
		!o.namePrefix &&
			!o.nameSuffix &&
			!o.kustomizeVersion &&
			!o.kustomizeNamespace &&
			len(o.kustomizeImages) == 0 &&
			len(o.kustomizeReplicas) == 0
}

// NewApplicationUnsetCommand returns a new instance of an `argocd app unset` command
func NewApplicationUnsetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	appOpts := cmdutil.AppOptions{}
	opts := unsetOpts{}
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
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName, AppNamespace: &appNs})
			errors.CheckError(err)

			source := app.Spec.GetSource()
			updated, nothingToUnset := unset(&source, opts)
			if nothingToUnset {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if !updated {
				return
			}

			cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
			_, err = appIf.UpdateSpec(ctx, &applicationpkg.ApplicationUpdateSpecRequest{
				Name:         &app.Name,
				Spec:         &app.Spec,
				Validate:     &appOpts.Validate,
				AppNamespace: &appNs,
			})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVarP(&opts.parameters, "parameter", "p", []string{}, "Unset a parameter override (e.g. -p guestbook=image)")
	command.Flags().StringArrayVar(&opts.valuesFiles, "values", []string{}, "Unset one or more Helm values files")
	command.Flags().BoolVar(&opts.valuesLiteral, "values-literal", false, "Unset literal Helm values block")
	command.Flags().BoolVar(&opts.ignoreMissingValueFiles, "ignore-missing-value-files", false, "Unset the helm ignore-missing-value-files option (revert to false)")
	command.Flags().BoolVar(&opts.nameSuffix, "namesuffix", false, "Kustomize namesuffix")
	command.Flags().BoolVar(&opts.namePrefix, "nameprefix", false, "Kustomize nameprefix")
	command.Flags().BoolVar(&opts.kustomizeVersion, "kustomize-version", false, "Kustomize version")
	command.Flags().BoolVar(&opts.kustomizeNamespace, "kustomize-namespace", false, "Kustomize namespace")
	command.Flags().StringArrayVar(&opts.kustomizeImages, "kustomize-image", []string{}, "Kustomize images name (e.g. --kustomize-image node --kustomize-image mysql)")
	command.Flags().StringArrayVar(&opts.kustomizeReplicas, "kustomize-replica", []string{}, "Kustomize replicas name (e.g. --kustomize-replica my-deployment --kustomize-replica my-statefulset)")
	command.Flags().StringArrayVar(&opts.pluginEnvs, "plugin-env", []string{}, "Unset plugin env variables (e.g --plugin-env name)")
	command.Flags().BoolVar(&opts.passCredentials, "pass-credentials", false, "Unset passCredentials")
	return command
}

func unset(source *argoappv1.ApplicationSource, opts unsetOpts) (updated bool, nothingToUnset bool) {
	if source.Kustomize != nil {
		if opts.KustomizeIsZero() {
			return false, true
		}

		if opts.namePrefix && source.Kustomize.NamePrefix != "" {
			updated = true
			source.Kustomize.NamePrefix = ""
		}

		if opts.nameSuffix && source.Kustomize.NameSuffix != "" {
			updated = true
			source.Kustomize.NameSuffix = ""
		}

		if opts.kustomizeVersion && source.Kustomize.Version != "" {
			updated = true
			source.Kustomize.Version = ""
		}

		if opts.kustomizeNamespace && source.Kustomize.Namespace != "" {
			updated = true
			source.Kustomize.Namespace = ""
		}

		for _, kustomizeImage := range opts.kustomizeImages {
			for i, item := range source.Kustomize.Images {
				if argoappv1.KustomizeImage(kustomizeImage).Match(item) {
					updated = true
					//remove i
					a := source.Kustomize.Images
					copy(a[i:], a[i+1:]) // Shift a[i+1:] left one index.
					a[len(a)-1] = ""     // Erase last element (write zero value).
					a = a[:len(a)-1]     // Truncate slice.
					source.Kustomize.Images = a
				}
			}
		}

		for _, kustomizeReplica := range opts.kustomizeReplicas {
			kustomizeReplicas := source.Kustomize.Replicas
			for i, item := range kustomizeReplicas {
				if kustomizeReplica == item.Name {
					source.Kustomize.Replicas = append(kustomizeReplicas[0:i], kustomizeReplicas[i+1:]...)
					updated = true
					break
				}
			}
		}
	}
	if source.Helm != nil {
		if len(opts.parameters) == 0 && len(opts.valuesFiles) == 0 && !opts.valuesLiteral && !opts.ignoreMissingValueFiles && !opts.passCredentials {
			return false, true
		}
		for _, paramStr := range opts.parameters {
			helmParams := source.Helm.Parameters
			for i, p := range helmParams {
				if p.Name == paramStr {
					source.Helm.Parameters = append(helmParams[0:i], helmParams[i+1:]...)
					updated = true
					break
				}
			}
		}
		if opts.valuesLiteral && source.Helm.Values != "" {
			source.Helm.Values = ""
			updated = true
		}
		for _, valuesFile := range opts.valuesFiles {
			specValueFiles := source.Helm.ValueFiles
			for i, vf := range specValueFiles {
				if vf == valuesFile {
					source.Helm.ValueFiles = append(specValueFiles[0:i], specValueFiles[i+1:]...)
					updated = true
					break
				}
			}
		}
		if opts.ignoreMissingValueFiles && source.Helm.IgnoreMissingValueFiles {
			source.Helm.IgnoreMissingValueFiles = false
			updated = true
		}
		if opts.passCredentials && source.Helm.PassCredentials {
			source.Helm.PassCredentials = false
			updated = true
		}
	}
	if source.Plugin != nil {
		if len(opts.pluginEnvs) == 0 {
			return false, true
		}
		for _, env := range opts.pluginEnvs {
			err := source.Plugin.RemoveEnvEntry(env)
			if err == nil {
				updated = true
			}
		}
	}
	return updated, false
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

func getLocalObjects(ctx context.Context, app *argoappv1.Application, local, localRepoRoot, appLabelKey, kubeVersion string, apiVersions []string, kustomizeOptions *argoappv1.KustomizeOptions,
	configManagementPlugins []*argoappv1.ConfigManagementPlugin, trackingMethod string) []*unstructured.Unstructured {
	manifestStrings := getLocalObjectsString(ctx, app, local, localRepoRoot, appLabelKey, kubeVersion, apiVersions, kustomizeOptions, configManagementPlugins, trackingMethod)
	objs := make([]*unstructured.Unstructured, len(manifestStrings))
	for i := range manifestStrings {
		obj := unstructured.Unstructured{}
		err := json.Unmarshal([]byte(manifestStrings[i]), &obj)
		errors.CheckError(err)
		objs[i] = &obj
	}
	return objs
}

func getLocalObjectsString(ctx context.Context, app *argoappv1.Application, local, localRepoRoot, appLabelKey, kubeVersion string, apiVersions []string, kustomizeOptions *argoappv1.KustomizeOptions,
	configManagementPlugins []*argoappv1.ConfigManagementPlugin, trackingMethod string) []string {
	source := app.Spec.GetSource()
	res, err := repository.GenerateManifests(ctx, local, localRepoRoot, source.TargetRevision, &repoapiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{Repo: source.RepoURL},
		AppLabelKey:       appLabelKey,
		AppName:           app.Name,
		Namespace:         app.Spec.Destination.Namespace,
		ApplicationSource: &source,
		KustomizeOptions:  kustomizeOptions,
		KubeVersion:       kubeVersion,
		ApiVersions:       apiVersions,
		Plugins:           configManagementPlugins,
		TrackingMethod:    trackingMethod,
	}, true, &git.NoopCredsStore{}, resource.MustParse("0"), nil)
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
		refresh            bool
		hardRefresh        bool
		exitCode           bool
		local              string
		revision           string
		localRepoRoot      string
		serverSideGenerate bool
		localIncludes      []string
	)
	shortDesc := "Perform a diff against the target and live state."
	var command = &cobra.Command{
		Use:   "diff APPNAME",
		Short: shortDesc,
		Long:  shortDesc + "\nUses 'diff' to render the difference. KUBECTL_EXTERNAL_DIFF environment variable can be used to select your own diff tool.\nReturns the following exit codes: 2 on general errors, 1 when a diff is found, and 0 when no diff is found",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(2)
			}
			clientset := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := clientset.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appName,
				Refresh:      getRefreshType(refresh, hardRefresh),
				AppNamespace: &appNs,
			})
			errors.CheckError(err)

			resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{ApplicationName: &appName, AppNamespace: &appNs})
			errors.CheckError(err)
			conn, settingsIf := clientset.NewSettingsClientOrDie()
			defer argoio.Close(conn)
			argoSettings, err := settingsIf.Get(ctx, &settingspkg.SettingsQuery{})
			errors.CheckError(err)
			diffOption := &DifferenceOption{}
			if revision != "" {
				q := applicationpkg.ApplicationManifestQuery{
					Name:         &appName,
					Revision:     &revision,
					AppNamespace: &appNs,
				}
				res, err := appIf.GetManifests(ctx, &q)
				errors.CheckError(err)
				diffOption.res = res
				diffOption.revision = revision
			} else if local != "" {
				if serverSideGenerate {
					client, err := appIf.GetManifestsWithFiles(ctx, grpc_retry.Disable())
					errors.CheckError(err)

					err = manifeststream.SendApplicationManifestQueryWithFiles(ctx, client, appName, appNs, local, localIncludes)
					errors.CheckError(err)

					res, err := client.CloseAndRecv()
					errors.CheckError(err)

					diffOption.serversideRes = res
				} else {
					fmt.Fprintf(os.Stderr, "Warning: local diff without --server-side-generate is deprecated and does not work with plugins. Server-side generation will be the default in v2.7.")
					conn, clusterIf := clientset.NewClusterClientOrDie()
					defer argoio.Close(conn)
					cluster, err := clusterIf.Get(ctx, &clusterpkg.ClusterQuery{Name: app.Spec.Destination.Name, Server: app.Spec.Destination.Server})
					errors.CheckError(err)
					diffOption.local = local
					diffOption.localRepoRoot = localRepoRoot
					diffOption.cluster = cluster
				}
			}
			foundDiffs := findandPrintDiff(ctx, app, resources, argoSettings, appName, diffOption)
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
	command.Flags().BoolVar(&serverSideGenerate, "server-side-generate", false, "Used with --local, this will send your manifests to the server for diffing")
	command.Flags().StringArrayVar(&localIncludes, "local-include", []string{"*.yaml", "*.yml", "*.json"}, "Used with --server-side-generate, specify patterns of filenames to send. Matching is based on filename and not path.")
	return command
}

// DifferenceOption struct to store diff options
type DifferenceOption struct {
	local         string
	localRepoRoot string
	revision      string
	cluster       *argoappv1.Cluster
	res           *repoapiclient.ManifestResponse
	serversideRes *repoapiclient.ManifestResponse
}

// findandPrintDiff ... Prints difference between application current state and state stored in git or locally, returns boolean as true if difference is found else returns false
func findandPrintDiff(ctx context.Context, app *argoappv1.Application, resources *applicationpkg.ManagedResourcesResponse, argoSettings *settingspkg.Settings, appName string, diffOptions *DifferenceOption) bool {
	var foundDiffs bool
	liveObjs, err := cmdutil.LiveObjects(resources.Items)
	errors.CheckError(err)
	items := make([]objKeyLiveTarget, 0)
	if diffOptions.local != "" {
		localObjs := groupObjsByKey(getLocalObjects(ctx, app, diffOptions.local, diffOptions.localRepoRoot, argoSettings.AppLabelKey, diffOptions.cluster.Info.ServerVersion, diffOptions.cluster.Info.APIVersions, argoSettings.KustomizeOptions, argoSettings.ConfigManagementPlugins, argoSettings.TrackingMethod), liveObjs, app.Spec.Destination.Namespace)
		items = groupObjsForDiff(resources, localObjs, items, argoSettings, app.InstanceName(argoSettings.ControllerNamespace))
	} else if diffOptions.revision != "" {
		var unstructureds []*unstructured.Unstructured
		for _, mfst := range diffOptions.res.Manifests {
			obj, err := argoappv1.UnmarshalToUnstructured(mfst)
			errors.CheckError(err)
			unstructureds = append(unstructureds, obj)
		}
		groupedObjs := groupObjsByKey(unstructureds, liveObjs, app.Spec.Destination.Namespace)
		items = groupObjsForDiff(resources, groupedObjs, items, argoSettings, app.Name)
	} else if diffOptions.serversideRes != nil {
		var unstructureds []*unstructured.Unstructured
		for _, mfst := range diffOptions.serversideRes.Manifests {
			obj, err := argoappv1.UnmarshalToUnstructured(mfst)
			errors.CheckError(err)
			unstructureds = append(unstructureds, obj)
		}
		groupedObjs := groupObjsByKey(unstructureds, liveObjs, app.Spec.Destination.Namespace)
		items = groupObjsForDiff(resources, groupedObjs, items, argoSettings, app.Name)
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

	for _, item := range items {
		if item.target != nil && hook.IsHook(item.target) || item.live != nil && hook.IsHook(item.live) {
			continue
		}
		overrides := make(map[string]argoappv1.ResourceOverride)
		for k := range argoSettings.ResourceOverrides {
			val := argoSettings.ResourceOverrides[k]
			overrides[k] = *val
		}

		// TODO remove hardcoded IgnoreAggregatedRoles and retrieve the
		// compareOptions in the protobuf
		ignoreAggregatedRoles := false
		diffConfig, err := argodiff.NewDiffConfigBuilder().
			WithDiffSettings(app.Spec.IgnoreDifferences, overrides, ignoreAggregatedRoles).
			WithTracking(argoSettings.AppLabelKey, argoSettings.TrackingMethod).
			WithNoCache().
			Build()
		errors.CheckError(err)
		diffRes, err := argodiff.StateDiff(item.live, item.target, diffConfig)
		errors.CheckError(err)

		if diffRes.Modified || item.target == nil || item.live == nil {
			fmt.Printf("\n===== %s/%s %s/%s ======\n", item.key.Group, item.key.Kind, item.key.Namespace, item.key.Name)
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
			if !foundDiffs {
				foundDiffs = true
			}
			_ = cli.PrintDiff(item.key.Name, live, target)
		}
	}
	return foundDiffs
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
		selector          string
	)
	var command = &cobra.Command{
		Use:   "delete APPNAME",
		Short: "Delete an application",
		Example: `  # Delete an app
  argocd app delete my-app

  # Delete multiple apps
  argocd app delete my-app other-app

  # Delete apps by label
  argocd app delete -l app.kubernetes.io/instance=my-app
  argocd app delete -l app.kubernetes.io/instance!=my-app
  argocd app delete -l app.kubernetes.io/instance
  argocd app delete -l '!app.kubernetes.io/instance'
  argocd app delete -l 'app.kubernetes.io/instance notin (my-app,other-app)'`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 && selector == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			var isTerminal bool = isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
			var isConfirmAll bool = false
			var numOfApps = len(args)
			var promptFlag = c.Flag("yes")
			if promptFlag.Changed && promptFlag.Value.String() == "true" {
				noPrompt = true
			}

			appNames, err := getAppNamesBySelector(ctx, appIf, selector)
			errors.CheckError(err)

			if len(appNames) == 0 {
				appNames = args
			}

			for _, appFullName := range appNames {
				appName, appNs := argo.ParseAppQualifiedName(appFullName, "")
				appDeleteReq := applicationpkg.ApplicationDeleteRequest{
					Name:         &appName,
					AppNamespace: &appNs,
				}
				if c.Flag("cascade").Changed {
					appDeleteReq.Cascade = &cascade
				}
				if c.Flag("propagation-policy").Changed {
					appDeleteReq.PropagationPolicy = &propagationPolicy
				}
				if cascade && isTerminal && !noPrompt {
					var lowercaseAnswer string
					if numOfApps == 1 {
						lowercaseAnswer = cli.AskToProceedS("Are you sure you want to delete '" + appFullName + "' and all its resources? [y/n] ")
					} else {
						if !isConfirmAll {
							lowercaseAnswer = cli.AskToProceedS("Are you sure you want to delete '" + appFullName + "' and all its resources? [y/n/A] where 'A' is to delete all specified apps and their resources without prompting ")
							if lowercaseAnswer == "a" {
								lowercaseAnswer = "y"
								isConfirmAll = true
							}
						} else {
							lowercaseAnswer = "y"
						}
					}
					if lowercaseAnswer == "y" {
						_, err := appIf.Delete(ctx, &appDeleteReq)
						errors.CheckError(err)
						fmt.Printf("application '%s' deleted\n", appFullName)
					} else {
						fmt.Println("The command to delete '" + appFullName + "' was cancelled.")
					}
				} else {
					_, err := appIf.Delete(ctx, &appDeleteReq)
					errors.CheckError(err)
				}
			}
		},
	}
	command.Flags().BoolVar(&cascade, "cascade", true, "Perform a cascaded deletion of all application resources")
	command.Flags().StringVarP(&propagationPolicy, "propagation-policy", "p", "foreground", "Specify propagation policy for deletion of application's resources. One of: foreground|background")
	command.Flags().BoolVarP(&noPrompt, "yes", "y", false, "Turn off prompting to confirm cascaded deletion of application resources")
	command.Flags().StringVarP(&selector, "selector", "l", "", "Delete all apps with matching label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.")
	return command
}

// Print simple list of application names
func printApplicationNames(apps []argoappv1.Application) {
	for _, app := range apps {
		fmt.Println(app.QualifiedName())
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
			app.QualifiedName(),
			getServer(&app),
			app.Spec.Destination.Namespace,
			app.Spec.GetProject(),
			app.Status.Sync.Status,
			app.Status.Health.Status,
			formatSyncPolicy(app),
			formatConditionsSummary(app),
		}
		if *output == "wide" {
			vals = append(vals, app.Spec.GetSource().RepoURL, app.Spec.GetSource().Path, app.Spec.GetSource().TargetRevision)
		}
		_, _ = fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
}

// NewApplicationListCommand returns a new instance of an `argocd app list` command
func NewApplicationListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output       string
		selector     string
		projects     []string
		repo         string
		appNamespace string
		cluster      string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List applications",
		Example: `  # List all apps
  argocd app list

  # List apps by label, in this example we listing apps that are children of another app (aka app-of-apps)
  argocd app list -l app.kubernetes.io/instance=my-app
  argocd app list -l app.kubernetes.io/instance!=my-app
  argocd app list -l app.kubernetes.io/instance
  argocd app list -l '!app.kubernetes.io/instance'
  argocd app list -l 'app.kubernetes.io/instance notin (my-app,other-app)'`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			apps, err := appIf.List(ctx, &applicationpkg.ApplicationQuery{
				Selector:     pointer.String(selector),
				AppNamespace: &appNamespace,
			})

			errors.CheckError(err)
			appList := apps.Items

			if len(projects) != 0 {
				appList = argo.FilterByProjects(appList, projects)
			}
			if repo != "" {
				appList = argo.FilterByRepo(appList, repo)
			}
			if cluster != "" {
				appList = argo.FilterByCluster(appList, cluster)
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
	command.Flags().StringVarP(&selector, "selector", "l", "", "List apps by label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.")
	command.Flags().StringArrayVarP(&projects, "project", "p", []string{}, "Filter by project name")
	command.Flags().StringVarP(&repo, "repo", "r", "", "List apps by source repo URL")
	command.Flags().StringVarP(&appNamespace, "app-namespace", "N", "", "Only list applications in namespace")
	command.Flags().StringVarP(&cluster, "cluster", "c", "", "List apps by cluster name or url")
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
	resourceExcludeIndicator            = "!"
)

// resource is GROUP:KIND:NAMESPACE/NAME or GROUP:KIND:NAME
func parseSelectedResources(resources []string) ([]*argoappv1.SyncOperationResource, error) {
	// retrieve name and namespace in case if format is GROUP:KIND:NAMESPACE/NAME, otherwise return name and empty namespace
	nameRetriever := func(resourceName, resource string) (string, string, error) {
		if !strings.Contains(resourceName, resourceFieldNamespaceDelimiter) {
			return resourceName, "", nil
		}
		nameFields := strings.Split(resourceName, resourceFieldNamespaceDelimiter)
		if len(nameFields) != resourceFieldNameWithNamespaceCount {
			return "", "", fmt.Errorf("Resource with namespace should have GROUP%sKIND%sNAMESPACE%sNAME, but instead got: %s", resourceFieldDelimiter, resourceFieldDelimiter, resourceFieldNamespaceDelimiter, resource)
		}
		namespace := nameFields[0]
		name := nameFields[1]
		return name, namespace, nil
	}

	var selectedResources []*argoappv1.SyncOperationResource
	if resources == nil {
		return selectedResources, nil
	}

	for _, resource := range resources {
		isExcluded := false
		// check if the resource flag starts with a '!'
		if strings.HasPrefix(resource, resourceExcludeIndicator) {
			resource = strings.TrimPrefix(resource, resourceExcludeIndicator)
			isExcluded = true
		}
		fields := strings.Split(resource, resourceFieldDelimiter)
		if len(fields) != resourceFieldCount {
			return nil, fmt.Errorf("Resource should have GROUP%sKIND%sNAME, but instead got: %s", resourceFieldDelimiter, resourceFieldDelimiter, resource)
		}
		name, namespace, err := nameRetriever(fields[2], resource)
		if err != nil {
			return nil, err
		}
		selectedResources = append(selectedResources, &argoappv1.SyncOperationResource{
			Group:     fields[0],
			Kind:      fields[1],
			Name:      name,
			Namespace: namespace,
			Exclude:   isExcluded,
		})
	}
	return selectedResources, nil
}

func getWatchOpts(watch watchOpts) watchOpts {
	// if no opts are defined should wait for sync,health,operation
	if (watch == watchOpts{}) {
		return watchOpts{
			sync:      true,
			health:    true,
			operation: true,
		}
	}
	return watch
}

// NewApplicationWaitCommand returns a new instance of an `argocd app wait` command
func NewApplicationWaitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		watch     watchOpts
		timeout   uint
		selector  string
		resources []string
	)
	var command = &cobra.Command{
		Use:   "wait [APPNAME.. | -l selector]",
		Short: "Wait for an application to reach a synced and healthy state",
		Example: `  # Wait for an app
  argocd app wait my-app

  # Wait for multiple apps
  argocd app wait my-app other-app

  # Wait for apps by resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME.
  argocd app wait my-app --resource :Service:my-service
  argocd app wait my-app --resource argoproj.io:Rollout:my-rollout
  argocd app wait my-app --resource '!apps:Deployment:my-service'
  argocd app wait my-app --resource apps:Deployment:my-service --resource :Service:my-service
  argocd app wait my-app --resource '!*:Service:*'
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app wait my-app --resource argoproj.io:Rollout:my-namespace/my-rollout

  # Wait for apps by label, in this example we waiting for apps that are children of another app (aka app-of-apps)
  argocd app wait -l app.kubernetes.io/instance=my-app
  argocd app wait -l app.kubernetes.io/instance!=my-app
  argocd app wait -l app.kubernetes.io/instance
  argocd app wait -l '!app.kubernetes.io/instance'
  argocd app wait -l 'app.kubernetes.io/instance notin (my-app,other-app)'`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 && selector == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			watch = getWatchOpts(watch)
			selectedResources, err := parseSelectedResources(resources)
			errors.CheckError(err)
			appNames := args
			acdClient := headless.NewClientOrDie(clientOpts, c)
			closer, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(closer)
			if selector != "" {
				list, err := appIf.List(ctx, &applicationpkg.ApplicationQuery{Selector: pointer.String(selector)})
				errors.CheckError(err)
				for _, i := range list.Items {
					appNames = append(appNames, i.Name)
				}
			}
			for _, appName := range appNames {
				_, err := waitOnApplicationStatus(ctx, acdClient, appName, timeout, watch, selectedResources)
				errors.CheckError(err)
			}
		},
	}
	command.Flags().BoolVar(&watch.sync, "sync", false, "Wait for sync")
	command.Flags().BoolVar(&watch.health, "health", false, "Wait for health")
	command.Flags().BoolVar(&watch.suspended, "suspended", false, "Wait for suspended")
	command.Flags().BoolVar(&watch.degraded, "degraded", false, "Wait for degraded")
	command.Flags().StringVarP(&selector, "selector", "l", "", "Wait for apps by label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.")
	command.Flags().StringArrayVar(&resources, "resource", []string{}, fmt.Sprintf("Sync only specific resources as GROUP%[1]sKIND%[1]sNAME or %[2]sGROUP%[1]sKIND%[1]sNAME. Fields may be blank and '*' can be used. This option may be specified repeatedly", resourceFieldDelimiter, resourceExcludeIndicator))
	command.Flags().BoolVar(&watch.operation, "operation", false, "Wait for pending operations")
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
		serverSideApply         bool
		async                   bool
		retryLimit              int64
		retryBackoffDuration    time.Duration
		retryBackoffMaxDuration time.Duration
		retryBackoffFactor      int64
		local                   string
		localRepoRoot           string
		infos                   []string
		diffChanges             bool
		diffChangesConfirm      bool
		projects                []string
	)
	var command = &cobra.Command{
		Use:   "sync [APPNAME... | -l selector | --project project-name]",
		Short: "Sync an application to its target state",
		Example: `  # Sync an app
  argocd app sync my-app

  # Sync multiples apps
  argocd app sync my-app other-app

  # Sync apps by label, in this example we sync apps that are children of another app (aka app-of-apps)
  argocd app sync -l app.kubernetes.io/instance=my-app
  argocd app sync -l app.kubernetes.io/instance!=my-app
  argocd app sync -l app.kubernetes.io/instance
  argocd app sync -l '!app.kubernetes.io/instance'
  argocd app sync -l 'app.kubernetes.io/instance notin (my-app,other-app)'

  # Sync a specific resource
  # Resource should be formatted as GROUP:KIND:NAME. If no GROUP is specified then :KIND:NAME
  argocd app sync my-app --resource :Service:my-service
  argocd app sync my-app --resource argoproj.io:Rollout:my-rollout
  argocd app sync my-app --resource '!apps:Deployment:my-service'
  argocd app sync my-app --resource apps:Deployment:my-service --resource :Service:my-service
  argocd app sync my-app --resource '!*:Service:*'
  # Specify namespace if the application has resources with the same name in different namespaces
  argocd app sync my-app --resource argoproj.io:Rollout:my-namespace/my-rollout`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) == 0 && selector == "" && len(projects) == 0 {

				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			if len(args) > 1 && selector != "" {
				log.Fatal("Cannot use selector option when application name(s) passed as argument(s)")
			}

			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)

			selectedLabels, err := label.Parse(labels)
			errors.CheckError(err)

			appNames := args
			if selector != "" || len(projects) > 0 {
				list, err := appIf.List(ctx, &applicationpkg.ApplicationQuery{Selector: pointer.String(selector), Projects: projects})
				errors.CheckError(err)

				// unlike list, we'd want to fail if nothing was found
				if len(list.Items) == 0 {
					errMsg := "No matching apps found for filter:"
					if selector != "" {
						errMsg += fmt.Sprintf(" selector %s", selector)
					}
					if len(projects) != 0 {
						errMsg += fmt.Sprintf(" projects %v", projects)
					}
					log.Fatalf(errMsg)
				}

				for _, i := range list.Items {
					appNames = append(appNames, i.QualifiedName())
				}
			}

			for _, appQualifiedName := range appNames {
				appName, appNs := argo.ParseAppQualifiedName(appQualifiedName, "")

				if len(selectedLabels) > 0 {
					q := applicationpkg.ApplicationManifestQuery{
						Name:         &appName,
						AppNamespace: &appNs,
						Revision:     &revision,
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

				selectedResources, err := parseSelectedResources(resources)
				errors.CheckError(err)

				var localObjsStrings []string
				diffOption := &DifferenceOption{}

				app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
					Name:         &appName,
					AppNamespace: &appNs,
				})
				errors.CheckError(err)

				if app.Spec.HasMultipleSources() {
					log.Fatal("argocd cli does not work on multi-source app")
					return
				}

				// filters out only those resources that needs to be synced
				filteredResources := filterAppResources(app, selectedResources)

				// if resources are provided and no app resources match, then return error
				if len(resources) > 0 && len(filteredResources) == 0 {
					log.Fatalf("No matching app resources found for resource filter: %v", strings.Join(resources, ", "))
				}

				if local != "" {
					if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil && !dryRun {
						log.Fatal("Cannot use local sync when Automatic Sync Policy is enabled except with --dry-run")
					}

					errors.CheckError(err)
					conn, settingsIf := acdClient.NewSettingsClientOrDie()
					argoSettings, err := settingsIf.Get(ctx, &settingspkg.SettingsQuery{})
					errors.CheckError(err)
					argoio.Close(conn)

					conn, clusterIf := acdClient.NewClusterClientOrDie()
					defer argoio.Close(conn)
					cluster, err := clusterIf.Get(ctx, &clusterpkg.ClusterQuery{Name: app.Spec.Destination.Name, Server: app.Spec.Destination.Server})
					errors.CheckError(err)
					argoio.Close(conn)
					localObjsStrings = getLocalObjectsString(ctx, app, local, localRepoRoot, argoSettings.AppLabelKey, cluster.Info.ServerVersion, cluster.Info.APIVersions, argoSettings.KustomizeOptions, argoSettings.ConfigManagementPlugins, argoSettings.TrackingMethod)
					errors.CheckError(err)
					diffOption.local = local
					diffOption.localRepoRoot = localRepoRoot
					diffOption.cluster = cluster
				}

				syncOptionsFactory := func() *applicationpkg.SyncOptions {
					syncOptions := applicationpkg.SyncOptions{}
					items := make([]string, 0)
					if replace {
						items = append(items, common.SyncOptionReplace)
					}
					if serverSideApply {
						items = append(items, common.SyncOptionServerSideApply)
					}

					if len(items) == 0 {
						// for prevent send even empty array if not need
						return nil
					}
					syncOptions.Items = items
					return &syncOptions
				}

				syncReq := applicationpkg.ApplicationSyncRequest{
					Name:         &appName,
					AppNamespace: &appNs,
					DryRun:       &dryRun,
					Revision:     &revision,
					Resources:    filteredResources,
					Prune:        &prune,
					Manifests:    localObjsStrings,
					Infos:        getInfos(infos),
					SyncOptions:  syncOptionsFactory(),
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
				if diffChanges {
					resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
						ApplicationName: &appName,
						AppNamespace:    &appNs,
					})
					errors.CheckError(err)
					conn, settingsIf := acdClient.NewSettingsClientOrDie()
					defer argoio.Close(conn)
					argoSettings, err := settingsIf.Get(ctx, &settingspkg.SettingsQuery{})
					errors.CheckError(err)
					foundDiffs := false
					fmt.Printf("====== Previewing differences between live and desired state of application %s ======\n", appQualifiedName)
					foundDiffs = findandPrintDiff(ctx, app, resources, argoSettings, appQualifiedName, diffOption)
					if foundDiffs {
						if !diffChangesConfirm {
							yesno := cli.AskToProceed(fmt.Sprintf("Please review changes to application %s shown above. Do you want to continue the sync process? (y/n): ", appQualifiedName))
							if !yesno {
								os.Exit(0)
							}
						}
					} else {
						fmt.Printf("====== No Differences found ======\n")
					}
				}
				_, err = appIf.Sync(ctx, &syncReq)
				errors.CheckError(err)

				if !async {
					app, err := waitOnApplicationStatus(ctx, acdClient, appQualifiedName, timeout, watchOpts{operation: true}, selectedResources)
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
	command.Flags().StringArrayVar(&resources, "resource", []string{}, fmt.Sprintf("Sync only specific resources as GROUP%[1]sKIND%[1]sNAME or %[2]sGROUP%[1]sKIND%[1]sNAME. Fields may be blank and '*' can be used. This option may be specified repeatedly", resourceFieldDelimiter, resourceExcludeIndicator))
	command.Flags().StringVarP(&selector, "selector", "l", "", "Sync apps that match this label. Supports '=', '==', '!=', in, notin, exists & not exists. Matching apps must satisfy all of the specified label constraints.")
	command.Flags().StringArrayVar(&labels, "label", []string{}, "Sync only specific resources with a label. This option may be specified repeatedly.")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	command.Flags().Int64Var(&retryLimit, "retry-limit", 0, "Max number of allowed sync retries")
	command.Flags().DurationVar(&retryBackoffDuration, "retry-backoff-duration", argoappv1.DefaultSyncRetryDuration, "Retry backoff base duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().DurationVar(&retryBackoffMaxDuration, "retry-backoff-max-duration", argoappv1.DefaultSyncRetryMaxDuration, "Max retry backoff duration. Input needs to be a duration (e.g. 2m, 1h)")
	command.Flags().Int64Var(&retryBackoffFactor, "retry-backoff-factor", argoappv1.DefaultSyncRetryFactor, "Factor multiplies the base duration after each failed retry")
	command.Flags().StringVar(&strategy, "strategy", "", "Sync strategy (one of: apply|hook)")
	command.Flags().BoolVar(&force, "force", false, "Use a force apply")
	command.Flags().BoolVar(&replace, "replace", false, "Use a kubectl create/replace instead apply")
	command.Flags().BoolVar(&serverSideApply, "server-side", false, "Use server-side apply while syncing the application")
	command.Flags().BoolVar(&async, "async", false, "Do not wait for application to sync before continuing")
	command.Flags().StringVar(&local, "local", "", "Path to a local directory. When this flag is present no git queries will be made")
	command.Flags().StringVar(&localRepoRoot, "local-repo-root", "/", "Path to the repository root. Used together with --local allows setting the repository root")
	command.Flags().StringArrayVar(&infos, "info", []string{}, "A list of key-value pairs during sync process. These infos will be persisted in app.")
	command.Flags().BoolVar(&diffChangesConfirm, "assumeYes", false, "Assume yes as answer for all user queries or prompts")
	command.Flags().BoolVar(&diffChanges, "preview-changes", false, "Preview difference against the target and live state before syncing app and wait for user confirmation")
	command.Flags().StringArrayVar(&projects, "project", []string{}, "Sync apps that belong to the specified projects. This option may be specified repeatedly.")
	return command
}

func getAppNamesBySelector(ctx context.Context, appIf applicationpkg.ApplicationServiceClient, selector string) ([]string, error) {
	appNames := []string{}
	if selector != "" {
		list, err := appIf.List(ctx, &applicationpkg.ApplicationQuery{Selector: pointer.String(selector)})
		if err != nil {
			return []string{}, err
		}
		// unlike list, we'd want to fail if nothing was found
		if len(list.Items) == 0 {
			return []string{}, fmt.Errorf("no apps match selector %v", selector)
		}
		for _, i := range list.Items {
			appNames = append(appNames, i.Name)
		}
	}
	return appNames, nil
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

func getResourceStates(app *argoappv1.Application, selectedResources []*argoappv1.SyncOperationResource) []*resourceState {
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
			if !argo.IncludeResource(res.Name, res.Namespace, schema.GroupVersionKind{Group: res.Group, Kind: res.Kind}, selectedResources) {
				states = append(states[:i], states[i+1:]...)
			}
		}
	}
	return states
}

// filterAppResources selects the app resources that match atleast one of the resource filters.
func filterAppResources(app *argoappv1.Application, selectedResources []*argoappv1.SyncOperationResource) []*argoappv1.SyncOperationResource {
	var filteredResources []*argoappv1.SyncOperationResource
	if app != nil && len(selectedResources) > 0 {
		for i := range app.Status.Resources {
			appResource := app.Status.Resources[i]
			if (argo.IncludeResource(appResource.Name, appResource.Namespace,
				schema.GroupVersionKind{Group: appResource.Group, Kind: appResource.Kind}, selectedResources)) {
				filteredResources = append(filteredResources, &argoappv1.SyncOperationResource{
					Group:     appResource.Group,
					Kind:      appResource.Kind,
					Name:      appResource.Name,
					Namespace: appResource.Namespace,
				})
			}
		}
	}
	return filteredResources
}

func groupResourceStates(app *argoappv1.Application, selectedResources []*argoappv1.SyncOperationResource) map[string]*resourceState {
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

// check if resource health, sync and operation statuses matches watch options
func checkResourceStatus(watch watchOpts, healthStatus string, syncStatus string, operationStatus *argoappv1.Operation) bool {
	healthCheckPassed := true

	if watch.suspended && watch.health && watch.degraded {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy) ||
			healthStatus == string(health.HealthStatusSuspended) ||
			healthStatus == string(health.HealthStatusDegraded)
	} else if watch.suspended && watch.degraded {
		healthCheckPassed = healthStatus == string(health.HealthStatusDegraded) ||
			healthStatus == string(health.HealthStatusSuspended)
	} else if watch.degraded && watch.health {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy) ||
			healthStatus == string(health.HealthStatusDegraded)
		//below are good
	} else if watch.suspended && watch.health {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy) ||
			healthStatus == string(health.HealthStatusSuspended)
	} else if watch.suspended {
		healthCheckPassed = healthStatus == string(health.HealthStatusSuspended)
	} else if watch.health {
		healthCheckPassed = healthStatus == string(health.HealthStatusHealthy)
	} else if watch.degraded {
		healthCheckPassed = healthStatus == string(health.HealthStatusDegraded)
	}

	synced := !watch.sync || syncStatus == string(argoappv1.SyncStatusCodeSynced)
	operational := !watch.operation || operationStatus == nil
	return synced && healthCheckPassed && operational
}

const waitFormatString = "%s\t%5s\t%10s\t%10s\t%20s\t%8s\t%7s\t%10s\t%s\n"

func waitOnApplicationStatus(ctx context.Context, acdClient argocdclient.Client, appName string, timeout uint, watch watchOpts, selectedResources []*argoappv1.SyncOperationResource) (*argoappv1.Application, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// refresh controls whether or not we refresh the app before printing the final status.
	// We only want to do this when an operation is in progress, since operations are the only
	// time when the sync status lags behind when an operation completes
	refresh := false

	appRealName, appNs := argo.ParseAppQualifiedName(appName, "")

	printFinalStatus := func(app *argoappv1.Application) *argoappv1.Application {
		var err error
		if refresh {
			conn, appClient := acdClient.NewApplicationClientOrDie()
			refreshType := string(argoappv1.RefreshTypeNormal)
			app, err = appClient.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appRealName,
				Refresh:      &refreshType,
				AppNamespace: &appNs,
			})
			errors.CheckError(err)
			_ = conn.Close()
		}

		fmt.Println()
		printAppSummaryTable(app, appURL(ctx, acdClient, appName), nil)
		fmt.Println()
		if watch.operation {
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
	app, err := appClient.Get(ctx, &applicationpkg.ApplicationQuery{
		Name:         &appRealName,
		AppNamespace: &appNs,
	})
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
				resourceIsReady := checkResourceStatus(watch, state.Health, state.Status, appEvent.Application.Operation)
				if !resourceIsReady {
					selectedResourcesAreReady = false
					break
				}
			}
		} else {
			// Wait on the application as a whole
			selectedResourcesAreReady = checkResourceStatus(watch, string(app.Status.Health.Status), string(app.Status.Sync.Status), appEvent.Application.Operation)
		}

		if selectedResourcesAreReady && (!operationInProgress || !watch.operation) {
			app = printFinalStatus(app)
			return app, nil
		}

		newStates := groupResourceStates(app, selectedResources)
		for _, newState := range newStates {
			var doPrint bool
			stateKey := newState.Key()
			if prevState, found := prevStates[stateKey]; found {
				if watch.health && prevState.Health != string(health.HealthStatusUnknown) && prevState.Health != string(health.HealthStatusDegraded) && newState.Health == string(health.HealthStatusDegraded) {
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
// the app is assumed to be a helm app and is expected to be in the form:
// param=value
func setParameterOverrides(app *argoappv1.Application, parameters []string) {
	if len(parameters) == 0 {
		return
	}
	source := app.Spec.GetSource()
	var sourceType argoappv1.ApplicationSourceType
	if st, _ := source.ExplicitType(); st != nil {
		sourceType = *st
	} else if app.Status.SourceType != "" {
		sourceType = app.Status.SourceType
	} else {
		if len(strings.SplitN(parameters[0], "=", 2)) == 2 {
			sourceType = argoappv1.ApplicationSourceTypeHelm
		}
	}

	switch sourceType {
	case argoappv1.ApplicationSourceTypeHelm:
		if source.Helm == nil {
			source.Helm = &argoappv1.ApplicationSourceHelm{}
		}
		for _, p := range parameters {
			newParam, err := argoappv1.NewHelmParameter(p, false)
			if err != nil {
				log.Error(err)
				continue
			}
			source.Helm.AddParameter(*newParam)
		}
	default:
		log.Fatalf("Parameters can only be set against Helm applications")
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
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appName,
				AppNamespace: &appNs,
			})
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
			ctx := c.Context()

			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			var err error
			depID := -1
			if len(args) > 1 {
				depID, err = strconv.Atoi(args[1])
				errors.CheckError(err)
			}
			acdClient := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appName,
				AppNamespace: &appNs,
			})
			errors.CheckError(err)

			depInfo, err := findRevisionHistory(app, int64(depID))
			errors.CheckError(err)

			_, err = appIf.Rollback(ctx, &applicationpkg.ApplicationRollbackRequest{
				Name:         &appName,
				AppNamespace: &appNs,
				Id:           pointer.Int64(depInfo.ID),
				Prune:        pointer.Bool(prune),
			})
			errors.CheckError(err)

			_, err = waitOnApplicationStatus(ctx, acdClient, app.QualifiedName(), timeout, watchOpts{
				operation: true,
			}, nil)
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
		source        string
		revision      string
		local         string
		localRepoRoot string
	)
	var command = &cobra.Command{
		Use:   "manifests APPNAME",
		Short: "Print manifests of an application",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			clientset := headless.NewClientOrDie(clientOpts, c)
			conn, appIf := clientset.NewApplicationClientOrDie()
			defer argoio.Close(conn)
			resources, err := appIf.ManagedResources(ctx, &applicationpkg.ResourcesQuery{
				ApplicationName: &appName,
				AppNamespace:    &appNs,
			})
			errors.CheckError(err)

			var unstructureds []*unstructured.Unstructured
			switch source {
			case "git":
				if local != "" {
					app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
					errors.CheckError(err)

					settingsConn, settingsIf := clientset.NewSettingsClientOrDie()
					defer argoio.Close(settingsConn)
					argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
					errors.CheckError(err)

					clusterConn, clusterIf := clientset.NewClusterClientOrDie()
					defer argoio.Close(clusterConn)
					cluster, err := clusterIf.Get(context.Background(), &clusterpkg.ClusterQuery{Name: app.Spec.Destination.Name, Server: app.Spec.Destination.Server})
					errors.CheckError(err)

					unstructureds = getLocalObjects(context.Background(), app, local, localRepoRoot, argoSettings.AppLabelKey, cluster.ServerVersion, cluster.Info.APIVersions, argoSettings.KustomizeOptions, argoSettings.ConfigManagementPlugins, argoSettings.TrackingMethod)
				} else if revision != "" {
					q := applicationpkg.ApplicationManifestQuery{
						Name:         &appName,
						AppNamespace: &appNs,
						Revision:     pointer.String(revision),
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
				liveObjs, err := cmdutil.LiveObjects(resources.Items)
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
	command.Flags().StringVar(&local, "local", "", "If set, show locally-generated manifests. Value is the absolute path to app manifests within the manifest repo. Example: '/home/username/apps/env/app-1'.")
	command.Flags().StringVar(&localRepoRoot, "local-repo-root", ".", "Path to the local repository root. Used together with --local allows setting the repository root. Example: '/home/username/apps'.")
	return command
}

// NewApplicationTerminateOpCommand returns a new instance of an `argocd app terminate-op` command
func NewApplicationTerminateOpCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "terminate-op APPNAME",
		Short: "Terminate running operation of an application",
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			_, err := appIf.TerminateOperation(ctx, &applicationpkg.OperationTerminateRequest{
				Name:         &appName,
				AppNamespace: &appNs,
			})
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
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{
				Name:         &appName,
				AppNamespace: &appNs,
			})
			errors.CheckError(err)

			appData, err := json.Marshal(app.Spec)
			errors.CheckError(err)
			appData, err = yaml.JSONToYAML(appData)
			errors.CheckError(err)

			cli.InteractiveEdit(fmt.Sprintf("%s-*-edit.yaml", appName), appData, func(input []byte) error {
				input, err = yaml.YAMLToJSON(input)
				if err != nil {
					return fmt.Errorf("error converting YAML to JSON: %w", err)
				}
				updatedSpec := argoappv1.ApplicationSpec{}
				err = json.Unmarshal(input, &updatedSpec)
				if err != nil {
					return fmt.Errorf("error unmarshaling input into application spec: %w", err)
				}

				var appOpts cmdutil.AppOptions
				cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
				_, err = appIf.UpdateSpec(ctx, &applicationpkg.ApplicationUpdateSpecRequest{
					Name:         &appName,
					Spec:         &updatedSpec,
					Validate:     &appOpts.Validate,
					AppNamespace: &appNs,
				})
				if err != nil {
					return fmt.Errorf("failed to update application spec: %w", err)
				}
				return nil
			})
		},
	}
	return command
}

func NewApplicationPatchCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var patch string
	var patchType string

	command := cobra.Command{
		Use:   "patch APPNAME",
		Short: "Patch application",
		Example: `  # Update an application's source path using json patch
  argocd app patch myapplication --patch='[{"op": "replace", "path": "/spec/source/path", "value": "newPath"}]' --type json

  # Update an application's repository target revision using merge patch
  argocd app patch myapplication --patch '{"spec": { "source": { "targetRevision": "master" } }}' --type merge`,
		Run: func(c *cobra.Command, args []string) {
			ctx := c.Context()

			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName, appNs := argo.ParseAppQualifiedName(args[0], "")
			conn, appIf := headless.NewClientOrDie(clientOpts, c).NewApplicationClientOrDie()
			defer argoio.Close(conn)

			patchedApp, err := appIf.Patch(ctx, &applicationpkg.ApplicationPatchRequest{
				Name:         &appName,
				Patch:        &patch,
				PatchType:    &patchType,
				AppNamespace: &appNs,
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
