package fixture

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

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
	repoUrl          string
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

// creates e2e tests fixture: ensures that Application CRD is installed, creates temporal namespace, starts repo and api server,
// configure currently available cluster.
func init() {

	// trouble-shooting check to see if this busted add-on is going to cause problems
	FailOnErr(Run("", "kubectl", "api-resources", "-o", "name"))

	// set-up variables
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

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		ServerAddr: apiServerAddress,
		AuthToken:  sessionResponse.Token,
		PlainText:  !tlsTestResult.TLS,
	})
	CheckError(err)

	SettingsManager = settings.NewSettingsManager(context.Background(), KubeClientset, "argocd-e2e")
	token = sessionResponse.Token
	plainText = !tlsTestResult.TLS

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")
}

func Name() string {
	return fmt.Sprintf("argocd-e2e-%s", id)
}

func repoDirectory() string {
	return path.Join(tmpDir, id)
}
func SetRepoURL(url string) {
	repoUrl = url
}

func RepoURL() string {
	return repoUrl
}

func DeploymentNamespace() string {
	return fmt.Sprintf("argocd-e2e-ns-%s", id)
}

// creates a secret for the current test, this currently can only create a single secret
func CreateSecret(username, password string) string {
	secretName := fmt.Sprintf("argocd-e2e-secret-%s", id)
	FailOnErr(Run("", "kubectl", "create", "secret", "generic", secretName,
		"--from-literal=username="+username,
		"--from-literal=password="+password))
	FailOnErr(Run("", "kubectl", "label", "secret", secretName, testingLabel+"=true"))
	return secretName
}

func EnsureCleanState() {

	start := time.Now()

	// delete resources
	text, err := Run("", "kubectl", "get", "app", "-o", "name")
	CheckError(err)
	for _, name := range strings.Split(text, "\n") {
		if name != "" {
			// is it much more reliable to get argocd to delete an app than kubectl
			FailOnErr(RunCli("app", "delete", strings.TrimPrefix(name, "application.argoproj.io/")))
		}
	}
	FailOnErr(Run("", "kubectl", "-n", ArgoCDNamespace, "delete", "appprojects", "--field-selector", "metadata.name!=default"))
	// takes around 5s, so we don't wait
	FailOnErr(Run("", "kubectl", "delete", "ns", "-l", testingLabel+"=true", "--field-selector", "status.phase=Active", "--wait=false"))
	FailOnErr(Run("", "kubectl", "delete", "secrets", "-l", testingLabel+"=true"))

	// reset settings
	argoSettings, err := SettingsManager.GetSettings()
	CheckError(err)
	CheckError(SettingsManager.SaveSettings(&settings.ArgoCDSettings{
		AdminPasswordHash:  argoSettings.AdminPasswordHash,
		AdminPasswordMtime: argoSettings.AdminPasswordMtime,
		Secrets:            argoSettings.Secrets,
	}))

	// remove tmp dir
	CheckError(os.RemoveAll(tmpDir))

	// new random ID
	id = strings.ToLower(rand.RandString(5))
	repoUrl = fmt.Sprintf("file:///%s", repoDirectory())

	// create tmp dir
	FailOnErr(Run("", "mkdir", "-p", tmpDir))

	// set-up tmp repo, must have unique name
	FailOnErr(Run("", "cp", "-Rf", "testdata", repoDirectory()))
	FailOnErr(Run(repoDirectory(), "chmod", "777", "."))
	FailOnErr(Run(repoDirectory(), "git", "init"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	// create namespace
	FailOnErr(Run("", "kubectl", "create", "ns", DeploymentNamespace()))
	FailOnErr(Run("", "kubectl", "label", "ns", DeploymentNamespace(), testingLabel+"=true"))

	log.WithFields(log.Fields{"duration": time.Since(start), "id": id}).Info("clean state")
}

func RunCli(args ...string) (string, error) {
	if plainText {
		args = append(args, "--plaintext")
	}

	args = append(args, "--server", apiServerAddress, "--auth-token", token, "--insecure")

	return Run("", "../../dist/argocd", args...)
}

func Patch(path string, jsonPatch string) {

	if !strings.HasPrefix(repoUrl, "file://") {
		log.WithFields(log.Fields{"repoUrl": repoUrl}).Fatal("cannot patch repo unless it is local")
	}

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
	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-am", "patch"))
}
