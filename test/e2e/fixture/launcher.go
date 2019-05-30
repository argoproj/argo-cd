package fixture

import (
	"os"
	"time"

	controller "github.com/argoproj/argo-cd/cmd/argocd-application-controller/commands"
	reposerver "github.com/argoproj/argo-cd/cmd/argocd-repo-server/commands"
	server "github.com/argoproj/argo-cd/cmd/argocd-server/commands"
	. "github.com/argoproj/argo-cd/errors"
	log "github.com/sirupsen/logrus"
)

const logLevel = "debug"

func Launch() {

	_, err := Run("", "git-ask-pass.sh")
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Fatal("git-ask-pass.sh must be in path")
	}

	if os.Getenv("FORCE_LOG_COLORS") != "1" {
		log.Fatal("envvar FORCE_LOG_COLORS must be '1'")
	}
	if os.Getenv("ARGOCD_FAKE_IN_CLUSTER") != "true" {
		log.Fatal("envvar ARGOCD_FAKE_IN_CLUSTER must be 'true'")
	}
	if os.Getenv("ARGOCD_OPTS") != "--server localhost:8080 --plaintext" {
		log.Fatal("ARGOCD_OPTS must be '--server localhost:8080 --plaintext'")
	}

	log.Info("launching...")
	go startApiServer()
	time.Sleep(3 * time.Second)
	go startRepoServer()
	go startController()

	log.Info("launched")
}

func startController() {
	log.Info("starting app controller...")
	command := controller.NewCommand()
	command.SetArgs([]string{"--loglevel", logLevel, "--redis", "localhost:6379", "--repo-server", "localhost:8081"})
	CheckError(command.Execute())
}

func startApiServer() {
	log.Info("starting API server...")
	command := server.NewCommand()
	command.SetArgs([]string{"--loglevel", logLevel, "--redis", "localhost:6379", "--disable-auth", "--insecure", "--repo-server", "localhost:8081"})
	CheckError(command.Execute())
}
func startRepoServer() {
	log.Info("starting repo server...")
	command := reposerver.NewCommand()
	command.SetArgs([]string{"--loglevel", logLevel, "--redis", "localhost:6379"})
	CheckError(command.Execute())
}
