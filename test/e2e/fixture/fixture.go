package fixture

import (
	"bufio"
	"context"
	goerrors "errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/k3s"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	appcontrollercommand "github.com/argoproj/argo-cd/v2/cmd/argocd-application-controller/commands"
	appsetcontrollercommand "github.com/argoproj/argo-cd/v2/cmd/argocd-applicationset-controller/commands"
	reposervercommand "github.com/argoproj/argo-cd/v2/cmd/argocd-repo-server/commands"
	servercommand "github.com/argoproj/argo-cd/v2/cmd/argocd-server/commands"
	grpcutil "github.com/argoproj/argo-cd/v2/util/grpc"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/argoproj/pkg/errors"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/util/env"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	utilio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rand"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	defaultk3sVersion       = "v1.27.15"
	defaultAdminPassword    = "password"
	defaultAdminUsername    = "admin"
	DefaultTestUserPassword = "password"
	TestingLabel            = "e2e.argoproj.io"
	ArgoCDNamespace         = "argocd-e2e"
	ArgoCDAppNamespace      = "argocd-e2e-external"

	// notifications controller, metrics server port
	defaultNotificationServer = "localhost:9001"

	// ensure all repos are in one directory tree, so we can easily clean them up
	TmpDir             = "/tmp/argo-e2e"
	repoDir            = "testdata.git"
	submoduleDir       = "submodule.git"
	submoduleParentDir = "submoduleParent.git"

	GuestbookPath = "guestbook"

	ProjectName = "argo-project"

	// cmp plugin sock file path
	PluginSockFilePath = "/app/config/plugin"

	E2ETestPrefix = "e2e-test-"
)

const (
	EnvAdminUsername           = "ARGOCD_E2E_ADMIN_USERNAME"
	EnvAdminPassword           = "ARGOCD_E2E_ADMIN_PASSWORD"
	EnvArgoCDServerName        = "ARGOCD_E2E_SERVER_NAME"
	EnvArgoCDRedisHAProxyName  = "ARGOCD_E2E_REDIS_HAPROXY_NAME"
	EnvArgoCDRedisName         = "ARGOCD_E2E_REDIS_NAME"
	EnvArgoCDRepoServerName    = "ARGOCD_E2E_REPO_SERVER_NAME"
	EnvArgoCDAppControllerName = "ARGOCD_E2E_APPLICATION_CONTROLLER_NAME"
	EnvArgoCDApiServerPort     = "ARGOCD_E2E_APISERVER_PORT"
	EnvArgoCDRepoServerPort    = "ARGOCD_E2E_REPOSERVER_PORT"
	EnvArgoCDUseTestContainers = "ARGOCD_E2E_USE_TESTCONTAINERS"
	EnvK3sVersion              = "ARGOCD_E2E_K3S_VERSION"
)

var (
	id                        string
	deploymentNamespace       string
	name                      string
	KubeClientset             kubernetes.Interface
	KubeConfig                *rest.Config
	DynamicClientset          dynamic.Interface
	AppClientset              appclientset.Interface
	ArgoCDClientset           apiclient.Client
	adminUsername             string
	AdminPassword             string
	apiServerAddress          string
	token                     string
	plainText                 bool
	testsRun                  map[string]bool
	argoCDServerName          string
	argoCDRedisHAProxyName    string
	argoCDRedisName           string
	argoCDRepoServerName      string
	argoCDAppControllerName   string
	k3sVersion                string
	apiServerPort             string
	repoServerPort            string
	mappedSSHPort             string // port 2222
	mappedGitNoAuthPort       string // port 9081
	MappedHttpsAuthPort       string // port 9443
	mappedHttpsClientAuthPort string // port 9444
	MappedOCIRegistryPort     string // port 5000
	mappedHelmHttpPort        string // port 9080
)

type RepoURLType string

type ACL struct {
	Resource string
	Action   string
	Scope    string
}

const (
	RepoURLTypeFile                 = "file"
	RepoURLTypeHTTPS                = "https"
	RepoURLTypeHTTPSOrg             = "https-org"
	RepoURLTypeHTTPSClientCert      = "https-cc"
	RepoURLTypeHTTPSSubmodule       = "https-sub"
	RepoURLTypeHTTPSSubmoduleParent = "https-par"
	RepoURLTypeSSH                  = "ssh"
	RepoURLTypeSSHSubmodule         = "ssh-sub"
	RepoURLTypeSSHSubmoduleParent   = "ssh-par"
	RepoURLTypeHelm                 = "helm"
	RepoURLTypeHelmParent           = "helm-par"
	RepoURLTypeHelmOCI              = "helm-oci"
	GitUsername                     = "admin"
	GitPassword                     = "password"
	GithubAppID                     = "2978632978"
	GithubAppInstallationID         = "7893789433789"
	GpgGoodKeyID                    = "D56C4FCA57A46444"
)

// TestNamespace returns the namespace where Argo CD E2E test instance will be
// running in.
func TestNamespace() string {
	return GetEnvWithDefault("ARGOCD_E2E_NAMESPACE", ArgoCDNamespace)
}

func AppNamespace() string {
	return GetEnvWithDefault("ARGOCD_E2E_APP_NAMESPACE", ArgoCDAppNamespace)
}

// getKubeConfig creates new kubernetes client config using specified config path and config overrides variables
func getKubeConfig(configPath string, overrides clientcmd.ConfigOverrides) *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	restConfig, err := clientConfig.ClientConfig()
	CheckError(err)
	return restConfig
}

func GetEnvWithDefault(envName, defaultValue string) string {
	r := os.Getenv(envName)
	if r == "" {
		return defaultValue
	}
	return r
}

func SetEnvWithDefaultIfNotSet(envName, value string) string {
	r := os.Getenv(envName)
	if r == "" {
		_ = os.Setenv(envName, value)
		return value
	}
	return r
}

// IsRunningTestContainers returns true when the tests are being run against a workload that
// is running in a testcontainer.
func IsRunningTestContainers() bool {
	return env.ParseBoolFromEnv(EnvArgoCDUseTestContainers, true)
}

// IsRemote returns true when the tests are being run against a workload that
// is running in a remote cluster.
func IsRemote() bool {
	return env.ParseBoolFromEnv("ARGOCD_E2E_REMOTE", false)
}

// IsLocal returns when the tests are being run against a local workload
func IsLocal() bool {
	return !IsRemote()
}

