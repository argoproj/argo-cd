package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yudai/gojsondiff/formatter"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/diff"
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
	command.AddCommand(NewApplicationUnsetCommand(clientOpts))
	command.AddCommand(NewApplicationSyncCommand(clientOpts))
	command.AddCommand(NewApplicationHistoryCommand(clientOpts))
	command.AddCommand(NewApplicationRollbackCommand(clientOpts))
	command.AddCommand(NewApplicationListCommand(clientOpts))
	command.AddCommand(NewApplicationDeleteCommand(clientOpts))
	command.AddCommand(NewApplicationWaitCommand(clientOpts))
	command.AddCommand(NewApplicationManifestsCommand(clientOpts))
	command.AddCommand(NewApplicationTerminateOpCommand(clientOpts))
	return command
}

// NewApplicationCreateCommand returns a new instance of an `argocd app create` command
func NewApplicationCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts appOptions
		fileURL string
		appName string
		upsert  bool
	)
	var command = &cobra.Command{
		Use:   "create APPNAME",
		Short: "Create an application from a git location",
		Run: func(c *cobra.Command, args []string) {
			var app argoappv1.Application
			if fileURL != "" {
				parsedURL, err := url.ParseRequestURI(fileURL)
				if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
					err = config.UnmarshalLocalFile(fileURL, &app)
				} else {
					err = config.UnmarshalRemoteFile(fileURL, &app)
				}
				errors.CheckError(err)
			} else {
				if len(args) == 1 {
					if appName != "" && appName != args[0] {
						log.Fatalf("--name argument '%s' does not match app name %s", appName, args[0])
					}
					appName = args[0]
				}
				if appOpts.repoURL == "" || appOpts.appPath == "" || appName == "" {
					log.Fatal("name, repo, path are required")
				}
				app = argoappv1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: appName,
					},
					Spec: argoappv1.ApplicationSpec{
						Project: appOpts.project,
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
			if appOpts.namePrefix != "" {
				app.Spec.Source.NamePrefix = appOpts.namePrefix
			}
			setParameterOverrides(&app, appOpts.parameters)
			if len(appOpts.valuesFiles) > 0 {
				app.Spec.Source.ValuesFiles = appOpts.valuesFiles
			}
			switch appOpts.syncPolicy {
			case "automated":
				app.Spec.SyncPolicy = &argoappv1.SyncPolicy{
					Automated: &argoappv1.SyncPolicyAutomated{
						Prune: appOpts.autoPrune,
					},
				}
			case "none", "":
				app.Spec.SyncPolicy = nil
			default:
				log.Fatalf("Invalid sync-policy: %s", appOpts.syncPolicy)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appCreateRequest := application.ApplicationCreateRequest{
				Application: app,
				Upsert:      &upsert,
			}
			created, err := appIf.Create(context.Background(), &appCreateRequest)
			errors.CheckError(err)
			fmt.Printf("application '%s' created\n", created.ObjectMeta.Name)
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override application with the same name even if supplied application spec is different from existing spec")
	addAppFlags(command, &appOpts)
	return command
}

// NewApplicationGetCommand returns a new instance of an `argocd app get` command
func NewApplicationGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		output        string
		showParams    bool
		showOperation bool
		refresh       bool
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
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName, Refresh: refresh})
			errors.CheckError(err)
			switch output {
			case "yaml":
				yamlBytes, err := yaml.Marshal(app)
				errors.CheckError(err)
				fmt.Println(string(yamlBytes))
			case "json":
				jsonBytes, err := json.MarshalIndent(app, "", "  ")
				errors.CheckError(err)
				fmt.Println(string(jsonBytes))
			case "":
				fmt.Printf(printOpFmtStr, "Name:", app.Name)
				fmt.Printf(printOpFmtStr, "Server:", app.Spec.Destination.Server)
				fmt.Printf(printOpFmtStr, "Namespace:", app.Spec.Destination.Namespace)
				fmt.Printf(printOpFmtStr, "URL:", appURL(acdClient, app))
				fmt.Printf(printOpFmtStr, "Repo:", app.Spec.Source.RepoURL)
				fmt.Printf(printOpFmtStr, "Target:", app.Spec.Source.TargetRevision)
				fmt.Printf(printOpFmtStr, "Path:", app.Spec.Source.Path)
				if app.Spec.Source.Environment != "" {
					fmt.Printf(printOpFmtStr, "Environment:", app.Spec.Source.Environment)
				}
				if len(app.Spec.Source.ValuesFiles) > 0 {
					fmt.Printf(printOpFmtStr, "Helm Values:", strings.Join(app.Spec.Source.ValuesFiles, ","))
				}
				if len(app.Spec.Source.NamePrefix) > 0 {
					fmt.Printf(printOpFmtStr, "Name Prefix:", app.Spec.Source.NamePrefix)
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
				if len(app.Status.ComparisonResult.Resources) > 0 {
					fmt.Println()
					w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
					printAppResources(w, app, showOperation)
					_ = w.Flush()
				}
			default:
				log.Fatalf("Unknown output format: %s", output)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: yaml, json")
	command.Flags().BoolVar(&showOperation, "show-operation", false, "Show application operation")
	command.Flags().BoolVar(&showParams, "show-params", false, "Show application parameters and overrides")
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	return command
}

func printAppConditions(w io.Writer, app *argoappv1.Application) {
	fmt.Fprintf(w, "CONDITION\tMESSAGE\n")
	for _, item := range app.Status.Conditions {
		fmt.Fprintf(w, "%s\t%s", item.Type, item.Message)
	}
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
	return fmt.Sprintf("%s://%s/applications/%s", scheme, server, app.Name)
}

func truncateString(str string, num int) string {
	bnoden := str
	if len(str) > num {
		if num > 3 {
			num -= 3
		}
		bnoden = str[0:num] + "..."
	}
	return bnoden
}

// printParams prints parameters and overrides
func printParams(app *argoappv1.Application) {
	paramLenLimit := 80
	overrides := make(map[string]string)
	for _, p := range app.Spec.Source.ComponentParameterOverrides {
		overrides[fmt.Sprintf("%s/%s", p.Component, p.Name)] = p.Value
	}
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	isKsonnet := app.Spec.Source.Environment != ""
	if isKsonnet {
		fmt.Fprintf(w, "COMPONENT\tNAME\tVALUE\tOVERRIDE\n")
		for _, p := range app.Status.Parameters {
			overrideValue := overrides[fmt.Sprintf("%s/%s", p.Component, p.Name)]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Component, p.Name, truncateString(p.Value, paramLenLimit), truncateString(overrideValue, paramLenLimit))
		}
	} else {
		fmt.Fprintf(w, "NAME\tVALUE\n")
		for _, p := range app.Spec.Source.ComponentParameterOverrides {
			fmt.Fprintf(w, "%s\t%s\n", p.Name, truncateString(p.Value, paramLenLimit))
		}

	}
	_ = w.Flush()
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
				case "values":
					app.Spec.Source.ValuesFiles = appOpts.valuesFiles
				case "dest-server":
					app.Spec.Destination.Server = appOpts.destServer
				case "dest-namespace":
					app.Spec.Destination.Namespace = appOpts.destNamespace
				case "project":
					app.Spec.Project = appOpts.project
				case "name-prefix":
					app.Spec.Source.NamePrefix = appOpts.namePrefix
				case "sync-policy":
					switch appOpts.syncPolicy {
					case "automated":
						app.Spec.SyncPolicy = &argoappv1.SyncPolicy{
							Automated: &argoappv1.SyncPolicyAutomated{},
						}
					case "none":
						app.Spec.SyncPolicy = nil
					default:
						log.Fatalf("Invalid sync-policy: %s", appOpts.syncPolicy)
					}
				}
			})
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if c.Flags().Changed("auto-prune") {
				if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
					log.Fatal("Cannot set --auto-prune: application not configured with automatic sync")
				}
				app.Spec.SyncPolicy.Automated.Prune = appOpts.autoPrune
			}

			setParameterOverrides(app, appOpts.parameters)
			oldOverrides := app.Spec.Source.ComponentParameterOverrides
			updatedSpec, err := appIf.UpdateSpec(context.Background(), &application.ApplicationUpdateSpecRequest{
				Name: &app.Name,
				Spec: app.Spec,
			})
			errors.CheckError(err)

			newOverrides := updatedSpec.Source.ComponentParameterOverrides
			checkDroppedParams(newOverrides, oldOverrides)
		},
	}
	addAppFlags(command, &appOpts)
	return command
}

