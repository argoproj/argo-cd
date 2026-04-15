package e2e

import (
	"fmt"
	"path/filepath"
	"regexp"
	"testing"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/test/e2e/testdata"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
)

// Values of `.data` & `.stringData“ fields in Secret resources are masked in UI/CLI
// Optionally, values of `.metadata.annotations` can also be masked, if needed.
func TestMaskSecretValues(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|NEWSECRETVAL|U0VDUkVUVkFM`)

	ctx := Given(t)
	ctx.Path("empty-dir").
		When().
		SetParamInSettingConfigMap("resource.sensitive.mask.annotations", "token"). // hide sensitive annotation
		AddFile("secrets.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: secret
  annotations:
    token: SECRETVAL
    app: test
stringData:
  username: SECRETVAL
data:
  password: U0VDUkVUVkFM
`).
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		// sensitive data should be masked in manifests output
		And(func(app *Application) {
			mnfs, _ := fixture.RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))
		}).
		When().
		PatchFile("secrets.yaml", `[{"op": "replace", "path": "/stringData/username", "value": "NEWSECRETVAL"}]`).
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"token": "NEWSECRETVAL"}}]`).
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"something": "else"}}]`).
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			localRepoRoot := fixture.LocalRepoRoot()
			appPath := filepath.Join(localRepoRoot, app.Spec.Source.Path)

			// Normal diff should show a diff with the sensitive value masked
			diff, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--exit-code=false")
			require.NoError(t, err)
			assert.False(t, sensitiveData.MatchString(diff))
			assert.Contains(t, diff, "===== /Secret")

			// Revision specific diff should show no differences
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--revision", app.Status.Sync.Revision, "--exit-code=false")
			require.NoError(t, err)
			assert.False(t, sensitiveData.MatchString(diff))
			assert.Contains(t, diff, "===== /Secret")

			// Server-Side diff should show a diff with the sensitive value masked
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, diff, "===== /Secret")

			// Local diff with server-side-generate should show a diff with the sensitive value masked
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", localRepoRoot, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, diff, "===== /Secret")

			// Local diff should exclude secret resources completely
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", appPath, "--local-repo-root", localRepoRoot, "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff, "Secret kind should not be displayed in CLI diff output")
		})
}

// Secret values shouldn't be exposed in error messages and the diff view
// when invalid secret is synced.
func TestMaskValuesInInvalidSecret(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|U0VDUkVUVkFM|12345`)

	ctx := Given(t)
	ctx.Path("empty-dir").
		When().
		// valid secret
		AddFile("secrets.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: secret
  annotations:
    app: test
stringData:
  username: SECRETVAL
data:
  password: U0VDUkVUVkFM
`).
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		// invalidate secret
		PatchFile("secrets.yaml", `[{"op": "replace", "path": "/data/password", "value": 12345}]`).
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(OperationPhaseIs(synccommon.OperationFailed)).
		// secret data shouldn't be exposed in manifests, diff & error output for invalid secret
		And(func(app *Application) {
			mnfs, _ := fixture.RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))
			localRepoRoot := fixture.LocalRepoRoot()
			appPath := filepath.Join(localRepoRoot, app.Spec.Source.Path)

			// Normal diff should show a diff with the sensitive value masked
			diff, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--exit-code=false")
			require.NoError(t, err)
			assert.False(t, sensitiveData.MatchString(diff))
			assert.Contains(t, diff, "===== /Secret")

			// Revision specific diff should show no differences
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--revision", app.Status.Sync.Revision, "--exit-code=false")
			require.NoError(t, err)
			assert.False(t, sensitiveData.MatchString(diff))
			assert.Contains(t, diff, "===== /Secret")

			// Server-Side diff should show a diff with the sensitive value masked
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, diff, "===== /Secret")

			// Local diff with server-side-generate should show a diff with the sensitive value masked
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", localRepoRoot, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, diff, "===== /Secret")

			// Local diff should exclude secret resources completely
			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", appPath, "--local-repo-root", localRepoRoot, "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff, "Secret kind should not be displayed in CLI diff output")

			msg := app.Status.OperationState.Message
			assert.False(t, sensitiveData.MatchString(msg))
		})
}

