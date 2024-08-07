package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	projectFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/project"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestHelmHooksAreCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "pre-install"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "hook", Message: "pod/hook created", HookType: HookTypePreSync, HookPhase: OperationSucceeded, SyncPhase: SyncPhasePreSync}))
}

// make sure we treat Helm weights as a sync wave
func TestHelmHookWeight(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		// this create a weird hook, that runs during sync - but before the pod, and because it'll fail - the pod will never be created
		PatchFile("hook.yaml", `[
	{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "Sync", "helm.sh/hook-weight": "-1"}},
	{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}
]`).
		CreateApp().
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(ResourceResultNumbering(1))
}

// make sure that execute the delete policy
func TestHelmHookDeletePolicy(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/helm.sh~1hook-delete-policy", "value": "hook-succeeded"}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceResultNumbering(2)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestDeclarativeHelm(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Declarative("declarative-apps/app.yaml").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDeclarativeHelmInvalidValuesFile(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Declarative("declarative-apps/invalid-helm.yaml").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		Expect(Condition(ApplicationConditionComparisonError, "does-not-exist-values.yaml: no such file or directory"))
}

func TestHelmRepo(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelmValues(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		AddFile("foo.yml", "").
		CreateApp().
		AppSet("--values", "foo.yml").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []string{"foo.yml"}, app.Spec.GetSource().Helm.ValueFiles)
		})
}

func TestHelmIgnoreMissingValueFiles(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Declarative("declarative-apps/invalid-helm.yaml").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []string{"does-not-exist-values.yaml"}, app.Spec.GetSource().Helm.ValueFiles)
			assert.False(t, app.Spec.GetSource().Helm.IgnoreMissingValueFiles)
		}).
		When().
		AppSet("--ignore-missing-value-files").
		Then().
		And(func(app *Application) {
			assert.True(t, app.Spec.GetSource().Helm.IgnoreMissingValueFiles)
		}).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		AppUnSet("--ignore-missing-value-files").
		Then().
		And(func(app *Application) {
			assert.False(t, app.Spec.GetSource().Helm.IgnoreMissingValueFiles)
		}).
		When().
		IgnoreErrors().
		Sync().
		Then().
		Expect(ErrorRegex("Error: open .*does-not-exist-values.yaml: no such file or directory", ""))
}

func TestHelmValuesMultipleUnset(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		AddFile("foo.yml", "").
		AddFile("baz.yml", "").
		CreateApp().
		AppSet("--values", "foo.yml", "--values", "baz.yml").
		Then().
		And(func(app *Application) {
			assert.NotNil(t, app.Spec.GetSource().Helm)
			assert.Equal(t, []string{"foo.yml", "baz.yml"}, app.Spec.GetSource().Helm.ValueFiles)
		}).
		When().
		AppUnSet("--values", "foo.yml").
		Then().
		And(func(app *Application) {
			assert.NotNil(t, app.Spec.GetSource().Helm)
			assert.Equal(t, []string{"baz.yml"}, app.Spec.GetSource().Helm.ValueFiles)
		}).
		When().
		AppUnSet("--values", "baz.yml").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.GetSource().Helm)
		})
}

func TestHelmValuesLiteralFileLocal(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--values-literal-file", "testdata/helm/baz.yaml").
		Then().
		And(func(app *Application) {
			data, err := os.ReadFile("testdata/helm/baz.yaml")
			if err != nil {
				panic(err)
			}
			assert.Equal(t, strings.TrimSuffix(string(data), "\n"), app.Spec.GetSource().Helm.ValuesString())
		}).
		When().
		AppUnSet("--values-literal").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.GetSource().Helm)
		})
}

func TestHelmValuesLiteralFileRemote(t *testing.T) {
	sentinel := "a: b"
	serve := func(c chan<- string) {
		// listen on first available dynamic (unprivileged) port
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			panic(err)
		}

		// send back the address so that it can be used
		c <- listener.Addr().String()
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// return the sentinel text at root URL
			fmt.Fprint(w, sentinel)
		})

		panic(http.Serve(listener, nil))
	}
	c := make(chan string, 1)

	// run a local webserver to test data retrieval
	go serve(c)
	address := <-c
	t.Logf("Listening at address: %s", address)

	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--values-literal-file", "http://"+address).
		Then().
		And(func(app *Application) {
			assert.Equal(t, "a: b", app.Spec.GetSource().Helm.ValuesString())
		}).
		When().
		AppUnSet("--values-literal").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.GetSource().Helm)
		})
}

func TestHelmCrdHook(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path("helm-crd").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(2))
}

func TestHelmReleaseName(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--release-name", "foo").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "foo", app.Spec.GetSource().Helm.ReleaseName)
		})
}

