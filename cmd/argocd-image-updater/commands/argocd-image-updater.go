package command

import (
	"text/template"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/argocd"
	"github.com/argoproj/argo-cd/v2/image-updater/kube"

	"github.com/spf13/cobra"
)

var lastRun time.Time

// Default ArgoCD server address when running in same cluster as ArgoCD
const defaultArgoCDServerAddr = "argocd-server.argocd"

// Default path to registry configuration
const defaultRegistriesConfPath = "/app/config/registries.conf"

// Default path to Git commit message template
const defaultCommitTemplatePath = "/app/config/commit.template"

const applicationsAPIKindK8S = "kubernetes"
const applicationsAPIKindArgoCD = "argocd"

// ImageUpdaterConfig contains global configuration and required runtime data
type ImageUpdaterConfig struct {
	ApplicationsAPIKind string
	ClientOpts          argocd.ClientOptions
	ArgocdNamespace     string
	DryRun              bool
	CheckInterval       time.Duration
	ArgoClient          argocd.ArgoCD
	LogLevel            string
	KubeClient          *kube.KubernetesClient
	MaxConcurrency      int
	HealthPort          int
	MetricsPort         int
	RegistriesConf      string
	AppNamePatterns     []string
	AppLabel            string
	GitCommitUser       string
	GitCommitMail       string
	GitCommitMessage    *template.Template
	DisableKubeEvents   bool
}

// NewCommand implements the root command of argocd-image-updater
func NewCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "argocd-image-updater",
		Short: "Automatically update container images with ArgoCD",
	}
	rootCmd.AddCommand(newRunCommand())
	rootCmd.AddCommand(newTestCommand())
	rootCmd.AddCommand(newTemplateCommand())
	return rootCmd
}