// make sure that hooks do not appear in "argocd app diff"
func TestHookDiff(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path("hook").
		When().
		CreateApp().
		Then().
		And(func(_ *Application) {
			output, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, output, "name: pod")
			assert.NotContains(t, output, "name: hook")

			output, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, output, "name: pod")
			assert.NotContains(t, output, "name: hook")
		})
}

func TestDiff_DuplicatedResourcesAndNamespaceNormalization(t *testing.T) {
	testDiffResultsAreEmptyWithoutChanges(t, "duplicated-resources", fixture.TestNamespace())
}

func TestDiff_CRDNoAnnotationTracking(t *testing.T) {
	testDiffResultsAreEmptyWithoutChanges(t, "crd-creation", fixture.TestNamespace())
}

func TestDiff_AppNamespace_DuplicatedResourcesAndNamespaceNormalization(t *testing.T) {
	testDiffResultsAreEmptyWithoutChanges(t, "duplicated-resources", fixture.AppNamespace())
}

func TestDiff_AppNamespace_CRDNoAnnotationTracking(t *testing.T) {
	testDiffResultsAreEmptyWithoutChanges(t, "crd-creation", fixture.AppNamespace())
}

func testDiffResultsAreEmptyWithoutChanges(t *testing.T, appPath string, appNamespace string) {
	t.Helper()
	ctx := Given(t)
	ctx.SetTrackingMethod(string(TrackingMethodAnnotation)).
		Path(appPath).
		SetAppNamespace(appNamespace).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			localRepoRoot := fixture.LocalRepoRoot()
			appPath := filepath.Join(localRepoRoot, app.Spec.Source.Path)

			diff, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff)

			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--revision", app.Status.Sync.Revision, "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff)

			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff)

			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", localRepoRoot, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff)

			diff, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", appPath, "--local-repo-root", localRepoRoot, "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diff)
		})
}

func TestHelmRepoDiffLocal(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	helmTmp := t.TempDir()
	ctx := Given(t)
	ctx.CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			appPath := "testdata/helm"

			t.Setenv("XDG_CONFIG_HOME", helmTmp)
			errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "repo", "add", "custom-repo", fixture.GetEnvWithDefault("ARGOCD_E2E_HELM_SERVICE", fixture.RepoURL(fixture.RepoURLTypeHelm)),
				"--username", fixture.GitUsername,
				"--password", fixture.GitPassword,
				"--cert-file", "../fixture/certs/argocd-test-client.crt",
				"--key-file", "../fixture/certs/argocd-test-client.key",
				"--ca-file", "../fixture/certs/argocd-test-ca.crt",
			))
			diffOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", appPath, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)

			diffOutput, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--local", appPath, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)
		})
}

// make sure we can sync and diff with --local
func TestCustomToolSyncAndDiffLocal(t *testing.T) {
	testdataPath := fixture.LocalRepoRoot()
	appPath := filepath.Join(testdataPath, "config-map")
	ctx := Given(t)
	ctx.
		RunningCMPServer("./testdata/cmp-kustomize").
		// does not matter what the path is
		Path("config-map").
		When().
		CreateApp("--config-management-plugin", "cmp-kustomize-v1.0").
		Sync("--local", appPath, "--local-repo-root", testdataPath).
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			_, err := fixture.RunCli("app", "sync", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath)
			require.NoError(t, err)
		}).
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", appPath, "--local-repo-root", testdataPath, "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)

			diffOutput, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", testdataPath, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)

			diffOutput, err = fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--local", testdataPath, "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)
		})
}