// creates e2e tests fixture: ensures that Application CRD is installed, creates temporal namespace, starts repo and api server,
// configure currently available cluster.
func init() {
	// ensure we log all shell execs
	log.SetLevel(log.DebugLevel)

	apiServerPort = GetEnvWithDefault(EnvArgoCDApiServerPort, randomPort())
	repoServerPort = GetEnvWithDefault(EnvArgoCDRepoServerPort, randomPort())
	apiServerAddress = GetEnvWithDefault(apiclient.EnvArgoCDServer, fmt.Sprintf("localhost:%s", apiServerPort))
	apiServerPort = strings.Split(apiServerAddress, ":")[1]

	adminUsername = GetEnvWithDefault(EnvAdminUsername, defaultAdminUsername)
	AdminPassword = GetEnvWithDefault(EnvAdminPassword, defaultAdminPassword)

	argoCDServerName = GetEnvWithDefault(EnvArgoCDServerName, common.DefaultServerName)
	argoCDRedisHAProxyName = GetEnvWithDefault(EnvArgoCDRedisHAProxyName, common.DefaultRedisHaProxyName)
	argoCDRedisName = GetEnvWithDefault(EnvArgoCDRedisName, common.DefaultRedisName)
	argoCDRepoServerName = GetEnvWithDefault(EnvArgoCDRepoServerName, common.DefaultRepoServerName)
	argoCDAppControllerName = GetEnvWithDefault(EnvArgoCDAppControllerName, common.DefaultApplicationControllerName)
	k3sVersion = GetEnvWithDefault(EnvK3sVersion, defaultk3sVersion)

	if IsRunningTestContainers() {
		ctx := context.Background()

		// Recreate temp dir
		CheckError(os.RemoveAll(TmpDir))
		FailOnErr(Run("", "mkdir", "-p", TmpDir))

		setupGPG()
		// setup k3s, redis and argocd-e2e-cluster as testcontainers
		initTestContainers(ctx)
		// configure app-controller, repo-server, appset-controller etc in-process
		initTestServices(ctx)
	} else {
		// set-up variables
		config := getKubeConfig("", clientcmd.ConfigOverrides{})
		AppClientset = appclientset.NewForConfigOrDie(config)
		KubeClientset = kubernetes.NewForConfigOrDie(config)
		DynamicClientset = dynamic.NewForConfigOrDie(config)
		KubeConfig = config
	}

	dialTime := 30 * time.Second
	tlsTestResult, err := grpcutil.TestTLS(apiServerAddress, dialTime)
	CheckError(err)

	ArgoCDClientset, err = apiclient.NewClient(&apiclient.ClientOptions{
		Insecure:          true,
		ServerAddr:        apiServerAddress,
		PlainText:         !tlsTestResult.TLS,
		ServerName:        argoCDServerName,
		RedisHaProxyName:  argoCDRedisHAProxyName,
		RedisName:         argoCDRedisName,
		RepoServerName:    argoCDRepoServerName,
		AppControllerName: argoCDAppControllerName,
	})
	CheckError(err)

	plainText = !tlsTestResult.TLS

	LoginAs(adminUsername)

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")

	// Preload a list of tests that should be skipped
	testsRun = make(map[string]bool)
	rf := os.Getenv("ARGOCD_E2E_RECORD")
	if rf == "" {
		return
	}
	f, err := os.Open(rf)
	if err != nil {
		if goerrors.Is(err, os.ErrNotExist) {
			return
		} else {
			panic(fmt.Sprintf("Could not read record file %s: %v", rf, err))
		}
	}
	defer func() {
		err := f.Close()
		if err != nil {
			panic(fmt.Sprintf("Could not close record file %s: %v", rf, err))
		}
	}()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		testsRun[scanner.Text()] = true
	}
}

func randomPort() string {
	l, err := net.Listen("tcp", ":0")
	defer func() {
		CheckError(l.Close())
	}()
	CheckError(err)
	port := l.Addr().(*net.TCPAddr).Port
	return strconv.FormatInt(int64(port), 10)
}

func loginAs(username, password string) {
	closer, client, err := ArgoCDClientset.NewSessionClient()
	CheckError(err)
	defer utilio.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &sessionpkg.SessionCreateRequest{Username: username, Password: password})
	CheckError(err)
	token = sessionResponse.Token

	ArgoCDClientset, err = apiclient.NewClient(&apiclient.ClientOptions{
		Insecure:          true,
		ServerAddr:        apiServerAddress,
		AuthToken:         token,
		PlainText:         plainText,
		ServerName:        argoCDServerName,
		RedisHaProxyName:  argoCDRedisHAProxyName,
		RedisName:         argoCDRedisName,
		RepoServerName:    argoCDRepoServerName,
		AppControllerName: argoCDAppControllerName,
	})
	CheckError(err)
}

func LoginAs(username string) {
	password := DefaultTestUserPassword
	if username == "admin" {
		password = AdminPassword
	}
	loginAs(username, password)
}

func Name() string {
	return name
}

func RepoDirectory() string {
	return path.Join(TmpDir, repoDir)
}

func submoduleDirectory() string {
	return path.Join(TmpDir, submoduleDir)
}

func submoduleParentDirectory() string {
	return path.Join(TmpDir, submoduleParentDir)
}

const (
	EnvRepoURLTypeSSH                  = "ARGOCD_E2E_REPO_SSH"
	EnvRepoURLTypeSSHSubmodule         = "ARGOCD_E2E_REPO_SSH_SUBMODULE"
	EnvRepoURLTypeSSHSubmoduleParent   = "ARGOCD_E2E_REPO_SSH_SUBMODULE_PARENT"
	EnvRepoURLTypeHTTPS                = "ARGOCD_E2E_REPO_HTTPS"
	EnvRepoURLTypeHTTPSOrg             = "ARGOCD_E2E_REPO_HTTPS_ORG"
	EnvRepoURLTypeHTTPSClientCert      = "ARGOCD_E2E_REPO_HTTPS_CLIENT_CERT"
	EnvRepoURLTypeHTTPSSubmodule       = "ARGOCD_E2E_REPO_HTTPS_SUBMODULE"
	EnvRepoURLTypeHTTPSSubmoduleParent = "ARGOCD_E2E_REPO_HTTPS_SUBMODULE_PARENT"
	EnvRepoURLTypeHelm                 = "ARGOCD_E2E_REPO_HELM"
	EnvRepoURLDefault                  = "ARGOCD_E2E_REPO_DEFAULT"
)

