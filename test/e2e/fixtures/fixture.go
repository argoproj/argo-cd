package fixtures

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	argoexec "github.com/argoproj/pkg/exec"
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
)

// Fixture represents e2e tests fixture.
type Fixture struct {
	KubeClientset       kubernetes.Interface
	AppClientset        appclientset.Interface
	ArgoCDNamespace     string
	DeploymentNamespace string
	ArgoCDClientset     argocdclient.Client
	SettingsManager     *settings.SettingsManager

	repoDirectory    string
	apiServerAddress string
	token            string
	plainText        bool
}

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
func NewFixture() (*Fixture, error) {
	config := getKubeConfig("", clientcmd.ConfigOverrides{})
	appClient := appclientset.NewForConfigOrDie(config)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	apiServerAddress := os.Getenv(argocdclient.EnvArgoCDServer)
	if apiServerAddress == "" {
		apiServerAddress = defaultAriServer
	}
	log.Warnf("Using Argo CD server %s", apiServerAddress)

	tlsTestResult, err := grpc_util.TestTLS(apiServerAddress)
	errors.CheckError(err)

	argocdclientset, err := argocdclient.NewClient(&argocdclient.ClientOptions{Insecure: true, ServerAddr: apiServerAddress, PlainText: !tlsTestResult.TLS})
	errors.CheckError(err)

	closer, client, err := argocdclientset.NewSessionClient()
	errors.CheckError(err)

	defer util.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &session.SessionCreateRequest{Username: "admin", Password: adminPassword})
	errors.CheckError(err)

	argocdclientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		ServerAddr: apiServerAddress,
		AuthToken:  sessionResponse.Token,
		PlainText:  !tlsTestResult.TLS,
	})
	errors.CheckError(err)

	fixture := &Fixture{
		AppClientset:     appClient,
		KubeClientset:    kubeClient,
		ArgoCDClientset:  argocdclientset,
		ArgoCDNamespace:  "argocd-e2e",
		SettingsManager:  settings.NewSettingsManager(context.Background(), kubeClient, "argocd-e2e"),
		apiServerAddress: apiServerAddress,
		token:            sessionResponse.Token,
		repoDirectory:    "/tmp/argocd-e2e",
		plainText:        !tlsTestResult.TLS,
	}

	fixture.setUpTestRepo()

	fixture.DeploymentNamespace = fixture.createDeploymentNamespace()
	return fixture, nil
}

func (f *Fixture) setUpTestRepo() {
	f.teardownTestRepo()
	_, err := argoexec.RunCommand(
		"sh", "-c",
		fmt.Sprintf("mkdir %s && cp -R testdata/* %s && chmod 777 %s && cd %s && git init && git add . && git commit -m 'initial commit'", f.repoDirectory, f.repoDirectory, f.repoDirectory, f.repoDirectory))
	errors.CheckError(err)
}

func (f *Fixture) RepoURL() string {
	return fmt.Sprintf("file:///%s", f.repoDirectory)
}

// Teardown deletes test namespace resources.
func (f *Fixture) Teardown() {
	log.Info("tearing down")
	f.deleteApps()
	f.deleteDeploymentNamespaces()
	f.teardownTestRepo()
}

func (f *Fixture) teardownTestRepo() {
	errors.CheckError(os.RemoveAll(path.Join(f.repoDirectory)))
}

func (f *Fixture) createDeploymentNamespace() string {
	ns, err := f.KubeClientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "argocd-e2e-",
			Labels: map[string]string{
				testingLabel: "true",
			},
		},
	})
	errors.CheckError(err)
	return ns.Name
}

func (f *Fixture) SetUp() {
	f.resetSettings()
	f.deleteApps()
	f.deleteAppProjects()
	f.setUpTestRepo()
}

func (f *Fixture) resetSettings() {
	argoSettings, err := f.SettingsManager.GetSettings()
	errors.CheckError(err)
	if len(argoSettings.ResourceOverrides) > 0 {
		argoSettings.ResourceOverrides = nil
		errors.CheckError(f.SettingsManager.SaveSettings(argoSettings))
	}
}

