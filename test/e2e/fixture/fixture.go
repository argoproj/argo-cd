package fixture

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	TestTimeout = time.Minute * 3

	defaultAriServer = "localhost:8080"
	adminPassword    = "password"
	testingLabel     = "e2e.argoproj.io"
	ArgoCDNamespace  = "argocd-e2e"

	// ensure all repos are in one directory tree, so we can easily clean them up
	tmpDir = "/tmp/argo-e2e"
)

var (
	id               int
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
	errors.CheckError(err)
	return restConfig
}

// NewFixture creates e2e tests fixture: ensures that Application CRD is installed, creates temporal namespace, starts repo and api server,
// configure currently available cluster.
func init() {
	config := getKubeConfig("", clientcmd.ConfigOverrides{})
	AppClientset = appclientset.NewForConfigOrDie(config)
	KubeClientset = kubernetes.NewForConfigOrDie(config)
	apiServerAddress = os.Getenv(argocdclient.EnvArgoCDServer)
	if apiServerAddress == "" {
		apiServerAddress = defaultAriServer
	}

	tlsTestResult, err := grpc_util.TestTLS(apiServerAddress)
	errors.CheckError(err)

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{Insecure: true, ServerAddr: apiServerAddress, PlainText: !tlsTestResult.TLS})
	errors.CheckError(err)

	closer, client, err := ArgoCDClientset.NewSessionClient()
	errors.CheckError(err)
	defer util.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &session.SessionCreateRequest{Username: "admin", Password: adminPassword})
	errors.CheckError(err)

	_, err = argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		ServerAddr: apiServerAddress,
		AuthToken:  sessionResponse.Token,
		PlainText:  !tlsTestResult.TLS,
	})
	errors.CheckError(err)

	SettingsManager = settings.NewSettingsManager(context.Background(), KubeClientset, "argocd-e2e")
	token = sessionResponse.Token
	plainText = !tlsTestResult.TLS

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")
}

func Name() string {
	return fmt.Sprintf("argocd-e2e-%d", id)
}

func repoDirectory() string {
	return tmpDir + "/" + Name()
}

func setUpTestRepo() {
	errors.CheckError2(execCommand("", "cp", "-Rf", "../testdata", repoDirectory()))
	errors.CheckError2(execCommand(repoDirectory(), "chmod", "777", "."))
	errors.CheckError2(execCommand(repoDirectory(), "git", "init"))
	errors.CheckError2(execCommand(repoDirectory(), "git", "add", "."))
	errors.CheckError2(execCommand(repoDirectory(), "git", "commit", "-q", "-m", "initial commit"))
}

func RepoURL() string {
	return fmt.Sprintf("file:///%s", repoDirectory())
}

// Teardown deletes test namespace resources.
func Teardown() {
	log.Info("tearing down")
	resetSettings()
	deleteApps()
	deleteAppProjects()
	deleteDeploymentNamespaces()
	deleteTmpDir()
}

func deleteTmpDir() {
	errors.CheckError(os.RemoveAll(tmpDir))
}

func createTmpDir() {
	errors.CheckError2(execCommand("", "mkdir", "-p", tmpDir))
}

func createDeploymentNamespace() {
	errors.CheckError2(KubeClientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: Name(),
			Labels: map[string]string{
				testingLabel: "true",
			},
		},
	}))
}

func EnsureCleanState() {
	Teardown()
	checkCleanliness()
	id = id + 1
	setUp()
}

func setUp() {
	log.Info("setting-up")
	createTmpDir()
	createDeploymentNamespace()
	setUpTestRepo()
}
func resetSettings() {
	argoSettings, err := SettingsManager.GetSettings()
	errors.CheckError(err)
	if len(argoSettings.ResourceOverrides) > 0 {
		argoSettings.ResourceOverrides = nil
		errors.CheckError(SettingsManager.SaveSettings(argoSettings))
	}
}

