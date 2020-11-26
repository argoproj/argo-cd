package fixture

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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

	"github.com/argoproj/argo-cd/common"
	argocdclient "github.com/argoproj/argo-cd/pkg/apiclient"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	. "github.com/argoproj/argo-cd/util/errors"
	grpcutil "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/rand"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	defaultApiServer = "localhost:8080"
	adminPassword    = "password"
	testingLabel     = "e2e.argoproj.io"
	ArgoCDNamespace  = "argocd-e2e"

	// ensure all repos are in one directory tree, so we can easily clean them up
	TmpDir             = "/tmp/argo-e2e"
	repoDir            = "testdata.git"
	submoduleDir       = "submodule.git"
	submoduleParentDir = "submoduleParent.git"

	GuestbookPath = "guestbook"
)

var (
	id                  string
	deploymentNamespace string
	name                string
	KubeClientset       kubernetes.Interface
	DynamicClientset    dynamic.Interface
	AppClientset        appclientset.Interface
	ArgoCDClientset     argocdclient.Client
	apiServerAddress    string
	token               string
	plainText           bool
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
	GitUsername                     = "admin"
	GitPassword                     = "password"
	GpgGoodKeyID                    = "D56C4FCA57A46444"
)

// getKubeConfig creates new kubernetes client config using specified config path and config overrides variables
func getKubeConfig(configPath string, overrides clientcmd.ConfigOverrides) *rest.Config {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = configPath
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &overrides, os.Stdin)

	restConfig, err := clientConfig.ClientConfig()
	CheckError(err)
	return restConfig
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
	apiServerAddress = os.Getenv(argocdclient.EnvArgoCDServer)
	if apiServerAddress == "" {
		apiServerAddress = defaultApiServer
	}

	tlsTestResult, err := grpcutil.TestTLS(apiServerAddress)
	CheckError(err)

	ArgoCDClientset, err = argocdclient.NewClient(&argocdclient.ClientOptions{Insecure: true, ServerAddr: apiServerAddress, PlainText: !tlsTestResult.TLS})
	CheckError(err)

	closer, client, err := ArgoCDClientset.NewSessionClient()
	CheckError(err)
	defer io.Close(closer)

	sessionResponse, err := client.Create(context.Background(), &sessionpkg.SessionCreateRequest{Username: "admin", Password: adminPassword})
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

func RepoURL(urlType RepoURLType) string {
	switch urlType {
	// Git server via SSH
	case RepoURLTypeSSH:
		return "ssh://root@localhost:2222/tmp/argo-e2e/testdata.git"
	// Git submodule repo
	case RepoURLTypeSSHSubmodule:
		return "ssh://root@localhost:2222/tmp/argo-e2e/submodule.git"
		// Git submodule parent repo
	case RepoURLTypeSSHSubmoduleParent:
		return "ssh://root@localhost:2222/tmp/argo-e2e/submoduleParent.git"
	// Git server via HTTPS
	case RepoURLTypeHTTPS:
		return "https://localhost:9443/argo-e2e/testdata.git"
	// Git server via HTTPS - Client Cert protected
	case RepoURLTypeHTTPSClientCert:
		return "https://localhost:9444/argo-e2e/testdata.git"
	case RepoURLTypeHTTPSSubmodule:
		return "https://localhost:9443/argo-e2e/submodule.git"
		// Git submodule parent repo
	case RepoURLTypeHTTPSSubmoduleParent:
		return "https://localhost:9443/argo-e2e/submoduleParent.git"
	// Default - file based Git repository
	case RepoURLTypeHelm:
		return "https://localhost:9444/argo-e2e/testdata.git/helm-repo"
	default:
		return fmt.Sprintf("file://%s", repoDirectory())
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
		"-n", ArgoCDNamespace))
	FailOnErr(Run("", "kubectl", "label", "secret", secretName, testingLabel+"=true", "-n", ArgoCDNamespace))
	return secretName
}

// Convinience wrapper for updating argocd-cm
func updateSettingConfigMap(updater func(cm *corev1.ConfigMap) error) {
	updateGenericConfigMap(common.ArgoCDConfigMapName, updater)
}

