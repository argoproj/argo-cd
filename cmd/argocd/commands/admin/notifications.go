package admin

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/env"
	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
	"github.com/argoproj/argo-cd/v3/util/notification/settings"
	"github.com/argoproj/argo-cd/v3/util/tls"

	"github.com/argoproj/notifications-engine/pkg/cmd"
	"github.com/spf13/cobra"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
)

var applications = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationPlural}

func NewNotificationsCommand() *cobra.Command {
	var (
		argocdRepoServer          string
		argocdRepoServerPlaintext bool
		argocdRepoServerStrictTLS bool
	)

	var argocdService service.Service
	toolsCommand := cmd.NewToolsCommand(
		"notifications",
		"argocd admin notifications",
		applications,
		settings.GetFactorySettingsForCLI(argocdService, "argocd-notifications-secret", "argocd-notifications-cm", false),
		func(clientConfig clientcmd.ClientConfig) {
			k8sCfg, err := clientConfig.ClientConfig()
			if err != nil {
				log.Fatalf("Failed to parse k8s config: %v", err)
			}
			ns, _, err := clientConfig.Namespace()
			if err != nil {
				log.Fatalf("Failed to parse k8s config: %v", err)
			}
			tlsConfig := apiclient.TLSConfiguration{
				DisableTLS:       argocdRepoServerPlaintext,
				StrictValidation: argocdRepoServerStrictTLS,
			}
			if !tlsConfig.DisableTLS && tlsConfig.StrictValidation {
				pool, err := tls.LoadX509CertPool(
					env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)+"/reposerver/tls/tls.crt",
					env.StringFromEnv(common.EnvAppConfigPath, common.DefaultAppConfigPath)+"/reposerver/tls/ca.crt",
				)
				if err != nil {
					log.Fatalf("Failed to load tls certs: %v", err)
				}
				tlsConfig.Certificates = pool
			}
			repoClientset := apiclient.NewRepoServerClientset(argocdRepoServer, 5, tlsConfig)
			argocdService, err = service.NewArgoCDService(kubernetes.NewForConfigOrDie(k8sCfg), ns, repoClientset)
			if err != nil {
				log.Fatalf("Failed to initialize Argo CD service: %v", err)
			}
		})
	toolsCommand.PersistentFlags().StringVar(&argocdRepoServer, "argocd-repo-server", common.DefaultRepoServerAddr, "Argo CD repo server address")
	toolsCommand.PersistentFlags().BoolVar(&argocdRepoServerPlaintext, "argocd-repo-server-plaintext", false, "Use a plaintext client (non-TLS) to connect to repository server")
	toolsCommand.PersistentFlags().BoolVar(&argocdRepoServerStrictTLS, "argocd-repo-server-strict-tls", false, "Perform strict validation of TLS certificates when connecting to repo server")
	return toolsCommand
}
