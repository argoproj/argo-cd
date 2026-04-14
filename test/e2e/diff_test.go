package e2e

import (
	"path/filepath"
	"testing"

	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"

	"regexp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
)

// Values of `.data` & `.stringData“ fields in Secret resources are masked in UI/CLI
// Optionally, values of `.metadata.annotations` can also be masked, if needed.
func TestMaskSecretValues(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|NEWSECRETVAL|U0VDUkVUVkFM`)

	Given(t).
		Path("empty-dir").
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
			mnfs, _ := RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))
		}).
		When().
		PatchFile("secrets.yaml", `[{"op": "replace", "path": "/stringData/username", "value": "NEWSECRETVAL"}]`).
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"token": "NEWSECRETVAL"}}]`).
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		// sensitive data should be masked in diff output
		And(func(app *Application) {
			diff, _ := RunCli("app", "diff", app.Name)
			assert.False(t, sensitiveData.MatchString(diff))
		})
}

// Secret values shouldn't be exposed in error messages and the diff view
// when invalid secret is synced.
func TestMaskValuesInInvalidSecret(t *testing.T) {
	sensitiveData := regexp.MustCompile(`SECRETVAL|U0VDUkVUVkFM|12345`)

	Given(t).
		Path("empty-dir").
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
		Expect(OperationPhaseIs(common.OperationFailed)).
		// secret data shouldn't be exposed in manifests, diff & error output for invalid secret
		And(func(app *Application) {
			mnfs, _ := RunCli("app", "manifests", app.Name)
			assert.False(t, sensitiveData.MatchString(mnfs))

			diff, _ := RunCli("app", "diff", app.Name)
			assert.False(t, sensitiveData.MatchString(diff))

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
			output, err := RunCli("app", "diff", ctx.GetName())
			require.Error(t, err)
			assert.Contains(t, output, "name: pod")
			assert.NotContains(t, output, "name: hook")
		})
}

func TestDuplicatedClusterResourcesAnnotationTracking(t *testing.T) {
	// This test will fail if the controller fails to fix the tracking annotation for malformed cluster resources
	// (i.e. resources where metadata.namespace is set). Before the bugfix, this test would fail with a diff in the
	// tracking annotation.
	Given(t).
		SetTrackingMethod(string(TrackingMethodAnnotation)).
		Path("duplicated-resources").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata", "--server-side-generate")
			assert.Empty(t, diffOutput)
			require.NoError(t, err)
		})
}

func TestDuplicatedResources(t *testing.T) {
	testEdgeCasesApplicationResources(t, "duplicated-resources", health.HealthStatusHealthy)
}

func TestConfigMap(t *testing.T) {
	testEdgeCasesApplicationResources(t, "config-map", health.HealthStatusHealthy, "my-map  Synced                configmap/my-map created")
}

func testEdgeCasesApplicationResources(t *testing.T, appPath string, statusCode health.HealthStatusCode, message ...string) {
	t.Helper()
	expect := Given(t).
		Path(appPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
	for i := range message {
		expect = expect.Expect(Success(message[i]))
	}
	expect.
		Expect(HealthIs(statusCode)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata", "--server-side-generate")
			assert.Empty(t, diffOutput)
			require.NoError(t, err)
		})
}

func TestHelmRepoDiffLocal(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	helmTmp := t.TempDir()
	Given(t).
		CustomCACertAdded().
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
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			t.Setenv("XDG_CONFIG_HOME", helmTmp)
			errors.NewHandler(t).FailOnErr(fixture.Run("", "helm", "repo", "add", "custom-repo", fixture.GetEnvWithDefault("ARGOCD_E2E_HELM_SERVICE", fixture.RepoURL(fixture.RepoURLTypeHelm)),
				"--username", fixture.GitUsername,
				"--password", fixture.GitPassword,
				"--cert-file", "../fixture/certs/argocd-test-client.crt",
				"--key-file", "../fixture/certs/argocd-test-client.key",
				"--ca-file", "../fixture/certs/argocd-test-ca.crt",
			))
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata/helm")
			assert.Empty(t, diffOutput)
			assert.NoError(t, err)
		})
}

// make sure we can sync and diff with --local
func TestCustomToolSyncAndDiffLocal(t *testing.T) {
	testdataPath, err := filepath.Abs("testdata")
	require.NoError(t, err)
	ctx := Given(t)
	appPath := filepath.Join(testdataPath, "guestbook")
	ctx.
		RunningCMPServer("./testdata/cmp-kustomize").
		// does not matter what the path is
		Path("guestbook").
		When().
		CreateApp("--config-management-plugin", "cmp-kustomize-v1.0").
		Sync("--local", appPath, "--local-repo-root", testdataPath).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "sync", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		}).
		And(func(_ *Application) {
			errors.NewHandler(t).FailOnErr(fixture.RunCli("app", "diff", ctx.AppName(), "--local", appPath, "--local-repo-root", testdataPath))
		})
}

func TestClusterScopedResourceDiff(t *testing.T) {
	Given(t).
		Path("cluster-role").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--revision=HEAD")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)
		}).
		When().
		SetTrackingMethod(string(TrackingMethodAnnotation)).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--revision=HEAD")
			require.NoError(t, err)
			assert.Empty(t, diffOutput)
		})
}

