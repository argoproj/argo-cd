package commands

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	cmdutil "github.com/argoproj/argo-cd/cmd/util"
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
			app, err := cmdutil.ConstructApp(fileURL, appName, labels, args, appOpts, c.Flags())
			errors.CheckError(err)

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
			proj, err := cmdutil.ConstructAppProj(fileURL, args, opts, c)
			errors.CheckError(err)

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
