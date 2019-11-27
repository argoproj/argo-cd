package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/errors"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
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
		Expect(HealthIs(HealthStatusHealthy)).
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
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDeclarativeHelmInvalidValuesFile(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Declarative("declarative-apps/invalid-helm.yaml").
		Then().
		Expect(HealthIs(HealthStatusHealthy)).
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
		Expect(HealthIs(HealthStatusHealthy)).
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

func TestHelmCrdHook(t *testing.T) {
	Given(t).
		Path("helm-crd").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusHealthy)).
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
			// Capabiliets.KubeVersion defaults to 1.9.0, we assume here you are running a later version
			assert.Equal(t, GetVersions().ServerVersion.Format("v%s.%s.0"), kubeVersion)
		})
}

func TestHelmWithDependencies(t *testing.T) {
	testHelmWithDependencies(t, false)
}

func TestHelmWithDependenciesLegacyRepo(t *testing.T) {
	testHelmWithDependencies(t, true)
}

func testHelmWithDependencies(t *testing.T, legacyRepo bool) {
	ctx := Given(t).
		CustomCACertAdded().
		// these are slow tests
		Timeout(30)
	if legacyRepo {
		ctx.And(func() {
			errors.FailOnErr(fixture.Run("", "kubectl", "create", "secret", "generic", "helm-repo",
				"-n", fixture.ArgoCDNamespace,
				fmt.Sprintf("--from-file=certSecret=%s", repos.CertPath),
				fmt.Sprintf("--from-file=keySecret=%s", repos.CertKeyPath),
				fmt.Sprintf("--from-literal=username=%s", GitUsername),
				fmt.Sprintf("--from-literal=password=%s", GitPassword),
			))
			errors.FailOnErr(fixture.KubeClientset.CoreV1().Secrets(fixture.ArgoCDNamespace).Patch(
				"helm-repo", types.MergePatchType, []byte(`{"metadata": { "labels": {"e2e.argoproj.io": "true"} }}`)))

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

	ctx.Path("helm-with-dependencies").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
