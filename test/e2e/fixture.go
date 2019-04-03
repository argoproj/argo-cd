package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/util"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/settings"

	argoexec "github.com/argoproj/pkg/exec"
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

	testRepo, err := ioutil.TempDir("/tmp", "argocd-e2e")
	errors.CheckError(err)
	testRepo = path.Base(testRepo)
	errors.CheckError(err)
	_, err = argoexec.RunCommand(
		"sh", "-c",
		fmt.Sprintf("cp -r testdata/* /tmp/%s && chmod 777 /tmp/%s && cd /tmp/%s && git init && git add . && git commit -m 'initial commit'", testRepo, testRepo, testRepo))
	errors.CheckError(err)

	fixture := &Fixture{
		AppClientset:     appClient,
		KubeClientset:    kubeClient,
		ArgoCDClientset:  argocdclientset,
		ArgoCDNamespace:  "argocd-e2e",
		SettingsManager:  settings.NewSettingsManager(context.Background(), kubeClient, "argocd-e2e"),
		apiServerAddress: apiServerAddress,
		token:            sessionResponse.Token,
		repoDirectory:    testRepo,
		plainText:        !tlsTestResult.TLS,
	}

	fixture.DeploymentNamespace = fixture.createDeploymentNamespace()
	return fixture, nil
}

func (f *Fixture) RepoURL() string {
	return fmt.Sprintf("file:///tmp/%s", f.repoDirectory)
}

// cleanup deletes test namespace resources.
func (f *Fixture) cleanup() {
	f.deleteDeploymentNamespace()
	f.cleanupTestRepo()
	f.EnsureCleanState()
}

func (f *Fixture) cleanupTestRepo() {
	err := os.RemoveAll(path.Join("/tmp", f.repoDirectory))
	errors.CheckError(err)
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

func (f *Fixture) EnsureCleanState() {
	argoSettings, err := f.SettingsManager.GetSettings()
	errors.CheckError(err)

	if len(argoSettings.ResourceOverrides) > 0 {
		argoSettings.ResourceOverrides = nil
		errors.CheckError(f.SettingsManager.SaveSettings(argoSettings))
	}

	closer, client := f.ArgoCDClientset.NewApplicationClientOrDie()
	defer util.Close(closer)
	apps, err := client.List(context.Background(), &application.ApplicationQuery{})
	errors.CheckError(err)
	err = util.RunAllAsync(len(apps.Items), func(i int) error {
		cascade := true
		appName := apps.Items[i].Name
		_, err := client.Delete(context.Background(), &application.ApplicationDeleteRequest{Name: &appName, Cascade: &cascade})
		if err != nil {
			return nil
		}
		return waitUntilE(func() (bool, error) {
			_, err := f.AppClientset.ArgoprojV1alpha1().Applications(f.ArgoCDNamespace).Get(appName, v1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})
	})
	errors.CheckError(err)

	projs, err := f.AppClientset.ArgoprojV1alpha1().AppProjects(f.ArgoCDNamespace).List(v1.ListOptions{})
	errors.CheckError(err)
	err = util.RunAllAsync(len(projs.Items), func(i int) error {
		if projs.Items[i].Name == "default" {
			return nil
		}
		return f.AppClientset.ArgoprojV1alpha1().AppProjects(f.ArgoCDNamespace).Delete(projs.Items[i].Name, &v1.DeleteOptions{})
	})
	errors.CheckError(err)
}

func (f *Fixture) deleteDeploymentNamespace() {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(testingLabel, selection.Equals, []string{"true"})
	errors.CheckError(err)
	labelSelector = labelSelector.Add(*req)

	namespaces, err := f.KubeClientset.CoreV1().Namespaces().List(v1.ListOptions{LabelSelector: labelSelector.String()})
	errors.CheckError(err)

	for _, ns := range namespaces.Items {
		if ns.DeletionTimestamp == nil {
			err = f.KubeClientset.CoreV1().Namespaces().Delete(ns.Name, &v1.DeleteOptions{})
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
	cmd := exec.Command("../../dist/argocd", append(args, "--server", f.apiServerAddress, "--auth-token", f.token, "--insecure")...)
	log.Infof("CLI: %s", strings.Join(cmd.Args, " "))
	outBytes, err := cmd.Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return "", err
		}
		errOutput := string(exErr.Stderr)
		if outBytes != nil {
			errOutput = string(outBytes) + "\n" + errOutput
		}
		return errOutput, fmt.Errorf(strings.TrimSpace(errOutput))
	}
	return string(outBytes), nil
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