func checkDroppedParams(newOverrides []argoappv1.ComponentParameter, oldOverrides []argoappv1.ComponentParameter) {
	newOverrideMap := argo.ParamToMap(newOverrides)

	if len(oldOverrides) > len(newOverrides) {
		for _, oldOverride := range oldOverrides {
			if !argo.CheckValidParam(newOverrideMap, oldOverride) {
				log.Warnf("Parameter %s in %s does not exist in ksonnet, parameter override dropped", oldOverride.Name, oldOverride.Component)
			}
		}
	}
}

type appOptions struct {
	repoURL       string
	appPath       string
	env           string
	revision      string
	destServer    string
	destNamespace string
	parameters    []string
	valuesFiles   []string
	project       string
	syncPolicy    string
	autoPrune     bool
	namePrefix    string
}

func addAppFlags(command *cobra.Command, opts *appOptions) {
	command.Flags().StringVar(&opts.repoURL, "repo", "", "Repository URL, ignored if a file is set")
	command.Flags().StringVar(&opts.appPath, "path", "", "Path in repository to the ksonnet app directory, ignored if a file is set")
	command.Flags().StringVar(&opts.env, "env", "", "Application environment to monitor")
	command.Flags().StringVar(&opts.revision, "revision", "HEAD", "The tracking source branch, tag, or commit the application will sync to")
	command.Flags().StringVar(&opts.destServer, "dest-server", "", "K8s cluster URL (overrides the server URL specified in the ksonnet app.yaml)")
	command.Flags().StringVar(&opts.destNamespace, "dest-namespace", "", "K8s target namespace (overrides the namespace specified in the ksonnet app.yaml)")
	command.Flags().StringArrayVarP(&opts.parameters, "parameter", "p", []string{}, "set a parameter override (e.g. -p guestbook=image=example/guestbook:latest)")
	command.Flags().StringArrayVar(&opts.valuesFiles, "values", []string{}, "Helm values file(s) to use")
	command.Flags().StringVar(&opts.project, "project", "", "Application project name")
	command.Flags().StringVar(&opts.syncPolicy, "sync-policy", "", "Set the sync policy (one of: automated, none)")
	command.Flags().BoolVar(&opts.autoPrune, "auto-prune", false, "Set automatic pruning when sync is automated")
	command.Flags().StringVar(&opts.namePrefix, "name-prefix", "", "Set a prefix to add to resource names for kustomize and helm app")
}