func RepoURL(urlType RepoURLType) string {
	switch urlType {
	// Git server via SSH
	case RepoURLTypeSSH:
		return GetEnvWithDefault(EnvRepoURLTypeSSH, fmt.Sprintf("ssh://user@localhost:%s/tmp/argo-e2e/testdata.git", mappedSSHPort))
	// Git submodule repo
	case RepoURLTypeSSHSubmodule:
		return GetEnvWithDefault(EnvRepoURLTypeSSHSubmodule, fmt.Sprintf("ssh://user@localhost:%s/tmp/argo-e2e/submodule.git", mappedSSHPort))
	// Git submodule parent repo
	case RepoURLTypeSSHSubmoduleParent:
		return GetEnvWithDefault(EnvRepoURLTypeSSHSubmoduleParent, fmt.Sprintf("ssh://user@localhost:%s/tmp/argo-e2e/submoduleParent.git", mappedSSHPort))
	// Git server via HTTPS
	case RepoURLTypeHTTPS:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPS, fmt.Sprintf("https://localhost:%s/argo-e2e/testdata.git", MappedHttpsAuthPort))
	// Git "organisation" via HTTPS
	case RepoURLTypeHTTPSOrg:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSOrg, fmt.Sprintf("https://localhost:%s/argo-e2e", MappedHttpsAuthPort))
	// Git server via HTTPS - Client Cert protected
	case RepoURLTypeHTTPSClientCert:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSClientCert, fmt.Sprintf("https://localhost:%s/argo-e2e/testdata.git", mappedHttpsClientAuthPort))
	case RepoURLTypeHTTPSSubmodule:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSSubmodule, fmt.Sprintf("https://localhost:%s/argo-e2e/submodule.git", MappedHttpsAuthPort))
		// Git submodule parent repo
	case RepoURLTypeHTTPSSubmoduleParent:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSSubmoduleParent, fmt.Sprintf("https://localhost:%s/argo-e2e/submoduleParent.git", MappedHttpsAuthPort))
	// Default - file based Git repository
	case RepoURLTypeHelm:
		return GetEnvWithDefault(EnvRepoURLTypeHelm, fmt.Sprintf("https://localhost:%s/argo-e2e/testdata.git/helm-repo/local", mappedHttpsClientAuthPort))
	// When Helm Repo has sub repos, this is the parent repo URL
	case RepoURLTypeHelmParent:
		return GetEnvWithDefault(EnvRepoURLTypeHelm, fmt.Sprintf("https://localhost:%s/argo-e2e/testdata.git/helm-repo", mappedHttpsClientAuthPort))
	case RepoURLTypeHelmOCI:
		return fmt.Sprintf("localhost:%s/myrepo", MappedOCIRegistryPort)
	default:
		return GetEnvWithDefault(EnvRepoURLDefault, fmt.Sprintf("file://%s", RepoDirectory()))
	}
}

func RepoBaseURL(urlType RepoURLType) string {
	return path.Base(RepoURL(urlType))
}

func DeploymentNamespace() string {
	return deploymentNamespace
}

// creates a secret for the current test, this currently can only create a single secret
func CreateSecret(username, password string) string {
	secretName := fmt.Sprintf("argocd-e2e-%s", name)
	FailOnErr(Run("", "kubectl", "create", "secret", "generic", secretName,
		"--from-literal=username="+username,
		"--from-literal=password="+password,
		"-n", TestNamespace()))
	FailOnErr(Run("", "kubectl", "label", "secret", secretName, TestingLabel+"=true", "-n", TestNamespace()))
	return secretName
}

// Convenience wrapper for updating argocd-cm
func updateSettingConfigMap(updater func(cm *corev1.ConfigMap) error) {
	updateGenericConfigMap(common.ArgoCDConfigMapName, updater)
}

// Convenience wrapper for updating argocd-notifications-cm
func updateNotificationsConfigMap(updater func(cm *corev1.ConfigMap) error) {
	updateGenericConfigMap(common.ArgoCDNotificationsConfigMapName, updater)
}

// Convenience wrapper for updating argocd-cm-rbac
func updateRBACConfigMap(updater func(cm *corev1.ConfigMap) error) {
	updateGenericConfigMap(common.ArgoCDRBACConfigMapName, updater)
}

// Updates a given config map in argocd-e2e namespace
func updateGenericConfigMap(name string, updater func(cm *corev1.ConfigMap) error) {
	cm, err := KubeClientset.CoreV1().ConfigMaps(TestNamespace()).Get(context.Background(), name, v1.GetOptions{})
	errors.CheckError(err)
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	errors.CheckError(updater(cm))
	_, err = KubeClientset.CoreV1().ConfigMaps(TestNamespace()).Update(context.Background(), cm, v1.UpdateOptions{})
	errors.CheckError(err)
}

func SetEnableManifestGeneration(val map[v1alpha1.ApplicationSourceType]bool) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		for k, v := range val {
			cm.Data[fmt.Sprintf("%s.enable", strings.ToLower(string(k)))] = strconv.FormatBool(v)
		}
		return nil
	})
}

func SetResourceOverrides(overrides map[string]v1alpha1.ResourceOverride) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		if len(overrides) > 0 {
			yamlBytes, err := yaml.Marshal(overrides)
			if err != nil {
				return err
			}
			cm.Data["resource.customizations"] = string(yamlBytes)
		} else {
			delete(cm.Data, "resource.customizations")
		}
		return nil
	})

	SetResourceOverridesSplitKeys(overrides)
}

func SetInstallationID(installationID string) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["installationID"] = installationID
		return nil
	})
}

func SetTrackingMethod(trackingMethod string) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["application.resourceTrackingMethod"] = trackingMethod
		return nil
	})
}

func SetTrackingLabel(trackingLabel string) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["application.instanceLabelKey"] = trackingLabel
		return nil
	})
}

func SetResourceOverridesSplitKeys(overrides map[string]v1alpha1.ResourceOverride) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		for k, v := range overrides {
			if v.HealthLua != "" {
				cm.Data[getResourceOverrideSplitKey(k, "health")] = v.HealthLua
			}
			cm.Data[getResourceOverrideSplitKey(k, "useOpenLibs")] = strconv.FormatBool(v.UseOpenLibs)
			if v.Actions != "" {
				cm.Data[getResourceOverrideSplitKey(k, "actions")] = v.Actions
			}
			if len(v.IgnoreDifferences.JSONPointers) > 0 ||
				len(v.IgnoreDifferences.JQPathExpressions) > 0 ||
				len(v.IgnoreDifferences.ManagedFieldsManagers) > 0 {
				yamlBytes, err := yaml.Marshal(v.IgnoreDifferences)
				if err != nil {
					return err
				}
				cm.Data[getResourceOverrideSplitKey(k, "ignoreDifferences")] = string(yamlBytes)
			}
			if len(v.KnownTypeFields) > 0 {
				yamlBytes, err := yaml.Marshal(v.KnownTypeFields)
				if err != nil {
					return err
				}
				cm.Data[getResourceOverrideSplitKey(k, "knownTypeFields")] = string(yamlBytes)
			}
		}
		return nil
	})
}

