package commands

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/diff"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yudai/gojsondiff/formatter"
	"golang.org/x/crypto/ssh/terminal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewApplicationCommand returns a new instance of an `argocd app` command
func NewApplicationCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "app",
		Short: "Manage applications",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
			os.Exit(1)
		},
	}
	command.AddCommand(NewApplicationCreateCommand(clientOpts))
	command.AddCommand(NewApplicationGetCommand(clientOpts))
	command.AddCommand(NewApplicationDiffCommand(clientOpts))
	command.AddCommand(NewApplicationSetCommand(clientOpts))
	command.AddCommand(NewApplicationSyncCommand(clientOpts))
	command.AddCommand(NewApplicationHistoryCommand(clientOpts))
	command.AddCommand(NewApplicationRollbackCommand(clientOpts))
	command.AddCommand(NewApplicationListCommand(clientOpts))
	command.AddCommand(NewApplicationDeleteCommand(clientOpts))
	command.AddCommand(NewApplicationWaitCommand(clientOpts))
	return command
}

// NewApplicationCreateCommand returns a new instance of an `argocd app create` command
func NewApplicationCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts appOptions
		fileURL string
		appName string
	)
	var command = &cobra.Command{
		Use:   "create",
		Short: "Create an application from a git location",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			var app argoappv1.Application
			if fileURL != "" {
				_, err := url.ParseRequestURI(fileURL)
				if err != nil {
					err = cli.UnmarshalLocalFile(fileURL, &app)
				} else {
					err = cli.UnmarshalRemoteFile(fileURL, &app)
				}
				if err != nil {
					log.Fatal(err)
				}

			} else {
				if appOpts.repoURL == "" || appOpts.appPath == "" || appOpts.env == "" || appName == "" {
					log.Fatal("name, repo, path, env are required")
					os.Exit(1)
				}
				app = argoappv1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: appName,
					},
					Spec: argoappv1.ApplicationSpec{
						Source: argoappv1.ApplicationSource{
							RepoURL:        appOpts.repoURL,
							Path:           appOpts.appPath,
							Environment:    appOpts.env,
							TargetRevision: appOpts.revision,
						},
					},
				}
			}
			if appOpts.destServer != "" {
				app.Spec.Destination.Server = appOpts.destServer
			}
			if appOpts.destNamespace != "" {
				app.Spec.Destination.Namespace = appOpts.destNamespace
			}
			setParameterOverrides(&app, appOpts.parameters)
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			created, err := appIf.Create(context.Background(), &app)
			errors.CheckError(err)
			fmt.Printf("application '%s' created\n", created.ObjectMeta.Name)
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set")
	addAppFlags(command, &appOpts)
	return command
}

// NewApplicationGetCommand returns a new instance of an `argocd app get` command
func NewApplicationGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
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
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			format := "%-15s%s\n"
			fmt.Printf(format, "Name:", app.Name)
			fmt.Printf(format, "Server:", app.Spec.Destination.Server)
			fmt.Printf(format, "Namespace:", app.Spec.Destination.Namespace)
			fmt.Printf(format, "URL:", appURL(acdClient, app))
			fmt.Printf(format, "Environment:", app.Spec.Source.Environment)
			fmt.Printf(format, "Repo:", app.Spec.Source.RepoURL)
			fmt.Printf(format, "Path:", app.Spec.Source.Path)
			fmt.Printf(format, "Target:", app.Spec.Source.TargetRevision)
			if app.Status.ComparisonResult.Error != "" {
				fmt.Printf(format, "Error:", app.Status.ComparisonResult.Error)
			}
			if len(app.Status.ComparisonResult.Resources) > 0 {
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "KIND\tNAME\tSTATUS\tHEALTH\n")
				for _, res := range app.Status.ComparisonResult.Resources {
					targetObj, err := argoappv1.UnmarshalToUnstructured(res.TargetState)
					errors.CheckError(err)
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", targetObj.GetKind(), targetObj.GetName(), res.Status, res.Health.Status)
				}
				_ = w.Flush()
			}
		},
	}
	return command
}

// appURL returns the URL of an application
func appURL(acdClient argocdclient.Client, app *argoappv1.Application) string {
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
	return fmt.Sprintf("%s://%s/applications/%s/%s", scheme, server, app.Namespace, app.Name)
}