// NewApplicationUnsetCommand returns a new instance of an `argocd app unset` command
func NewApplicationUnsetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		parameters  []string
		valuesFiles []string
		namePrefix  bool
	)
	var command = &cobra.Command{
		Use:   "unset APPNAME -p COMPONENT=PARAM",
		Short: "Unset application parameters",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 || (len(parameters) == 0 && len(valuesFiles) == 0 && !namePrefix) {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			isKsonnetApp := app.Spec.Source.Environment != ""

			updated := false
			for _, paramStr := range parameters {
				if isKsonnetApp {
					parts := strings.SplitN(paramStr, "=", 2)
					if len(parts) != 2 {
						log.Fatalf("Expected parameter of the form: component=param. Received: %s", paramStr)
					}
					overrides := app.Spec.Source.ComponentParameterOverrides
					for i, override := range overrides {
						if override.Component == parts[0] && override.Name == parts[1] {
							app.Spec.Source.ComponentParameterOverrides = append(overrides[0:i], overrides[i+1:]...)
							updated = true
							break
						}
					}
				} else {
					overrides := app.Spec.Source.ComponentParameterOverrides
					for i, override := range overrides {
						if override.Name == paramStr {
							app.Spec.Source.ComponentParameterOverrides = append(overrides[0:i], overrides[i+1:]...)
							updated = true
							break
						}
					}
				}
			}
			for _, valuesFile := range valuesFiles {
				for i, vf := range app.Spec.Source.ValuesFiles {
					if vf == valuesFile {
						app.Spec.Source.ValuesFiles = append(app.Spec.Source.ValuesFiles[0:i], app.Spec.Source.ValuesFiles[i+1:]...)
						updated = true
						break
					}
				}
			}
			if namePrefix {
				app.Spec.Source.NamePrefix = ""
				updated = true
			}

			if !updated {
				return
			}
			_, err = appIf.UpdateSpec(context.Background(), &application.ApplicationUpdateSpecRequest{
				Name: &app.Name,
				Spec: app.Spec,
			})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVarP(&parameters, "parameter", "p", []string{}, "unset a parameter override (e.g. -p guestbook=image)")
	command.Flags().StringArrayVar(&valuesFiles, "values", []string{}, "unset one or more helm values files")
	command.Flags().BoolVar(&namePrefix, "name-prefix", false, "Unset the name prefix")

	return command
}

// NewApplicationDiffCommand returns a new instance of an `argocd app diff` command
func NewApplicationDiffCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		refresh bool
		local   string
		env     string
	)
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
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName, Refresh: refresh})
			errors.CheckError(err)

			compareObjs, err := app.Status.ComparisonResult.TargetObjects()
			errors.CheckError(err)

			liveObjs, err := app.Status.ComparisonResult.LiveObjects()
			errors.CheckError(err)

			diffManifestResponse, err := appIf.DiffManifests(context.Background(), &application.ApplicationManifestDiffRequest{
				AppName: &appName,
			})
			errors.CheckError(err)

			diffs := diffManifestResponse.Diffs

			for i := 0; i < len(compareObjs); i++ {
				kind, name := getObjKindName(compareObjs[i], liveObjs[i])
				diffResult, err := diff.UnmarshalDiffString(diffs[i])
				errors.CheckError(err)
				fmt.Printf("===== %s %s ======\n", kind, name)
				if diffResult.Modified {
					formatOpts := formatter.AsciiFormatterConfig{
						Coloring: terminal.IsTerminal(int(os.Stdout.Fd())),
					}
					out, err := diffResult.ASCIIFormat(compareObjs[i], formatOpts)
					errors.CheckError(err)
					fmt.Println(out)
				}
			}
			if local != "" && len(app.Spec.Source.ComponentParameterOverrides) > 0 {
				log.Warnf("Unable to display parameter overrides")
			}
		},
	}
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().StringVar(&local, "local", "", "Compare live app to a local ksonnet app")
	command.Flags().StringVar(&env, "env", "", "Compare live app to a specific environment")
	return command
}

