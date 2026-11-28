package admin

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/cli"
	"github.com/argoproj/argo-cd/v3/util/errors"
	"github.com/argoproj/argo-cd/v3/util/kube"
	"github.com/argoproj/argo-cd/v3/util/password"
)

const initialPasswordSecretName = "argocd-initial-admin-secret"

// NewInitialPasswordCommand defines a new command to retrieve Argo CD initial password.
func NewInitialPasswordCommand() *cobra.Command {
	var clientConfig clientcmd.ClientConfig
	command := cobra.Command{
		Use:   "initial-password",
		Short: "Prints initial password to log in to Argo CD for the first time",
		Run: func(c *cobra.Command, _ []string) {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClientset := kubernetes.NewForConfigOrDie(config)
			secret, err := kubeClientset.CoreV1().Secrets(namespace).Get(c.Context(), initialPasswordSecretName, metav1.GetOptions{})
			errors.CheckError(err)

			if initialPass, ok := secret.Data["password"]; ok {
				fmt.Println(string(initialPass))
				fmt.Println("\n This password must be only used for first time login. We strongly recommend you update the password using `argocd account update-password`.")
			}
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)
	command.AddCommand(newInitialPasswordResetCommand(&clientConfig))

	return &command
}

// newInitialPasswordResetCommand defines a new command to reset the admin password to a new auto-generated value.
func newInitialPasswordResetCommand(clientConfig *clientcmd.ClientConfig) *cobra.Command {
	command := &cobra.Command{
		Use:   "reset",
		Short: "Reset the admin password and store the new password in the initial admin secret",
		Long: `Reset the admin account password to a new randomly generated value.
The new password is written to the argocd-initial-admin-secret Kubernetes secret
and printed to stdout. The argocd-server does not need to be restarted.`,
		Example: `  # Reset the initial admin password
  argocd admin initial-password reset

  # Reset the initial admin password in a custom namespace
  argocd admin initial-password reset -n my-argocd`,
		Run: func(c *cobra.Command, _ []string) {
			config, err := (*clientConfig).ClientConfig()
			errors.CheckError(err)
			namespace, _, err := (*clientConfig).Namespace()
			errors.CheckError(err)

			kubeClientset := kubernetes.NewForConfigOrDie(config)

			// Generate a new random password
			newPassword, err := generateRandomPassword()
			errors.CheckError(err)

			// Hash the new password using bcrypt
			hashedPassword, err := password.HashPassword(newPassword)
			errors.CheckError(err)

			// Update the admin.password and admin.passwordMtime fields in argocd-secret
			ku := kube.NewKubeUtil(c.Context(), kubeClientset)
			now := time.Now().UTC().Format(time.RFC3339)
			err = ku.CreateOrUpdateSecretData(namespace, common.ArgoCDSecretName, map[string][]byte{
				"admin.password":      []byte(hashedPassword),
				"admin.passwordMtime": []byte(now),
			}, true)
			errors.CheckError(err)

			// Store the new plaintext password in argocd-initial-admin-secret
			err = ku.CreateOrUpdateSecretField(namespace, initialPasswordSecretName, "password", newPassword)
			errors.CheckError(err)

			fmt.Println(newPassword)
			fmt.Println("\n Password has been reset. You can log in using the new password above.")
			fmt.Println(" We strongly recommend you update the password using `argocd account update-password`.")
		},
	}
	return command
}