func deleteApps() {
	for _, app := range getApps() {
		appName := app.Name
		log.WithFields(log.Fields{"app": appName}).Info("terminating and deleting app")
		_, _ = execCommand("", "argocd", "app", "terminate-op", appName)
		errors.CheckError2(execCommand("", "argocd", "app", "delete", appName))
		err := waitUntilE(func() (bool, error) {
			_, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(appName, v1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		errors.CheckError(err)
	}
}

func getApps() []v1alpha1.Application {
	closer, client := ArgoCDClientset.NewApplicationClientOrDie()
	defer util.Close(closer)
	apps, err := client.List(context.Background(), &application.ApplicationQuery{})
	errors.CheckError(err)
	return apps.Items
}

func deleteAppProjects() {
	projs, err := AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).List(v1.ListOptions{})
	errors.CheckError(err)
	for _, proj := range projs.Items {
		if proj.Name == "default" {
			continue
		}
		err := AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Delete(proj.Name, &v1.DeleteOptions{})
		errors.CheckError(err)
	}
}

func checkCleanliness() {
	apps := getApps()
	if len(apps) > 0 {
		log.WithFields(log.Fields{"apps": apps}).Fatal("expected zero apps")
	}
	projects := getAppProjects()
	if len(projects) > 1 {
		log.WithFields(log.Fields{"projects": projects}).Fatalf("expected all projects to be deleted")
	}
}

func getAppProjects() []v1alpha1.AppProject {
	projects, err := AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).List(v1.ListOptions{})
	errors.CheckError(err)
	return projects.Items
}

func deleteDeploymentNamespaces() {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(testingLabel, selection.Equals, []string{"true"})
	errors.CheckError(err)
	labelSelector = labelSelector.Add(*req)

	namespaces, err := KubeClientset.CoreV1().Namespaces().List(v1.ListOptions{LabelSelector: labelSelector.String()})
	errors.CheckError(err)

	for _, ns := range namespaces.Items {
		if ns.DeletionTimestamp == nil {
			namespace := ns.Name
			log.WithFields(log.Fields{"namespace": namespace}).Info("deleting namespace")
			err = KubeClientset.CoreV1().Namespaces().Delete(namespace, &v1.DeleteOptions{})
			if err != nil {
				log.Warnf("Failed to delete e2e deployment namespace: %s", ns.Name)
			}
		}
	}
}

func RunCli(args ...string) (string, error) {
	if plainText {
		args = append(args, "--plaintext")
	}

	args = append(args, "--server", apiServerAddress, "--auth-token", token, "--insecure")

	return execCommand("", "../../../dist/argocd", args...)
}

func Patch(path string, jsonPatch string) {

	log.WithFields(log.Fields{"path": path, "jsonPatch": jsonPatch}).Info("patching")

	filename := filepath.Join(repoDirectory(), path)
	bytes, err := ioutil.ReadFile(filename)
	errors.CheckError(err)

	patch, err := jsonpatch.DecodePatch([]byte(jsonPatch))
	errors.CheckError(err)

	isYaml := strings.HasSuffix(filename, ".yaml")
	if isYaml {
		log.Info("converting YAML to JSON")
		bytes, err = yaml.YAMLToJSON(bytes)
		errors.CheckError(err)
	}

	log.WithFields(log.Fields{"bytes": string(bytes)}).Info("JSON")

	bytes, err = patch.Apply(bytes)
	errors.CheckError(err)

	if isYaml {
		log.Info("converting JSON back to YAML")
		bytes, err = yaml.JSONToYAML(bytes)
		errors.CheckError(err)
	}

	errors.CheckError(ioutil.WriteFile(filename, bytes, 0644))
	errors.CheckError2(execCommand(repoDirectory(), "git", "diff"))
	errors.CheckError2(execCommand(repoDirectory(), "git", "commit", "-am", "patch"))
}

func DeploymentNamespace() string {
	return Name()
}

func waitUntilE(condition wait.ConditionFunc) error {
	stop := make(chan struct{})
	isClosed := false
	makeSureClosed := func() {
		if !isClosed {
			close(stop)
			isClosed = true
		}
	}
	defer makeSureClosed()
	go func() {
		time.Sleep(TestTimeout)
		makeSureClosed()
	}()
	return wait.PollUntil(time.Second, condition, stop)
}
