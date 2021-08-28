package fixture

import (
	"bufio"
	"context"
	goerrors "errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/pkg/errors"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/common"
	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	grpcutil "github.com/argoproj/argo-cd/v2/util/grpc"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rand"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	defaultApiServer     = "localhost:8080"
	defaultAdminPassword = "password"
	defaultAdminUsername = "admin"
	testingLabel         = "e2e.argoproj.io"
	ArgoCDNamespace      = "argocd-e2e"

	// ensure all repos are in one directory tree, so we can easily clean them up
	TmpDir             = "/tmp/argo-e2e"
	repoDir            = "testdata.git"
	submoduleDir       = "submodule.git"
	submoduleParentDir = "submoduleParent.git"

	GuestbookPath = "guestbook"
)

const (
	EnvAdminUsername = "ARGOCD_E2E_ADMIN_USERNAME"
	EnvAdminPassword = "ARGOCD_E2E_ADMIN_PASSWORD"
)

var (
	id                  string
	deploymentNamespace string
	name                string
	KubeClientset       kubernetes.Interface
	DynamicClientset    dynamic.Interface
	AppClientset        appclientset.Interface
	ArgoCDClientset     argocdclient.Client
	adminUsername       string
	adminPassword       string
	apiServerAddress    string
	token               string
	plainText           bool
	testsRun            map[string]bool
)

type RepoURLType string

const (
	RepoURLTypeFile                 = "file"
	RepoURLTypeHTTPS                = "https"
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
	GpgGoodKeyID                    = "D56C4FCA57A46444"
	HelmOCIRegistryURL              = "localhost:5000/myrepo"
)

// TestNamespace returns the namespace where Argo CD E2E test instance will be
// running in.
func TestNamespace() string {
	return GetEnvWithDefault("ARGOCD_E2E_NAMESPACE", ArgoCDNamespace)
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
	r := os.Getenv("ARGOCD_E2E_REMOTE")
	return r == "true"
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

	apiServerAddress = GetEnvWithDefault(argocdclient.EnvArgoCDServer, defaultApiServer)
	adminUsername = GetEnvWithDefault(EnvAdminUsername, defaultAdminUsername)
	adminPassword = GetEnvWithDefault(EnvAdminPassword, defaultAdminPassword)

	tlsTestResult, err := grpcutil.TestTLS(apiServerAddress)
	CheckError(err)

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{Insecure: true, ServerAddr: apiServerAddress, PlainText: !tlsTestResult.TLS})
	CheckError(err)

	closer, client, err := ArgoCDClientset.NewSessionClient()
	CheckError(err)
	defer io.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &sessionpkg.SessionCreateRequest{Username: adminUsername, Password: adminPassword})
	CheckError(err)

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{
		Insecure:   true,
		ServerAddr: apiServerAddress,
		AuthToken:  sessionResponse.Token,
		PlainText:  !tlsTestResult.TLS,
	})
	CheckError(err)

	token = sessionResponse.Token
	plainText = !tlsTestResult.TLS

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
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		testsRun[scanner.Text()] = true
	}

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
		return GetEnvWithDefault(EnvRepoURLDefault, fmt.Sprintf("file://%s", repoDirectory()))
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
	FailOnErr(Run("", "kubectl", "label", "secret", secretName, testingLabel+"=true", "-n", TestNamespace()))
	return secretName
}

// Convenience wrapper for updating argocd-cm
func updateSettingConfigMap(updater func(cm *corev1.ConfigMap) error) {
	updateGenericConfigMap(common.ArgoCDConfigMapName, updater)
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
			if len(v.IgnoreDifferences.JSONPointers) > 0 || len(v.IgnoreDifferences.JQPathExpressions) > 0 {
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

func SetConfigManagementPlugins(plugin ...v1alpha1.ConfigManagementPlugin) {
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		yamlBytes, err := yaml.Marshal(plugin)
		if err != nil {
			return err
		}
		cm.Data["configManagementPlugins"] = string(yamlBytes)
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

func EnsureCleanState(t *testing.T) {
	// In large scenarios, we can skip tests that already run
	SkipIfAlreadyRun(t)
	// Register this test after it has been run & was successfull
	t.Cleanup(func() {
		RecordTestRun(t)
	})

	start := time.Now()

	policy := v1.DeletePropagationBackground
	// delete resources
	// kubectl delete apps --all
	CheckError(AppClientset.ArgoprojV1alpha1().Applications(TestNamespace()).DeleteCollection(context.Background(), v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{}))
	// kubectl delete appprojects --field-selector metadata.name!=default
	CheckError(AppClientset.ArgoprojV1alpha1().AppProjects(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{FieldSelector: "metadata.name!=default"}))
	// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-config
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepository}))
	// kubectl delete secrets -l argocd.argoproj.io/secret-type=repo-creds
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeRepoCreds}))
	// kubectl delete secrets -l e2e.argoproj.io=true
	CheckError(KubeClientset.CoreV1().Secrets(TestNamespace()).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{LabelSelector: testingLabel + "=true"}))

	FailOnErr(Run("", "kubectl", "delete", "ns", "-l", testingLabel+"=true", "--field-selector", "status.phase=Active", "--wait=false"))
	FailOnErr(Run("", "kubectl", "delete", "crd", "-l", testingLabel+"=true", "--wait=false"))

	// reset settings
	updateSettingConfigMap(func(cm *corev1.ConfigMap) error {
		cm.Data = map[string]string{}
		return nil
	})

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
	})

	// Create separate project for testing gpg signature verification
	FailOnErr(RunCli("proj", "create", "gpg"))
	SetProjectSpec("gpg", v1alpha1.AppProjectSpec{
		OrphanedResources:        nil,
		SourceRepos:              []string{"*"},
		Destinations:             []v1alpha1.ApplicationDestination{{Namespace: "*", Server: "*"}},
		ClusterResourceWhitelist: []v1.GroupKind{{Group: "*", Kind: "*"}},
		SignatureKeys:            []v1alpha1.SignatureKey{{KeyID: GpgGoodKeyID}},
	})

	// Recreate temp dir
	CheckError(os.RemoveAll(TmpDir))
	FailOnErr(Run("", "mkdir", "-p", TmpDir))

	// random id - unique across test runs
	postFix := "-" + strings.ToLower(rand.RandString(5))
	id = t.Name() + postFix
	name = DnsFriendly(t.Name(), "")
	deploymentNamespace = DnsFriendly(fmt.Sprintf("argocd-e2e-%s", t.Name()), postFix)

	// create TLS and SSH certificate directories
	if IsLocal() {
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/tls"))
		FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/ssh"))
	}

	// For signing during the tests
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
	}

	// set-up tmp repo, must have unique name
	FailOnErr(Run("", "cp", "-Rf", "testdata", repoDirectory()))
	FailOnErr(Run(repoDirectory(), "chmod", "777", "."))
	FailOnErr(Run(repoDirectory(), "git", "init"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE")))
		FailOnErr(Run(repoDirectory(), "git", "push", "origin", "master", "-f"))
	}

	// create namespace
	FailOnErr(Run("", "kubectl", "create", "ns", DeploymentNamespace()))
	FailOnErr(Run("", "kubectl", "label", "ns", DeploymentNamespace(), testingLabel+"=true"))

	log.WithFields(log.Fields{"duration": time.Since(start), "name": t.Name(), "id": id, "username": "admin", "password": "password"}).Info("clean state")
}

