package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	controller "github.com/argoproj/argo-cd/v2/cmd/argocd-application-controller/commands"
	argocddex "github.com/argoproj/argo-cd/v2/cmd/argocd-dex/commands"
	reposerver "github.com/argoproj/argo-cd/v2/cmd/argocd-repo-server/commands"
	argocdserver "github.com/argoproj/argo-cd/v2/cmd/argocd-server/commands"
	argocdcli "github.com/argoproj/argo-cd/v2/cmd/argocd/commands"
)

func main() {
	// set HOME env var so that default values involve user's home directory do not depend on the running user.
	os.Setenv("HOME", "/home/user")
	os.Setenv("XDG_CONFIG_HOME", "/home/user/.config")

	err := doc.GenMarkdownTree(argocdcli.NewCommand(), "./docs/user-guide/commands")
	if err != nil {
		log.Fatal(err)
	}

	err = doc.GenMarkdownTree(argocdserver.NewCommand(), "./docs/operator-manual/server-commands")
	if err != nil {
		log.Fatal(err)
	}

	err = doc.GenMarkdownTree(controller.NewCommand(), "./docs/operator-manual/server-commands")
	if err != nil {
		log.Fatal(err)
	}

	err = doc.GenMarkdownTree(reposerver.NewCommand(), "./docs/operator-manual/server-commands")
	if err != nil {
		log.Fatal(err)
	}

	err = doc.GenMarkdownTree(argocddex.NewCommand(), "./docs/operator-manual/server-commands")
	if err != nil {
		log.Fatal(err)
	}

}