func getResourceOverrideSplitKey(key string, customizeType string) string {
	groupKind := key
	parts := strings.Split(key, "/")
	if len(parts) == 2 {
		groupKind = fmt.Sprintf("%s_%s", parts[0], parts[1])
	}
	return fmt.Sprintf("resource.customizations.%s.%s", customizeType, groupKind)
}

func SetAccounts(accounts map[string][]string) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		for k, v := range accounts {
			cm.Data[fmt.Sprintf("accounts.%s", k)] = strings.Join(v, ",")
		}
		return nil
	})
}

func SetPermissions(permissions []ACL, username string, roleName string) {
	updateRBACConfigMap(func(cm *corev1.ConfigMap) error {
		var aclstr string

		for _, permission := range permissions {
			aclstr += fmt.Sprintf("p, role:%s, %s, %s, %s, allow \n", roleName, permission.Resource, permission.Action, permission.Scope)
		}

		aclstr += fmt.Sprintf("g, %s, role:%s", username, roleName)
		cm.Data["policy.csv"] = aclstr

		return nil
	})
}

func SetResourceFilter(filters settings.ResourcesFilter) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		exclusions, err := yaml.Marshal(filters.ResourceExclusions)
		if err != nil {
			return err
		}
		inclusions, err := yaml.Marshal(filters.ResourceInclusions)
		if err != nil {
			return err
		}
		cm.Data["resource.exclusions"] = string(exclusions)
		cm.Data["resource.inclusions"] = string(inclusions)
		return nil
	})
}

func SetHelmRepos(repos ...settings.HelmRepoCredentials) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		yamlBytes, err := yaml.Marshal(repos)
		if err != nil {
			return err
		}
		cm.Data["helm.repositories"] = string(yamlBytes)
		return nil
	})
}

func SetRepos(repos ...settings.RepositoryCredentials) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		yamlBytes, err := yaml.Marshal(repos)
		if err != nil {
			return err
		}
		cm.Data["repositories"] = string(yamlBytes)
		return nil
	})
}

func SetProjectSpec(project string, spec v1alpha1.AppProjectSpec) {
	proj, err := AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).Get(context.Background(), project, v1.GetOptions{})
	errors.CheckError(err)
	proj.Spec = spec
	_, err = AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).Update(context.Background(), proj, v1.UpdateOptions{})
	errors.CheckError(err)
}

func SetParamInSettingConfigMap(key, value string) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data[key] = value
		return nil
	})
}

func SetParamInNotificationsConfigMap(key, value string) {
	updateNotificationsConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data[key] = value
		return nil
	})
}

type TestOption func(option *testOption)

type testOption struct {
	testdata string
}

func newTestOption(opts ...TestOption) *testOption {
	to := &testOption{
		testdata: "testdata",
	}
	for _, opt := range opts {
		opt(to)
	}
	return to
}

func WithTestData(testdata string) TestOption {
	return func(option *testOption) {
		option.testdata = testdata
	}
}

type testLogConsumer struct{}

func (g *testLogConsumer) Accept(l testcontainers.Log) {
	log.Info(string(l.Content))
}