// TestServerSideDiffWithLocal tests server-side diff with --local flag
func TestServerSideDiffWithLocal(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Modify the live deployment in the cluster to create differences
			// Apply patches to the deployment
			_, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.JSONPatchType, []byte(`[
					{"op": "add", "path": "/spec/template/spec/containers/0/env", "value": [{"name": "LOCAL_CHANGE", "value": "true"}]},
					{"op": "replace", "path": "/spec/replicas", "value": 2}
				]`), metav1.PatchOptions{})
			require.NoError(t, err)

			// Verify the patch was applied by reading back the deployment
			modifiedDeployment, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(2), *modifiedDeployment.Spec.Replicas, "Replica count should be updated to 2")
			assert.Len(t, modifiedDeployment.Spec.Template.Spec.Containers[0].Env, 1, "Should have one environment variable")
			assert.Equal(t, "LOCAL_CHANGE", modifiedDeployment.Spec.Template.Spec.Containers[0].Env[0].Name)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(_ *Application) {
			// Test regular diff with --local (add --server-side-generate to avoid deprecation warning)
			regularOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata", "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, regularOutput, "===== apps/Deployment")
			assert.Contains(t, regularOutput, "guestbook-ui")
			assert.Contains(t, regularOutput, "replicas:")

			// Test server-side diff with --local (add --server-side-generate for consistency)
			serverSideOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--local", "testdata", "--server-side-generate", "--exit-code=false")
			require.NoError(t, err)
			assert.Contains(t, serverSideOutput, "===== apps/Deployment")
			assert.Contains(t, serverSideOutput, "guestbook-ui")
			assert.Contains(t, serverSideOutput, "replicas:")

			// Both outputs should show similar differences
			assert.Contains(t, regularOutput, "replicas: 2")
			assert.Contains(t, serverSideOutput, "replicas: 2")
		})
}

func TestServerSideDiffWithLocalValidation(t *testing.T) {
	ctx := Given(t)
	ctx.Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Test that --server-side-diff with --local without --server-side-generate fails with proper error
			_, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--server-side-diff", "--local", "testdata")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--server-side-diff with --local requires --server-side-generate")
		})
}

//

func assetSecretDataHidden(t *testing.T, manifest string) {
	t.Helper()
	secret, err := UnmarshalToUnstructured(manifest)
	require.NoError(t, err)

	_, hasStringData, err := unstructured.NestedMap(secret.Object, "stringData")
	require.NoError(t, err)
	assert.False(t, hasStringData)

	secretData, hasData, err := unstructured.NestedMap(secret.Object, "data")
	require.NoError(t, err)
	assert.True(t, hasData)
	for _, v := range secretData {
		assert.Regexp(t, `[*]*`, v)
	}
	var lastAppliedConfigAnnotation string
	annotations := secret.GetAnnotations()
	if annotations != nil {
		lastAppliedConfigAnnotation = annotations[corev1.LastAppliedConfigAnnotation]
	}
	if lastAppliedConfigAnnotation != "" {
		assetSecretDataHidden(t, lastAppliedConfigAnnotation)
	}
}

func TestAppWithSecrets(t *testing.T) {
	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	require.NoError(t, err)
	defer utilio.Close(closer)

	ctx := Given(t)
	ctx.Path("secrets").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res := errors.NewHandler(t).FailOnErr(client.GetResource(t.Context(), &applicationpkg.ApplicationResourceRequest{
				Namespace:    &app.Spec.Destination.Namespace,
				Kind:         new(kube.SecretKind),
				Group:        new(""),
				Name:         &app.Name,
				Version:      new("v1"),
				ResourceName: new("test-secret"),
			})).(*applicationpkg.ApplicationResourceResponse)
			assetSecretDataHidden(t, res.GetManifest())

			manifests, err := client.GetManifests(t.Context(), &applicationpkg.ApplicationManifestQuery{Name: &app.Name})
			require.NoError(t, err)

			for _, manifest := range manifests.Manifests {
				assetSecretDataHidden(t, manifest)
			}

			diffOutput := errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "diff", ctx.AppQualifiedName())).(string)
			assert.Empty(t, diffOutput)

			// make sure resource update error does not print secret details
			_, err = fixture.RunCli("app", "patch-resource", app.GetName(), "--resource-name", "test-secret",
				"--kind", "Secret", "--patch", `{"op": "add", "path": "/data", "value": "hello"}'`,
				"--patch-type", "application/json-patch+json")
			require.ErrorContains(t, err, fmt.Sprintf("failed to patch Secret %s/test-secret", ctx.DeploymentNamespace()))
			assert.NotContains(t, err.Error(), "username")
			assert.NotContains(t, err.Error(), "password")

			// patch secret and make sure app is out of sync and diff detects the change
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.CoreV1().Secrets(ctx.DeploymentNamespace()).Patch(t.Context(),
				"test-secret", types.JSONPatchType, []byte(`[
	{"op": "remove", "path": "/data/username"},
	{"op": "add", "path": "/stringData", "value": {"password": "foo"}}
]`), metav1.PatchOptions{}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName())
			require.Error(t, err)
			assert.Contains(t, diffOutput, "username: ++++++++")
			assert.Contains(t, diffOutput, "password: ++++++++++++")

			// ignore missing field and make sure diff shows no difference
			app.Spec.IgnoreDifferences = []ResourceIgnoreDifferences{{
				Kind: kube.SecretKind, JSONPointers: []string{"/data"},
			}}
			errors.NewHandler(t).FailOnErr(client.UpdateSpec(t.Context(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: &app.Spec}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			diffOutput := errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "diff", ctx.AppQualifiedName())).(string)
			assert.Empty(t, diffOutput)
		}).
		// verify not committed secret also ignore during diffing
		When().
		WriteFile("secret3.yaml", `
apiVersion: v1
kind: Secret
metadata:
  name: test-secret3
stringData:
  username: test-username`).
		Then().
		And(func(_ *Application) {
			diffOutput := errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata", "--server-side-generate")).(string)
			assert.Empty(t, diffOutput)
		})
}