// NewApplicationSetCommand returns a new instance of an `argocd app set` command
func NewApplicationSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts appOptions
	)
	var command = &cobra.Command{
		Use:   "set APPNAME",
		Short: "Set application parameters",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			visited := 0
			c.Flags().Visit(func(f *pflag.Flag) {
				visited++
				switch f.Name {
				case "repo":
					app.Spec.Source.RepoURL = appOpts.repoURL
				case "path":
					app.Spec.Source.Path = appOpts.appPath
				case "env":
					app.Spec.Source.Environment = appOpts.env
				case "revision":
					app.Spec.Source.TargetRevision = appOpts.revision
				case "dest-server":
					app.Spec.Destination.Server = appOpts.destServer
				case "dest-namespace":
					app.Spec.Destination.Namespace = appOpts.destNamespace
				}
			})
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			setParameterOverrides(app, appOpts.parameters)
			_, err = appIf.UpdateSpec(context.Background(), &application.ApplicationSpecRequest{
				AppName: &app.Name,
				Spec:    app.Spec,
			})
			errors.CheckError(err)
		},
	}
	addAppFlags(command, &appOpts)
	return command
}

type appOptions struct {
	repoURL       string
	appPath       string
	env           string
	revision      string
	destServer    string
	destNamespace string
	parameters    []string
}

func addAppFlags(command *cobra.Command, opts *appOptions) {
	command.Flags().StringVar(&opts.repoURL, "repo", "", "Repository URL, ignored if a file is set")
	command.Flags().StringVar(&opts.appPath, "path", "", "Path in repository to the ksonnet app directory, ignored if a file is set")
	command.Flags().StringVar(&opts.env, "env", "", "Application environment to monitor")
	command.Flags().StringVar(&opts.revision, "revision", "HEAD", "The tracking source branch, tag, or commit the application will sync to")
	command.Flags().StringVar(&opts.destServer, "dest-server", "", "K8s cluster URL (overrides the server URL specified in the ksonnet app.yaml)")
	command.Flags().StringVar(&opts.destNamespace, "dest-namespace", "", "K8s target namespace (overrides the namespace specified in the ksonnet app.yaml)")
	command.Flags().StringArrayVarP(&opts.parameters, "parameter", "p", []string{}, "set a parameter override (e.g. -p guestbook=image=example/guestbook:latest)")
}

// NewApplicationDiffCommand returns a new instance of an `argocd app diff` command
func NewApplicationDiffCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "diff APPNAME",
		Short: "Perform a diff against the target and live state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			targetObjs, err := app.Status.ComparisonResult.TargetObjects()
			errors.CheckError(err)
			liveObjs, err := app.Status.ComparisonResult.LiveObjects()
			errors.CheckError(err)
			diffResults, err := diff.DiffArray(targetObjs, liveObjs)
			errors.CheckError(err)
			for i := 0; i < len(targetObjs); i++ {
				targetObj := targetObjs[i]
				diffRes := diffResults.Diffs[i]
				fmt.Printf("===== %s %s ======\n", targetObj.GetKind(), targetObj.GetName())
				if diffRes.Modified {
					formatOpts := formatter.AsciiFormatterConfig{
						Coloring: terminal.IsTerminal(int(os.Stdout.Fd())),
					}
					out, err := diffResults.Diffs[i].ASCIIFormat(targetObj, formatOpts)
					errors.CheckError(err)
					fmt.Println(out)
				}
			}
		},
	}
	return command
}

// NewApplicationDeleteCommand returns a new instance of an `argocd app delete` command
func NewApplicationDeleteCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		cascade bool
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
			defer util.Close(conn)
			for _, appName := range args {
				appDeleteReq := application.DeleteApplicationRequest{
					Name: &appName,
				}
				if c.Flag("cascade").Changed {
					appDeleteReq.Cascade = &cascade
				}
				_, err := appIf.Delete(context.Background(), &appDeleteReq)
				errors.CheckError(err)
			}
		},
	}
	command.Flags().BoolVar(&cascade, "cascade", true, "Perform a cascaded deletion of all application resources")
	return command
}

