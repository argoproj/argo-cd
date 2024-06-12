package admin

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/util/cli"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

const initialPasswordSecretName = "argocd-initial-admin-secret"

// NewInitialPasswordCommand defines a new command to retrieve Argo CD initial password.
func NewInitialPasswordCommand() *cobra.Command {
	var clientConfig clientcmd.ClientConfig
	command := cobra.Command{
		Use:   "initial-password",
		Short: "Prints initial password to log in to Argo CD for the first time",
		Run: func(c *cobra.Command, args []string) {
			config, err := clientConfig.ClientConfig()
			errors.CheckError(err)
			namespace, _, err := clientConfig.Namespace()
			errors.CheckError(err)

			kubeClientset := kubernetes.NewForConfigOrDie(config)
			secret, err := kubeClientset.CoreV1().Secrets(namespace).Get(context.Background(), initialPasswordSecretName, v1.GetOptions{})
			errors.CheckError(err)

			if initialPass, ok := secret.Data["password"]; ok {
				fmt.Println(string(initialPass))
				fmt.Println("\n This password must be only used for first time login. We strongly recommend you update the password using `argocd account update-password`.")
			}
		},
	}
	clientConfig = cli.AddKubectlFlagsToCmd(&command)

	return &command
}
