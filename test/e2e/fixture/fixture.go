package fixture

import (
	"bufio"
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	grpcutil "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rand"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	defaultApiServer        = "localhost:8080"
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
)

var (
	id                      string
	deploymentNamespace     string
	name                    string
	KubeClientset           kubernetes.Interface
	KubeConfig              *rest.Config
	DynamicClientset        dynamic.Interface
	AppClientset            appclientset.Interface
	ArgoCDClientset         apiclient.Client
	adminUsername           string
	AdminPassword           string
	apiServerAddress        string
	token                   string
	plainText               bool
	testsRun                map[string]bool
	argoCDServerName        string
	argoCDRedisHAProxyName  string
	argoCDRedisName         string
	argoCDRepoServerName    string
	argoCDAppControllerName string
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
	HelmOCIRegistryURL              = "localhost:5000/myrepo"
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
	// set-up variables
	config := getKubeConfig("", clientcmd.ConfigOverrides{})
	AppClientset = appclientset.NewForConfigOrDie(config)
	KubeClientset = kubernetes.NewForConfigOrDie(config)
	DynamicClientset = dynamic.NewForConfigOrDie(config)
	KubeConfig = config

	apiServerAddress = GetEnvWithDefault(apiclient.EnvArgoCDServer, defaultApiServer)
	adminUsername = GetEnvWithDefault(EnvAdminUsername, defaultAdminUsername)
	AdminPassword = GetEnvWithDefault(EnvAdminPassword, defaultAdminPassword)

	argoCDServerName = GetEnvWithDefault(EnvArgoCDServerName, common.DefaultServerName)
	argoCDRedisHAProxyName = GetEnvWithDefault(EnvArgoCDRedisHAProxyName, common.DefaultRedisHaProxyName)
	argoCDRedisName = GetEnvWithDefault(EnvArgoCDRedisName, common.DefaultRedisName)
	argoCDRepoServerName = GetEnvWithDefault(EnvArgoCDRepoServerName, common.DefaultRepoServerName)
	argoCDAppControllerName = GetEnvWithDefault(EnvArgoCDAppControllerName, common.DefaultApplicationControllerName)

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

	CheckError(LoginAs(adminUsername))

	log.WithFields(log.Fields{"apiServerAddress": apiServerAddress}).Info("initialized")

	// Preload a list of tests that should be skipped
	testsRun = make(map[string]bool)
	rf := os.Getenv("ARGOCD_E2E_RECORD")
	if rf == "" {
		return
	}
	f, err := os.Open(rf)
	if err != nil {
		if stderrors.Is(err, os.ErrNotExist) {
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

func loginAs(username, password string) error {
	closer, client, err := ArgoCDClientset.NewSessionClient()
	if err != nil {
		return err
	}
	defer io.Close(closer)

	userInfoResponse, err := client.GetUserInfo(context.Background(), &sessionpkg.GetUserInfoRequest{})
	if err != nil {
		return err
	}
	if userInfoResponse.Username == username && userInfoResponse.LoggedIn {
		return nil
	}

	sessionResponse, err := client.Create(context.Background(), &sessionpkg.SessionCreateRequest{Username: username, Password: password})
	if err != nil {
		return err
	}
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
	return err
}

func LoginAs(username string) error {
	password := DefaultTestUserPassword
	if username == "admin" {
		password = AdminPassword
	}
	return loginAs(username, password)
}

func Name() string {
	return name
}

func repoDirectory() string {
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
		return GetEnvWithDefault(EnvRepoURLTypeSSH, "ssh://root@localhost:2222/tmp/argo-e2e/testdata.git")
	// Git submodule repo
	case RepoURLTypeSSHSubmodule:
		return GetEnvWithDefault(EnvRepoURLTypeSSHSubmodule, "ssh://root@localhost:2222/tmp/argo-e2e/submodule.git")
	// Git submodule parent repo
	case RepoURLTypeSSHSubmoduleParent:
		return GetEnvWithDefault(EnvRepoURLTypeSSHSubmoduleParent, "ssh://root@localhost:2222/tmp/argo-e2e/submoduleParent.git")
	// Git server via HTTPS
	case RepoURLTypeHTTPS:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPS, "https://localhost:9443/argo-e2e/testdata.git")
	// Git "organisation" via HTTPS
	case RepoURLTypeHTTPSOrg:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSOrg, "https://localhost:9443/argo-e2e")
	// Git server via HTTPS - Client Cert protected
	case RepoURLTypeHTTPSClientCert:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSClientCert, "https://localhost:9444/argo-e2e/testdata.git")
	case RepoURLTypeHTTPSSubmodule:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSSubmodule, "https://localhost:9443/argo-e2e/submodule.git")
		// Git submodule parent repo
	case RepoURLTypeHTTPSSubmoduleParent:
		return GetEnvWithDefault(EnvRepoURLTypeHTTPSSubmoduleParent, "https://localhost:9443/argo-e2e/submoduleParent.git")
	// Default - file based Git repository
	case RepoURLTypeHelm:
		return GetEnvWithDefault(EnvRepoURLTypeHelm, "https://localhost:9444/argo-e2e/testdata.git/helm-repo/local")
	// When Helm Repo has sub repos, this is the parent repo URL
	case RepoURLTypeHelmParent:
		return GetEnvWithDefault(EnvRepoURLTypeHelm, "https://localhost:9444/argo-e2e/testdata.git/helm-repo")
	case RepoURLTypeHelmOCI:
		return HelmOCIRegistryURL
	default:
		return GetEnvWithDefault(EnvRepoURLDefault, "file://"+repoDirectory())
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
	secretName := "argocd-e2e-" + name
	FailOnErr(Run("", "kubectl", "create", "secret", "generic", secretName,
		"--from-literal=username="+username,
		"--from-literal=password="+password,
		"-n", TestNamespace()))
	FailOnErr(Run("", "kubectl", "label", "secret", secretName, TestingLabel+"=true", "-n", TestNamespace()))
	return secretName
}