func TestResourceDiffing(t *testing.T) {
	ctx := Given(t)
	ctx.Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// Patch deployment
			_, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`), metav1.PatchOptions{})
			require.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(_ *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata", "--server-side-generate")
			require.Error(t, err)
			assert.Contains(t, diffOutput, fmt.Sprintf("===== apps/Deployment %s/guestbook-ui ======", ctx.DeploymentNamespace()))
		}).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {
			IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers/0/image"}},
		}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata", "--server-side-generate")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)
		}).
		Given().
		When().
		// Now we migrate from client-side apply to server-side apply
		// This is necessary, as starting with kubectl 1.26, all previously
		// client-side owned fields have ownership migrated to the manager from
		// the first ssa.
		// More details: https://github.com/kubernetes/kubectl/issues/1337
		PatchApp(`[{
			"op": "add",
			"path": "/spec/syncPolicy",
			"value": { "syncOptions": ["ServerSideApply=true"] }
			}]`).
		Sync().
		And(func() {
			output, err := fixture.RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", ctx.DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
			require.NoError(t, err)
			assert.Contains(t, output, "serverside-applied")
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {
			IgnoreDifferences: OverrideIgnoreDiff{
				ManagedFieldsManagers: []string{"revision-history-manager"},
				JSONPointers:          []string{"/spec/template/spec/containers/0/image"},
			},
		}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given().
		When().
		Sync().
		PatchApp(`[{
			"op": "add",
			"path": "/spec/syncPolicy",
			"value": { "syncOptions": ["RespectIgnoreDifferences=true"] }
			}]`).
		And(func() {
			deployment, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(3), *deployment.Spec.RevisionHistoryLimit)
		}).
		And(func() {
			output, err := fixture.RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", ctx.DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
			require.NoError(t, err)
			assert.Contains(t, output, "serverside-applied")
		}).
		Then().
		When().Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			deployment, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		}).
		When().Sync().Then().Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			deployment, err := fixture.KubeClientset.AppsV1().Deployments(ctx.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		})
}

func TestKnownTypesInCRDDiffing(t *testing.T) {
	dummiesGVR := schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: "dummies"}

	ctx := Given(t)
	ctx.Path("crd-creation").
		When().CreateApp().Sync().Then().
		Expect(OperationPhaseIs(synccommon.OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		And(func() {
			dummyResIf := fixture.DynamicClientset.Resource(dummiesGVR).Namespace(ctx.DeploymentNamespace())
			patchData := []byte(`{"spec":{"cpu": "2"}}`)
			errors.NewHandler(t).FailOnErr(dummyResIf.Patch(t.Context(), "dummy-crd-instance", types.MergePatchType, patchData, metav1.PatchOptions{}))
		}).Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		And(func() {
			require.NoError(t, fixture.SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/Dummy": {
					KnownTypeFields: []KnownTypeField{{
						Field: "spec",
						Type:  "core/v1/ResourceList",
					}},
				},
			}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