func initTestContainers(ctx context.Context) {
	currentWorkingDirectory, err := os.Getwd()
	CheckError(err)
	base := filepath.Join(currentWorkingDirectory, "../../")
	l := testLogConsumer{}

	// Get the current user
	currentUser, err := user.Current()
	CheckError(err)

	e2eServer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			User: fmt.Sprintf("%s:%s", currentUser.Uid, currentUser.Gid),
			FromDockerfile: testcontainers.FromDockerfile{
				Context:       base,
				PrintBuildLog: true,
				Dockerfile:    "test/remote/Dockerfile",
				BuildArgs: map[string]*string{
					"UID": &currentUser.Uid,
				},
			},
			ExposedPorts: []string{"2222/tcp", "9080/tcp", "9081/tcp", "9443/tcp", "9444/tcp"},
			Cmd:          []string{"goreman", "start"},
			LogConsumerCfg: &testcontainers.LogConsumerConfig{
				Opts:      []testcontainers.LogProductionOption{testcontainers.WithLogProductionTimeout(10 * time.Second)},
				Consumers: []testcontainers.LogConsumer{&l},
			},
		},
		Started: true,
	})
	CheckError(err)

	ociRegistry, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			ExposedPorts: []string{"5000/tcp"},
			Image:        "docker.io/registry:2",
		},
		Started: true,
	})
	CheckError(err)

	port, _ := e2eServer.MappedPort(ctx, "2222")
	mappedSSHPort = port.Port()

	port, _ = e2eServer.MappedPort(ctx, "9080")
	mappedHelmHttpPort = port.Port()

	port, _ = e2eServer.MappedPort(ctx, "9081")
	mappedGitNoAuthPort = port.Port()

	port, _ = e2eServer.MappedPort(ctx, "9443")
	MappedHttpsAuthPort = port.Port()

	port, _ = e2eServer.MappedPort(ctx, "9444")
	mappedHttpsClientAuthPort = port.Port()

	port, _ = ociRegistry.MappedPort(ctx, "5000")
	MappedOCIRegistryPort = port.Port()

	// TODO: Figure nicer way to template helm index files with the forwarded port
	c, reader, err := e2eServer.Exec(ctx, []string{
		"sudo", "sed", "-i",
		fmt.Sprintf("s/9080/%s/g", mappedHelmHttpPort),
		"/tmp/argo-e2e/testdata.git/helm-repo/local/index.yaml",
		"/tmp/argo-e2e/testdata.git/helm-repo/local2/index.yaml",
		"/tmp/argo-e2e/testdata.git/helm-repo/remote/index.yaml",
	})
	CheckError(err)

	if c != 0 {
		buf := new(strings.Builder)
		_, _ = io.Copy(buf, reader)
		panic(buf.String())
	}

	k3sContainer, err := k3s.RunContainer(ctx,
		testcontainers.WithImage(fmt.Sprintf("rancher/k3s:%s-k3s1", k3sVersion)),
		testcontainers.WithWaitStrategy(wait.ForExec([]string{"kubectl", "wait", "--timeout=60s", "apiservice", "v1beta1.metrics.k8s.io", "--for", "condition=Available=True"})),
	)
	CheckError(err)

	kubeConfigYaml, err := k3sContainer.GetKubeConfig(ctx)
	CheckError(err)

	config, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigYaml)
	CheckError(err)

	// set-up variables
	AppClientset = appclientset.NewForConfigOrDie(config)
	KubeClientset = kubernetes.NewForConfigOrDie(config)
	DynamicClientset = dynamic.NewForConfigOrDie(config)
	KubeConfig = config

	redisContainer, err := redis.RunContainer(ctx, testcontainers.WithImage("redis:6"))
	CheckError(err)

	endpoint, err := redisContainer.Endpoint(ctx, "")
	CheckError(err)

	temp, err := os.MkdirTemp("", "kubeconfig")
	CheckError(err)
	kubeConfigPath := temp + "/kubeconfig.yaml"

	CheckError(os.WriteFile(kubeConfigPath, kubeConfigYaml, 0o600))
	CheckError(os.Setenv("REDIS_SERVER", endpoint))
	CheckError(os.Setenv("KUBECONFIG", kubeConfigPath))
	CheckError(os.Setenv("ARGOCD_FAKE_IN_CLUSTER", "true"))
	CheckError(os.Setenv("ARGOCD_E2E_K3S", "true"))
	SetEnvWithDefaultIfNotSet("ARGOCD_E2E_GIT_SERVICE_SUBMODULE", fmt.Sprintf("http://127.0.0.1:%s/argo-e2e/submodule.git", mappedGitNoAuthPort))
	SetEnvWithDefaultIfNotSet("ARGOCD_E2E_GIT_SERVICE_SUBMODULE_PARENT", fmt.Sprintf("http://127.0.0.1:%s/argo-e2e/submoduleParent.git", mappedGitNoAuthPort))
	SetEnvWithDefaultIfNotSet("ARGOCD_E2E_GIT_SERVICE", fmt.Sprintf("http://127.0.0.1:%s/argo-e2e/testdata.git", mappedGitNoAuthPort))
	SetEnvWithDefaultIfNotSet("ARGOCD_PLUGINCONFIGFILEPATH", "/tmp/argo-e2e/app/config/plugin")
	SetEnvWithDefaultIfNotSet("ARGOCD_PLUGINSOCKFILEPATH", "/tmp/argo-e2e/app/config/plugin")
	SetEnvWithDefaultIfNotSet("ARGOCD_GPG_DATA_PATH", "/tmp/argo-e2e/app/config/gpg/source")
	SetEnvWithDefaultIfNotSet("ARGOCD_TLS_DATA_PATH", "/tmp/argo-e2e/app/config/tls")
	SetEnvWithDefaultIfNotSet("ARGOCD_SSH_DATA_PATH", "/tmp/argo-e2e/app/config/ssh")
	SetEnvWithDefaultIfNotSet("ARGOCD_GNUPGHOME", "/tmp/argo-e2e/app/config/gpg/keys")
	SetEnvWithDefaultIfNotSet("ARGOCD_APPLICATION_NAMESPACES", "argocd-e2e-external,argocd-e2e-external-2")
	SetEnvWithDefaultIfNotSet("ARGOCD_APPLICATIONSET_CONTROLLER_NAMESPACES", "argocd-e2e-external,argocd-e2e-external-2")
	SetEnvWithDefaultIfNotSet("ARGOCD_GPG_WRAPPER_PATH", currentWorkingDirectory+"/../../hack/")
	SetEnvWithDefaultIfNotSet("ARGOCD_APPLICATIONSET_CONTROLLER_ALLOWED_SCM_PROVIDERS", "http://127.0.0.1:8341,http://127.0.0.1:8342,http://127.0.0.1:8343,http://127.0.0.1:8344")

	namespaces := []string{TestNamespace(), AppNamespace(), "argocd-e2e-external-2"}
	for _, s := range namespaces {
		namespace := &corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: s,
			},
		}

		_, err = KubeClientset.CoreV1().Namespaces().Create(context.TODO(), namespace, v1.CreateOptions{})
		CheckError(err)
	}

	FailOnErr(Run("", "kubectl", "config", "set-context", "--current", fmt.Sprintf("--namespace=%s", TestNamespace())))
	FailOnErr(Run("", "kubectl", "apply", "-f", "https://raw.githubusercontent.com/open-cluster-management/api/a6845f2ebcb186ec26b832f60c988537a58f3859/cluster/v1alpha1/0000_04_clusters.open-cluster-management.io_placementdecisions.crd.yaml"))
	FailOnErr(Run("", "kubectl", "apply", "-k", "../manifests/base"))

	// TODO: For now this logging seems a bit verbose, this should be tweakable
	// cli.SetLogLevel("debug")
	// cli.SetGLogLevel(0)
}