// NewApplicationListCommand returns a new instance of an `argocd app list` command
func NewApplicationListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output string
	)
	var command = &cobra.Command{
		Use:   "list",
		Short: "List applications",
		Run: func(c *cobra.Command, args []string) {
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			var fmtStr string
			headers := []interface{}{"NAME", "CLUSTER", "NAMESPACE", "STATUS", "HEALTH"}
			if output == "wide" {
				fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
				headers = append(headers, "ENV", "REPO", "TARGET")
			} else {
				fmtStr = "%s\t%s\t%s\t%s\t%s\n"
			}
			fmt.Fprintf(w, fmtStr, headers...)
			for _, app := range apps.Items {
				vals := []interface{}{
					app.Name,
					app.Spec.Destination.Server,
					app.Spec.Destination.Namespace,
					app.Status.ComparisonResult.Status,
					app.Status.Health.Status,
				}
				if output == "wide" {
					vals = append(vals, app.Spec.Source.Environment, app.Spec.Source.RepoURL, app.Spec.Source.TargetRevision)
				}
				fmt.Fprintf(w, fmtStr, vals...)
			}
			_ = w.Flush()
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: wide")
	return command
}

// NewApplicationWaitCommand returns a new instance of an `argocd app wait` command
func NewApplicationWaitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		syncOnly   bool
		healthOnly bool
		timeout    uint
	)
	const defaultCheckTimeoutSeconds = 0
	var command = &cobra.Command{
		Use:   "wait APPNAME",
		Short: "Wait for an application to reach a synced and healthy state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if syncOnly && healthOnly {
				log.Fatalln("Please specify at most one of --sync-only or --health-only.")
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)

			appName := args[0]
			wc, err := appIf.Watch(context.Background(), &application.ApplicationQuery{
				Name: &appName,
			})
			errors.CheckError(err)

			success := util.Wait(timeout, func(done chan<- bool) {
				for {
					appEvent, err := wc.Recv()
					errors.CheckError(err)

					app := appEvent.Application
					healthStatus := app.Status.Health.Status
					syncStatus := app.Status.ComparisonResult.Status

					log.Printf("App %q has sync status %q and health status %q", appName, syncStatus, healthStatus)
					synced := (syncStatus == argoappv1.ComparisonStatusSynced)
					healthy := (healthStatus == argoappv1.HealthStatusHealthy)

					if (synced && healthy) || (synced && syncOnly) || (healthy && healthOnly) {
						done <- true
					}
				}
			})

			if success {
				log.Printf("App %q matches desired state", appName)
			} else {
				app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
				errors.CheckError(err)

				log.Errorf("Timed out before seeing app %q match desired state", appName)
				if len(app.Status.ComparisonResult.Resources) > 0 {
					for _, res := range app.Status.ComparisonResult.Resources {
						targetObj, err := argoappv1.UnmarshalToUnstructured(res.TargetState)
						errors.CheckError(err)
						if res.Status != argoappv1.ComparisonStatusSynced || res.Health.Status != argoappv1.HealthStatusHealthy {
							log.Warnf("%s %q has sync status %q and health status %q: %s", targetObj.GetKind(), targetObj.GetName(), res.Status, res.Health.Status, res.Health.StatusDetails)
						}
					}
				}
			}
		},
	}
	command.Flags().BoolVar(&syncOnly, "sync-only", false, "Wait only for sync")
	command.Flags().BoolVar(&healthOnly, "health-only", false, "Wait only for health")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	return command
}

// NewApplicationSyncCommand returns a new instance of an `argocd app sync` command
func NewApplicationSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		revision string
		prune    bool
		dryRun   bool
	)
	var command = &cobra.Command{
		Use:   "sync APPNAME",
		Short: "Sync an application to its target state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			syncReq := application.ApplicationSyncRequest{
				Name:     &appName,
				DryRun:   dryRun,
				Revision: revision,
				Prune:    prune,
			}
			_, err := appIf.Sync(context.Background(), &syncReq)
			errors.CheckError(err)
			status, err := waitUntilOperationCompleted(appIf, appName)
			errors.CheckError(err)
			printOperationResult(appName, status)
			if !status.Phase.Successful() {
				os.Exit(1)
			}
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview apply without affecting cluster")
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	command.Flags().StringVar(&revision, "revision", "", "Sync to a specific revision. Preserves parameter overrides")
	return command
}

func waitUntilOperationCompleted(appClient application.ApplicationServiceClient, appName string) (*argoappv1.OperationState, error) {
	wc, err := appClient.Watch(context.Background(), &application.ApplicationQuery{
		Name: &appName,
	})
	if err != nil {
		return nil, err
	}
	appEvent, err := wc.Recv()
	if err != nil {
		return nil, err
	}
	for {
		if appEvent.Application.Status.OperationState != nil && appEvent.Application.Status.OperationState.Phase.Completed() {
			return appEvent.Application.Status.OperationState, nil
		}
		appEvent, err = wc.Recv()
		if err != nil {
			return nil, err
		}
	}
}