func getObjKindName(compare, live *unstructured.Unstructured) (string, string) {
	if compare == nil {
		return live.GetKind(), live.GetName()
	}
	return compare.GetKind(), compare.GetName()
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
				appDeleteReq := application.ApplicationDeleteRequest{
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
			headers := []interface{}{"NAME", "CLUSTER", "NAMESPACE", "PROJECT", "STATUS", "HEALTH", "CONDITIONS"}
			if output == "wide" {
				fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
				headers = append(headers, "ENV", "REPO", "PATH", "TARGET")
			} else {
				fmtStr = "%s\t%s\t%s\t%s\t%s\t%s\t%s\n"
			}
			fmt.Fprintf(w, fmtStr, headers...)
			for _, app := range apps.Items {
				vals := []interface{}{
					app.Name,
					app.Spec.Destination.Server,
					app.Spec.Destination.Namespace,
					app.Spec.GetProject(),
					app.Status.ComparisonResult.Status,
					app.Status.Health.Status,
					formatConditionsSummary(app),
				}
				if output == "wide" {
					vals = append(vals, app.Spec.Source.Environment, app.Spec.Source.RepoURL, app.Spec.Source.Path, app.Spec.Source.TargetRevision)
				}
				fmt.Fprintf(w, fmtStr, vals...)
			}
			_ = w.Flush()
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: wide")
	return command
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

// NewApplicationWaitCommand returns a new instance of an `argocd app wait` command
func NewApplicationWaitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		watchSync       bool
		watchHealth     bool
		watchOperations bool
		timeout         uint
	)
	var command = &cobra.Command{
		Use:   "wait APPNAME",
		Short: "Wait for an application to reach a synced and healthy state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			if !watchSync && !watchHealth && !watchOperations {
				watchSync = true
				watchHealth = true
				watchOperations = true
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)

			_, err := waitOnApplicationStatus(appIf, appName, timeout, watchSync, watchHealth, watchOperations, nil)
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVar(&watchSync, "sync", false, "Wait for sync")
	command.Flags().BoolVar(&watchHealth, "health", false, "Wait for health")
	command.Flags().BoolVar(&watchOperations, "operation", false, "Wait for pending operations")
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	return command
}

