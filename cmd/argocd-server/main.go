package main

import (
	"github.com/argoproj/gitops-engine/pkg/utils/errors"

	commands "github.com/argoproj/argo-cd/cmd/argocd-server/commands"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// load the azure plugin (required to authenticate with AKS clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
)

func main() {
	err := commands.NewCommand().Execute()
	errors.CheckError(err)
}