func initTestServices(ctx context.Context) {
	// Start repo-server
	repoServerCmd := cobra.Command{}
	reposerverConfig := reposervercommand.NewRepoServerConfig(&repoServerCmd).WithDefaultFlags()
	CheckError(repoServerCmd.Flags().Set("port", repoServerPort))

	go func() {
		if err := reposerverConfig.CreateRepoServer(ctx); err != nil {
			log.Error(err, "problem starting repo-server")
			os.Exit(1)
		}
	}()

	repoServerAddress := fmt.Sprintf("localhost:%s", repoServerPort)

	// Start argocd-server
	serverCmd := cobra.Command{}
	serverRestConfig := rest.CopyConfig(KubeConfig)
	// serverRestConfig.Impersonate.UserName = fmt.Sprintf("system:serviceaccount:%s:argocd-server", TestNamespace()) // TODO: Run controller with serviceaccount perms
	serverConfig := servercommand.NewServerConfig(&serverCmd).WithDefaultFlags().WithK8sSettings(TestNamespace(), serverRestConfig)
	CheckError(serverCmd.Flags().Set("insecure", "true"))
	CheckError(serverCmd.Flags().Set("port", apiServerPort))
	CheckError(serverCmd.Flags().Set("repo-server", repoServerAddress))
	serverConfig.CreateServer(ctx)

	// Start application-controller
	appRestConfig := rest.CopyConfig(KubeConfig)
	// appRestConfig.Impersonate.UserName = fmt.Sprintf("system:serviceaccount:%s:argocd-application-controller", TestNamespace()) // TODO: Run controller with serviceaccount perms
	appControllerCmd := &cobra.Command{}
	appControllerConfig := appcontrollercommand.NewApplicationControllerConfig(appControllerCmd).WithDefaultFlags().WithK8sSettings(TestNamespace(), appRestConfig)
	CheckError(appControllerCmd.Flags().Set("repo-server", repoServerAddress))
	CheckError(appControllerConfig.CreateApplicationController(ctx))

	// Start applicationset-controller
	appsetCmd := cobra.Command{}
	appsetRestConfig := rest.CopyConfig(KubeConfig)
	// appsetRestConfig.Impersonate.UserName = fmt.Sprintf("system:serviceaccount:%s:argocd-applicationset-controller", TestNamespace()) // TODO: Run controller with serviceaccount perms
	appSetControllerConfig := appsetcontrollercommand.NewApplicationSetControllerConfig(&appsetCmd).WithDefaultFlags().WithK8sSettings(TestNamespace(), appsetRestConfig)
	CheckError(appsetCmd.Flags().Set("probe-addr", fmt.Sprintf(":%s", randomPort())))
	CheckError(appsetCmd.Flags().Set("metrics-addr", fmt.Sprintf(":%s", randomPort())))
	CheckError(appsetCmd.Flags().Set("webhook-addr", fmt.Sprintf(":%s", randomPort())))
	CheckError(appsetCmd.Flags().Set("argocd-repo-server", repoServerAddress))
	mgr, err := appSetControllerConfig.CreateApplicationSetController(ctx)
	CheckError(err)
	go func() {
		log.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()
}

func replaceInFile(path string, replacements map[string]string) error {
	input, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	output := string(input)
	for k, v := range replacements {
		output = strings.ReplaceAll(output, k, v)
	}

	return os.WriteFile(path, []byte(output), 0o666)
}

func EnsureCleanState(t *testing.T, opts ...TestOption) {
	t.Helper()
	opt := newTestOption(opts...)
	// In large scenarios, we can skip tests that already run
	SkipIfAlreadyRun(t)
	// Register this test after it has been run & was successful
	t.Cleanup(func() {
		RecordTestRun(t)
	})

	start := time.Now()

	policy := v1.DeletePropagationBackground
	// delete resources
	// kubectl delete apps --all
	CheckError(AppClientset.ArgoprojV1alpha1().Applications(TestNamespace()).DeleteCollection(context.Background(), v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{}))
	CheckError(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).DeleteCollection(context.Background(), v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{}))
	// kubectl delete appprojects --field-selector metadata.name!=default
	CheckError(AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{FieldSelector: "metadata.name!=default"}))
	// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-config
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepository}))
	// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-creds
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepoCreds}))
	// kubectl delete secrets -l argocd.argoproj.io/secret-type=cluster
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster}))
	// kubectl delete secrets -l e2e.argoproj.io=true
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: TestingLabel + "=true"}))

	FailOnErr(Run("", "kubectl", "delete", "ns", "-l", TestingLabel+"=true", "--field-selector", "status.phase=Active", "--wait=false"))
	FailOnErr(Run("", "kubectl", "delete", "crd", "-l", TestingLabel+"=true", "--wait=false"))
	FailOnErr(Run("", "kubectl", "delete", "clusterroles", "-l", TestingLabel+"=true", "--wait=false"))

	// reset settings
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	})

	updateNotificationsConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	})

	// reset rbac
	updateRBACConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	})

	// We can switch user and as result in previous state we will have non-admin user, this case should be reset
	LoginAs(adminUsername)

	// reset gpg-keys config map
	updateGenericConfigMap(common.ArgoCDGPGKeysConfigMapName, func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	})

	SetProjectSpec("default", v1alpha1.AppProjectSpec{
		OrphanedResources:        nil,
		SourceRepos:              []string{"*"},
		Destinations:             []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}},
		ClusterResourceWhitelist: []v1.GroupKind{{Group: "*", Kind: "*"}},
		SourceNamespaces:         []string{AppNamespace()},
	})

	// Create separate project for testing gpg signature verification
	FailOnErr(RunCli("proj", "create", "gpg"))
	SetProjectSpec("gpg", v1alpha1.AppProjectSpec{
		OrphanedResources:        nil,
		SourceRepos:              []string{"*"},
		Destinations:             []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}},
		ClusterResourceWhitelist: []v1.GroupKind{{Group: "*", Kind: "*"}},
		SignatureKeys:            []v1alpha1.SignatureKey{{KeyID: GpgGoodKeyID}},
		SourceNamespaces:         []string{AppNamespace()},
	})

	// Recreate temp dir
	CheckError(os.RemoveAll(TmpDir))
	FailOnErr(Run("", "mkdir", "-p", TmpDir))

	// random id - unique across test runs
	randString, err := rand.String(5)
	CheckError(err)
	postFix := "-" + strings.ToLower(randString)
	id = t.Name() + postFix
	name = DnsFriendly(t.Name(), "")
	deploymentNamespace = DnsFriendly(fmt.Sprintf("argocd-e2e-%s", t.Name()), postFix)

	// create TLS and SSH certificate directories
	if IsLocal() {
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/tls"))
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/ssh"))
	}

	// For signing during the tests
	setupGPG()

	// set-up tmp repo, must have unique name
	FailOnErr(Run("", "cp", "-Rf", opt.testdata, RepoDirectory()))
	FailOnErr(Run(RepoDirectory(), "chmod", "777", "."))
	FailOnErr(Run(RepoDirectory(), "git", "init", "-b", "master"))

	FailOnErr(Run(".", "cp", "../fixture/testrepos/ssh_known_hosts", TmpDir+"/ssh_known_hosts"))

	if IsRunningTestContainers() {
		CheckError(replaceInFile(TmpDir+"/ssh_known_hosts", map[string]string{
			"2222": mappedSSHPort,
		}))

		CheckError(filepath.Walk(RepoDirectory(), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".yaml") {
				CheckError(replaceInFile(path, map[string]string{
					"9080": mappedHelmHttpPort,
					"2222": mappedSSHPort,
					"5000": MappedOCIRegistryPort,
					"9443": MappedHttpsAuthPort,
					"9444": mappedHttpsClientAuthPort,
				}))
			}
			return nil
		}))
	}

	FailOnErr(Run(RepoDirectory(), "git", "add", "."))
	FailOnErr(Run(RepoDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE")))
		FailOnErr(Run(RepoDirectory(), "git", "push", "origin", "master", "-f"))
	}

	// create namespace
	FailOnErr(Run("", "kubectl", "create", "ns", DeploymentNamespace()))
	FailOnErr(Run("", "kubectl", "label", "ns", DeploymentNamespace(), TestingLabel+"=true"))

	// delete old namespaces used by E2E tests
	namespaces, err := KubeClientset.CoreV1().Namespaces().List(context.Background(), v1.ListOptions{})
	CheckError(err)
	for _, namespace := range namespaces.Items {
		if strings.HasPrefix(namespace.Name, E2ETestPrefix) {
			FailOnErr(Run("", "kubectl", "delete", "ns", namespace.Name))
		}
	}

	// delete old ClusterRoles that begin with "e2e-test-" prefix (E2ETestPrefix), which were created by tests
	clusterRoles, err := KubeClientset.RbacV1().ClusterRoles().List(context.Background(), v1.ListOptions{})
	CheckError(err)
	for _, clusterRole := range clusterRoles.Items {
		if strings.HasPrefix(clusterRole.Name, E2ETestPrefix) {
			FailOnErr(Run("", "kubectl", "delete", "clusterrole", clusterRole.Name))
		}
	}

	// delete old ClusterRoleBindings that begin with "e2e-test-prefix", which were created by E2E tests
	clusterRoleBindings, err := KubeClientset.RbacV1().ClusterRoleBindings().List(context.Background(), v1.ListOptions{})
	CheckError(err)
	for _, clusterRoleBinding := range clusterRoleBindings.Items {
		if strings.HasPrefix(clusterRoleBinding.Name, E2ETestPrefix) {
			FailOnErr(Run("", "kubectl", "delete", "clusterrolebinding", clusterRoleBinding.Name))
		}
	}

	LoginAs(adminUsername)

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")

	log.WithFields(log.Fields{"duration": time.Since(start), "name": t.Name(), "id": id, "username": "admin", "password": "password"}).Info("clean state")
}