// Convenience wrapper for updating argocd-cm
func updateSettingConfigMap(updater func(cm *corev1.ConfigMap) error) error {
	return updateGenericConfigMap(common.ArgoCDConfigMapName, updater)
}

// Convenience wrapper for updating argocd-notifications-cm
func updateNotificationsConfigMap(updater func(cm *corev1.ConfigMap) error) error {
	return updateGenericConfigMap(common.ArgoCDNotificationsConfigMapName, updater)
}

// Convenience wrapper for updating argocd-cm-rbac
func updateRBACConfigMap(updater func(cm *corev1.ConfigMap) error) error {
	return updateGenericConfigMap(common.ArgoCDRBACConfigMapName, updater)
}

func configMapsEquivalent(a *corev1.ConfigMap, b *corev1.ConfigMap) bool {
	return reflect.DeepEqual(a.Immutable, b.Immutable) &&
		reflect.DeepEqual(a.TypeMeta, b.TypeMeta) &&
		reflect.DeepEqual(a.ObjectMeta, b.ObjectMeta) &&
		// Covers cases when one map is nil and another is empty map
		(len(a.Data) == 0 && len(b.Data) == 0 || reflect.DeepEqual(a.Data, b.Data)) &&
		(len(a.BinaryData) == 0 && len(b.BinaryData) == 0 || reflect.DeepEqual(a.BinaryData, b.BinaryData))
}

// Updates a given config map in argocd-e2e namespace
func updateGenericConfigMap(name string, updater func(cm *corev1.ConfigMap) error) error {
	cm, err := KubeClientset.CoreV1().ConfigMaps(TestNamespace()).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	oldCm := cm.DeepCopy()
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	err = updater(cm)
	if err != nil {
		return err
	}
	if !configMapsEquivalent(cm, oldCm) {
		_, err = KubeClientset.CoreV1().ConfigMaps(TestNamespace()).Update(context.Background(), cm, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func SetEnableManifestGeneration(val map[v1alpha1.ApplicationSourceType]bool) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		for k, v := range val {
			cm.Data[strings.ToLower(string(k))+".enable"] = strconv.FormatBool(v)
		}
		return nil
	})
}

func SetResourceOverrides(overrides map[string]v1alpha1.ResourceOverride) error {
	err := updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
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
	if err != nil {
		return err
	}

	return SetResourceOverridesSplitKeys(overrides)
}

func SetInstallationID(installationID string) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["installationID"] = installationID
		return nil
	})
}

func SetTrackingMethod(trackingMethod string) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["application.resourceTrackingMethod"] = trackingMethod
		return nil
	})
}

func SetTrackingLabel(trackingLabel string) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data["application.instanceLabelKey"] = trackingLabel
		return nil
	})
}

func SetResourceOverridesSplitKeys(overrides map[string]v1alpha1.ResourceOverride) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
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

func SetAccounts(accounts map[string][]string) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		for k, v := range accounts {
			cm.Data["accounts."+k] = strings.Join(v, ",")
		}
		return nil
	})
}