func (f *Fixture) deleteApps() {
	closer, client := f.ArgoCDClientset.NewApplicationClientOrDie()
	defer util.Close(closer)
	apps, err := client.List(context.Background(), &application.ApplicationQuery{})
	errors.CheckError(err)
	cascade := false
	for _, app := range apps.Items {
		appName := app.Name
		log.WithFields(log.Fields{"app": appName}).Info("deleting app")
		_, err := client.Delete(context.Background(), &application.ApplicationDeleteRequest{Name: &appName, Cascade: &cascade})
		errors.CheckError(err)
		err = waitUntilE(func() (bool, error) {
			_, err := f.AppClientset.ArgoprojV1alpha1().Applications(f.ArgoCDNamespace).Get(appName, v1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
		errors.CheckError(err)
	}
}

func (f *Fixture) deleteAppProjects() {
	projs, err := f.AppClientset.ArgoprojV1alpha1().AppProjects(f.ArgoCDNamespace).List(v1.ListOptions{})
	errors.CheckError(err)
	for _, proj := range projs.Items {
		if proj.Name == "default" {
			continue
		}
		err := f.AppClientset.ArgoprojV1alpha1().AppProjects(f.ArgoCDNamespace).Delete(proj.Name, &v1.DeleteOptions{})
		errors.CheckError(err)
	}
}

func (f *Fixture) deleteDeploymentNamespaces() {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(testingLabel, selection.Equals, []string{"true"})
	errors.CheckError(err)
	labelSelector = labelSelector.Add(*req)

	namespaces, err := f.KubeClientset.CoreV1().Namespaces().List(v1.ListOptions{LabelSelector: labelSelector.String()})
	errors.CheckError(err)

	for _, ns := range namespaces.Items {
		if ns.DeletionTimestamp == nil {
			namespace := ns.Name
			log.WithFields(log.Fields{"namespace": namespace}).Info("deleting namespace")
			err = f.KubeClientset.CoreV1().Namespaces().Delete(namespace, &v1.DeleteOptions{})
			if err != nil {
				log.Warnf("Failed to delete e2e deployment namespace: %s", ns.Name)
			}
		}
	}
}

func (f *Fixture) RunCli(args ...string) (string, error) {
	if f.plainText {
		args = append(args, "--plaintext")
	}

	args = append(args, "--server", f.apiServerAddress, "--auth-token", f.token, "--insecure")

	log.WithFields(log.Fields{"args": args}).Info("running command")

	cmd := exec.Command("../../dist/argocd", args...)
	outBytes, err := cmd.Output()
	output := string(outBytes)

	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if ok {
			output = output + string(exErr.Stderr)
		}
	}

	for i, line := range strings.Split(output, "\n") {
		log.WithFields(log.Fields{"line": line, "i": i}).Info("command output")
	}

	log.WithFields(log.Fields{"err": err}).Info("ran command")

	return output, err
}

func (f *Fixture) Patch(path string, jsonPatch string) {

	log.WithFields(log.Fields{"path": path, "jsonPatch": jsonPatch}).Info("patching")

	filename := filepath.Join(f.repoDirectory, path)
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

	err = ioutil.WriteFile(filename, bytes, 0644)
	errors.CheckError(err)

	output, err := argoexec.RunCommand("sh", "-c", fmt.Sprintf("cd %s && git diff", f.repoDirectory))
	for i, line := range strings.Split(output, "\n") {
		log.Infof("%d: %s", i, line)
	}

	log.Info("committing")

	_, err = argoexec.RunCommand("sh", "-c", fmt.Sprintf("cd %s && git commit -am 'patch'", f.repoDirectory))
	errors.CheckError(err)
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

// WaitUntil periodically executes specified condition until it returns true.
func WaitUntil(t *testing.T, condition wait.ConditionFunc) {
	err := waitUntilE(condition)
	if err != nil {
		t.Fatalf("Failed to wait for expected condition: %v", err)
	}
}