func setupGPG() {
	FailOnErr(Run("", "mkdir", "-p", TmpDir+"/gpg"))
	FailOnErr(Run("", "chmod", "0700", TmpDir+"/gpg"))
	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	// nolint:errcheck
	Run("", "pkill", "-9", "gpg-agent")
	FailOnErr(Run("", "gpg", "--import", "../fixture/gpg/signingkey.asc"))
	os.Setenv("GNUPGHOME", prevGnuPGHome)

	// recreate GPG directories
	if IsLocal() {
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/source"))
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/keys"))
		FailOnErr(Run("", "chmod", "0700", TmpDir+"/app/config/gpg/keys"))
		FailOnErr(Run("", "mkdir", "-p", TmpDir+PluginSockFilePath))
		FailOnErr(Run("", "chmod", "0700", TmpDir+PluginSockFilePath))
	}
}

func RunCliWithRetry(maxRetries int, args ...string) (string, error) {
	var out string
	var err error
	for i := 0; i < maxRetries; i++ {
		out, err = RunCli(args...)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	return out, err
}

func RunCli(args ...string) (string, error) {
	return RunCliWithStdin("", false, args...)
}

func RunCliWithStdin(stdin string, isKubeConextOnlyCli bool, args ...string) (string, error) {
	if plainText {
		args = append(args, "--plaintext")
	}

	// For commands executed with Kubernetes context server argument causes a conflict (for those commands server argument is for KubeAPI server), also authentication is not required
	if !isKubeConextOnlyCli {
		args = append(args, "--server", apiServerAddress, "--auth-token", token)
	}

	args = append(args, "--insecure")

	return RunWithStdin(stdin, "", "../../dist/argocd", args...)
}

func Patch(path string, jsonPatch string) {
	log.WithFields(log.Fields{"path": path, "jsonPatch": jsonPatch}).Info("patching")

	filename := filepath.Join(RepoDirectory(), path)
	bytes, err := os.ReadFile(filename)
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
		log.WithFields(log.Fields{"bytes": string(bytes)}).Info("Patched YAML")
		CheckError(err)
	}

	CheckError(os.WriteFile(filename, bytes, 0o644))
	FailOnErr(Run(RepoDirectory(), "git", "diff"))
	FailOnErr(Run(RepoDirectory(), "git", "commit", "-am", "patch"))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func Delete(path string) {
	log.WithFields(log.Fields{"path": path}).Info("deleting")

	CheckError(os.Remove(filepath.Join(RepoDirectory(), path)))

	FailOnErr(Run(RepoDirectory(), "git", "diff"))
	FailOnErr(Run(RepoDirectory(), "git", "commit", "-am", "delete"))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func WriteFile(path, contents string) {
	log.WithFields(log.Fields{"path": path}).Info("adding")

	CheckError(os.WriteFile(filepath.Join(RepoDirectory(), path), []byte(contents), 0o644))
}

func AddFile(path, contents string) {
	WriteFile(path, contents)

	FailOnErr(Run(RepoDirectory(), "git", "diff"))
	FailOnErr(Run(RepoDirectory(), "git", "add", "."))
	FailOnErr(Run(RepoDirectory(), "git", "commit", "-am", "add file"))

	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func AddSignedFile(path, contents string) {
	WriteFile(path, contents)

	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	FailOnErr(Run(RepoDirectory(), "git", "diff"))
	FailOnErr(Run(RepoDirectory(), "git", "add", "."))
	FailOnErr(Run(RepoDirectory(), "git", "-c", fmt.Sprintf("user.signingkey=%s", GpgGoodKeyID), "commit", "-S", "-am", "add file"))
	os.Setenv("GNUPGHOME", prevGnuPGHome)
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func AddSignedTag(name string) {
	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	defer os.Setenv("GNUPGHOME", prevGnuPGHome)
	FailOnErr(Run(RepoDirectory(), "git", "-c", fmt.Sprintf("user.signingkey=%s", GpgGoodKeyID), "tag", "-sm", "add signed tag", name))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "--tags", "-f", "origin", "master"))
	}
}

func AddTag(name string) {
	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	defer os.Setenv("GNUPGHOME", prevGnuPGHome)
	FailOnErr(Run(RepoDirectory(), "git", "tag", name))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(RepoDirectory(), "git", "push", "--tags", "-f", "origin", "master"))
	}
}

// create the resource by creating using "kubectl apply", with bonus templating
func Declarative(filename string, values interface{}) (string, error) {
	bytes, err := os.ReadFile(path.Join("testdata", filename))
	CheckError(err)

	tmpFile, err := os.CreateTemp("", "")
	CheckError(err)
	_, err = tmpFile.WriteString(Tmpl(string(bytes), values))
	CheckError(err)
	defer tmpFile.Close()
	return Run("", "kubectl", "-n", TestNamespace(), "apply", "-f", tmpFile.Name())
}

