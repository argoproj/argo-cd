package fixture

import (
	controller "github.com/argoproj/argo-cd/cmd/argocd-application-controller/commands"
	reposerver "github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
	server "github.com/argoproj/argo-cd/cmd/argocd-server/commands"
	. "github.com/argoproj/argo-cd/errors"
	log "github.com/sirupsen/logrus"
	"os"
)

const logLevel = "info"

var launched = false

func Launch() {

	log.Info("launching...")

	// TODO - not needed?
	if launched {
		return
	}

	CheckError(os.Setenv("ARGOCD_FAKE_IN_CLUSTER", "true"))
	CheckError(os.Setenv("ARGOCD_OPTS", "--server localhost:8080 --plaintext"))

	launched = true

	go startApiServer()
	go startRepoServer()
	go startController()

	log.Info("launched")
}

func startController() {
	log.Info("starting controller...")
	command := controller.NewCommand()
	command.SetArgs([]string{
		"--loglevel", logLevel,
		"--redis", "localhost:6379",
		"--repo-server", "localhost:8081",
	})
	CheckError(command.Execute())
}

func startApiServer() {
	log.Info("starting API server...")
	command := server.NewCommand()
	command.SetArgs([]string{
		"--loglevel", logLevel,
		"--redis", "localhost:6379",
		"--disable-auth", "--insecure",
		"--repo-server", "localhost:8081",
	})
	CheckError(command.Execute())
}
func startRepoServer() {
	log.Info("starting repo server...")
	command := reposerver.NewCommand()
	command.SetArgs([]string{
		"--loglevel", logLevel,
		"--redis", "localhost:6379",
	})
	CheckError(command.Execute())
}
