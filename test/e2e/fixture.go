package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/cmd/argocd/commands"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller"
	"github.com/argoproj/argo-cd/install"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/git"
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	TestTimeout = time.Minute * 3
)

// Fixture represents e2e tests fixture.
type Fixture struct {
	Config            *rest.Config
	KubeClient        kubernetes.Interface
	ExtensionsClient  apiextensionsclient.Interface
	AppClient         appclientset.Interface
	DB                db.ArgoDB
	Namespace         string
	InstanceID        string
	RepoServerAddress string
	ApiServerAddress  string
	Enforcer          *rbac.Enforcer

	tearDownCallback func()
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

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer util.Close(l)
	return l.Addr().(*net.TCPAddr).Port, nil
}

func (f *Fixture) setup() error {
	installer, err := install.NewInstaller(f.Config, install.InstallOptions{})
	if err != nil {
		return err
	}
	installer.InstallApplicationCRD()

	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: common.ArgoCDRBACConfigMapName,
		},
		Data: map[string]string{
			rbac.ConfigMapPolicyDefaultKey: "role:admin",
		},
	}
	_, err = f.KubeClient.CoreV1().ConfigMaps(f.Namespace).Create(&cm)
	if err != nil {
		return err
	}

	settingsMgr := settings.NewSettingsManager(f.KubeClient, f.Namespace)
	err = settingsMgr.SaveSettings(&settings.ArgoCDSettings{})
	if err != nil {
		return err
	}

	err = f.ensureClusterRegistered()
	if err != nil {
		return err
	}

	apiServerPort, err := getFreePort()
	if err != nil {
		return err
	}

	memCache := cache.NewInMemoryCache(repository.DefaultRepoCacheExpiration)
	repoServerGRPC := reposerver.NewServer(&FakeGitClientFactory{}, memCache).CreateGRPC()
	repoServerListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	f.RepoServerAddress = repoServerListener.Addr().String()
	f.ApiServerAddress = fmt.Sprintf("127.0.0.1:%d", apiServerPort)

	apiServer := server.NewServer(server.ArgoCDServerOpts{
		Namespace:     f.Namespace,
		AppClientset:  f.AppClient,
		DisableAuth:   true,
		Insecure:      true,
		KubeClientset: f.KubeClient,
		RepoClientset: reposerver.NewRepositoryServerClientset(f.RepoServerAddress),
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err = repoServerGRPC.Serve(repoServerListener)
	}()
	go func() {
		apiServer.Run(ctx, apiServerPort)
	}()

	f.tearDownCallback = func() {
		cancel()
		repoServerGRPC.Stop()
	}
	if err != nil {
		return err
	}

	return waitUntilE(func() (done bool, err error) {
		clientset, err := f.NewApiClientset()
		if err != nil {
			return false, nil
		}
		conn, appClient, err := clientset.NewApplicationClient()
		if err != nil {
			return false, nil
		}
		defer util.Close(conn)
		_, err = appClient.List(context.Background(), &application.ApplicationQuery{})
		return err == nil, nil
	})
}

func (f *Fixture) ensureClusterRegistered() error {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig
	overrides := clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)
	conf, err := clientConfig.ClientConfig()
	if err != nil {
		return err
	}
	// Install RBAC resources for managing the cluster
	managerBearerToken := common.InstallClusterManagerRBAC(conf)
	clst := commands.NewCluster(f.Config.Host, conf, managerBearerToken)
	clstCreateReq := cluster.ClusterCreateRequest{Cluster: clst}
	_, err = cluster.NewServer(f.DB, f.Enforcer).Create(context.Background(), &clstCreateReq)
	return err
}