func CreateSubmoduleRepos(repoType string) {
	// set-up submodule repo
	FailOnErr(Run("", "cp", "-Rf", "testdata/git-submodule/", submoduleDirectory()))
	FailOnErr(Run(submoduleDirectory(), "chmod", "777", "."))
	FailOnErr(Run(submoduleDirectory(), "git", "init", "-b", "master"))
	FailOnErr(Run(submoduleDirectory(), "git", "add", "."))
	FailOnErr(Run(submoduleDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(submoduleDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE_SUBMODULE")))
		FailOnErr(Run(submoduleDirectory(), "git", "push", "origin", "master", "-f"))
	}

	// set-up submodule parent repo
	FailOnErr(Run("", "mkdir", submoduleParentDirectory()))
	FailOnErr(Run(submoduleParentDirectory(), "chmod", "777", "."))
	FailOnErr(Run(submoduleParentDirectory(), "git", "init", "-b", "master"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "add", "."))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(submoduleParentDirectory(), "git", "submodule", "add", "-b", "master", os.Getenv("ARGOCD_E2E_GIT_SERVICE_SUBMODULE"), "submodule/test"))
	} else {
		oldAllowProtocol, isAllowProtocolSet := os.LookupEnv("GIT_ALLOW_PROTOCOL")
		CheckError(os.Setenv("GIT_ALLOW_PROTOCOL", "file"))
		FailOnErr(Run(submoduleParentDirectory(), "git", "submodule", "add", "-b", "master", "../submodule.git", "submodule/test"))
		if isAllowProtocolSet {
			CheckError(os.Setenv("GIT_ALLOW_PROTOCOL", oldAllowProtocol))
		} else {
			CheckError(os.Unsetenv("GIT_ALLOW_PROTOCOL"))
		}
	}
	if repoType == "ssh" {
		FailOnErr(Run(submoduleParentDirectory(), "git", "config", "--file=.gitmodules", "submodule.submodule/test.url", RepoURL(RepoURLTypeSSHSubmodule)))
	} else if repoType == "https" {
		FailOnErr(Run(submoduleParentDirectory(), "git", "config", "--file=.gitmodules", "submodule.submodule/test.url", RepoURL(RepoURLTypeHTTPSSubmodule)))
	}
	FailOnErr(Run(submoduleParentDirectory(), "git", "add", "--all"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "commit", "-q", "-m", "commit with submodule"))

	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(submoduleParentDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE_SUBMODULE_PARENT")))
		FailOnErr(Run(submoduleParentDirectory(), "git", "push", "origin", "master", "-f"))
	}
}

func RemoveSubmodule() {
	log.Info("removing submodule")

	FailOnErr(Run(submoduleParentDirectory(), "git", "rm", "submodule/test"))
	FailOnErr(Run(submoduleParentDirectory(), "touch", "submodule/.gitkeep"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "add", "submodule/.gitkeep"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "commit", "-m", "remove submodule"))
	if IsRemote() || IsRunningTestContainers() {
		FailOnErr(Run(submoduleParentDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

// RestartRepoServer performs a restart of the repo server deployment and waits
// until the rollout has completed.
func RestartRepoServer() {
	if IsRemote() {
		log.Infof("Waiting for repo server to restart")
		prefix := os.Getenv("ARGOCD_E2E_NAME_PREFIX")
		workload := "argocd-repo-server"
		if prefix != "" {
			workload = prefix + "-repo-server"
		}
		FailOnErr(Run("", "kubectl", "rollout", "-n", TestNamespace(), "restart", "deployment", workload))
		FailOnErr(Run("", "kubectl", "rollout", "-n", TestNamespace(), "status", "deployment", workload))
		// wait longer to avoid error on s390x
		time.Sleep(10 * time.Second)
	}
}

// RestartAPIServer performs a restart of the API server deployment and waits
// until the rollout has completed.
func RestartAPIServer() {
	if IsRemote() {
		log.Infof("Waiting for API server to restart")
		prefix := os.Getenv("ARGOCD_E2E_NAME_PREFIX")
		workload := "argocd-server"
		if prefix != "" {
			workload = prefix + "-server"
		}
		FailOnErr(Run("", "kubectl", "rollout", "-n", TestNamespace(), "restart", "deployment", workload))
		FailOnErr(Run("", "kubectl", "rollout", "-n", TestNamespace(), "status", "deployment", workload))
	}
}

// LocalOrRemotePath selects a path for a given application based on whether
// tests are running local or remote.
func LocalOrRemotePath(base string) string {
	if IsRemote() {
		return base + "/remote"
	} else {
		return base + "/local"
	}
}

// SkipOnEnv allows to skip a test when a given environment variable is set.
// Environment variable names follow the ARGOCD_E2E_SKIP_<suffix> pattern,
// and must be set to the string value 'true' in order to skip a test.
func SkipOnEnv(t *testing.T, suffixes ...string) {
	t.Helper()
	for _, suffix := range suffixes {
		e := os.Getenv("ARGOCD_E2E_SKIP_" + suffix)
		if e == "true" {
			t.Skip()
		}
	}
}

// SkipIfAlreadyRun skips a test if it has been already run by a previous
// test cycle and was recorded.
func SkipIfAlreadyRun(t *testing.T) {
	t.Helper()
	if _, ok := testsRun[t.Name()]; ok {
		t.Skip()
	}
}

// RecordTestRun records a test that has been run successfully to a text file,
// so that it can be automatically skipped if requested.
func RecordTestRun(t *testing.T) {
	t.Helper()
	if t.Skipped() || t.Failed() {
		return
	}
	rf := os.Getenv("ARGOCD_E2E_RECORD")
	if rf == "" {
		return
	}
	log.Infof("Registering test execution at %s", rf)
	f, err := os.OpenFile(rf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("could not open record file %s: %v", rf, err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			t.Fatalf("could not close record file %s: %v", rf, err)
		}
	}()
	if _, err := f.WriteString(fmt.Sprintf("%s\n", t.Name())); err != nil {
		t.Fatalf("could not write to %s: %v", rf, err)
	}
}

func GetApiServerAddress() string {
	return apiServerAddress
}

func GetNotificationServerAddress() string {
	return defaultNotificationServer
}

func GetToken() string {
	return token
}
