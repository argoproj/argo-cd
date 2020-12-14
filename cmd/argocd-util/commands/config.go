package commands

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
	"github.com/argoproj/argo-cd/pkg/apis/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/errors"
)

func NewGenerateConfigCommand() *cobra.Command {
	var command = &cobra.Command{
		Use:   "config",
		Short: "Generate declarative configuration files",
		Run: func(c *cobra.Command, args []string) {
			c.HelpFunc()(c, args)
		},
	}
	command.AddCommand(NewGenAppConfigCommand())
	command.AddCommand(NewGenProjectConfigCommand())

	return command
}

// NewGenAppConfigCommand generates declarative configuration file for given application
func NewGenAppConfigCommand() *cobra.Command {
	var (
		appOpts      cmdutil.AppOptions
		fileURL      string
		appName      string
		labels       []string
		outputFormat string
	)
	var command = &cobra.Command{
		Use:   "app APPNAME",
		Short: "Generate declarative config for an application",
		Example: `
	# Generate declarative config for a directory app
	argocd-util config app guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --directory-recurse

	# Generate declarative config for a Jsonnet app
	argocd-util config app jsonnet-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path jsonnet-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --jsonnet-ext-str replicas=2

	# Generate declarative config for a Helm app
	argocd-util config app helm-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path helm-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --helm-set replicaCount=2

	# Generate declarative config for a Helm app from a Helm repo
	argocd-util config app nginx-ingress --repo https://kubernetes-charts.storage.googleapis.com --helm-chart nginx-ingress --revision 1.24.3 --dest-namespace default --dest-server https://kubernetes.default.svc

	# Generate declarative config for a Kustomize app
	argocd-util config app kustomize-guestbook --repo https://github.com/argoproj/argocd-example-apps.git --path kustomize-guestbook --dest-namespace default --dest-server https://kubernetes.default.svc --kustomize-image gcr.io/heptio-images/ks-guestbook-demo:0.1

	# Generate declarative config for a app using a custom tool:
	argocd-util config app ksane --repo https://github.com/argoproj/argocd-example-apps.git --path plugins/kasane --dest-namespace default --dest-server https://kubernetes.default.svc --config-management-plugin kasane
`,
		Run: func(c *cobra.Command, args []string) {
			var app v1alpha1.Application
			if fileURL == "-" {
				// read stdin
				err := cmdutil.ReadAppFromStdin(&app)
				errors.CheckError(err)
			} else if fileURL != "" {
				// read uri
				err := cmdutil.ReadAppFromURI(fileURL, &app)
				errors.CheckError(err)

				if len(args) == 1 && args[0] != app.Name {
					log.Fatalf("app name '%s' does not match app spec metadata.name '%s'", args[0], app.Name)
				}
				if appName != "" && appName != app.Name {
					app.Name = appName
				}
				if app.Name == "" {
					log.Fatalf("app.Name is empty. --name argument can be used to provide app.Name")
				}
				cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
				cmdutil.SetParameterOverrides(&app, appOpts.Parameters)
				cmdutil.SetLabels(&app, labels)
			} else {
				// read arguments
				if len(args) == 1 {
					if appName != "" && appName != args[0] {
						log.Fatalf("--name argument '%s' does not match app name %s", appName, args[0])
					}
					appName = args[0]
				}
				app = v1alpha1.Application{
					TypeMeta: v1.TypeMeta{
						Kind:       application.ApplicationKind,
						APIVersion: application.Group + "/v1aplha1",
					},
					ObjectMeta: v1.ObjectMeta{
						Name: appName,
					},
				}
				cmdutil.SetAppSpecOptions(c.Flags(), &app.Spec, &appOpts)
				cmdutil.SetParameterOverrides(&app, appOpts.Parameters)
				cmdutil.SetLabels(&app, labels)
			}
			if app.Name == "" {
				c.HelpFunc()(c, args)
				os.Exit(1)
			}

			errors.CheckError(cmdutil.PrintResource(app, outputFormat))
		},
	}
	command.Flags().StringVar(&appName, "name", "", "A name for the app, ignored if a file is set (DEPRECATED)")
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the app")
	command.Flags().StringArrayVarP(&labels, "label", "l", []string{}, "Labels to apply to the app")
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")

	// Only complete files with appropriate extension.
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	errors.CheckError(err)

	cmdutil.AddAppFlags(command, &appOpts)
	return command
}

// NewGenProjectConfigCommand generates declarative configuration file for given project
func NewGenProjectConfigCommand() *cobra.Command {
	var (
		opts         cmdutil.ProjectOpts
		fileURL      string
		outputFormat string
	)
	var command = &cobra.Command{
		Use:   "proj PROJECT",
		Short: "Generate declarative config for a project",
		Run: func(c *cobra.Command, args []string) {
			var proj v1alpha1.AppProject
			if fileURL == "-" {
				// read stdin
				err := cmdutil.ReadProjFromStdin(&proj)
				errors.CheckError(err)
			} else if fileURL != "" {
				// read uri
				err := cmdutil.ReadProjFromURI(fileURL, &proj)
				errors.CheckError(err)

				if len(args) == 1 && args[0] != proj.Name {
					log.Fatalf("project name '%s' does not match project spec metadata.name '%s'", args[0], proj.Name)
				}
			} else {
				// read arguments
				if len(args) == 0 {
					c.HelpFunc()(c, args)
					os.Exit(1)
				}
				projName := args[0]
				proj = v1alpha1.AppProject{
					TypeMeta: v1.TypeMeta{
						Kind:       application.AppProjectKind,
						APIVersion: application.Group + "/v1aplha1",
					},
					ObjectMeta: v1.ObjectMeta{Name: projName},
					Spec: v1alpha1.AppProjectSpec{
						Description:       opts.Description,
						Destinations:      opts.GetDestinations(),
						SourceRepos:       opts.Sources,
						SignatureKeys:     opts.GetSignatureKeys(),
						OrphanedResources: cmdutil.GetOrphanedResourcesSettings(c, opts),
					},
				}
			}

			errors.CheckError(cmdutil.PrintResource(proj, outputFormat))
		},
	}
	command.Flags().StringVarP(&fileURL, "file", "f", "", "Filename or URL to Kubernetes manifests for the project")
	command.Flags().StringVar(&outputFormat, "o", "yaml", "Output format (yaml|json)")
	err := command.Flags().SetAnnotation("file", cobra.BashCompFilenameExt, []string{"json", "yaml", "yml"})
	if err != nil {
		log.Fatal(err)
	}
	cmdutil.AddProjFlags(command, &opts)
	return command
}