// TearDown deletes fixture resources.
func (f *Fixture) TearDown() {
	if f.tearDownCallback != nil {
		f.tearDownCallback()
	}
	apps, err := f.AppClient.ArgoprojV1alpha1().Applications(f.Namespace).List(metav1.ListOptions{})
	if err == nil {
		for _, app := range apps.Items {
			if len(app.Finalizers) > 0 {
				var patch []byte
				patch, err = json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": make([]string, 0),
					},
				})
				if err == nil {
					_, err = f.AppClient.ArgoprojV1alpha1().Applications(app.Namespace).Patch(app.Name, types.MergePatchType, patch)
				}
			}
			if err != nil {
				break
			}
		}
	}
	if err == nil {
		err = f.KubeClient.CoreV1().Namespaces().Delete(f.Namespace, &metav1.DeleteOptions{})
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

// NewFixture creates e2e tests fixture: ensures that Application CRD is installed, creates temporal namespace, starts repo and api server,
// configure currently available cluster.
func NewFixture() (*Fixture, error) {
	config := GetKubeConfig("", clientcmd.ConfigOverrides{})
	extensionsClient := apiextensionsclient.NewForConfigOrDie(config)
	appClient := appclientset.NewForConfigOrDie(config)
	kubeClient := kubernetes.NewForConfigOrDie(config)
	namespace, err := createNamespace(kubeClient)
	if err != nil {
		return nil, err
	}
	db := db.NewDB(namespace, kubeClient)
	enforcer := rbac.NewEnforcer(kubeClient, namespace, common.ArgoCDRBACConfigMapName)
	enforcer.SetDefaultRole("role:admin")

	fixture := &Fixture{
		Config:           config,
		ExtensionsClient: extensionsClient,
		AppClient:        appClient,
		DB:               db,
		KubeClient:       kubeClient,
		Namespace:        namespace,
		InstanceID:       namespace,
		Enforcer:         enforcer,
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
	appStateManager := controller.NewAppStateManager(
		f.DB, f.AppClient, reposerver.NewRepositoryServerClientset(f.RepoServerAddress), f.Namespace)

	appHealthManager := controller.NewAppHealthManager(f.DB, f.Namespace)

	return controller.NewApplicationController(
		f.Namespace,
		f.KubeClient,
		f.AppClient,
		f.DB,
		appStateManager,
		appHealthManager,
		10*time.Second,
		&controller.ApplicationControllerConfig{Namespace: f.Namespace, InstanceID: f.InstanceID})
}

func (f *Fixture) NewApiClientset() (argocdclient.Client, error) {
	return argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		PlainText:  true,
		ServerAddr: f.ApiServerAddress,
	})
}

func (f *Fixture) RunCli(args ...string) (string, error) {
	cmd := commands.NewCommand()
	cmd.SetArgs(append(args, "--server", f.ApiServerAddress, "--plaintext"))
	output := new(bytes.Buffer)
	cmd.SetOutput(output)
	err := cmd.Execute()
	return output.String(), err
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
		t.Fatal("Failed to wait for expected condition")
	}
}

type FakeGitClientFactory struct{}

func (f *FakeGitClientFactory) NewClient(repoURL, path, username, password, sshPrivateKey string) git.Client {
	return &FakeGitClient{
		root: path,
	}
}

// FakeGitClient is a test git client implementation which always clone local test repo.
type FakeGitClient struct {
	root string
}

func (c *FakeGitClient) Init() error {
	_, err := exec.Command("rm", "-rf", c.root).Output()
	if err != nil {
		return err
	}
	_, err = exec.Command("cp", "-r", "../../examples/guestbook", c.root).Output()
	return err
}

func (c *FakeGitClient) Root() string {
	return c.root
}

func (c *FakeGitClient) Fetch() error {
	// do nothing
	return nil
}

func (c *FakeGitClient) Checkout(revision string) error {
	// do nothing
	return nil
}

func (c *FakeGitClient) Reset() error {
	// do nothing
	return nil
}

func (c *FakeGitClient) LsRemote(s string) (string, error) {
	return "abcdef123456890", nil
}

func (c *FakeGitClient) LsFiles(s string) ([]string, error) {
	return []string{"abcdef123456890"}, nil
}

func (c *FakeGitClient) CommitSHA() (string, error) {
	return "abcdef123456890", nil
}