func RunCli(args ...string) (string, error) {
	return RunCliWithStdin("", args...)
}

func RunCliWithStdin(stdin string, args ...string) (string, error) {
	if plainText {
		args = append(args, "--plaintext")
	}

	args = append(args, "--server", apiServerAddress, "--auth-token", token, "--insecure")

	return RunWithStdin(stdin, "", "../../dist/argocd", args...)
}

func Patch(path string, jsonPatch string) {

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

	CheckError(ioutil.WriteFile(filepath.Join(repoDirectory(), path), []byte(contents), 0644))
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
	FailOnErr(Run(repoDirectory(), "git", "-c", fmt.Sprintf("user.signingkey=%s", GpgGoodKeyID), "commit", "-S", "-am", "add file"))
	os.Setenv("GNUPGHOME", prevGnuPGHome)
	if IsRemote() {
		FailOnErr(Run(repoDirectory(), "git", "push", "-f", "origin", "master"))
	}
}

// create the resource by creating using "kubectl apply", with bonus templating
func Declarative(filename string, values interface{}) (string, error) {

	bytes, err := ioutil.ReadFile(path.Join("testdata", filename))
	CheckError(err)

	tmpFile, err := ioutil.TempFile("", "")
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
	FailOnErr(Run(submoduleDirectory(), "git", "init"))
	FailOnErr(Run(submoduleDirectory(), "git", "add", "."))
	FailOnErr(Run(submoduleDirectory(), "git", "commit", "-q", "-m", "initial commit"))

	if IsRemote() {
		FailOnErr(Run(submoduleDirectory(), "git", "remote", "add", "origin", os.Getenv("ARGOCD_E2E_GIT_SERVICE_SUBMODULE")))
		FailOnErr(Run(submoduleDirectory(), "git", "push", "origin", "master", "-f"))
	}

	// set-up submodule parent repo
	FailOnErr(Run("", "mkdir", submoduleParentDirectory()))
	FailOnErr(Run(submoduleParentDirectory(), "chmod", "777", "."))
	FailOnErr(Run(submoduleParentDirectory(), "git", "init"))
	FailOnErr(Run(submoduleParentDirectory(), "git", "add", "."))
	FailOnErr(Run(submoduleParentDirectory(), "git", "submodule", "add", "-b", "master", "../submodule.git", "submodule/test"))
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
		FailOnErr(Run("", "kubectl", "rollout", "restart", "deployment", workload))
		FailOnErr(Run("", "kubectl", "rollout", "status", "deployment", workload))
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
		FailOnErr(Run("", "kubectl", "rollout", "restart", "deployment", workload))
		FailOnErr(Run("", "kubectl", "rollout", "status", "deployment", workload))
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
	if _, ok := testsRun[t.Name()]; ok {
		t.Skip()
	}
}

// RecordTestRun records a test that has been run successfully to a text file,
// so that it can be automatically skipped if requested.
func RecordTestRun(t *testing.T) {
	if t.Skipped() || t.Failed() {
		return
	}
	rf := os.Getenv("ARGOCD_E2E_RECORD")
	if rf == "" {
		return
	}
	log.Infof("Registering test execution at %s", rf)
	f, err := os.OpenFile(rf, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("could not open record file %s: %v", rf, err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%s\n", t.Name())); err != nil {
		t.Fatalf("could not write to %s: %v", rf, err)
	}
}
