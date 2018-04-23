package commands

import (
	"context"
	"fmt"
	"net/url"
	"os"
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
		Short: fmt.Sprintf("%s app COMMAND", cliName),
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
	command.AddCommand(NewApplicationListCommand(clientOpts))
	command.AddCommand(NewApplicationDeleteCommand(clientOpts))
	return command
}

// NewApplicationCreateCommand returns a new instance of an `argocd app create` command
func NewApplicationCreateCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts    appOptions
		fileURL    string
		appName    string
		syncPolicy string
	)
	var command = &cobra.Command{
		Use:   "create",
		Short: fmt.Sprintf("%s app create", cliName),
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
				if syncPolicy != "" && syncPolicy != "Always" {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
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
						SyncPolicy: syncPolicy,
					},
				}
			}
			if appOpts.destServer != "" || appOpts.destNamespace != "" {
				app.Spec.Destination = &argoappv1.ApplicationDestination{
					Server:    appOpts.destServer,
					Namespace: appOpts.destNamespace,
				}
			}
			setParameterOverrides(&app, appOpts.parameters)
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			created, err := appIf.Create(context.Background(), &app)
			errors.CheckError(err)
			fmt.Printf("application '%s' created", created.ObjectMeta.Name)
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set")
	addAppFlags(command, &appOpts)
	//command.Flags().StringVar(&syncPolicy, "sync-policy", "", "Synchronization policy for application (e.g., Always)")
	return command
}

// NewApplicationGetCommand returns a new instance of an `argocd app get` command
func NewApplicationGetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "get",
		Short: fmt.Sprintf("%s app get APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: appName})
			errors.CheckError(err)
			format := "%-15s%s\n"
			fmt.Printf(format, "Name:", app.Name)
			fmt.Printf(format, "Environment:", app.Spec.Source.Environment)
			fmt.Printf(format, "Repo:", app.Spec.Source.RepoURL)
			fmt.Printf(format, "Path:", app.Spec.Source.Path)
			if app.Spec.Source.TargetRevision == "" {
				fmt.Printf(format, "Target:", "HEAD")
			} else {
				fmt.Printf(format, "Target:", app.Spec.Source.TargetRevision)
			}
			if app.Status.ComparisonResult.Error != "" {
				fmt.Printf(format, "Error:", app.Status.ComparisonResult.Error)
			}
			if app.Status.ComparisonResult.Server != "" {
				fmt.Printf(format, "Server:", app.Status.ComparisonResult.Server)
			}
			if app.Status.ComparisonResult.Namespace != "" {
				fmt.Printf(format, "Namespace:", app.Status.ComparisonResult.Namespace)
			}
			if len(app.Status.ComparisonResult.Resources) > 0 {
				fmt.Println()
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				fmt.Fprintf(w, "KIND\tNAME\tSTATUS\n")
				for _, res := range app.Status.ComparisonResult.Resources {
					targetObj, err := argoappv1.UnmarshalToUnstructured(res.TargetState)
					errors.CheckError(err)
					fmt.Fprintf(w, "%s\t%s\t%s\n", targetObj.GetKind(), targetObj.GetName(), res.Status)
				}
				_ = w.Flush()
			}
		},
	}
	return command
}

// NewApplicationSetCommand returns a new instance of an `argocd app set` command
func NewApplicationSetCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		appOpts appOptions
	)
	var command = &cobra.Command{
		Use:   "set",
		Short: fmt.Sprintf("%s app set APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			appName := args[0]
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: appName})
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
					if app.Spec.Destination == nil {
						app.Spec.Destination = &argoappv1.ApplicationDestination{}
					}
					app.Spec.Destination.Server = appOpts.destServer
				case "dest-namespace":
					if app.Spec.Destination == nil {
						app.Spec.Destination = &argoappv1.ApplicationDestination{}
					}
					app.Spec.Destination.Namespace = appOpts.destNamespace
				}
			})
			if visited == 0 {
				log.Error("Please set at least one option to update")
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			setParameterOverrides(app, appOpts.parameters)
			_, err = appIf.Update(context.Background(), app)
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
		Use:   "diff",
		Short: fmt.Sprintf("%s app diff APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			app, err := appIf.Get(context.Background(), &application.ApplicationQuery{Name: appName})
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
		force bool
	)
	var command = &cobra.Command{
		Use:   "delete",
		Short: fmt.Sprintf("%s app delete APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			for _, appName := range args {
				appDeleteReq := application.DeleteApplicationRequest{
					Name:  appName,
					Force: force,
				}
				_, err := appIf.Delete(context.Background(), &appDeleteReq)
				errors.CheckError(err)
			}
		},
	}
	command.Flags().BoolVar(&force, "force", false, "Force delete application even if cascaded deletion unsuccessful")
	return command
}

// NewApplicationListCommand returns a new instance of an `argocd app list` command
func NewApplicationListCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "list",
		Short: fmt.Sprintf("%s app list", cliName),
		Run: func(c *cobra.Command, args []string) {
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			apps, err := appIf.List(context.Background(), &application.ApplicationQuery{})
			errors.CheckError(err)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tENVIRONMENT\tTARGET\tCLUSTER\tNAMESPACE\tSTATUS\n")
			for _, app := range apps.Items {
				targetRev := app.Spec.Source.TargetRevision
				if targetRev == "" {
					targetRev = "HEAD"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					app.Name,
					app.Spec.Source.Environment,
					targetRev,
					app.Status.ComparisonResult.Server,
					app.Status.ComparisonResult.Namespace,
					app.Status.ComparisonResult.Status,
				)
			}
			_ = w.Flush()
		},
	}
	return command
}

// NewApplicationSyncCommand returns a new instance of an `argocd app sync` command
func NewApplicationSyncCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		dryRun bool
	)
	var command = &cobra.Command{
		Use:   "sync",
		Short: fmt.Sprintf("%s app sync APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			appName := args[0]
			syncReq := application.ApplicationSyncRequest{
				Name:   appName,
				DryRun: dryRun,
			}
			syncRes, err := appIf.Sync(context.Background(), &syncReq)
			errors.CheckError(err)
			fmt.Printf("%s %s\n", appName, syncRes.Message)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "NAME\tKIND\tMESSAGE\n")
			for _, resDetails := range syncRes.Resources {
				fmt.Fprintf(w, "%s\t%s\t%s\n", resDetails.Name, resDetails.Kind, resDetails.Message)
			}
			_ = w.Flush()
		},
	}
	command.Flags().BoolVar(&dryRun, "dry-run", false, "Preview apply without affecting cluster")
	return command
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
