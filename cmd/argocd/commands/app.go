package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/yudai/gojsondiff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/cli"
	"github.com/argoproj/argo-cd/util/config"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/hook"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/resource"
	"github.com/argoproj/argo-cd/util/templates"
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
	command.AddCommand(NewApplicationResourceActionsCommand(clientOpts))
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
			argocdClient := argocdclient.NewClientOrDie(clientOpts)
			if fileURL != "" {
				parsedURL, err := url.ParseRequestURI(fileURL)
				if err != nil || !(parsedURL.Scheme == "http" || parsedURL.Scheme == "https") {
					err = config.UnmarshalLocalFile(fileURL, &app)
				} else {
					err = config.UnmarshalRemoteFile(fileURL, &app)
				}
				errors.CheckError(err)
				if len(args) == 1 && args[0] != app.Name {
					log.Fatalf("app name '%s' does not match app spec metadata.name '%s'", args[0], app.Name)
				}
				if appName != "" && appName != app.Name {
					log.Fatalf("--name argument '%s' does not match app spec metadata.name '%s'", appName, app.Name)
				}
			} else {
				if len(args) == 1 {
					if appName != "" && appName != args[0] {
						log.Fatalf("--name argument '%s' does not match app name %s", appName, args[0])
					}
					appName = args[0]
				}
				app = argoappv1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: appName,
					},
				}
				setAppOptions(c.Flags(), &app, &appOpts)
				setParameterOverrides(&app, appOpts.parameters)
			}
			if app.Name == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdClient.NewApplicationClientOrDie()
			defer util.Close(conn)
			appCreateRequest := applicationpkg.ApplicationCreateRequest{
				Application: app,
				Upsert:      &upsert,
			}
			created, err := appIf.Create(context.Background(), &appCreateRequest)
			errors.CheckError(err)
			fmt.Printf("application '%s' created\n", created.ObjectMeta.Name)
		},
	}
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().BoolVar(&upsert, "upsert", false, "Allows to override application with the same name even if supplied application spec is different from existing spec")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	// Only complete files with appropriate extension.
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	if err != nil {
		log.Fatal(err)
	}
	addAppFlags(command, &appOpts)
	return command
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
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: getRefreshType(refresh, hardRefresh)})
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
				aURL := appURL(acdClient, app.Name)
				printAppSummaryTable(app, aURL)

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
				log.Fatalf("Unknown output format: %s", output)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: yaml, json")
	command.Flags().BoolVar(&showOperation, "show-operation", false, "Show application operation")
	command.Flags().BoolVar(&showParams, "show-params", false, "Show application parameters and overrides")
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "Refresh application data as well as target manifests cache")
	return command
}

func printAppSummaryTable(app *argoappv1.Application, appURL string) {
	fmt.Printf(printOpFmtStr, "Name:", app.Name)
	fmt.Printf(printOpFmtStr, "Project:", app.Spec.GetProject())
	fmt.Printf(printOpFmtStr, "Server:", app.Spec.Destination.Server)
	fmt.Printf(printOpFmtStr, "Namespace:", app.Spec.Destination.Namespace)
	fmt.Printf(printOpFmtStr, "URL:", appURL)
	fmt.Printf(printOpFmtStr, "Repo:", app.Spec.Source.RepoURL)
	fmt.Printf(printOpFmtStr, "Target:", app.Spec.Source.TargetRevision)
	fmt.Printf(printOpFmtStr, "Path:", app.Spec.Source.Path)
	printAppSourceDetails(&app.Spec.Source)
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
	healthStr := app.Status.Health.Status
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
	fmt.Fprintf(w, "CONDITION\tMESSAGE\n")
	for _, item := range app.Status.Conditions {
		fmt.Fprintf(w, "%s\t%s\n", item.Type, item.Message)
	}
}

