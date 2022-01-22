package admin

import (
	"log"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"
	settings "github.com/argoproj/argo-cd/v2/util/notification/settings"

	"github.com/argoproj/notifications-engine/pkg/cmd"
	"github.com/spf13/cobra"
)

var (
	applications = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
)

func NewNotificationsCommand() *cobra.Command {
	var (
		argocdRepoServer          string
		argocdRepoServerPlaintext bool
		argocdRepoServerStrictTLS bool
	)

	var argocdService service.Service
	toolsCommand := cmd.NewToolsCommand(
		"notifications",
		"notifications",
		applications,
		settings.GetFactorySettings(argocdService, "argocd-notifications-secret", "argocd-notifications-cm"), func(clientConfig clientcmd.ClientConfig) {
			k8sCfg, err := clientConfig.ClientConfig()
			if err != nil {
				log.Fatalf("Failed to parse k8s config: %v", err)
			}
			ns, _, err := clientConfig.Namespace()
			if err != nil {
				log.Fatalf("Failed to parse k8s config: %v", err)
			}
			argocdService, err = service.NewArgoCDService(kubernetes.NewForConfigOrDie(k8sCfg), ns, argocdRepoServer, argocdRepoServerPlaintext, argocdRepoServerStrictTLS)
			if err != nil {
				log.Fatalf("Failed to initalize Argo CD service: %v", err)
			}
		})
	toolsCommand.PersistentFlags().StringVar(&argocdRepoServer, "argocd-repo-server", "argocd-repo-server:8081", "Argo CD repo server address")
	toolsCommand.PersistentFlags().BoolVar(&argocdRepoServerPlaintext, "argocd-repo-server-plaintext", false, "Use a plaintext client (non-TLS) to connect to repository server")
	toolsCommand.PersistentFlags().BoolVar(&argocdRepoServerStrictTLS, "argocd-repo-server-strict-tls", false, "Perform strict validation of TLS certificates when connecting to repo server")
	return toolsCommand
}