func SetPermissions(permissions []ACL, username string, roleName string) error {
	return updateRBACConfigMap(func(cm *corev1.ConfigMap) error {
		var aclstr string

		for _, permission := range permissions {
			aclstr += fmt.Sprintf("p, role:%s, %s, %s, %s, allow \n", roleName, permission.Resource, permission.Action, permission.Scope)
		}

		aclstr += fmt.Sprintf("g, %s, role:%s", username, roleName)
		cm.Data["policy.csv"] = aclstr

		return nil
	})
}

func SetResourceFilter(filters settings.ResourcesFilter) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
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

func SetHelmRepos(repos ...settings.HelmRepoCredentials) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		yamlBytes, err := yaml.Marshal(repos)
		if err != nil {
			return err
		}
		cm.Data["helm.repositories"] = string(yamlBytes)
		return nil
	})
}

func SetRepos(repos ...settings.RepositoryCredentials) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		yamlBytes, err := yaml.Marshal(repos)
		if err != nil {
			return err
		}
		cm.Data["repositories"] = string(yamlBytes)
		return nil
	})
}

func SetProjectSpec(project string, spec v1alpha1.AppProjectSpec) error {
	proj, err := AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).Get(context.Background(), project, metav1.GetOptions{})
	if err != nil {
		return err
	}
	proj.Spec = spec
	_, err = AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).Update(context.Background(), proj, metav1.UpdateOptions{})
	return err
}

func SetParamInSettingConfigMap(key, value string) error {
	return updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data[key] = value
		return nil
	})
}