// Updates a given config map in argocd-e2e namespace
func updateGenericConfigMap(name string, updater func(cm *corev1.ConfigMap) error) {
	cm, err := KubeClientset.CoreV1().ConfigMaps(ArgoCDNamespace).Get(context.Background(), name, v1.GetOptions{})
	errors.CheckError(err)
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	errors.CheckError(updater(cm))
	_, err = KubeClientset.CoreV1().ConfigMaps(ArgoCDNamespace).Update(context.Background(), cm, v1.UpdateOptions{})
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
	proj, err := AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Get(context.Background(), project, v1.GetOptions{})
	errors.CheckError(err)
	proj.Spec = spec
	_, err = AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Update(context.Background(), proj, v1.UpdateOptions{})
	errors.CheckError(err)
}

func EnsureCleanState(t *testing.T) {

	start := time.Now()

	policy := v1.DeletePropagationBackground
	// delete resources
	// kubectl delete apps --all
	CheckError(AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).DeleteCollection(context.Background(), v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{}))
	// kubectl delete appprojects --field-selector metadata.name!=default
	CheckError(AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).DeleteCollection(context.Background(),
		v1.DeleteOptions{PropagationPolicy: &policy}, v1.ListOptions{FieldSelector: "metadata.name!=default"}))
	// kubectl delete secrets -l e2e.argoproj.io=true
	CheckError(KubeClientset.CoreV1().Secrets(ArgoCDNamespace).DeleteCollection(context.Background(),
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

	// remove tmp dir
	CheckError(os.RemoveAll(TmpDir))

	// random id - unique across test runs
	postFix := "-" + strings.ToLower(rand.RandString(5))
	id = t.Name() + postFix
	name = DnsFriendly(t.Name(), "")
	deploymentNamespace = DnsFriendly(fmt.Sprintf("argocd-e2e-%s", t.Name()), postFix)

	// create tmp dir
	FailOnErr(Run("", "mkdir", "-p", TmpDir))

	// create TLS and SSH certificate directories
	FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/tls"))
	FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/ssh"))

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
	FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/source"))
	FailOnErr(Run("", "mkdir", "-p", TmpDir+"/app/config/gpg/keys"))
	FailOnErr(Run("", "chmod", "0700", TmpDir+"/app/config/gpg/keys"))

	// set-up tmp repo, must have unique name
	FailOnErr(Run("", "cp", "-Rf", "testdata", repoDirectory()))
	FailOnErr(Run(repoDirectory(), "chmod", "777", "."))
	FailOnErr(Run(repoDirectory(), "git", "init"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-q", "-m", "initial commit"))

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
}

func Delete(path string) {

	log.WithFields(log.Fields{"path": path}).Info("deleting")

	CheckError(os.Remove(filepath.Join(repoDirectory(), path)))

	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "commit", "-am", "delete"))
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
}

func AddSignedFile(path, contents string) {
	WriteFile(path, contents)

	prevGnuPGHome := os.Getenv("GNUPGHOME")
	os.Setenv("GNUPGHOME", TmpDir+"/gpg")
	FailOnErr(Run(repoDirectory(), "git", "diff"))
	FailOnErr(Run(repoDirectory(), "git", "add", "."))
	FailOnErr(Run(repoDirectory(), "git", "-c", fmt.Sprintf("user.signingkey=%s", GpgGoodKeyID), "commit", "-S", "-am", "add file"))
	os.Setenv("GNUPGHOME", prevGnuPGHome)
}

// create the resource by creating using "kubectl apply", with bonus templating
func Declarative(filename string, values interface{}) (string, error) {

	bytes, err := ioutil.ReadFile(path.Join("testdata", filename))
	CheckError(err)

	tmpFile, err := ioutil.TempFile("", "")
	CheckError(err)
	_, err = tmpFile.WriteString(Tmpl(string(bytes), values))
	CheckError(err)

	return Run("", "kubectl", "-n", ArgoCDNamespace, "apply", "-f", tmpFile.Name())
}

func CreateSubmoduleRepos(repoType string) {

	// set-up submodule repo
	FailOnErr(Run("", "cp", "-Rf", "testdata/git-submodule/", submoduleDirectory()))
	FailOnErr(Run(submoduleDirectory(), "chmod", "777", "."))
	FailOnErr(Run(submoduleDirectory(), "git", "init"))
	FailOnErr(Run(submoduleDirectory(), "git", "add", "."))
	FailOnErr(Run(submoduleDirectory(), "git", "commit", "-q", "-m", "initial commit"))

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
}