// appURL returns the URL of an application
func appURL(acdClient argocdclient.Client, appName string) string {
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
	fmt.Println()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if app.Spec.Source.Ksonnet != nil {
		fmt.Println()
		fmt.Fprintf(w, "COMPONENT\tNAME\tVALUE\n")
		for _, p := range app.Spec.Source.Ksonnet.Parameters {
			fmt.Fprintf(w, "%s\t%s\t%s\n", p.Component, p.Name, truncateString(p.Value, paramLenLimit))
		}
	} else if app.Spec.Source.Helm != nil {
		fmt.Println()
		fmt.Fprintf(w, "NAME\tVALUE\n")
		for _, p := range app.Spec.Source.Helm.Parameters {
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
			ctx := context.Background()
			appName := args[0]
			argocdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := argocdClient.NewApplicationClientOrDie()
			defer util.Close(conn)
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			visited := setAppOptions(c.Flags(), app, &appOpts)
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			setParameterOverrides(app, appOpts.parameters)
			_, err = appIf.UpdateSpec(ctx, &applicationpkg.ApplicationUpdateSpecRequest{
				Name: &app.Name,
				Spec: app.Spec,
			})
			errors.CheckError(err)
		},
	}
	addAppFlags(command, &appOpts)
	return command
}

func setAppOptions(flags *pflag.FlagSet, app *argoappv1.Application, appOpts *appOptions) int {
	visited := 0
	flags.Visit(func(f *pflag.Flag) {
		visited++
		switch f.Name {
		case "repo":
			app.Spec.Source.RepoURL = appOpts.repoURL
		case "path":
			app.Spec.Source.Path = appOpts.appPath
		case "env":
			setKsonnetOpt(&app.Spec.Source, &appOpts.env)
		case "revision":
			app.Spec.Source.TargetRevision = appOpts.revision
		case "values":
			setHelmOpt(&app.Spec.Source, appOpts.valuesFiles, nil)
		case "release-name":
			setHelmOpt(&app.Spec.Source, nil, &appOpts.releaseName)
		case "directory-recurse":
			app.Spec.Source.Directory = &argoappv1.ApplicationSourceDirectory{Recurse: appOpts.directoryRecurse}
		case "config-management-plugin":
			app.Spec.Source.Plugin = &argoappv1.ApplicationSourcePlugin{Name: appOpts.configManagementPlugin}
		case "dest-server":
			app.Spec.Destination.Server = appOpts.destServer
		case "dest-namespace":
			app.Spec.Destination.Namespace = appOpts.destNamespace
		case "project":
			app.Spec.Project = appOpts.project
		case "nameprefix":
			setKustomizeOpt(&app.Spec.Source, &appOpts.namePrefix)
		case "kustomize-image":
			setKustomizeImages(&app.Spec.Source, appOpts.kustomizeImages)
		case "jsonnet-tla-str":
			setJsonnetOpt(&app.Spec.Source, appOpts.jsonnetTlaStr, false)
		case "jsonnet-tla-code":
			setJsonnetOpt(&app.Spec.Source, appOpts.jsonnetTlaCode, true)
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
	if flags.Changed("auto-prune") {
		if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
			log.Fatal("Cannot set --auto-prune: application not configured with automatic sync")
		}
		app.Spec.SyncPolicy.Automated.Prune = appOpts.autoPrune
	}
	if flags.Changed("self-heal") {
		if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
			log.Fatal("Cannot set --self-helf: application not configured with automatic sync")
		}
		app.Spec.SyncPolicy.Automated.SelfHeal = appOpts.selfHeal
	}

	return visited
}

func setKsonnetOpt(src *argoappv1.ApplicationSource, env *string) {
	if src.Ksonnet == nil {
		src.Ksonnet = &argoappv1.ApplicationSourceKsonnet{}
	}
	if env != nil {
		src.Ksonnet.Environment = *env
	}
	if src.Ksonnet.IsZero() {
		src.Ksonnet = nil
	}
}

func setKustomizeOpt(src *argoappv1.ApplicationSource, namePrefix *string) {
	if src.Kustomize == nil {
		src.Kustomize = &argoappv1.ApplicationSourceKustomize{}
	}
	if namePrefix != nil {
		src.Kustomize.NamePrefix = *namePrefix
	}
	if src.Kustomize.IsZero() {
		src.Kustomize = nil
	}
}
func setKustomizeImages(src *argoappv1.ApplicationSource, images []string) {
	if src.Kustomize == nil {
		src.Kustomize = &argoappv1.ApplicationSourceKustomize{}
	}
	for _, image := range images {
		src.Kustomize.MergeImage(argoappv1.KustomizeImage(image))
	}
	if src.Kustomize.IsZero() {
		src.Kustomize = nil
	}
}

func setHelmOpt(src *argoappv1.ApplicationSource, valueFiles []string, releaseName *string) {
	if src.Helm == nil {
		src.Helm = &argoappv1.ApplicationSourceHelm{}
	}
	if valueFiles != nil {
		src.Helm.ValueFiles = valueFiles
	}
	if releaseName != nil {
		src.Helm.ReleaseName = *releaseName
	}
	if src.Helm.IsZero() {
		src.Helm = nil
	}
}

func setJsonnetOpt(src *argoappv1.ApplicationSource, tlaParameters []string, code bool) {
	if src.Directory == nil {
		src.Directory = &argoappv1.ApplicationSourceDirectory{}
	}

	if len(tlaParameters) != 0 {
		tlas := make([]argoappv1.JsonnetVar, len(tlaParameters))
		for index, paramStr := range tlaParameters {
			parts := strings.SplitN(paramStr, "=", 2)
			if len(parts) != 2 {
				log.Fatalf("Expected parameter of the form: param=value. Received: %s", paramStr)
				break
			}
			tlas[index] = argoappv1.JsonnetVar{
				Name:  parts[0],
				Value: parts[1],
				Code:  code}
		}
		existingTLAs := []argoappv1.JsonnetVar{}
		for i := range src.Directory.Jsonnet.TLAs {
			if src.Directory.Jsonnet.TLAs[i].Code != code {
				existingTLAs = append(existingTLAs, src.Directory.Jsonnet.TLAs[i])
			}
		}
		src.Directory.Jsonnet.TLAs = append(existingTLAs, tlas...)
	}

	if src.Directory.IsZero() {
		src.Directory = nil
	}

}

type appOptions struct {
	repoURL                string
	appPath                string
	env                    string
	revision               string
	destServer             string
	destNamespace          string
	parameters             []string
	valuesFiles            []string
	releaseName            string
	project                string
	syncPolicy             string
	autoPrune              bool
	selfHeal               bool
	namePrefix             string
	directoryRecurse       bool
	configManagementPlugin string
	jsonnetTlaStr          []string
	jsonnetTlaCode         []string
	kustomizeImages        []string
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
	command.Flags().StringVar(&opts.releaseName, "release-name", "", "Helm release-name")
	command.Flags().StringVar(&opts.project, "project", "", "Application project name")
	command.Flags().StringVar(&opts.syncPolicy, "sync-policy", "", "Set the sync policy (one of: automated, none)")
	command.Flags().BoolVar(&opts.autoPrune, "auto-prune", false, "Set automatic pruning when sync is automated")
	command.Flags().BoolVar(&opts.selfHeal, "self-heal", false, "Set self healing when sync is automated")
	command.Flags().StringVar(&opts.namePrefix, "nameprefix", "", "Kustomize nameprefix")
	command.Flags().BoolVar(&opts.directoryRecurse, "directory-recurse", false, "Recurse directory")
	command.Flags().StringVar(&opts.configManagementPlugin, "config-management-plugin", "", "Config management plugin name")
	command.Flags().StringArrayVar(&opts.jsonnetTlaStr, "jsonnet-tla-str", []string{}, "Jsonnet top level string arguments")
	command.Flags().StringArrayVar(&opts.jsonnetTlaCode, "jsonnet-tla-code", []string{}, "Jsonnet top level code arguments")
	command.Flags().StringArrayVar(&opts.kustomizeImages, "kustomize-image", []string{}, "Kustomize images (e.g. --kustomize-image node:8.15.0 --kustomize-image mysql=mariadb,alpine@sha256:24a0c4b4a4c0eb97a1aabb8e29f18e917d05abfe1b7a7c07857230879ce7d3d)")
}

// NewApplicationUnsetCommand returns a new instance of an `argocd app unset` command
func NewApplicationUnsetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		parameters  []string
		valuesFiles []string
	)
	var command = &cobra.Command{
		Use:   "unset APPNAME -p COMPONENT=PARAM",
		Short: "Unset application parameters",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 || (len(parameters) == 0 && len(valuesFiles) == 0) {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)

			updated := false
			if app.Spec.Source.Ksonnet != nil {
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
				specValueFiles := app.Spec.Source.Helm.ValueFiles
				for _, valuesFile := range valuesFiles {
					for i, vf := range specValueFiles {
						if vf == valuesFile {
							specValueFiles = append(specValueFiles[0:i], specValueFiles[i+1:]...)
							updated = true
							break
						}
					}
				}
				setHelmOpt(&app.Spec.Source, specValueFiles, nil)
				if !updated {
					return
				}
			}

			_, err = appIf.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{
				Name: &app.Name,
				Spec: app.Spec,
			})
			errors.CheckError(err)
		},
	}
	command.Flags().StringArrayVarP(&parameters, "parameter", "p", []string{}, "unset a parameter override (e.g. -p guestbook=image)")
	command.Flags().StringArrayVar(&valuesFiles, "values", []string{}, "unset one or more helm values files")
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