func isCanceledContextErr(err error) bool {
	if err == context.Canceled {
		return true
	}
	if stat, ok := status.FromError(err); ok {
		if stat.Code() == codes.Canceled {
			return true
		}
	}
	return false
}

// watchApp returns a channel of watch events for an app, retrying the watch upon errors. Closes
// the returned channel when the context is discovered to be canceled.
func watchApp(ctx context.Context, appIf application.ApplicationServiceClient, appName string) chan *argoappv1.ApplicationWatchEvent {
	appEventsCh := make(chan *argoappv1.ApplicationWatchEvent)
	go func() {
		defer close(appEventsCh)
		for {
			wc, err := appIf.Watch(ctx, &application.ApplicationQuery{
				Name: &appName,
			})
			if err != nil {
				if isCanceledContextErr(err) {
					return
				}
				if err != io.EOF {
					log.Warnf("watch err: %v", err)
				}
				time.Sleep(1 * time.Second)
				continue
			}
			for {
				appEvent, err := wc.Recv()
				if err != nil {
					if isCanceledContextErr(err) {
						return
					}
					if err != io.EOF {
						log.Warnf("recv err: %v", err)
					}
					time.Sleep(1 * time.Second)
					break
				} else {
					appEventsCh <- appEvent
				}
			}
		}

	}()
	return appEventsCh
}