func TestHelmSet(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--helm-set", "foo=bar", "--helm-set", "foo=baz", "--helm-set", "app=$ARGOCD_APP_NAME").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz"}, {Name: "app", Value: "$ARGOCD_APP_NAME"}}, app.Spec.GetSource().Helm.Parameters)
		})
}

func TestHelmSetString(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--helm-set-string", "foo=bar", "--helm-set-string", "foo=baz", "--helm-set-string", "app=$ARGOCD_APP_NAME").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz", ForceString: true}, {Name: "app", Value: "$ARGOCD_APP_NAME", ForceString: true}}, app.Spec.GetSource().Helm.Parameters)
		})
}

func TestHelmSetFile(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		CreateApp().
		AppSet("--helm-set-file", "foo=bar.yaml", "--helm-set-file", "foo=baz.yaml").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmFileParameter{{Name: "foo", Path: "baz.yaml"}}, app.Spec.GetSource().Helm.FileParameters)
		})
}

// ensure we can use envsubst in "set" variables
func TestHelmSetEnv(t *testing.T) {
	Given(t).
		Path("helm-values").
		When().
		CreateApp().
		AppSet("--helm-set", "foo=$ARGOCD_APP_NAME").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
		})
}

func TestHelmSetStringEnv(t *testing.T) {
	Given(t).
		Path("helm-values").
		When().
		CreateApp().
		AppSet("--helm-set-string", "foo=$ARGOCD_APP_NAME").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, Name(), FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map", "-o", "jsonpath={.data.foo}")).(string))
		})
}

// make sure kube-version gets passed down to resources
func TestKubeVersion(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path("helm-kube-version").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			kubeVersion := FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map",
				"-o", "jsonpath={.data.kubeVersion}")).(string)
			// Capabilities.KubeVersion defaults to 1.9.0, we assume here you are running a later version
			assert.LessOrEqual(t, GetVersions().ServerVersion.Format("v%s.%s.0"), kubeVersion)
		}).
		When().
		// Make sure override works.
		AppSet("--helm-kube-version", "999.999.999").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "v999.999.999", FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map",
				"-o", "jsonpath={.data.kubeVersion}")).(string))
		})
}

// make sure api versions gets passed down to resources
func TestApiVersions(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path("helm-api-versions").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			apiVersions := FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map",
				"-o", "jsonpath={.data.apiVersions}")).(string)
			// The v1 API shouldn't be going anywhere.
			assert.Contains(t, apiVersions, "v1")
		}).
		When().
		// Make sure override works.
		AppSet("--helm-api-versions", "v1/MyTestResource").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			apiVersions := FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map",
				"-o", "jsonpath={.data.apiVersions}")).(string)
			assert.Contains(t, apiVersions, "v1/MyTestResource")
		})
}

