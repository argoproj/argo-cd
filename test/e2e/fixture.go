package e2e

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/install"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server/cluster"
	apirepository "github.com/argoproj/argo-cd/server/repository"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	TestTimeout = time.Minute * 3
)

// Fixture represents e2e tests fixture.
type Fixture struct {
	Config             *rest.Config
	KubeClient         kubernetes.Interface
	ExtensionsClient   apiextensionsclient.Interface
	AppClient          appclientset.Interface
	ApiRepoService     apirepository.RepositoryServiceServer
	RepoClientset      reposerver.Clientset
	AppComparator      controller.AppComparator
	Namespace          string
	InstanceID         string
	repoServerGRPC     *grpc.Server
	repoServerListener net.Listener
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
	installer, err := install.NewInstaller(f.Config, install.InstallOptions{})
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	f.repoServerListener = listener
	go func() {
		err = f.repoServerGRPC.Serve(listener)
	}()
	installer.InstallApplicationCRD()
	return err
}

// TearDown deletes fixture resources.
func (f *Fixture) TearDown() {
	err := f.KubeClient.CoreV1().Namespaces().Delete(f.Namespace, &metav1.DeleteOptions{})
	if err != nil {
		f.repoServerGRPC.Stop()
	}
	if err != nil {
		println("Unable to tear down fixture")
	}
}

// GetKubeConfig creates new kubernetes client config using specified config path and config overrides variables
func GetKubeConfig(configPath string, overrides clientcmd.ConfigOverrides) *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	var err error
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}
	return restConfig
}

// NewFixture creates e2e tests fixture.
func NewFixture() (*Fixture, error) {
	config := GetKubeConfig("", clientcmd.ConfigOverrides{})
	extensionsClient := apiextensionsclient.NewForConfigOrDie(config)
	appClient := appclientset.NewForConfigOrDie(config)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	namespace, err := createNamespace(kubeClient)
	clusterService := cluster.NewServer(namespace, kubeClient, appClient)
	repoServerGRPC := reposerver.NewServer(kubeClient, namespace).CreateGRPC(&FakeGitClient{})
	if err != nil {
		return nil, err
	}
	appComparator := controller.NewKsonnetAppComparator(clusterService)
	fixture := &Fixture{
		Config:           config,
		ExtensionsClient: extensionsClient,
		AppClient:        appClient,
		KubeClient:       kubeClient,
		Namespace:        namespace,
		InstanceID:       namespace,
		ApiRepoService:   apirepository.NewServer(namespace, kubeClient, appClient),
		AppComparator:    appComparator,
		repoServerGRPC:   repoServerGRPC,
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
	return controller.NewApplicationController(
		f.KubeClient,
		f.AppClient,
		reposerver.NewRepositoryServerClientset(f.repoServerListener.Addr().String()),
		f.ApiRepoService,
		f.AppComparator,
		time.Second,
		&controller.ApplicationControllerConfig{Namespace: f.Namespace, InstanceID: f.InstanceID})
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
	_, err := exec.Command("rm", "-rf", repoPath).Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("cp", "-r", "functional/ks-example", repoPath).Output()
	return err
}

func (c *FakeGitClient) Checkout(repoPath string, sha string) error {
	// do nothing
	return nil
}
