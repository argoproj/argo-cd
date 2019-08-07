package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
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

func TestHelmCrdInstallIsCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "crd-install"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(2))
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
		Expect(Condition(ApplicationConditionComparisonError, "open does-not-exist-values.yaml: no such file or directory"))
}

func TestHelmValues(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--values", "foo").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []string{"foo"}, app.Spec.Source.Helm.ValueFiles)
		})
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
		AppSet("--helm-set", "foo=bar", "--helm-set", "foo=baz").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz"}}, app.Spec.Source.Helm.Parameters)
		})
}

func TestHelmSetString(t *testing.T) {
	Given(t).
		Path("helm").
		When().
		Create().
		AppSet("--helm-set-string", "foo=bar", "--helm-set-string", "foo=baz").
		Then().
		And(func(app *Application) {
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "baz", ForceString: true}}, app.Spec.Source.Helm.Parameters)
		})
}