func getLocalObjects(app *argoappv1.Application, local string, appLabelKey string) []*unstructured.Unstructured {
	manifestStrings := getLocalObjectsString(app, local, appLabelKey, nil)
	objs := make([]*unstructured.Unstructured, len(manifestStrings))
	for i := range manifestStrings {
		obj := unstructured.Unstructured{}
		err := json.Unmarshal([]byte(manifestStrings[i]), &obj)
		errors.CheckError(err)
		objs[i] = &obj
	}
	return objs
}

func getLocalObjectsString(app *argoappv1.Application, local string, appLabelKey string, kustomizeOptions *argoappv1.KustomizeOptions) []string {
	res, err := repository.GenerateManifests(filepath.Dir(local), filepath.Base(local), &repoapiclient.ManifestRequest{
		ApplicationSource: &app.Spec.Source,
		AppLabelKey:       appLabelKey,
		AppLabelValue:     app.Name,
		Namespace:         app.Spec.Destination.Namespace,
		KustomizeOptions:  kustomizeOptions,
	})
	errors.CheckError(err)

	return res.Manifests
}

type resourceInfoProvider struct {
	namespacedByGk map[schema.GroupKind]bool
}

// Infer if obj is namespaced or not from corresponding live objects list. If corresponding live object has namespace then target object is also namespaced.
// If live object is missing then it does not matter if target is namespaced or not.
func (p *resourceInfoProvider) IsNamespaced(server string, obj *unstructured.Unstructured) (bool, error) {
	key := kube.GetResourceKey(obj)
	return p.namespacedByGk[key.GroupKind()], nil
}