// printAppResources prints the resources of an application in a tabwriter table
// Optionally prints the message from the operation state
func printAppResources(w io.Writer, app *argoappv1.Application, showOperation bool) {
	messages := make(map[string]string)
	opState := app.Status.OperationState
	var syncRes *argoappv1.SyncOperationResult

	if showOperation {
		fmt.Fprintf(w, "KIND\tNAME\tSTATUS\tHEALTH\tHOOK\tOPERATIONMSG\n")
		if opState != nil {
			if opState.SyncResult != nil {
				syncRes = opState.SyncResult
			}
		}
		if syncRes != nil {
			for _, resDetails := range syncRes.Resources {
				messages[fmt.Sprintf("%s/%s", resDetails.Kind, resDetails.Name)] = resDetails.Message
			}
			for _, hook := range syncRes.Hooks {
				if hook.Type == argoappv1.HookTypePreSync {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", hook.Kind, hook.Name, hook.Status, "", hook.Type, hook.Message)
				}
			}
		}
	} else {
		fmt.Fprintf(w, "KIND\tNAME\tSTATUS\tHEALTH\n")
	}
	for _, res := range app.Status.ComparisonResult.Resources {
		obj, err := argoappv1.UnmarshalToUnstructured(res.TargetState)
		errors.CheckError(err)
		if obj == nil {
			obj, err = argoappv1.UnmarshalToUnstructured(res.LiveState)
			errors.CheckError(err)
		}
		if showOperation {
			message := messages[fmt.Sprintf("%s/%s", obj.GetKind(), obj.GetName())]
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s", obj.GetKind(), obj.GetName(), res.Status, res.Health.Status, "", message)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s", obj.GetKind(), obj.GetName(), res.Status, res.Health.Status)
		}
		fmt.Fprint(w, "\n")
	}
	if showOperation && syncRes != nil {
		for _, hook := range syncRes.Hooks {
			if hook.Type == argoappv1.HookTypeSync || hook.Type == argoappv1.HookTypePostSync {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", hook.Kind, hook.Name, hook.Status, "", hook.Type, hook.Message)
			}
		}

	}
}

