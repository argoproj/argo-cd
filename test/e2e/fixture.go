package e2e

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
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
	"github.com/argoproj/argo-cd/errors"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/assets"
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
	Config                  *rest.Config
	KubeClient              kubernetes.Interface
	ExtensionsClient        apiextensionsclient.Interface
	AppClient               appclientset.Interface
	DB                      db.ArgoDB
	Namespace               string
	RepoServerAddress       string
	ApiServerAddress        string
	ControllerServerAddress string
	Enforcer                *rbac.Enforcer
	SettingsMgr             *settings.SettingsManager

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

func (f *Fixture) setup() error {
	_, err := exec.Command("kubectl", "apply", "-f", "../../manifests/crds/application-crd.yaml", "-f", "../../manifests/crds/appproject-crd.yaml").Output()
	if err != nil {
		return err
	}
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

	err = f.SettingsMgr.SaveSettings(&settings.ArgoCDSettings{})
	if err != nil {
		return err
	}

	err = f.ensureClusterRegistered()
	if err != nil {
		return err
	}

	repoServerPort, err := test.GetFreePort()
	if err != nil {
		return err
	}

	apiServerPort, err := test.GetFreePort()
	if err != nil {
		return err
	}

	controllerServerPort, err := test.GetFreePort()
	if err != nil {
		return err
	}

	repoSrv, err := reposerver.NewServer(&FakeGitClientFactory{}, cache.NewCache(cache.NewInMemoryCache(1*time.Hour)), func(config *tls.Config) {}, 0)
	if err != nil {
		return err
	}
	repoServerGRPC := repoSrv.CreateGRPC()

	f.RepoServerAddress = fmt.Sprintf("127.0.0.1:%d", repoServerPort)
	f.ApiServerAddress = fmt.Sprintf("127.0.0.1:%d", apiServerPort)
	f.ControllerServerAddress = fmt.Sprintf("127.0.0.1:%d", controllerServerPort)

	ctx, cancel := context.WithCancel(context.Background())
	apiServer := server.NewServer(ctx, server.ArgoCDServerOpts{
		Namespace:     f.Namespace,
		AppClientset:  f.AppClient,
		DisableAuth:   true,
		Insecure:      true,
		KubeClientset: f.KubeClient,
		RepoClientset: reposerver.NewRepositoryServerClientset(f.RepoServerAddress),
		Cache:         cache.NewCache(cache.NewInMemoryCache(1 * time.Hour)),
	})

	go func() {
		apiServer.Run(ctx, apiServerPort)
	}()

	err = waitUntilE(func() (done bool, err error) {
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

	if err != nil {
		cancel()
		return err
	}

	ctrl, err := f.createController()
	if err != nil {
		cancel()
		return err
	}

	ctrlCtx, cancelCtrl := context.WithCancel(context.Background())
	go ctrl.Run(ctrlCtx, 1, 1)

	go func() {
		var listener net.Listener
		listener, err = net.Listen("tcp", f.RepoServerAddress)
		if err == nil {
			err = repoServerGRPC.Serve(listener)
		}
	}()

	f.tearDownCallback = func() {
		cancel()
		cancelCtrl()
		repoServerGRPC.Stop()
	}

	return err
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
	clientset, err := kubernetes.NewForConfig(conf)
	errors.CheckError(err)
	managerBearerToken, err := common.InstallClusterManagerRBAC(clientset)
	errors.CheckError(err)
	clst := commands.NewCluster(f.Config.Host, conf, managerBearerToken, nil)
	clstCreateReq := cluster.ClusterCreateRequest{Cluster: clst}
	_, err = cluster.NewServer(f.DB, f.Enforcer, cache.NewCache(cache.NewInMemoryCache(1*time.Minute))).Create(context.Background(), &clstCreateReq)
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
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, namespace)
	db := db.NewDB(namespace, settingsMgr, kubeClient)
	enforcer := rbac.NewEnforcer(kubeClient, namespace, common.ArgoCDRBACConfigMapName, nil)
	err = enforcer.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	if err != nil {
		return nil, err
	}
	enforcer.SetDefaultRole("role:admin")

	fixture := &Fixture{
		Config:           config,
		ExtensionsClient: extensionsClient,
		AppClient:        appClient,
		DB:               db,
		KubeClient:       kubeClient,
		Namespace:        namespace,
		Enforcer:         enforcer,
		SettingsMgr:      settingsMgr,
	}
	err = fixture.setup()
	if err != nil {
		return nil, err
	}
	return fixture, nil
}

// CreateApp creates application
func (f *Fixture) CreateApp(t *testing.T, application *v1alpha1.Application) *v1alpha1.Application {
	application = application.DeepCopy()
	application.Name = fmt.Sprintf("e2e-test-%v", time.Now().Unix())
	labels := application.ObjectMeta.Labels
	if labels == nil {
		labels = make(map[string]string)
		application.ObjectMeta.Labels = labels
	}

	application.Spec.Source.ComponentParameterOverrides = append(
		application.Spec.Source.ComponentParameterOverrides,
		v1alpha1.ComponentParameter{Name: "name", Value: application.Name, Component: "guestbook-ui"})

	app, err := f.AppClient.ArgoprojV1alpha1().Applications(f.Namespace).Create(application)
	if err != nil {
		t.Fatal(fmt.Sprintf("Unable to create app %v", err))
	}
	return app
}

// createController creates new controller instance
func (f *Fixture) createController() (*controller.ApplicationController, error) {
	return controller.NewApplicationController(
		f.Namespace,
		f.SettingsMgr,
		f.KubeClient,
		f.AppClient,
		reposerver.NewRepositoryServerClientset(f.RepoServerAddress),
		cache.NewCache(cache.NewInMemoryCache(1*time.Hour)),
		10*time.Second)
}

func (f *Fixture) NewApiClientset() (argocdclient.Client, error) {
	return argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		PlainText:  true,
		ServerAddr: f.ApiServerAddress,
	})
}

func (f *Fixture) RunCli(args ...string) (string, error) {
	args = append([]string{"run", "../../cmd/argocd/main.go"}, args...)
	cmd := exec.Command("go", append(args, "--server", f.ApiServerAddress, "--plaintext")...)
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
		return "", fmt.Errorf(strings.TrimSpace(errOutput))
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

type FakeGitClientFactory struct{}

func (f *FakeGitClientFactory) NewClient(repoURL, path, username, password, sshPrivateKey string) (git.Client, error) {
	return &FakeGitClient{
		root: path,
	}, nil
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
	matches, err := filepath.Glob(path.Join(c.root, s))
	if err != nil {
		return nil, err
	}
	for i := range matches {
		matches[i] = strings.TrimPrefix(matches[i], c.root)
	}
	return matches, nil
}

func (c *FakeGitClient) CommitSHA() (string, error) {
	return "abcdef123456890", nil
}