func groupLocalObjs(localObs []*unstructured.Unstructured, liveObjs []*unstructured.Unstructured, appNamespace string) map[kube.ResourceKey]*unstructured.Unstructured {
	namespacedByGk := make(map[schema.GroupKind]bool)
	for i := range liveObjs {
		if liveObjs[i] != nil {
			key := kube.GetResourceKey(liveObjs[i])
			namespacedByGk[schema.GroupKind{Group: key.Group, Kind: key.Kind}] = key.Namespace != ""
		}
	}
	localObs, _, err := controller.DeduplicateTargetObjects("", appNamespace, localObs, &resourceInfoProvider{namespacedByGk: namespacedByGk})
	errors.CheckError(err)
	objByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for i := range localObs {
		obj := localObs[i]
		if !(hook.IsHook(obj) || resource.Ignore(obj)) {
			objByKey[kube.GetResourceKey(obj)] = obj
		}
	}
	return objByKey
}

// NewApplicationDiffCommand returns a new instance of an `argocd app diff` command
func NewApplicationDiffCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		refresh     bool
		hardRefresh bool
		local       string
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
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: getRefreshType(refresh, hardRefresh)})
			errors.CheckError(err)
			resources, err := appIf.ManagedResources(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &appName})
			errors.CheckError(err)
			liveObjs, err := liveObjects(resources.Items)
			errors.CheckError(err)
			items := make([]struct {
				key    kube.ResourceKey
				live   *unstructured.Unstructured
				target *unstructured.Unstructured
			}, 0)

			conn, settingsIf := clientset.NewSettingsClientOrDie()
			defer util.Close(conn)
			argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
			errors.CheckError(err)

			if local != "" {
				localObjs := groupLocalObjs(getLocalObjects(app, local, argoSettings.AppLabelKey), liveObjs, app.Spec.Destination.Namespace)
				for _, res := range resources.Items {
					var live = &unstructured.Unstructured{}
					err := json.Unmarshal([]byte(res.LiveState), &live)
					errors.CheckError(err)

					var key kube.ResourceKey
					if live != nil {
						key = kube.GetResourceKey(live)
					} else {
						var target = &unstructured.Unstructured{}
						err = json.Unmarshal([]byte(res.TargetState), &target)
						errors.CheckError(err)
						key = kube.GetResourceKey(target)
					}
					if key.Kind == kube.SecretKind && key.Group == "" {
						// Don't bother comparing secrets, argo-cd doesn't have access to k8s secret data
						delete(localObjs, key)
						continue
					}
					if local, ok := localObjs[key]; ok || live != nil {
						if local != nil && !kube.IsCRD(local) {
							err = kube.SetAppInstanceLabel(local, argoSettings.AppLabelKey, appName)
							errors.CheckError(err)
						}

						items = append(items, struct {
							key    kube.ResourceKey
							live   *unstructured.Unstructured
							target *unstructured.Unstructured
						}{
							live:   live,
							target: local,
							key:    key,
						})
						delete(localObjs, key)
					}
				}
				for key, local := range localObjs {
					items = append(items, struct {
						key    kube.ResourceKey
						live   *unstructured.Unstructured
						target *unstructured.Unstructured
					}{
						live:   nil,
						target: local,
						key:    key,
					})
				}
			} else {
				for i := range resources.Items {
					res := resources.Items[i]
					var live = &unstructured.Unstructured{}
					err := json.Unmarshal([]byte(res.LiveState), &live)
					errors.CheckError(err)

					var target = &unstructured.Unstructured{}
					err = json.Unmarshal([]byte(res.TargetState), &target)
					errors.CheckError(err)

					items = append(items, struct {
						key    kube.ResourceKey
						live   *unstructured.Unstructured
						target *unstructured.Unstructured
					}{
						live:   live,
						target: target,
						key:    kube.NewResourceKey(res.Group, res.Kind, res.Namespace, res.Name),
					})
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
				// Diff is already available in ResourceDiff Diff field but we have to recalculate diff again due to https://github.com/yudai/gojsondiff/issues/31
				diffRes := diff.Diff(item.target, item.live, normalizer)
				if diffRes.Modified || item.target == nil || item.live == nil {
					fmt.Printf("===== %s/%s %s/%s ======\n", item.key.Group, item.key.Kind, item.key.Namespace, item.key.Name)
					var live *unstructured.Unstructured
					var target *unstructured.Unstructured
					if item.target != nil && item.live != nil {
						target = item.live
						live = item.live.DeepCopy()
						gojsondiff.New().ApplyPatch(live.Object, diffRes.Diff)
					} else {
						live = item.live
						target = item.target
					}

					foundDiffs = true
					err = diff.PrintDiff(item.key.Name, target, live)
					errors.CheckError(err)
				}
			}
			if foundDiffs {
				os.Exit(1)
			}

		},
	}
	command.Flags().BoolVar(&refresh, "refresh", false, "Refresh application data when retrieving")
	command.Flags().BoolVar(&hardRefresh, "hard-refresh", false, "Refresh application data as well as target manifests cache")
	command.Flags().StringVar(&local, "local", "", "Compare live app to a local ksonnet app")
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
				appDeleteReq := applicationpkg.ApplicationDeleteRequest{
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
	fmt.Fprintf(w, fmtStr, headers...)
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
		fmt.Fprintf(w, fmtStr, vals...)
	}
	_ = w.Flush()
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
			apps, err := appIf.List(context.Background(), &applicationpkg.ApplicationQuery{})
			errors.CheckError(err)
			if output == "name" {
				printApplicationNames(apps.Items)
			} else {
				printApplicationTable(apps.Items, &output)
			}
		},
	}
	command.Flags().StringVarP(&output, "output", "o", "wide", "Output format. One of: wide|name")
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
	resourceFieldDelimiter = ":"
	resourceFieldCount     = 3
	labelFieldDelimiter    = "="
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
			rsrc := argoappv1.SyncOperationResource{
				Group: fields[0],
				Kind:  fields[1],
				Name:  fields[2],
			}
			selectedResources = append(selectedResources, rsrc)
		}
	}
	return selectedResources
}