func TestHelmNamespaceOverride(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path("helm-namespace").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		AppSet("--helm-namespace", "does-not-exist").
		Then().
		// The app should go out of sync, because the resource's target namespace changed.
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestHelmValuesHiddenDirectory(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path(".hidden-helm").
		When().
		AddFile("foo.yaml", "").
		CreateApp().
		AppSet("--values", "foo.yaml").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelmWithDependencies(t *testing.T) {
	SkipOnEnv(t, "HELM")
	testHelmWithDependencies(t, "helm-with-dependencies", false)
}

func TestHelmWithMultipleDependencies(t *testing.T) {
	SkipOnEnv(t, "HELM")

	Given(t).Path("helm-with-multiple-dependencies").
		CustomCACertAdded().
		// these are slow tests
		Timeout(30).
		HelmHTTPSCredentialsUserPassAdded().
		HelmPassCredentials().
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelmDependenciesPermissionDenied(t *testing.T) {
	SkipOnEnv(t, "HELM")

	projName := "argo-helm-project-denied"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("*,*").
		When().
		Create().
		AddSource(RepoURL(RepoURLTypeFile))

	expectedErr := fmt.Sprintf("helm repos localhost:5000/myrepo are not permitted in project '%s'", projName)
	GivenWithSameState(t).
		Project(projName).
		Path("helm-oci-with-dependencies").
		CustomCACertAdded().
		HelmHTTPSCredentialsUserPassAdded().
		HelmPassCredentials().
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", expectedErr))

	expectedErr = fmt.Sprintf("helm repos https://localhost:9443/argo-e2e/testdata.git/helm-repo/local, https://localhost:9443/argo-e2e/testdata.git/helm-repo/local2 are not permitted in project '%s'", projName)
	GivenWithSameState(t).
		Project(projName).
		Path("helm-with-multiple-dependencies-permission-denied").
		CustomCACertAdded().
		HelmHTTPSCredentialsUserPassAdded().
		HelmPassCredentials().
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", expectedErr))
}

func TestHelmWithDependenciesLegacyRepo(t *testing.T) {
	SkipOnEnv(t, "HELM")
	testHelmWithDependencies(t, "helm-with-dependencies", true)
}

func testHelmWithDependencies(t *testing.T, chartPath string, legacyRepo bool) {
	ctx := Given(t).
		CustomCACertAdded().
		// these are slow tests
		Timeout(30).
		HelmPassCredentials()
	if legacyRepo {
		ctx.And(func() {
			FailOnErr(fixture.Run("", "kubectl", "create", "secret", "generic", "helm-repo",
				"-n", fixture.TestNamespace(),
				fmt.Sprintf("--from-file=certSecret=%s", repos.CertPath),
				fmt.Sprintf("--from-file=keySecret=%s", repos.CertKeyPath),
				fmt.Sprintf("--from-literal=username=%s", GitUsername),
				fmt.Sprintf("--from-literal=password=%s", GitPassword),
			))
			FailOnErr(fixture.KubeClientset.CoreV1().Secrets(fixture.TestNamespace()).Patch(context.Background(),
				"helm-repo", types.MergePatchType, []byte(`{"metadata": { "labels": {"e2e.argoproj.io": "true"} }}`), metav1.PatchOptions{}))

			fixture.SetHelmRepos(settings.HelmRepoCredentials{
				URL:            RepoURL(RepoURLTypeHelm),
				Name:           "custom-repo",
				KeySecret:      &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "helm-repo"}, Key: "keySecret"},
				CertSecret:     &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "helm-repo"}, Key: "certSecret"},
				UsernameSecret: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "helm-repo"}, Key: "username"},
				PasswordSecret: &v1.SecretKeySelector{LocalObjectReference: v1.LocalObjectReference{Name: "helm-repo"}, Key: "password"},
			})
		})
	} else {
		ctx = ctx.HelmRepoAdded("custom-repo")
	}

	helmVer := ""

	ctx.Path(chartPath).
		When().
		CreateApp("--helm-version", helmVer).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelm3CRD(t *testing.T) {
	SkipOnEnv(t, "HELM")
	Given(t).
		Path("helm3-crd").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("CustomResourceDefinition", "crontabs.stable.example.com", SyncStatusCodeSynced))
}

func TestHelmRepoDiffLocal(t *testing.T) {
	SkipOnEnv(t, "HELM")
	helmTmp := t.TempDir()
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_ = os.Setenv("XDG_CONFIG_HOME", helmTmp)
			FailOnErr(Run("", "helm", "repo", "add", "custom-repo", GetEnvWithDefault("ARGOCD_E2E_HELM_SERVICE", RepoURL(RepoURLTypeHelm)),
				"--username", GitUsername,
				"--password", GitPassword,
				"--cert-file", "../fixture/certs/argocd-test-client.crt",
				"--key-file", "../fixture/certs/argocd-test-client.key",
				"--ca-file", "../fixture/certs/argocd-test-ca.crt",
			))
			diffOutput := FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/helm")).(string)
			assert.Empty(t, diffOutput)
		})
}

func TestHelmOCIRegistry(t *testing.T) {
	Given(t).
		PushChartToOCIRegistry("helm-values", "helm-values", "1.0.0").
		HelmOCIRepoAdded("myrepo").
		RepoURLType(RepoURLTypeHelmOCI).
		Chart("helm-values").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestGitWithHelmOCIRegistryDependencies(t *testing.T) {
	Given(t).
		PushChartToOCIRegistry("helm-values", "helm-values", "1.0.0").
		HelmOCIRepoAdded("myrepo").
		Path("helm-oci-with-dependencies").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelmOCIRegistryWithDependencies(t *testing.T) {
	Given(t).
		PushChartToOCIRegistry("helm-values", "helm-values", "1.0.0").
		PushChartToOCIRegistry("helm-oci-with-dependencies", "helm-oci-with-dependencies", "1.0.0").
		HelmOCIRepoAdded("myrepo").
		RepoURLType(RepoURLTypeHelmOCI).
		Chart("helm-oci-with-dependencies").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestTemplatesGitWithHelmOCIDependencies(t *testing.T) {
	Given(t).
		PushChartToOCIRegistry("helm-values", "helm-values", "1.0.0").
		HelmoOCICredentialsWithoutUserPassAdded().
		Path("helm-oci-with-dependencies").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestTemplatesHelmOCIWithDependencies(t *testing.T) {
	Given(t).
		PushChartToOCIRegistry("helm-values", "helm-values", "1.0.0").
		PushChartToOCIRegistry("helm-oci-with-dependencies", "helm-oci-with-dependencies", "1.0.0").
		HelmoOCICredentialsWithoutUserPassAdded().
		RepoURLType(RepoURLTypeHelmOCI).
		Chart("helm-oci-with-dependencies").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