func SetParamInNotificationsConfigMap(key, value string) error {
	return updateNotificationsConfigMap(func(cm *corev1.ConfigMap) error {
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
	policy := metav1.DeletePropagationBackground

	RunFunctionsInParallelAndCheckErrors(t, []func() error{
		func() error {
			// kubectl delete apps ...
			return AppClientset.ArgoprojV1alpha1().Applications(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{})
		},
		func() error {
			// kubectl delete apps ...
			return AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{})
		},
		func() error {
			// kubectl delete appprojects --field-selector metadata.name!=default
			return AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{FieldSelector: "metadata.name!=default"})
		},
		func() error {
			// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-config
			return KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepository})
		},
		func() error {
			// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-creds
			return KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepoCreds})
		},
		func() error {
			// kubectl delete secrets -l argocd.argoproj.io/secret-type=cluster
			return KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster})
		},
		func() error {
			// kubectl delete secrets -l e2e.argoproj.io=true
			return KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(
				context.Background(),
				metav1.DeleteOptions{PropagationPolicy: &policy},
				metav1.ListOptions{LabelSelector: TestingLabel + "=true"})
		},
	})

	RunFunctionsInParallelAndCheckErrors(t, []func() error{
		func() error {
			// delete old namespaces which were created by tests
			namespaces, err := KubeClientset.CoreV1().Namespaces().List(
				context.Background(),
				metav1.ListOptions{
					LabelSelector: TestingLabel + "=true",
					FieldSelector: "status.phase=Active",
				},
			)
			if err != nil {
				return err
			}
			if len(namespaces.Items) > 0 {
				args := []string{"delete", "ns", "--wait=false"}
				for _, namespace := range namespaces.Items {
					args = append(args, namespace.Name)
				}
				_, err := Run("", "kubectl", args...)
				if err != nil {
					return err
				}
			}

			namespaces, err = KubeClientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			testNamespaceNames := []string{}
			for _, namespace := range namespaces.Items {
				if strings.HasPrefix(namespace.Name, E2ETestPrefix) {
					testNamespaceNames = append(testNamespaceNames, namespace.Name)
				}
			}
			if len(testNamespaceNames) > 0 {
				args := []string{"delete", "ns"}
				args = append(args, testNamespaceNames...)
				_, err := Run("", "kubectl", args...)
				if err != nil {
					return err
				}
			}
			return nil
		},
		func() error {
			// delete old CRDs which were created by tests, doesn't seem to have kube api to get items
			_, err := Run("", "kubectl", "delete", "crd", "-l", TestingLabel+"=true", "--wait=false")
			return err
		},
		func() error {
			// delete old ClusterRoles which were created by tests
			clusterRoles, err := KubeClientset.RbacV1().ClusterRoles().List(
				context.Background(),
				metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s", TestingLabel, "true"),
				},
			)
			if err != nil {
				return err
			}
			if len(clusterRoles.Items) > 0 {
				args := []string{"delete", "clusterrole", "--wait=false"}
				for _, clusterRole := range clusterRoles.Items {
					args = append(args, clusterRole.Name)
				}
				_, err := Run("", "kubectl", args...)
				if err != nil {
					return err
				}
			}

			clusterRoles, err = KubeClientset.RbacV1().ClusterRoles().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			testClusterRoleNames := []string{}
			for _, clusterRole := range clusterRoles.Items {
				if strings.HasPrefix(clusterRole.Name, E2ETestPrefix) {
					testClusterRoleNames = append(testClusterRoleNames, clusterRole.Name)
				}
			}
			if len(testClusterRoleNames) > 0 {
				args := []string{"delete", "clusterrole"}
				args = append(args, testClusterRoleNames...)
				_, err := Run("", "kubectl", args...)
				if err != nil {
					return err
				}
			}
			return nil
		},
		func() error {
			// delete old ClusterRoleBindings which were created by tests
			clusterRoleBindings, err := KubeClientset.RbacV1().ClusterRoleBindings().List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			testClusterRoleBindingNames := []string{}
			for _, clusterRoleBinding := range clusterRoleBindings.Items {
				if strings.HasPrefix(clusterRoleBinding.Name, E2ETestPrefix) {
					testClusterRoleBindingNames = append(testClusterRoleBindingNames, clusterRoleBinding.Name)
				}
			}
			if len(testClusterRoleBindingNames) > 0 {
				args := []string{"delete", "clusterrolebinding"}
				args = append(args, testClusterRoleBindingNames...)
				_, err := Run("", "kubectl", args...)
				if err != nil {
					return err
				}
			}
			return nil
		},
		func() error {
			err := updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
				cm.Data = map[string]string{}
				return nil
			})
			if err != nil {
				return err
			}
			err = updateNotificationsConfigMap(func(cm *corev1.ConfigMap) error {
				cm.Data = map[string]string{}
				return nil
			})
			if err != nil {
				return err
			}
			err = updateRBACConfigMap(func(cm *corev1.ConfigMap) error {
				cm.Data = map[string]string{}
				return nil
			})
			if err != nil {
				return err
			}
			return updateGenericConfigMap(common.ArgoCDGPGKeysConfigMapName, func(cm *corev1.ConfigMap) error {
				cm.Data = map[string]string{}
				return nil
			})
		},
		func() error {
			// We can switch user and as result in previous state we will have non-admin user, this case should be reset
			return LoginAs(adminUsername)
		},
	})

	RunFunctionsInParallelAndCheckErrors(t, []func() error{
		func() error {
			err := SetProjectSpec("default", v1alpha1.AppProjectSpec{
				OrphanedResources:        nil,
				SourceRepos:              []string{"*"},
				Destinations:             []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}},
				ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
				SourceNamespaces:         []string{AppNamespace()},
			})
			if err != nil {
				return err
			}

			// Create separate project for testing gpg signature verification
			_, err = AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).Create(
				context.Background(),
				&v1alpha1.AppProject{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpg",
					},
					Spec: v1alpha1.AppProjectSpec{
						OrphanedResources:        nil,
						SourceRepos:              []string{"*"},
						Destinations:             []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}},
						ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
						SignatureKeys:            []v1alpha1.SignatureKey{{KeyID: GpgGoodKeyID}},
						SourceNamespaces:         []string{AppNamespace()},
					},
				},
				metav1.CreateOptions{},
			)
			return err
		},
		func() error {
			err := os.RemoveAll(TmpDir)
			if err != nil {
				return err
			}
			_, err = Run("", "mkdir", "-p", TmpDir)
			if err != nil {
				return err
			}

			// create TLS and SSH certificate directories
			if IsLocal() {
				_, err = Run("", "mkdir", "-p", TmpDir+"/app/config/tls")
				if err != nil {
					return err
				}
				_, err = Run("", "mkdir", "-p", TmpDir+"/app/config/ssh")
				if err != nil {
					return err
				}
			}

			// For signing during the tests
			_, err = Run("", "mkdir", "-p", TmpDir+"/gpg")
			if err != nil {
				return err
			}
			_, err = Run("", "chmod", "0700", TmpDir+"/gpg")
			if err != nil {
				return err
			}
			prevGnuPGHome := os.Getenv("GNUPGHOME")
			os.Setenv("GNUPGHOME", TmpDir+"/gpg")
			// nolint:errcheck
			Run("", "pkill", "-9", "gpg-agent")
			_, err = Run("", "gpg", "--import", "../fixture/gpg/signingkey.asc")
			if err != nil {
				return err
			}
			os.Setenv("GNUPGHOME", prevGnuPGHome)

			// recreate GPG directories
			if IsLocal() {
				_, err = Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/source")
				if err != nil {
					return err
				}
				_, err = Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/keys")
				if err != nil {
					return err
				}
				_, err = Run("", "chmod", "0700", TmpDir+"/app/config/gpg/keys")
				if err != nil {
					return err
				}
				_, err = Run("", "mkdir", "-p", TmpDir+PluginSockFilePath)
				if err != nil {
					return err
				}
				_, err = Run("", "chmod", "0700", TmpDir+PluginSockFilePath)
				if err != nil {
					return err
				}
			}

			// set-up tmp repo, must have unique name
			_, err = Run("", "cp", "-Rf", opt.testdata, repoDirectory())
			if err != nil {
				return err
			}
			_, err = Run(repoDirectory(), "chmod", "777", ".")
			if err != nil {
				return err
			}
			_, err = Run(repoDirectory(), "git", "init", "-b", "master")
			if err != nil {
				return err
			}
			_, err = Run(repoDirectory(), "git", "add", ".")
			if err != nil {
				return err
			}
			_, err = Run(repoDirectory(), "git", "commit", "-q", "-m", "initial commit")
			if err != nil {
				return err
			}

			if IsRemote() {
				_, err = Run(repoDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE"))
				if err != nil {
					return err
				}
				_, err = Run(repoDirectory(), "git", "push", "origin", "master", "-f")
				if err != nil {
					return err
				}
			}
			return nil
		},
		func() error {
			// random id - unique across test runs
			randString, err := rand.String(5)
			if err != nil {
				return err
			}
			postFix := "-" + strings.ToLower(randString)
			id = t.Name() + postFix
			name = DnsFriendly(t.Name(), "")
			deploymentNamespace = DnsFriendly("argocd-e2e-"+t.Name(), postFix)
			// create namespace
			_, err = Run("", "kubectl", "create", "ns", DeploymentNamespace())
			if err != nil {
				return err
			}
			_, err = Run("", "kubectl", "label", "ns", DeploymentNamespace(), TestingLabel+"=true")
			return err
		},
	})

	log.WithFields(log.Fields{
		"duration": time.Since(start),
		"name":     t.Name(),
		"id":       id,
		"username": "admin",
		"password": "password",
	}).Info("clean state")
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

	filename := filepath.Join(repoDirectory(), path)
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
		CheckError(err)
	}

	CheckError(os.WriteFile(filename, bytes, 0o644))
	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-am", "patch"))
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func Delete(path string) {
	log.WithFields(log.Fields{"path": path}).Info("deleting")

	CheckError(os.Remove(filepath.Join(repoDirectory(), path)))

	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-am", "delete"))
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func WriteFile(path, contents string) {
	log.WithFields(log.Fields{"path": path}).Info("adding")

	CheckError(os.WriteFile(filepath.Join(repoDirectory(), path), []byte(contents), 0o644))
}

func AddFile(path, contents string) {
	WriteFile(path, contents)

	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-am", "add file"))

	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func AddSignedFile(path, contents string) {
	WriteFile(path, contents)

	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "-c", "user.signingkey="+GpgGoodKeyID, "commit", "-S", "-am", "add file"))
	os.Setenv("GNUPGHOME", prevGnuPGHome)
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

func AddSignedTag(name string) {
	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	defer os.Setenv("GNUPGHOME", prevGnuPGHome)
	FailOnErr(Run(repoDirectory(), "git", "-c", "user.signingkey="+GpgGoodKeyID, "tag", "-sm", "add signed tag", name))
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "--tags", "-f", "origin", "master"))
	}
}

func AddTag(name string) {
	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	defer os.Setenv("GNUPGHOME", prevGnuPGHome)
	FailOnErr(Run(repoDirectory(), "git", "tag", name))
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "--tags", "-f", "origin", "master"))
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

	if IsRemote() {
		FailOnErr(Run(submoduleDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE_SUBMODULE")))
		FailOnErr(Run(submoduleDirectory(), "git", "push", "origin", "master", "-f"))
	}

	// set-up submodule parent repo
	FailOnErr(Run("", "mkdir", submoduleParentDirectory()))
	FailOnErr(Run(submoduleParentDirectory(), "chmod", "777", "."))
	FailOnErr(Run(submoduleParentDirectory(), "git", "init", "-b", "master"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "add", "."))
	if IsRemote() {
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

	if IsRemote() {
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
	if IsRemote() {
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
		time.Sleep(5 * time.Second)
	}
}

// RestartAPIServer performs a restart of the API server deployemt and waits
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
	if _, err := f.WriteString(t.Name() + "\n"); err != nil {
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
