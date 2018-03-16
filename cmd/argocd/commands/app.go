package commands

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/spf13/cobra"
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

	command.AddCommand(NewApplicationAddCommand(clientOpts))
	command.AddCommand(NewApplicationGetCommand(clientOpts))
	command.AddCommand(NewApplicationDiffCommand(clientOpts))
	command.AddCommand(NewApplicationSyncCommand(clientOpts))
	command.AddCommand(NewApplicationListCommand(clientOpts))
	command.AddCommand(NewApplicationRemoveCommand(clientOpts))
	return command
}

// NewApplicationAddCommand returns a new instance of an `argocd app add` command
func NewApplicationAddCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var (
		fileURL       string
		repoURL       string
		appPath       string
		appName       string
		env           string
		destServer    string
		destNamespace string
	)
	var command = &cobra.Command{
		Use:   "add",
		Short: fmt.Sprintf("%s app add", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			var app argoappv1.Application
			if fileURL != "" {
				var (
					fileContents []byte
					err          error
				)
				_, err = url.ParseRequestURI(fileURL)
				if err != nil {
					fileContents, err = readLocalFile(fileURL)
				} else {
					fileContents, err = readRemoteFile(fileURL)
				}
				if err != nil {
					log.Fatal(err)
				}
				unmarshalApplication(fileContents, &app)

			} else {
				// all these params are required if we're here
				if repoURL == "" || appPath == "" || appName == "" {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				app = argoappv1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: appName,
					},
					Spec: argoappv1.ApplicationSpec{
						Source: argoappv1.ApplicationSource{
							RepoURL:     repoURL,
							Path:        appPath,
							Environment: env,
						},
					},
				}
			}
			if destServer != "" || destNamespace != "" {
				app.Spec.Destination = &argoappv1.ApplicationDestination{
					Server:    destServer,
					Namespace: destNamespace,
				}
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			_, err := appIf.Create(context.Background(), &app)
			errors.CheckError(err)
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set")
	command.Flags().StringVar(&repoURL, "repo", "", "Repository URL, ignored if a file is set")
	command.Flags().StringVar(&appPath, "path", "", "Path in repository to the ksonnet app directory, ignored if a file is set")
	command.Flags().StringVar(&env, "env", "", "Application environment to monitor")
	command.Flags().StringVar(&destServer, "dest-server", "", "K8s cluster URL (overrides the server URL specified in the ksonnet app.yaml)")
	command.Flags().StringVar(&destNamespace, "dest-namespace", "", "K8s target namespace (overrides the namespace specified in the ksonnet app.yaml)")
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

// NewApplicationRemoveCommand returns a new instance of an `argocd app list` command
func NewApplicationRemoveCommand(clientOpts *argocdclient.ClientOptions) *cobra.Command {
	var command = &cobra.Command{
		Use:   "rm",
		Short: fmt.Sprintf("%s app rm APPNAME", cliName),
		Run: func(c *cobra.Command, args []string) {
			if len(args) == 0 {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}
			conn, appIf := argocdclient.NewClientOrDie(clientOpts).NewApplicationClientOrDie()
			defer util.Close(conn)
			for _, appName := range args {
				_, err := appIf.Delete(context.Background(), &application.DeleteApplicationRequest{Name: appName})
				errors.CheckError(err)
			}
		},
	}
	return command
}

// NewApplicationListCommand returns a new instance of an `argocd app rm` command
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
