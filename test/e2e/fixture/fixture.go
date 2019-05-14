package fixture

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	. "github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	grpcutil "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/rand"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	defaultAriServer = "localhost:8080"
	adminPassword    = "password"
	testingLabel     = "e2e.argoproj.io"
	ArgoCDNamespace  = "argocd-e2e"

	// ensure all repos are in one directory tree, so we can easily clean them up
	tmpDir = "/tmp/argo-e2e"
)

var (
	id               string
	KubeClientset    kubernetes.Interface
	AppClientset     appclientset.Interface
	ArgoCDClientset  argocdclient.Client
	SettingsManager  *settings.SettingsManager
	apiServerAddress string
	token            string
	plainText        bool
)

// getKubeConfig creates new kubernetes client config using specified config path and config overrides variables
func getKubeConfig(configPath string, overrides clientcmd.ConfigOverrides) *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	restConfig, err := clientConfig.ClientConfig()
	CheckError(err)
	return restConfig
}

// NewFixture creates e2e tests fixture: ensures that Application CRD is installed, creates temporal namespace, starts repo and api server,
// configure currently available cluster.
func init() {
	CheckError2(execCommand("", "kubectl", "api-resources"))

	config := getKubeConfig("", clientcmd.ConfigOverrides{})
	AppClientset = appclientset.NewForConfigOrDie(config)
	KubeClientset = kubernetes.NewForConfigOrDie(config)
	apiServerAddress = os.Getenv(argocdclient.EnvArgoCDServer)
	if apiServerAddress == "" {
		apiServerAddress = defaultAriServer
	}

	tlsTestResult, err := grpcutil.TestTLS(apiServerAddress)
	CheckError(err)

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{Insecure: true, ServerAddr: apiServerAddress, PlainText: !tlsTestResult.TLS})
	CheckError(err)

	closer, client, err := ArgoCDClientset.NewSessionClient()
	CheckError(err)
	defer util.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &session.SessionCreateRequest{Username: "admin", Password: adminPassword})
	CheckError(err)

	CheckError2(argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		ServerAddr: apiServerAddress,
		AuthToken:  sessionResponse.Token,
		PlainText:  !tlsTestResult.TLS,
	}))

	SettingsManager = settings.NewSettingsManager(context.Background(), KubeClientset, "argocd-e2e")
	token = sessionResponse.Token
	plainText = !tlsTestResult.TLS

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")
}

func Name() string {
	return fmt.Sprintf("argocd-e2e-%s", id)
}

func repoDirectory() string {
	return tmpDir + "/" + Name()
}

func RepoURL() string {
	return fmt.Sprintf("file:///%s", repoDirectory())
}

func DeploymentNamespace() string {
	return Name()
}

func EnsureCleanState() {

	// reset settings
	argoSettings, err := SettingsManager.GetSettings()
	CheckError(err)
	if len(argoSettings.ResourceOverrides) > 0 {
		argoSettings.ResourceOverrides = nil
		CheckError(SettingsManager.SaveSettings(argoSettings))
	}

	// delete resources
	CheckError2(execCommand("", "kubectl", "-n", ArgoCDNamespace, "delete", "app", "--all", "--wait"))
	CheckError2(execCommand("", "kubectl", "-n", ArgoCDNamespace, "delete", "appprojects", "--field-selector", "metadata.name!=default", "--wait"))
	CheckError2(execCommand("", "kubectl", "delete", "ns", "-l", testingLabel+"=true", "--field-selector", "status.phase=Active", "--wait"))

	// remove tmp dir
	CheckError(os.RemoveAll(tmpDir))

	id = strings.ToLower(rand.RandString(8))

	// create tmp dir
	CheckError2(execCommand("", "mkdir", "-p", tmpDir))

	// set-up tmp repo, must have unique name
	CheckError2(execCommand("", "cp", "-Rf", "testdata", repoDirectory()))
	CheckError2(execCommand(repoDirectory(), "chmod", "777", "."))
	CheckError2(execCommand(repoDirectory(), "git", "init"))
	CheckError2(execCommand(repoDirectory(), "git", "add", "."))
	CheckError2(execCommand(repoDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	// create namespace
	CheckError2(execCommand(repoDirectory(), "kubectl", "create", "ns", DeploymentNamespace()))
	CheckError2(execCommand(repoDirectory(), "kubectl", "label", "ns", DeploymentNamespace(), testingLabel+"=true"))

}

func RunCli(args ...string) (string, error) {
	if plainText {
		args = append(args, "--plaintext")
	}

	args = append(args, "--server", apiServerAddress, "--auth-token", token, "--insecure")

	return execCommand("", "../../dist/argocd", args...)
}

func Patch(path string, jsonPatch string) {

	log.WithFields(log.Fields{"path": path, "jsonPatch": jsonPatch}).Info("patching")

	filename := filepath.Join(repoDirectory(), path)
	bytes, err := ioutil.ReadFile(filename)
	CheckError(err)

	patch, err := jsonpatch.DecodePatch([]byte(jsonPatch))
	CheckError(err)

	isYaml := strings.HasSuffix(filename, ".yaml")
	if isYaml {
		log.Info("converting YAML to JSON")
		bytes, err = yaml.YAMLToJSON(bytes)
		CheckError(err)
	}

	log.WithFields(log.Fields{"bytes": string(bytes)}).Info("JSON")

	bytes, err = patch.Apply(bytes)
	CheckError(err)

	if isYaml {
		log.Info("converting JSON back to YAML")
		bytes, err = yaml.JSONToYAML(bytes)
		CheckError(err)
	}

	CheckError(ioutil.WriteFile(filename, bytes, 0644))
	CheckError2(execCommand(repoDirectory(), "git", "diff"))
	CheckError2(execCommand(repoDirectory(), "git", "commit", "-am", "patch"))
}