func parseLabels(labels []string) (map[string]string, error) {
	var selectedLabels map[string]string
	if labels != nil {
		selectedLabels = map[string]string{}
		for _, r := range labels {
			fields := strings.Split(r, labelFieldDelimiter)
			if len(fields) != 2 {
				return nil, fmt.Errorf("labels should have key%svalue, but instead got: %s", labelFieldDelimiter, r)
			}
			selectedLabels[fields[0]] = fields[1]
		}
	}
	return selectedLabels, nil
}

// NewApplicationWaitCommand returns a new instance of an `argocd app wait` command
func NewApplicationWaitCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		watchSync       bool
		watchHealth     bool
		watchSuspended  bool
		watchOperations bool
		timeout         uint
		resources       []string
	)
	var command = &cobra.Command{
		Use:   "wait APPNAME",
		Short: "Wait for an application to reach a synced and healthy state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
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
			appName := args[0]
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			_, err := waitOnApplicationStatus(acdClient, appName, timeout, watchSync, watchHealth, watchOperations, watchSuspended, selectedResources)
			errors.CheckError(err)
		},
	}
	command.Flags().BoolVar(&watchSync, "sync", false, "Wait for sync")
	command.Flags().BoolVar(&watchHealth, "health", false, "Wait for health")
	command.Flags().BoolVar(&watchSuspended, "suspended", false, "Wait for suspended")
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
		revision  string
		resources []string
		labels    []string
		prune     bool
		dryRun    bool
		timeout   uint
		strategy  string
		force     bool
		async     bool
		local     string
	)
	var command = &cobra.Command{
		Use:   "sync APPNAME",
		Short: "Sync an application to its target state",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer util.Close(conn)

			appName := args[0]

			selectedLabels, parseErr := parseLabels(labels)
			if parseErr != nil {
				log.Fatal(parseErr)
			}

			if len(selectedLabels) > 0 {
				ctx := context.Background()

				if revision == "" {
					revision = "HEAD"
				}

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
				if app.Spec.SyncPolicy != nil && app.Spec.SyncPolicy.Automated != nil {
					log.Fatal("Cannot use local sync when Automatic Sync Policy is enabled")
				}

				errors.CheckError(err)
				conn, settingsIf := acdClient.NewSettingsClientOrDie()
				argoSettings, err := settingsIf.Get(context.Background(), &settingspkg.SettingsQuery{})
				errors.CheckError(err)
				util.Close(conn)

				localObjsStrings = getLocalObjectsString(app, local, argoSettings.AppLabelKey, argoSettings.KustomizeOptions)

			}

			syncReq := applicationpkg.ApplicationSyncRequest{
				Name:      &appName,
				DryRun:    dryRun,
				Revision:  revision,
				Resources: selectedResources,
				Prune:     prune,
				Manifests: localObjsStrings,
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

			if !async {
				app, err := waitOnApplicationStatus(acdClient, appName, timeout, false, false, true, false, selectedResources)
				errors.CheckError(err)

				// Only get resources to be pruned if sync was application-wide
				if len(selectedResources) == 0 {
					pruningRequired := app.Status.OperationState.SyncResult.Resources.PruningRequired()
					if pruningRequired > 0 {
						log.Fatalf("%d resources require pruning", pruningRequired)
					}

					if !app.Status.OperationState.Phase.Successful() && !dryRun {
						os.Exit(1)
					}
				}
			}
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview apply without affecting cluster")
	command.Flags().BoolVar(&prune, "prune", false, "Allow deleting unexpected resources")
	command.Flags().StringVar(&revision, "revision", "", "Sync to a specific revision. Preserves parameter overrides")
	command.Flags().StringArrayVar(&resources, "resource", []string{}, fmt.Sprintf("Sync only specific resources as GROUP%sKIND%sNAME. Fields may be blank. This option may be specified repeatedly", resourceFieldDelimiter, resourceFieldDelimiter))
	command.Flags().StringArrayVar(&labels, "label", []string{}, fmt.Sprintf("Sync only specific resources with a label. This option may be specified repeatedly."))
	command.Flags().UintVar(&timeout, "timeout", defaultCheckTimeoutSeconds, "Time out after this many seconds")
	command.Flags().StringVar(&strategy, "strategy", "", "Sync strategy (one of: apply|hook)")
	command.Flags().BoolVar(&force, "force", false, "Use a force apply")
	command.Flags().BoolVar(&async, "async", false, "Do not wait for application to sync before continuing")
	command.Flags().StringVar(&local, "local", "", "Path to a local directory. When this flag is present no git queries will be made")
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
				health = argoappv1.HealthStatusUnknown
				if resource.Health != nil {
					health = resource.Health.Status
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
		health := argoappv1.HealthStatusUnknown
		if res.Health != nil {
			health = res.Health.Status
		}
		states = append(states, &resourceState{
			Group: res.Group, Kind: res.Kind, Namespace: res.Namespace, Name: res.Name, Status: string(res.Status), Health: health, Hook: "", Message: ""})
	}
	// filter out not selected resources
	if len(selectedResources) > 0 {
		for i := len(states) - 1; i >= 0; i-- {
			res := states[i]
			if !argo.ContainsSyncResource(res.Name, schema.GroupVersionKind{Group: res.Group, Kind: res.Kind}, selectedResources) {
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
		healthCheckPassed = healthStatus == argoappv1.HealthStatusHealthy ||
			healthStatus == argoappv1.HealthStatusSuspended
	} else if watchSuspended {
		healthCheckPassed = healthStatus == argoappv1.HealthStatusSuspended
	} else if watchHealth {
		healthCheckPassed = healthStatus == argoappv1.HealthStatusHealthy
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

	printFinalStatus := func(app *argoappv1.Application) {
		var err error
		if refresh {
			conn, appClient := acdClient.NewApplicationClientOrDie()
			refreshType := string(argoappv1.RefreshTypeNormal)
			app, err = appClient.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: &refreshType})
			errors.CheckError(err)
			_ = conn.Close()
		}

		fmt.Println()
		printAppSummaryTable(app, appURL(acdClient, appName))
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
	}

	if timeout != 0 {
		time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			cancel()
		})
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 0, 2, ' ', 0)
	_, _ = fmt.Fprintf(w, waitFormatString, "TIMESTAMP", "GROUP", "KIND", "NAMESPACE", "NAME", "STATUS", "HEALTH", "HOOK", "MESSAGE")

	prevStates := make(map[string]*resourceState)
	appEventCh := acdClient.WatchApplicationWithRetry(ctx, appName)
	conn, appClient := acdClient.NewApplicationClientOrDie()
	defer util.Close(conn)
	app, err := appClient.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
	errors.CheckError(err)

	for appEvent := range appEventCh {
		app = &appEvent.Application
		if app.Operation != nil {
			refresh = true
		}

		var selectedResourcesAreReady bool

		// If selected resources are included, wait only on those resources, otherwise wait on the application as a whole.
		if len(selectedResources) > 0 {
			selectedResourcesAreReady = true
			for _, state := range getResourceStates(app, selectedResources) {
				resourceIsReady := checkResourceStatus(watchSync, watchHealth, false, watchSuspended, state.Health, state.Status, nil)
				if !resourceIsReady {
					selectedResourcesAreReady = false
					break
				}
			}
		} else {
			// Wait on the application as a whole
			selectedResourcesAreReady = checkResourceStatus(watchSync, watchHealth, watchOperation, watchSuspended, app.Status.Health.Status, string(app.Status.Sync.Status), appEvent.Application.Operation)
		}

		if selectedResourcesAreReady {
			printFinalStatus(app)
			return app, nil
		}

		newStates := groupResourceStates(app, selectedResources)
		for _, newState := range newStates {
			var doPrint bool
			stateKey := newState.Key()
			if prevState, found := prevStates[stateKey]; found {
				if watchHealth && prevState.Health != argoappv1.HealthStatusUnknown && prevState.Health != argoappv1.HealthStatusDegraded && newState.Health == argoappv1.HealthStatusDegraded {
					printFinalStatus(app)
					return nil, fmt.Errorf("Application '%s' health state has transitioned from %s to %s", appName, prevState.Health, newState.Health)
				}
				doPrint = prevState.Merge(newState)
			} else {
				prevStates[stateKey] = newState
				doPrint = true
			}
			if doPrint {
				fmt.Fprintf(w, waitFormatString, prevStates[stateKey].FormatItems()...)
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
		re := regexp.MustCompile(`([^\\]),`)
		for _, paramStr := range parameters {
			parts := strings.SplitN(paramStr, "=", 2)
			if len(parts) != 2 {
				log.Fatalf("Expected helm parameter of the form: param=value. Received: %s", paramStr)
			}
			newParam := argoappv1.HelmParameter{
				Name:  parts[0],
				Value: re.ReplaceAllString(parts[1], `$1\,`),
			}
			found := false
			for i, cp := range app.Spec.Source.Helm.Parameters {
				if cp.Name == newParam.Name {
					found = true
					app.Spec.Source.Helm.Parameters[i] = newParam
					break
				}
			}
			if !found {
				app.Spec.Source.Helm.Parameters = append(app.Spec.Source.Helm.Parameters, newParam)
			}
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
	fmt.Fprintf(w, "ID\tDATE\tREVISION\n")
	for _, depInfo := range revHistory {
		rev := depInfo.Source.TargetRevision
		if len(depInfo.Revision) >= 7 {
			rev = fmt.Sprintf("%s (%s)", rev, depInfo.Revision[0:7])
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", depInfo.ID, depInfo.DeployedAt, rev)
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
			defer util.Close(conn)
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

// NewApplicationRollbackCommand returns a new instance of an `argocd app rollback` command
func NewApplicationRollbackCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		prune   bool
		timeout uint
	)
	var command = &cobra.Command{
		Use:   "rollback APPNAME ID",
		Short: "Rollback application to a previous deployed version by History ID",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			depID, err := strconv.Atoi(args[1])
			errors.CheckError(err)
			acdClient := argocdclient.NewClientOrDie(clientOpts)
			conn, appIf := acdClient.NewApplicationClientOrDie()
			defer util.Close(conn)
			ctx := context.Background()
			app, err := appIf.Get(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
			errors.CheckError(err)
			var depInfo *argoappv1.RevisionHistory
			for _, di := range app.Status.History {
				if di.ID == int64(depID) {
					depInfo = &di
					break
				}
			}
			if depInfo == nil {
				log.Fatalf("Application '%s' does not have deployment id '%d' in history\n", app.ObjectMeta.Name, depID)
			}

			_, err = appIf.Rollback(ctx, &applicationpkg.ApplicationRollbackRequest{
				Name:  &appName,
				ID:    int64(depID),
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
			defer util.Close(conn)
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
			defer util.Close(conn)
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
			defer util.Close(conn)
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
				_, err = appIf.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: updatedSpec})
				if err != nil {
					return fmt.Errorf("Failed to update application spec:\n%v", err)
				}
				return err
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
			defer util.Close(conn)

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
		if kind == gvk.Kind {
			copy := obj.DeepCopy()
			filteredObjects = append(filteredObjects, copy)
		}
	}
	if len(filteredObjects) == 0 {
		log.Fatal("No matching resource found")
	}
	if len(filteredObjects) > 1 && !all {
		log.Fatal("Multiple resources match inputs. Use the --all flag to patch multiple resources")
	}
	firstGroup := filteredObjects[0].GroupVersionKind().Group
	for i := range filteredObjects {
		obj := filteredObjects[i]
		if obj.GroupVersionKind().Group != firstGroup {
			log.Fatal("Multiple groups found in objects to patch. Specify which group to patch with --group flag")
		}
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
		defer util.Close(conn)
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