// TestServerSideDiffCommand tests the --server-side-diff flag for the app diff command
func TestServerSideDiffCommand(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// Create a diff by modifying a pod
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"test": "server-side-diff"}}]`).
		AddFile("pod-3.yaml", `apiVersion: v1
kind: Pod
metadata:
  name: pod-3
  annotations:
    new: "pod"
spec:
  containers:
    - name: main
      image: quay.io/argoprojlabs/argocd-e2e-container:0.1
      imagePullPolicy: IfNotPresent
      command:
        - "true"
  restartPolicy: Never
`).
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			// Test regular diff command
			regularOutput, err := fixture.RunCli("app", "diff", app.Name)
			require.Error(t, err) // diff command returns non-zero exit code when differences found
			assert.Contains(t, regularOutput, "===== /Pod")
			assert.Contains(t, regularOutput, "pod-1")
			assert.Contains(t, regularOutput, "pod-3")

			// Test server-side diff command
			serverSideOutput, err := fixture.RunCli("app", "diff", app.Name, "--server-side-diff")
			require.Error(t, err) // diff command returns non-zero exit code when differences found
			assert.Contains(t, serverSideOutput, "===== /Pod")
			assert.Contains(t, serverSideOutput, "pod-1")
			assert.Contains(t, serverSideOutput, "pod-3")

			// Both outputs should contain similar resource headers
			assert.Contains(t, regularOutput, "test: server-side-diff")
			assert.Contains(t, serverSideOutput, "test: server-side-diff")
			assert.Contains(t, regularOutput, "new: pod")
			assert.Contains(t, serverSideOutput, "new: pod")
		})
}

// TestServerSideDiffWithSyncedApp tests server-side diff when app is already synced (no differences)
func TestServerSideDiffWithSyncedApp(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Test regular diff command with synced app
			regularOutput, err := fixture.RunCli("app", "diff", app.Name)
			require.NoError(t, err) // no differences, should return 0

			// Test server-side diff command with synced app
			serverSideOutput, err := fixture.RunCli("app", "diff", app.Name, "--server-side-diff")
			require.NoError(t, err) // no differences, should return 0

			// Both should produce similar output (minimal/no diff output)
			// The exact output may vary, but both should succeed without errors
			assert.NotContains(t, regularOutput, "===== ")
			assert.NotContains(t, serverSideOutput, "===== ")
		})
}

// TestServerSideDiffWithRevision tests server-side diff with a specific revision
func TestServerSideDiffWithRevision(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/labels", "value": {"version": "v1.1"}}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		And(func(app *Application) {
			// Get the current revision
			currentRevision := ""
			if len(app.Status.History) > 0 {
				currentRevision = app.Status.History[len(app.Status.History)-1].Revision
			}

			if currentRevision != "" {
				// Test server-side diff with current revision (should show no differences)
				output, err := fixture.RunCli("app", "diff", app.Name, "--server-side-diff", "--revision", currentRevision)
				require.NoError(t, err) // no differences expected
				assert.NotContains(t, output, "===== ")
			}
		})
}

// TestServerSideDiffErrorHandling tests error scenarios for server-side diff
func TestServerSideDiffErrorHandling(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		When().
		CreateApp().
		Then().
		And(func(_ *Application) {
			// Test server-side diff with non-existent app should fail gracefully
			_, err := fixture.RunCli("app", "diff", "non-existent-app", "--server-side-diff")
			require.Error(t, err)
			// Error occurred as expected - this verifies the command fails gracefully
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
		Expect(OperationPhaseIs(OperationSucceeded)).
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
		And(func(app *Application) {
			// Test regular diff with --local (add --server-side-generate to avoid deprecation warning)
			regularOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata", "--server-side-generate")
			require.Error(t, err) // diff command returns non-zero exit code when differences found
			assert.Contains(t, regularOutput, "===== apps/Deployment")
			assert.Contains(t, regularOutput, "guestbook-ui")
			assert.Contains(t, regularOutput, "replicas:")

			// Test server-side diff with --local (add --server-side-generate for consistency)
			serverSideOutput, err := fixture.RunCli("app", "diff", app.Name, "--server-side-diff", "--local", "testdata", "--server-side-generate")
			require.Error(t, err) // diff command returns non-zero exit code when differences found
			assert.Contains(t, serverSideOutput, "===== apps/Deployment")
			assert.Contains(t, serverSideOutput, "guestbook-ui")
			assert.Contains(t, serverSideOutput, "replicas:")

			// Both outputs should show similar differences
			assert.Contains(t, regularOutput, "replicas: 2")
			assert.Contains(t, serverSideOutput, "replicas: 2")
		})
}

func TestServerSideDiffWithLocalValidation(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Test that --server-side-diff with --local without --server-side-generate fails with proper error
			_, err := fixture.RunCli("app", "diff", app.Name, "--server-side-diff", "--local", "testdata")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--server-side-diff with --local requires --server-side-generate")
		})
}
