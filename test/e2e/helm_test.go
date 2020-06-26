package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
	. "github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/settings"
)

func TestHelmHooksAreCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "pre-install"}}]`).
		Create().
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
		Create().
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
		Create().
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
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		Create().
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
		Create().
		AppSet("--values", "foo.yml").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []string{"foo.yml"}, app.Spec.Source.Helm.ValueFiles)
		})
}

func TestHelmValuesMultipleUnset(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		AddFile("foo.yml", "").
		AddFile("baz.yml", "").
		Create().
		AppSet("--values", "foo.yml", "--values", "baz.yml").
		Then().
		And(func(app *Application) {
			assert.NotNil(t, app.Spec.Source.Helm)
			assert.Equal(t, []string{"foo.yml", "baz.yml"}, app.Spec.Source.Helm.ValueFiles)
		}).
		When().
		AppUnSet("--values", "foo.yml").
		Then().
		And(func(app *Application) {
			assert.NotNil(t, app.Spec.Source.Helm)
			assert.Equal(t, []string{"baz.yml"}, app.Spec.Source.Helm.ValueFiles)
		}).
		When().
		AppUnSet("--values", "baz.yml").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.Source.Helm)
		})
}

func TestHelmValuesLiteralFileLocal(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--values-literal-file", "testdata/helm/baz.yaml").
		Then().
		And(func(app *Application) {
			data, err := ioutil.ReadFile("testdata/helm/baz.yaml")
			if err != nil {
				panic(err)
			}
			assert.Equal(t, string(data), app.Spec.Source.Helm.Values)
		}).
		When().
		AppUnSet("--values-literal").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.Source.Helm)
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
		Create().
		AppSet("--values-literal-file", "http://"+address).
		Then().
		And(func(app *Application) {
			assert.Equal(t, "a: b", app.Spec.Source.Helm.Values)
		}).
		When().
		AppUnSet("--values-literal").
		Then().
		And(func(app *Application) {
			assert.Nil(t, app.Spec.Source.Helm)
		})
}

func TestHelmCrdHook(t *testing.T) {
	Given(t).
		Path("helm-crd").
		When().
		Create().
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
		Create().
		AppSet("--release-name", "foo").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "foo", app.Spec.Source.Helm.ReleaseName)
		})
}

func TestHelmSet(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--helm-set", "foo=bar", "--helm-set", "foo=baz", "--helm-set", "app=$ARGOCD_APP_NAME").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz"}, {Name: "app", Value: "$ARGOCD_APP_NAME"}}, app.Spec.Source.Helm.Parameters)
		})
}

func TestHelmSetString(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--helm-set-string", "foo=bar", "--helm-set-string", "foo=baz", "--helm-set-string", "app=$ARGOCD_APP_NAME").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz", ForceString: true}, {Name: "app", Value: "$ARGOCD_APP_NAME", ForceString: true}}, app.Spec.Source.Helm.Parameters)
		})
}

func TestHelmSetFile(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--helm-set-file", "foo=bar.yaml", "--helm-set-file", "foo=baz.yaml").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmFileParameter{{Name: "foo", Path: "baz.yaml"}}, app.Spec.Source.Helm.FileParameters)
		})
}

// ensure we can use envsubst in "set" variables
func TestHelmSetEnv(t *testing.T) {
	Given(t).
		Path("helm-values").
		When().
		Create().
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
		Create().
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
	Given(t).
		Path("helm-kube-version").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			kubeVersion := FailOnErr(Run(".", "kubectl", "-n", DeploymentNamespace(), "get", "cm", "my-map",
				"-o", "jsonpath={.data.kubeVersion}")).(string)
			// Capabilities.KubeVersion defaults to 1.9.0, we assume here you are running a later version
			assert.Equal(t, GetVersions().ServerVersion.Format("v%s.%s.0"), kubeVersion)
		})
}

func TestHelmValuesHiddenDirectory(t *testing.T) {
	Given(t).
		Path(".hidden-helm").
		When().
		AddFile("foo.yaml", "").
		Create().
		AppSet("--values", "foo.yaml").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelmWithDependencies(t *testing.T) {
	testHelmWithDependencies(t, "helm-with-dependencies", false)
}

func TestHelm2WithDependencies(t *testing.T) {
	testHelmWithDependencies(t, "helm2-with-dependencies", false)
}

func TestHelmWithDependenciesLegacyRepo(t *testing.T) {
	testHelmWithDependencies(t, "helm2-with-dependencies", false)
}

func testHelmWithDependencies(t *testing.T, chartPath string, legacyRepo bool) {
	ctx := Given(t).
		CustomCACertAdded().
		// these are slow tests
		Timeout(30)
	if legacyRepo {
		ctx.And(func() {
			FailOnErr(fixture.Run("", "kubectl", "create", "secret", "generic", "helm-repo",
				"-n", fixture.ArgoCDNamespace,
				fmt.Sprintf("--from-file=certSecret=%s", repos.CertPath),
				fmt.Sprintf("--from-file=keySecret=%s", repos.CertKeyPath),
				fmt.Sprintf("--from-literal=username=%s", GitUsername),
				fmt.Sprintf("--from-literal=password=%s", GitPassword),
			))
			FailOnErr(fixture.KubeClientset.CoreV1().Secrets(fixture.ArgoCDNamespace).Patch(context.Background(),
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

	ctx.Path(chartPath).
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestHelm3CRD(t *testing.T) {
	Given(t).
		Path("helm3-crd").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("CustomResourceDefinition", "crontabs.stable.example.com", SyncStatusCodeSynced))
}

func TestHelmRepoDiffLocal(t *testing.T) {
	helmTmp, err := ioutil.TempDir("", "argocd-helm-repo-diff-local-test")
	assert.NoError(t, err)
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		Create().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_ = os.Setenv("XDG_CONFIG_HOME", helmTmp)
			FailOnErr(Run("", "helm", "repo", "add", "custom-repo", RepoURL(RepoURLTypeHelm),
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