// setParameterOverrides updates an existing or appends a new parameter override in the application
func setParameterOverrides(app *argoappv1.Application, parameters []string) {
	if len(parameters) == 0 {
		return
	}
	var newParams []argoappv1.ComponentParameter
	if len(app.Spec.Source.ComponentParameterOverrides) > 0 {
		newParams = app.Spec.Source.ComponentParameterOverrides
	} else {
		newParams = make([]argoappv1.ComponentParameter, 0)
	}
	for _, paramStr := range parameters {
		parts := strings.SplitN(paramStr, "=", 3)
		if len(parts) != 3 {
			log.Fatalf("Expected parameter of the form: component=param=value. Received: %s", paramStr)
		}
		newParam := argoappv1.ComponentParameter{
			Component: parts[0],
			Name:      parts[1],
			Value:     parts[2],
		}
		index := -1
		for i, cp := range newParams {
			if cp.Component == newParam.Component && cp.Name == newParam.Name {
				index = i
				break
			}
		}
		if index == -1 {
			newParams = append(newParams, newParam)
		} else {
			newParams[index] = newParam
		}
	}
	app.Spec.Source.ComponentParameterOverrides = newParams
}

// NewApplicationHistoryCommand returns a new instance of an `argocd app history` command
func NewApplicationHistoryCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "history APPNAME",
		Short: "Show application deployment history",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "ID\tDATE\tCOMMIT\tPARAMETERS\n")
			for _, depInfo := range app.Status.History {
				paramStr := paramString(depInfo.Params)
				fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", depInfo.ID, depInfo.DeployedAt, depInfo.Revision, paramStr)
			}
			_ = w.Flush()
		},
	}
	return command
}

func paramString(params []argoappv1.ComponentParameter) string {
	if len(params) == 0 {
		return ""
	}
	paramNames := []string{}
	for _, param := range params {
		paramNames = append(paramNames, fmt.Sprintf("%s=%s=%s", param.Component, param.Name, param.Value))
	}
	return strings.Join(paramNames, ",")
}

// NewApplicationRollbackCommand returns a new instance of an `argocd app rollback` command
func NewApplicationRollbackCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		prune bool
	)
	var command = &cobra.Command{
		Use:   "rollback APPNAME",
		Short: "Rollback application to a previous deployed version",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			depID, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			ctx := context.Background()
			app, err := appIf.Get(ctx, &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			var depInfo *argoappv1.DeploymentInfo
			for _, di := range app.Status.History {
				if di.ID == int64(depID) {
					depInfo = &di
					break
				}
			}
			if depInfo == nil {
				log.Fatalf("Application '%s' does not have deployment id '%d' in history\n", app.ObjectMeta.Name, depID)
			}

			_, err = appIf.Rollback(ctx, &application.ApplicationRollbackRequest{
				Name:  &appName,
				ID:    int64(depID),
				Prune: prune,
			})
			errors.CheckError(err)

			status, err := waitUntilOperationCompleted(appIf, appName)
			errors.CheckError(err)
			printOperationResult(appName, status)
			if !status.Phase.Successful() {
				os.Exit(1)
			}
		},
	}
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	return command
}

const printOpFmtStr = "%-20s%s\n"

func printOperationResult(appName string, opState *argoappv1.OperationState) {
	fmt.Printf(printOpFmtStr, "Application:", appName)
	var syncRes *argoappv1.SyncOperationResult
	if opState.SyncResult != nil {
		syncRes = opState.SyncResult
		fmt.Printf(printOpFmtStr, "Operation:", "Sync")
	} else if opState.RollbackResult != nil {
		fmt.Printf(printOpFmtStr, "Operation:", "Rollback")
		syncRes = opState.RollbackResult
	}
	fmt.Printf(printOpFmtStr, "Phase:", opState.Phase)
	if opState.Message != "" {
		fmt.Printf(printOpFmtStr, "Message:", opState.Message)
	}
	if syncRes != nil {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Printf("\n")
		fmt.Fprintf(w, "KIND\tNAME\tMESSAGE\n")
		for _, resDetails := range syncRes.Resources {
			fmt.Fprintf(w, "%s\t%s\t%s\n", resDetails.Kind, resDetails.Name, resDetails.Message)
		}
		_ = w.Flush()
	}
}