// NewApplicationSyncCommand returns a new instance of an `argocd app sync` command
func NewApplicationSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		revision  string
		resources *[]string
		prune     bool
		dryRun    bool
		timeout   uint
		strategy  string
		force     bool
	)
	const (
		resourceFieldDelimiter = ":"
		resourceFieldCount     = 3
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
			var syncResources []argoappv1.SyncOperationResource
			if resources != nil {
				syncResources = []argoappv1.SyncOperationResource{}
				for _, r := range *resources {
					fields := strings.Split(r, resourceFieldDelimiter)
					if len(fields) != resourceFieldCount {
						log.Fatalf("Resource should have GROUP%sKIND%sNAME, but instead got: %s", resourceFieldDelimiter, resourceFieldDelimiter, r)
					}
					rsrc := argoappv1.SyncOperationResource{
						Group: fields[0],
						Kind:  fields[1],
						Name:  fields[2],
					}
					syncResources = append(syncResources, rsrc)
				}
			}
			syncReq := application.ApplicationSyncRequest{
				Name:      &appName,
				DryRun:    dryRun,
				Revision:  revision,
				Resources: syncResources,
				Prune:     prune,
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
			ctx := context.Background()
			_, err := appIf.Sync(ctx, &syncReq)
			errors.CheckError(err)

			app, err := waitOnApplicationStatus(appIf, appName, timeout, false, false, true, syncResources)
			errors.CheckError(err)

			pruningRequired := 0
			for _, resDetails := range app.Status.OperationState.SyncResult.Resources {
				if resDetails.Status == argoappv1.ResourceDetailsPruningRequired {
					pruningRequired++
				}
			}
			if pruningRequired > 0 {
				log.Fatalf("%d resources require pruning", pruningRequired)
			}

			if !app.Status.OperationState.Phase.Successful() && !dryRun {
				os.Exit(1)
			}
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview apply without affecting cluster")
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	command.Flags().StringVar(&revision, "revision", "", "Sync to a specific revision. Preserves parameter overrides")
	resources = command.Flags().StringArray("resource", nil, fmt.Sprintf("Sync only specific resources as GROUP%sKIND%sNAME. Fields may be blank. This option may be specified repeatedly", resourceFieldDelimiter, resourceFieldDelimiter))
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	command.Flags().StringVar(&strategy, "strategy", "", "Sync strategy (one of: apply|hook)")
	command.Flags().BoolVar(&force, "force", false, "Use a force apply")
	return command
}

// ResourceState tracks the state of a resource when waiting on an application status.
type resourceState struct {
	Kind    string
	Name    string
	Status  string
	Health  string
	Hook    string
	Message string
}

func newResourceState(kind, name, status, health, hook, message string) *resourceState {
	return &resourceState{
		Kind:    kind,
		Name:    name,
		Status:  status,
		Health:  health,
		Hook:    hook,
		Message: message,
	}
}

// Key returns a unique-ish key for the resource.
func (rs *resourceState) Key() string {
	return fmt.Sprintf("%s/%s", rs.Kind, rs.Name)
}

func (rs *resourceState) String() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", rs.Kind, rs.Name, rs.Status, rs.Health, rs.Hook, rs.Message)
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

func calculateResourceStates(app *argoappv1.Application, syncResources []argoappv1.SyncOperationResource) map[string]*resourceState {
	resStates := make(map[string]*resourceState)
	for _, res := range app.Status.ComparisonResult.Resources {
		obj, err := argoappv1.UnmarshalToUnstructured(res.TargetState)
		errors.CheckError(err)
		if obj == nil {
			obj, err = argoappv1.UnmarshalToUnstructured(res.LiveState)
			errors.CheckError(err)
		}
		if syncResources != nil && !argo.ContainsSyncResource(obj, syncResources) {
			continue
		}
		newState := newResourceState(obj.GetKind(), obj.GetName(), string(res.Status), res.Health.Status, "", "")
		key := newState.Key()
		if prev, ok := resStates[key]; ok {
			prev.Merge(newState)
		} else {
			resStates[key] = newState
		}
	}

	var opResult *argoappv1.SyncOperationResult
	if app.Status.OperationState != nil {
		if app.Status.OperationState.SyncResult != nil {
			opResult = app.Status.OperationState.SyncResult
		}
	}
	if opResult == nil {
		return resStates
	}

	for _, hook := range opResult.Hooks {
		newState := newResourceState(hook.Kind, hook.Name, string(hook.Status), "", string(hook.Type), hook.Message)
		key := newState.Key()
		if prev, ok := resStates[key]; ok {
			prev.Merge(newState)
		} else {
			resStates[key] = newState
		}
	}

	for _, res := range opResult.Resources {
		newState := newResourceState(res.Kind, res.Name, "", "", "", res.Message)
		key := newState.Key()
		if prev, ok := resStates[key]; ok {
			prev.Merge(newState)
		} else {
			resStates[key] = newState
		}
	}
	return resStates
}

func waitOnApplicationStatus(appClient application.ApplicationServiceClient, appName string, timeout uint, watchSync bool, watchHealth bool, watchOperation bool, syncResources []argoappv1.SyncOperationResource) (*argoappv1.Application, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// refresh controls whether or not we refresh the app before printing the final status.
	// We only want to do this when an operation is in progress, since operations are the only
	// time when the sync status lags behind when an operation completes
	refresh := false

	printFinalStatus := func(app *argoappv1.Application) {
		var err error
		if refresh {
			app, err = appClient.Get(context.Background(), &application.ApplicationQuery{Name: &appName, Refresh: true})
			errors.CheckError(err)
		}

		fmt.Println()
		fmt.Printf(printOpFmtStr, "Application:", app.Name)
		if watchOperation {
			printOperationResult(app.Status.OperationState)
		}

		if len(app.Status.ComparisonResult.Resources) > 0 {
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 5, 0, 2, ' ', 0)
			printAppResources(w, app, watchOperation)
			_ = w.Flush()
		}
	}

	if timeout != 0 {
		time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			cancel()
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KIND\tNAME\tSTATUS\tHEALTH\tHOOK\tOPERATIONMSG")

	prevStates := make(map[string]*resourceState)
	appEventCh := watchApp(ctx, appClient, appName)
	var app *argoappv1.Application

	for appEvent := range appEventCh {
		app = &appEvent.Application
		if app.Operation != nil {
			refresh = true
		}
		// consider skipped checks successful
		synced := !watchSync || app.Status.ComparisonResult.Status == argoappv1.ComparisonStatusSynced
		healthy := !watchHealth || app.Status.Health.Status == argoappv1.HealthStatusHealthy
		operational := !watchOperation || appEvent.Application.Operation == nil
		if len(app.Status.GetErrorConditions()) == 0 && synced && healthy && operational {
			printFinalStatus(app)
			return app, nil
		}

		newStates := calculateResourceStates(app, syncResources)
		for _, newState := range newStates {
			var doPrint bool
			stateKey := newState.Key()
			if prevState, found := prevStates[stateKey]; found {
				doPrint = prevState.Merge(newState)
			} else {
				prevStates[stateKey] = newState
				doPrint = true
			}
			if doPrint {
				fmt.Fprintln(w, prevStates[stateKey])
			}
		}
		_ = w.Flush()
	}
	printFinalStatus(app)
	return nil, fmt.Errorf("Timed out (%ds) waiting for app %q match desired state", timeout, appName)
}

// setParameterOverrides updates an existing or appends a new parameter override in the application
// If the app is a ksonnet app, then parameters are expected to be in the form: component=param=value
// Otherwise, the app is assumed to be a helm app and is expected to be in the form:
// param=value
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
	isKsonnetApp := app.Spec.Source.Environment != ""
	for _, paramStr := range parameters {
		var newParam argoappv1.ComponentParameter
		if isKsonnetApp {
			parts := strings.SplitN(paramStr, "=", 3)
			if len(parts) != 3 {
				log.Fatalf("Expected ksonnet parameter of the form: component=param=value. Received: %s", paramStr)
			}
			newParam = argoappv1.ComponentParameter{
				Component: parts[0],
				Name:      parts[1],
				Value:     parts[2],
			}
		} else {
			parts := strings.SplitN(paramStr, "=", 2)
			if len(parts) != 2 {
				log.Fatalf("Expected helm parameter of the form: param=value. Received: %s", paramStr)
			}
			newParam = argoappv1.ComponentParameter{
				Name:  parts[0],
				Value: parts[1],
			}
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
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			switch output {
			case "wide":
				fmt.Fprintf(w, "ID\tDATE\tCOMMIT\tPARAMETERS\n")
			default:
				fmt.Fprintf(w, "ID\tDATE\tCOMMIT\n")
			}
			for _, depInfo := range app.Status.History {
				switch output {
				case "wide":
					manifest, err := appIf.GetManifests(context.Background(), &application.ApplicationManifestQuery{Name: &appName, Revision: depInfo.Revision})
					errors.CheckError(err)
					paramStr := paramString(manifest.GetParams())
					fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", depInfo.ID, depInfo.DeployedAt, depInfo.Revision, paramStr)
				default:
					fmt.Fprintf(w, "%d\t%s\t%s\n", depInfo.ID, depInfo.DeployedAt, depInfo.Revision)
				}
			}
			_ = w.Flush()
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: wide")
	return command
}

func paramString(params []*argoappv1.ComponentParameter) string {
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
		prune   bool
		timeout uint
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

			_, err = waitOnApplicationStatus(appIf, appName, timeout, false, false, true, nil)
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
	if opState.SyncResult != nil {
		fmt.Printf(printOpFmtStr, "Operation:", "Sync")
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
			defer util.Close(conn)
			ctx := context.Background()
			app, err := appIf.Get(ctx, &application.ApplicationQuery{Name: &appName})
			errors.CheckError(err)

			var unstructureds []*unstructured.Unstructured
			switch source {
			case "git":
				if revision != "" {
					q := application.ApplicationManifestQuery{
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
					targetObjs, err := app.Status.ComparisonResult.TargetObjects()
					errors.CheckError(err)
					unstructureds = targetObjs
				}
			case "live":
				liveObjs, err := app.Status.ComparisonResult.LiveObjects()
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
			defer util.Close(conn)
			ctx := context.Background()
			_, err := appIf.TerminateOperation(ctx, &application.OperationTerminateRequest{Name: &appName})
			errors.CheckError(err)
			fmt.Printf("Application '%s' operation terminating\n", appName)
		},
	}
	return command
}
