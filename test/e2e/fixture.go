package e2e

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/application"
	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/repository"
	"k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	TestTimeout = time.Second * 3
)

// Fixture represents e2e tests fixture.
type Fixture struct {
	AppManager       *application.Manager
	KubeClient       kubernetes.Interface
	ExtensionsClient apiextensionsclient.Interface
	AppClient        appclientset.Interface
	RepoService      repository.RepositoryServiceServer
	Namespace        string
	InstanceID       string
}

func createNamespace(kubeClient *kubernetes.Clientset) (string, error) {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "argo-e2e-test-",
		},
	}
	cns, err := kubeClient.CoreV1().Namespaces().Create(ns)
	if err != nil {
		return "", err
	}
	return cns.Name, nil
}

func (f *Fixture) setup() error {
	common.NewInstaller(f.ExtensionsClient, f.KubeClient).Install(common.InstallParameters{
		DryRun:         false,
		SkipController: true,
	})
	return nil
}

// TearDown deletes fixture resources.
func (f *Fixture) TearDown() {
	err := f.KubeClient.CoreV1().Namespaces().Delete(f.Namespace, &metav1.DeleteOptions{})
	if err != nil {
		println("Unable to tear down fixture")
	}
}

// NewFixture creates e2e tests fixture.
func NewFixture() (*Fixture, error) {
	config := commands.GetKubeConfig("", clientcmd.ConfigOverrides{})
	extensionsClient := apiextensionsclient.NewForConfigOrDie(config)
	appClient := appclientset.NewForConfigOrDie(config)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	namespace, err := createNamespace(kubeClient)
	clusterService := cluster.NewServer(namespace, kubeClient, appClient)
	appManager := application.NewAppManager(
		&FakeGitClient{},
		repository.NewServer(namespace, kubeClient, appClient),
		clusterService,
		application.NewKsonnetAppComparator(clusterService),
		time.Second)
	if err != nil {
		return nil, err
	}
	fixture := &Fixture{
		ExtensionsClient: extensionsClient,
		AppClient:        appClient,
		KubeClient:       kubeClient,
		Namespace:        namespace,
		InstanceID:       namespace,
		RepoService:      repository.NewServer(namespace, kubeClient, appClient),
		AppManager:       appManager,
	}
	err = fixture.setup()
	if err != nil {
		return nil, err
	}
	return fixture, nil
}

// CreateApp creates application with appropriate controller instance id.
func (f *Fixture) CreateApp(t *testing.T, application *v1alpha1.Application) *v1alpha1.Application {
	labels := application.ObjectMeta.Labels
	if labels == nil {
		labels = make(map[string]string)
		application.ObjectMeta.Labels = labels
	}
	labels[common.LabelKeyApplicationControllerInstanceID] = f.InstanceID

	app, err := f.AppClient.ArgoprojV1alpha1().Applications(f.Namespace).Create(application)
	if err != nil {
		t.Fatal(fmt.Sprintf("Unable to create app %v", err))
	}
	return app
}

// CreateController creates new controller instance
func (f *Fixture) CreateController() *controller.ApplicationController {
	return controller.NewApplicationController(f.KubeClient, f.AppClient, f.AppManager, time.Second, &controller.ApplicationControllerConfig{
		Namespace:  f.Namespace,
		InstanceID: f.InstanceID,
	})
}

// PollUntil periodically executes specified condition until it returns true.
func PollUntil(t *testing.T, condition wait.ConditionFunc) {
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
	err := wait.PollUntil(time.Second, condition, stop)
	if err != nil {
		t.Fatal("Failed to wait for expected condition")
	}
}

// FakeGitClient is a test git client implementation which always clone local test repo.
type FakeGitClient struct {
}

func (c *FakeGitClient) CloneOrFetch(repo string, username string, password string, repoPath string) error {
	_, err := exec.Command("cp", "-r", "functional/ks-example", repoPath).Output()
	return err
}

func (c *FakeGitClient) Checkout(repoPath string, sha string) error {
	// do nothing
	return nil
}
