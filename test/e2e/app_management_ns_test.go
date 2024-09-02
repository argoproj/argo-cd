package e2e

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/common"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/account"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	projectFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/project"
	repoFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/v2/test/e2e/testdata"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	. "github.com/argoproj/argo-cd/v2/util/argo"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// This empty test is here only for clarity, to conform to logs rbac tests structure in account. This exact usecase is covered in the TestAppLogs test
func TestNamespacedGetLogsAllowNoSwitch(t *testing.T) {
}

func TestNamespacedGetLogsDenySwitchOn(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")

	ctx := GivenWithSameState(t)
	ctx.SetAppNamespace(ArgoCDAppNamespace)
	ctx.
		Path("guestbook-logs").
		SetTrackingMethod("annotation").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "true").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			_, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "permission denied")
		})
}

func TestNamespacedGetLogsAllowSwitchOnNS(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "logs",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")

	ctx := GivenWithSameState(t)
	ctx.SetAppNamespace(AppNamespace())
	ctx.
		Path("guestbook-logs").
		SetTrackingMethod("annotation").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "true").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Pod")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Service")
			require.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestNamespacedGetLogsAllowSwitchOff(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")

	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "applications",
				Action:   "create",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "get",
				Scope:    "*",
			},
			{
				Resource: "applications",
				Action:   "sync",
				Scope:    "*",
			},
			{
				Resource: "projects",
				Action:   "get",
				Scope:    "*",
			},
		}, "app-creator")
	ctx := GivenWithSameState(t)
	ctx.SetAppNamespace(AppNamespace())
	ctx.
		Path("guestbook-logs").
		SetTrackingMethod("annotation").
		When().
		CreateApp().
		Sync().
		SetParamInSettingConfigMap("server.rbac.log.enforce.enable", "false").
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Pod")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", ctx.AppQualifiedName(), "--kind", "Service")
			require.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestNamespacedSyncToUnsignedCommit(t *testing.T) {
	SkipOnEnv(t, "GPG")
	GivenWithNamespace(t, AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestNamespacedSyncToSignedCommitWKK(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestNamespacedSyncToSignedCommitKWKK(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestNamespacedAppCreation(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, Name(), app.Name)
			assert.Equal(t, AppNamespace(), app.Namespace)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.GetSource().RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.GetSource().Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(NamespacedEvent(AppNamespace(), EventReasonResourceCreated, "create")).
		And(func(app *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.AppQualifiedName())
		}).
		When().
		// ensure that create is idempotent
		CreateApp().
		Then().
		Given().
		Revision("master").
		When().
		// ensure that update replaces spec and merge labels and annotations
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.GetName(), types.MergePatchType, []byte(`{"metadata": {"labels": { "test": "label" }, "annotations": { "test": "annotation" }}}`), metav1.PatchOptions{}))
		}).
		CreateApp("--upsert").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "label", app.Labels["test"])
			assert.Equal(t, "annotation", app.Annotations["test"])
			assert.Equal(t, "master", app.Spec.GetSource().TargetRevision)
		})
}

func TestNamespacedAppCreationWithoutForceUpdate(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		DestName("in-cluster").
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, ctx.AppName(), app.Name)
			assert.Equal(t, AppNamespace(), app.Namespace)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.GetSource().RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.GetSource().Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, "in-cluster", app.Spec.Destination.Name)
		}).
		Expect(NamespacedEvent(AppNamespace(), EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.AppQualifiedName())
		}).
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "existing application spec is different, use upsert flag to force update"))
}

func TestNamespacedDeleteAppResource(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			// app should be listed
			if _, err := RunCli("app", "delete-resource", ctx.AppQualifiedName(), "--kind", "Service", "--resource-name", "guestbook-ui"); err != nil {
				require.NoError(t, err)
			}
		}).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

// demonstrate that we cannot use a standard sync when an immutable field is changed, we must use "force"
func TestNamespacedImmutableChange(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		Path("secrets").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/data/new-field", "value": "dGVzdA=="}, {"op": "add", "path": "/immutable", "value": true}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		PatchFile("secrets.yaml", `[{"op": "add", "path": "/data/new-field", "value": "dGVzdDI="}]`).
		IgnoreErrors().
		Sync().
		DoNotIgnoreErrors().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceResultMatches(ResourceResult{
			Kind:      "Secret",
			Version:   "v1",
			Namespace: DeploymentNamespace(),
			Name:      "test-secret",
			SyncPhase: "Sync",
			Status:    "SyncFailed",
			HookPhase: "Failed",
			Message:   `Secret "test-secret" is invalid`,
		})).
		// now we can do this will a force
		Given().
		Force().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestNamespacedInvalidAppProject(t *testing.T) {
	Given(t).
		SetTrackingMethod("annotation").
		Path(guestbookPath).
		SetAppNamespace(AppNamespace()).
		Project("does-not-exist").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		// We're not allowed to infer whether the project exists based on this error message. Instead, we get a generic
		// permission denied error.
		Expect(Error("", "is not allowed"))
}

func TestNamespacedAppDeletion(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		Expect(NamespacedEvent(AppNamespace(), EventReasonResourceDeleted, "delete"))

	output, err := RunCli("app", "list")
	require.NoError(t, err)
	assert.NotContains(t, output, ctx.AppQualifiedName())
}

func TestNamespacedAppLabels(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path("config-map").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp("-l", "foo=bar").
		Then().
		And(func(app *Application) {
			assert.Contains(t, FailOnErr(RunCli("app", "list")), ctx.AppQualifiedName())
			assert.Contains(t, FailOnErr(RunCli("app", "list", "-l", "foo=bar")), ctx.AppQualifiedName())
			assert.NotContains(t, FailOnErr(RunCli("app", "list", "-l", "foo=rubbish")), ctx.AppQualifiedName())
		}).
		Given().
		// remove both name and replace labels means nothing will sync
		Name("").
		When().
		IgnoreErrors().
		Sync("-l", "foo=rubbish").
		DoNotIgnoreErrors().
		Then().
		Expect(Error("", "No matching apps found for filter: selector foo=rubbish")).
		// check we can update the app and it is then sync'd
		Given().
		When().
		Sync("-l", "foo=bar")
}

func TestNamespacedTrackAppStateAndSyncApp(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(Success(fmt.Sprintf("Service     %s  guestbook-ui  Synced ", DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("apps   Deployment  %s  guestbook-ui  Synced", DeploymentNamespace()))).
		Expect(NamespacedEvent(AppNamespace(), EventReasonResourceUpdated, "sync")).
		And(func(app *Application) {
			assert.NotNil(t, app.Status.OperationState.SyncResult)
		})
}

func TestNamespacedAppRollbackSuccessful(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.NotEmpty(t, app.Status.Sync.Revision)
		}).
		And(func(app *Application) {
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{{
				ID:         1,
				Revision:   app.Status.Sync.Revision,
				DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
				Source:     app.Spec.GetSource(),
			}, {
				ID:         2,
				Revision:   "cdb",
				DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-2 * time.Minute)},
				Source:     app.Spec.GetSource(),
			}}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			require.NoError(t, err)
			app, err = AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)

			// sync app and make sure it reaches InSync state
			_, err = RunCli("app", "rollback", app.QualifiedName(), "1")
			require.NoError(t, err)
		}).
		Expect(NamespacedEvent(AppNamespace(), EventReasonOperationStarted, "rollback")).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, SyncStatusCodeSynced, app.Status.Sync.Status)
			require.NotNil(t, app.Status.OperationState.SyncResult)
			assert.Len(t, app.Status.OperationState.SyncResult.Resources, 2)
			assert.Equal(t, OperationSucceeded, app.Status.OperationState.Phase)
			assert.Len(t, app.Status.History, 3)
		})
}

func TestNamespacedComparisonFailsIfClusterNotAdded(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		DestServer("https://not-registered-cluster/api").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(DoesNotExist())
}

func TestNamespacedCannotSetInvalidPath(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		IgnoreErrors().
		AppSet("--path", "garbage").
		Then().
		Expect(Error("", "app path does not exist"))
}

func TestNamespacedManipulateApplicationResources(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", ctx.AppQualifiedName(), "--source", "live")
			require.NoError(t, err)
			resources, err := kube.SplitYAML([]byte(manifests))
			require.NoError(t, err)

			index := -1
			for i := range resources {
				if resources[i].GetKind() == kube.DeploymentKind {
					index = i
					break
				}
			}
			assert.Greater(t, index, -1)

			deployment := resources[index]

			closer, client, err := ArgoCDClientset.NewApplicationClient()
			require.NoError(t, err)
			defer io.Close(closer)

			_, err = client.DeleteResource(context.Background(), &applicationpkg.ApplicationResourceDeleteRequest{
				Name:         &app.Name,
				AppNamespace: ptr.To(AppNamespace()),
				Group:        ptr.To(deployment.GroupVersionKind().Group),
				Kind:         ptr.To(deployment.GroupVersionKind().Kind),
				Version:      ptr.To(deployment.GroupVersionKind().Version),
				Namespace:    ptr.To(deployment.GetNamespace()),
				ResourceName: ptr.To(deployment.GetName()),
			})
			require.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestNamespacedAppWithSecrets(t *testing.T) {
	closer, client, err := ArgoCDClientset.NewApplicationClient()
	require.NoError(t, err)
	defer io.Close(closer)

	ctx := Given(t)
	ctx.
		Path("secrets").
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res := FailOnErr(client.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Namespace:    &app.Spec.Destination.Namespace,
				AppNamespace: ptr.To(AppNamespace()),
				Kind:         ptr.To(kube.SecretKind),
				Group:        ptr.To(""),
				Name:         &app.Name,
				Version:      ptr.To("v1"),
				ResourceName: ptr.To("test-secret"),
			})).(*applicationpkg.ApplicationResourceResponse)
			assetSecretDataHidden(t, res.GetManifest())

			manifests, err := client.GetManifests(context.Background(), &applicationpkg.ApplicationManifestQuery{
				Name:         &app.Name,
				AppNamespace: ptr.To(AppNamespace()),
			})
			errors.CheckError(err)

			for _, manifest := range manifests.Manifests {
				assetSecretDataHidden(t, manifest)
			}

			diffOutput := FailOnErr(RunCli("app", "diff", ctx.AppQualifiedName())).(string)
			assert.Empty(t, diffOutput)

			// make sure resource update error does not print secret details
			_, err = RunCli("app", "patch-resource", ctx.AppQualifiedName(), "--resource-name", "test-secret",
				"--kind", "Secret", "--patch", `{"op": "add", "path": "/data", "value": "hello"}'`,
				"--patch-type", "application/json-patch+json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf("failed to patch Secret %s/test-secret", DeploymentNamespace()))
			assert.NotContains(t, err.Error(), "username")
			assert.NotContains(t, err.Error(), "password")

			// patch secret and make sure app is out of sync and diff detects the change
			FailOnErr(KubeClientset.CoreV1().Secrets(DeploymentNamespace()).Patch(context.Background(),
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
			diffOutput, err := RunCli("app", "diff", ctx.AppQualifiedName())
			require.Error(t, err)
			assert.Contains(t, diffOutput, "username: ++++++++")
			assert.Contains(t, diffOutput, "password: ++++++++++++")

			// local diff should ignore secrets
			diffOutput = FailOnErr(RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)

			// ignore missing field and make sure diff shows no difference
			app.Spec.IgnoreDifferences = []ResourceIgnoreDifferences{{
				Kind: kube.SecretKind, JSONPointers: []string{"/data"},
			}}
			FailOnErr(client.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, AppNamespace: ptr.To(AppNamespace()), Spec: &app.Spec}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput := FailOnErr(RunCli("app", "diff", ctx.AppQualifiedName())).(string)
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
		And(func(app *Application) {
			diffOutput := FailOnErr(RunCli("app", "diff", ctx.AppQualifiedName(), "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)
		})
}

func TestNamespacedResourceDiffing(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Patch deployment
			_, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Patch(context.Background(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`), metav1.PatchOptions{})
			require.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", ctx.AppQualifiedName(), "--local-repo-root", ".", "--local", "testdata/guestbook")
			require.Error(t, err)
			assert.Contains(t, diffOutput, fmt.Sprintf("===== apps/Deployment %s/guestbook-ui ======", DeploymentNamespace()))
		}).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {
			IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/spec/template/spec/containers/0/image"}},
		}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", ctx.AppQualifiedName(), "--local-repo-root", ".", "--local", "testdata/guestbook")
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
			output, err := RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
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
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(3), *deployment.Spec.RevisionHistoryLimit)
		}).
		And(func() {
			output, err := RunWithStdin(testdata.SSARevisionHistoryDeployment, "", "kubectl", "apply", "-n", DeploymentNamespace(), "--server-side=true", "--field-manager=revision-history-manager", "--validate=false", "--force-conflicts", "-f", "-")
			require.NoError(t, err)
			assert.Contains(t, output, "serverside-applied")
		}).
		Then().
		When().Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		}).
		When().Sync().Then().Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, int32(1), *deployment.Spec.RevisionHistoryLimit)
		})
}

// func TestCRDs(t *testing.T) {
// 	testEdgeCasesApplicationResources(t, "crd-creation", health.HealthStatusHealthy)
// }

func TestNamespacedKnownTypesInCRDDiffing(t *testing.T) {
	dummiesGVR := schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: "dummies"}

	ctx := Given(t)
	ctx.
		Path("crd-creation").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().CreateApp().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		And(func() {
			dummyResIf := DynamicClientset.Resource(dummiesGVR).Namespace(DeploymentNamespace())
			patchData := []byte(`{"spec":{"cpu": "2"}}`)
			FailOnErr(dummyResIf.Patch(context.Background(), "dummy-crd-instance", types.MergePatchType, patchData, metav1.PatchOptions{}))
		}).Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/Dummy": {
					KnownTypeFields: []KnownTypeField{{
						Field: "spec",
						Type:  "core/v1/ResourceList",
					}},
				},
			})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

// TODO(jannfis): This somehow doesn't work -- I suspect tracking method
// func TestNamespacedDuplicatedResources(t *testing.T) {
// 	testNSEdgeCasesApplicationResources(t, "duplicated-resources", health.HealthStatusHealthy)
// }

func TestNamespacedConfigMap(t *testing.T) {
	testNSEdgeCasesApplicationResources(t, "config-map", health.HealthStatusHealthy, "my-map  Synced                configmap/my-map created")
}

func testNSEdgeCasesApplicationResources(t *testing.T, appPath string, statusCode health.HealthStatusCode, message ...string) {
	ctx := Given(t)
	expect := ctx.
		Path(appPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
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
			diffOutput, err := RunCli("app", "diff", ctx.AppQualifiedName(), "--local-repo-root", ".", "--local", path.Join("testdata", appPath))
			assert.Empty(t, diffOutput)
			require.NoError(t, err)
		})
}

// // We don't have tracking label in namespaced tests, thus we need a unique
// // resource action that modifies annotations instead of labels.
// const nsActionsConfig = `discovery.lua: return { sample = {} }
// definitions:
// - name: sample
//   action.lua: |
//     obj.metadata.annotations.sample = 'test'
//     return obj`

func TestNamespacedResourceAction(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			closer, client, err := ArgoCDClientset.NewApplicationClient()
			require.NoError(t, err)
			defer io.Close(closer)

			actions, err := client.ListResourceActions(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Name:         &app.Name,
				AppNamespace: ptr.To(AppNamespace()),
				Group:        ptr.To("apps"),
				Kind:         ptr.To("Deployment"),
				Version:      ptr.To("v1"),
				Namespace:    ptr.To(DeploymentNamespace()),
				ResourceName: ptr.To("guestbook-ui"),
			})
			require.NoError(t, err)
			assert.Equal(t, []*ResourceAction{{Name: "sample", Disabled: false}}, actions.Actions)

			_, err = client.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{
				Name:         &app.Name,
				Group:        ptr.To("apps"),
				Kind:         ptr.To("Deployment"),
				Version:      ptr.To("v1"),
				Namespace:    ptr.To(DeploymentNamespace()),
				ResourceName: ptr.To("guestbook-ui"),
				Action:       ptr.To("sample"),
				AppNamespace: ptr.To(AppNamespace()),
			})
			require.NoError(t, err)

			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, "test", deployment.Labels["sample"])
		})
}

func TestNamespacedSyncResourceByLabel(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, _ = RunCli("app", "sync", ctx.AppQualifiedName(), "--label", fmt.Sprintf("app.kubernetes.io/instance=%s", app.Name))
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_, err := RunCli("app", "sync", ctx.AppQualifiedName(), "--label", "this-label=does-not-exist")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "level=fatal")
		})
}

func TestNamespacedLocalManifestSync(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", ctx.AppQualifiedName())
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.2")
		}).
		Given().
		LocalPath(guestbookPathLocal).
		When().
		Sync("--local-repo-root", ".").
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", ctx.AppQualifiedName())
			assert.Contains(t, res, "containerPort: 81")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.3")
		}).
		Given().
		LocalPath("").
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", ctx.AppQualifiedName())
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.2")
		})
}

func TestNamespacedLocalSync(t *testing.T) {
	Given(t).
		// we've got to use Helm as this uses kubeVersion
		Path("helm").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			FailOnErr(RunCli("app", "sync", app.QualifiedName(), "--local", "testdata/helm"))
		})
}

func TestNamespacedNoLocalSyncWithAutosyncEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.QualifiedName(), "--sync-policy", "automated")
			require.NoError(t, err)

			_, err = RunCli("app", "sync", app.QualifiedName(), "--local", guestbookPathLocal)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Cannot use local sync")
		})
}

func TestNamespacedLocalSyncDryRunWithASEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.QualifiedName(), "--sync-policy", "automated")
			require.NoError(t, err)

			appBefore := app.DeepCopy()
			_, err = RunCli("app", "sync", app.QualifiedName(), "--dry-run", "--local-repo-root", ".", "--local", guestbookPathLocal)
			require.NoError(t, err)

			appAfter := app.DeepCopy()
			assert.True(t, reflect.DeepEqual(appBefore, appAfter))
		})
}

func TestNamespacedSyncAsync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Async(true).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

// assertResourceActions verifies if view/modify resource actions are successful/failing for given application
func assertNSResourceActions(t *testing.T, appName string, successful bool) {
	assertError := func(err error, message string) {
		if successful {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			assert.Contains(t, err.Error(), message)
		}
	}

	closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
	defer io.Close(closer)

	deploymentResource, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
	require.NoError(t, err)

	logs, err := cdClient.PodLogs(context.Background(), &applicationpkg.ApplicationPodLogsQuery{
		Group:        ptr.To("apps"),
		Kind:         ptr.To("Deployment"),
		Name:         &appName,
		AppNamespace: ptr.To(AppNamespace()),
		Namespace:    ptr.To(DeploymentNamespace()),
		Container:    ptr.To(""),
		SinceSeconds: ptr.To(int64(0)),
		TailLines:    ptr.To(int64(0)),
		Follow:       ptr.To(false),
	})
	require.NoError(t, err)
	_, err = logs.Recv()
	assertError(err, "EOF")

	expectedError := fmt.Sprintf("Deployment apps guestbook-ui not found as part of application %s", appName)

	_, err = cdClient.ListResourceEvents(context.Background(), &applicationpkg.ApplicationResourceEventsQuery{
		Name:              &appName,
		AppNamespace:      ptr.To(AppNamespace()),
		ResourceName:      ptr.To("guestbook-ui"),
		ResourceNamespace: ptr.To(DeploymentNamespace()),
		ResourceUID:       ptr.To(string(deploymentResource.UID)),
	})
	assertError(err, fmt.Sprintf("%s not found as part of application %s", "guestbook-ui", appName))

	_, err = cdClient.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
		Name:         &appName,
		AppNamespace: ptr.To(AppNamespace()),
		ResourceName: ptr.To("guestbook-ui"),
		Namespace:    ptr.To(DeploymentNamespace()),
		Version:      ptr.To("v1"),
		Group:        ptr.To("apps"),
		Kind:         ptr.To("Deployment"),
	})
	assertError(err, expectedError)

	_, err = cdClient.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{
		Name:         &appName,
		AppNamespace: ptr.To(AppNamespace()),
		ResourceName: ptr.To("guestbook-ui"),
		Namespace:    ptr.To(DeploymentNamespace()),
		Version:      ptr.To("v1"),
		Group:        ptr.To("apps"),
		Kind:         ptr.To("Deployment"),
		Action:       ptr.To("restart"),
	})
	assertError(err, expectedError)

	_, err = cdClient.DeleteResource(context.Background(), &applicationpkg.ApplicationResourceDeleteRequest{
		Name:         &appName,
		AppNamespace: ptr.To(AppNamespace()),
		ResourceName: ptr.To("guestbook-ui"),
		Namespace:    ptr.To(DeploymentNamespace()),
		Version:      ptr.To("v1"),
		Group:        ptr.To("apps"),
		Kind:         ptr.To("Deployment"),
	})
	assertError(err, expectedError)
}

func TestNamespacedPermissions(t *testing.T) {
	appCtx := Given(t)
	projName := "argo-project"
	projActions := projectFixture.
		Given(t).
		Name(projName).
		SourceNamespaces([]string{AppNamespace()}).
		When().
		Create()

	sourceError := fmt.Sprintf("application repo %s is not permitted in project 'argo-project'", RepoURL(RepoURLTypeFile))
	destinationError := fmt.Sprintf("application destination server '%s' and namespace '%s' do not match any of the allowed destinations in project 'argo-project'", KubernetesInternalAPIServerAddr, DeploymentNamespace())

	appCtx.
		Path("guestbook-logs").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Project(projName).
		When().
		IgnoreErrors().
		// ensure app is not created if project permissions are missing
		CreateApp().
		Then().
		Expect(Error("", sourceError)).
		Expect(Error("", destinationError)).
		When().
		DoNotIgnoreErrors().
		// add missing permissions, create and sync app
		And(func() {
			projActions.AddDestination("*", "*")
			projActions.AddSource("*")
		}).
		CreateApp().
		Sync().
		Then().
		// make sure application resource actiions are successful
		And(func(app *Application) {
			assertNSResourceActions(t, app.Name, true)
		}).
		When().
		// remove projet permissions and "refresh" app
		And(func() {
			projActions.UpdateProject(func(proj *AppProject) {
				proj.Spec.Destinations = nil
				proj.Spec.SourceRepos = nil
			})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		// ensure app resource tree is empty when source/destination permissions are missing
		Expect(Condition(ApplicationConditionInvalidSpecError, destinationError)).
		Expect(Condition(ApplicationConditionInvalidSpecError, sourceError)).
		And(func(app *Application) {
			closer, cdClient := ArgoCDClientset.NewApplicationClientOrDie()
			defer io.Close(closer)
			tree, err := cdClient.ResourceTree(context.Background(), &applicationpkg.ResourcesQuery{ApplicationName: &app.Name, AppNamespace: &app.Namespace})
			require.NoError(t, err)
			assert.Empty(t, tree.Nodes)
			assert.Empty(t, tree.OrphanedNodes)
		}).
		When().
		// add missing permissions but deny management of Deployment kind
		And(func() {
			projActions.
				AddDestination("*", "*").
				AddSource("*").
				UpdateProject(func(proj *AppProject) {
					proj.Spec.NamespaceResourceBlacklist = []metav1.GroupKind{{Group: "*", Kind: "Deployment"}}
				})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		// make sure application resource actiions are failing
		And(func(app *Application) {
			assertNSResourceActions(t, app.Name, false)
		})
}

func TestNamespacedPermissionWithScopedRepo(t *testing.T) {
	projName := "argo-project"
	fixture.EnsureCleanState(t)
	projectFixture.
		Given(t).
		Name(projName).
		SourceNamespaces([]string{AppNamespace()}).
		Destination("*,*").
		When().
		Create()

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Project(projName).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		Path("two-nice-pods").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeOutOfSync))
}

func TestNamespacedPermissionDeniedWithScopedRepo(t *testing.T) {
	projName := "argo-project"
	projectFixture.
		Given(t).
		Name(projName).
		Destination("*,*").
		SourceNamespaces([]string{AppNamespace()}).
		When().
		Create()

	repoFixture.Given(t, true).
		When().
		Path(RepoURL(RepoURLTypeFile)).
		Create()

	GivenWithSameState(t).
		Project(projName).
		RepoURLType(RepoURLTypeFile).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "is not permitted in project"))
}

// make sure that if we deleted a resource from the app, it is not pruned if annotated with Prune=false
func TestNamespacedSyncOptionPruneFalse(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeOutOfSync))
}

// make sure that if we have an invalid manifest, we can add it if we disable validation, we get a server error rather than a client error
func TestNamespacedSyncOptionValidateFalse(t *testing.T) {
	Given(t).
		Path("crd-validation").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		IgnoreErrors().
		Sync().
		Then().
		// client error. K8s API changed error message w/ 1.25, so for now, we need to check both
		Expect(ErrorRegex("error validating data|of type int32", "")).
		When().
		PatchFile("deployment.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Validate=false"}}]`).
		Sync().
		Then().
		// server error
		Expect(Error("cannot be handled as a Deployment", ""))
}

// make sure that, if we have a resource that needs pruning, but we're ignoring it, the app is in-sync
func TestNamespacedCompareOptionIgnoreExtraneous(t *testing.T) {
	Given(t).
		Prune(false).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/compare-options": "IgnoreExtraneous"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteFile("pod-1.yaml").
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.Resources, 2)
			statusByName := map[string]SyncStatusCode{}
			for _, r := range app.Status.Resources {
				statusByName[r.Name] = r.Status
			}
			assert.Equal(t, SyncStatusCodeOutOfSync, statusByName["pod-1"])
			assert.Equal(t, SyncStatusCodeSynced, statusByName["pod-2"])
		}).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestNamespacedSelfManagedApps(t *testing.T) {
	Given(t).
		Path("self-managed-app").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		PatchFile("resources.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/spec/source/repoURL", "value": "%s"}]`, RepoURL(RepoURLTypeFile))).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(a *Application) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()

			reconciledCount := 0
			var lastReconciledAt *metav1.Time
			for event := range ArgoCDClientset.WatchApplicationWithRetry(ctx, a.QualifiedName(), a.ResourceVersion) {
				reconciledAt := event.Application.Status.ReconciledAt
				if reconciledAt == nil {
					reconciledAt = &metav1.Time{}
				}
				if lastReconciledAt != nil && !lastReconciledAt.Equal(reconciledAt) {
					reconciledCount = reconciledCount + 1
				}
				lastReconciledAt = reconciledAt
			}

			assert.Less(t, reconciledCount, 3, "Application was reconciled too many times")
		})
}

func TestNamespacedExcludedResource(t *testing.T) {
	Given(t).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Path(guestbookPath).
		ResourceFilter(settings.ResourcesFilter{
			ResourceExclusions: []settings.FilteredResource{{Kinds: []string{kube.DeploymentKind}}},
		}).
		When().
		CreateApp().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionExcludedResourceWarning, "Resource apps/Deployment guestbook-ui is excluded in the settings"))
}

func TestNamespacedRevisionHistoryLimit(t *testing.T) {
	Given(t).
		Path("config-map").
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.History, 1)
		}).
		When().
		AppSet("--revision-history-limit", "1").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Len(t, app.Status.History, 1)
		})
}

func TestNamespacedOrphanedResource(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: ptr.To(true)},
			SourceNamespaces:  []string{AppNamespace()},
		}).
		SetTrackingMethod("annotation").
		SetAppNamespace(AppNamespace()).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		When().
		And(func() {
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "orphaned-configmap",
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName())
			require.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: ptr.To(true), Ignore: []OrphanedResourceKey{{Group: "Test", Kind: "ConfigMap"}}},
			SourceNamespaces:  []string{AppNamespace()},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName())
			require.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: ptr.To(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap"}}},
			SourceNamespaces:  []string{AppNamespace()},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName())
			require.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: ptr.To(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap", Name: "orphaned-configmap"}}},
			SourceNamespaces:  []string{AppNamespace()},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName())
			require.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
			SourceNamespaces:  []string{AppNamespace()},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

func TestNamespacedNotPermittedResources(t *testing.T) {
	ctx := Given(t)
	ctx.SetAppNamespace(AppNamespace())
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-ingress",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: fmt.Sprintf("%s_%s:networking/Ingress:%s/sample-ingress", AppNamespace(), ctx.AppName(), DeploymentNamespace()),
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "guestbook-ui",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
							PathType: &pathType,
						}},
					},
				},
			}},
		},
	}
	defer func() {
		log.Infof("Ingress 'sample-ingress' deleted from %s", TestNamespace())
		CheckError(KubeClientset.NetworkingV1().Ingresses(TestNamespace()).Delete(context.Background(), "sample-ingress", metav1.DeleteOptions{}))
	}()

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "guestbook-ui",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: fmt.Sprintf("%s_%s:Service:%s/guesbook-ui", TestNamespace(), ctx.AppQualifiedName(), DeploymentNamespace()),
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Port:       80,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
			}},
			Selector: map[string]string{
				"app": "guestbook-ui",
			},
		},
	}

	ctx.ProjectSpec(AppProjectSpec{
		SourceRepos:      []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: DeploymentNamespace(), Server: "*"}},
		SourceNamespaces: []string{AppNamespace()},
		NamespaceResourceBlacklist: []metav1.GroupKind{
			{Group: "", Kind: "Service"},
		},
	}).
		And(func() {
			FailOnErr(KubeClientset.NetworkingV1().Ingresses(TestNamespace()).Create(context.Background(), ingress, metav1.CreateOptions{}))
			FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Create(context.Background(), svc, metav1.CreateOptions{}))
		}).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			statusByKind := make(map[string]ResourceStatus)
			for _, res := range app.Status.Resources {
				statusByKind[res.Kind] = res
			}
			_, hasIngress := statusByKind[kube.IngressKind]
			assert.False(t, hasIngress, "Ingress is prohibited not managed object and should be even visible to user")
			serviceStatus := statusByKind[kube.ServiceKind]
			assert.Equal(t, SyncStatusCodeUnknown, serviceStatus.Status, "Service is prohibited managed resource so should be set to Unknown")
			deploymentStatus := statusByKind[kube.DeploymentKind]
			assert.Equal(t, SyncStatusCodeOutOfSync, deploymentStatus.Status)
		}).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())

	// Make sure prohibited resources are not deleted during application deletion
	FailOnErr(KubeClientset.NetworkingV1().Ingresses(TestNamespace()).Get(context.Background(), "sample-ingress", metav1.GetOptions{}))
	FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{}))
}

func TestNamespacedSyncWithInfos(t *testing.T) {
	expectedInfo := make([]*Info, 2)
	expectedInfo[0] = &Info{Name: "name1", Value: "val1"}
	expectedInfo[1] = &Info{Name: "name2", Value: "val2"}

	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "sync", app.QualifiedName(),
				"--info", fmt.Sprintf("%s=%s", expectedInfo[0].Name, expectedInfo[0].Value),
				"--info", fmt.Sprintf("%s=%s", expectedInfo[1].Name, expectedInfo[1].Value))
			require.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.ElementsMatch(t, app.Status.OperationState.Operation.Info, expectedInfo)
		})
}

// Given: argocd app create does not provide --dest-namespace
//
//	Manifest contains resource console which does not require namespace
//
// Expect: no app.Status.Conditions
func TestNamespacedCreateAppWithNoNameSpaceForGlobalResource(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path(globalWithNoNameSpace).
		When().
		CreateWithNoNameSpace().
		Then().
		And(func(app *Application) {
			time.Sleep(500 * time.Millisecond)
			app, err := AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Get(context.Background(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Empty(t, app.Status.Conditions)
		})
}

// Given: argocd app create does not provide --dest-namespace
//
//	Manifest contains resource deployment, and service which requires namespace
//	Deployment and service do not have namespace in manifest
//
// Expect: app.Status.Conditions for deployment ans service which does not have namespace in manifest
func TestNamespacedCreateAppWithNoNameSpaceWhenRequired(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path(guestbookPath).
		When().
		CreateWithNoNameSpace().
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			updatedApp, err := AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Get(context.Background(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
			assert.Equal(t, ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[1].Type)
		})
}

// Given: argocd app create does not provide --dest-namespace
//
//	Manifest contains resource deployment, and service which requires namespace
//	Some deployment and service has namespace in manifest
//	Some deployment and service does not have namespace in manifest
//
// Expect: app.Status.Conditions for deployment and service which does not have namespace in manifest
func TestNamespacedCreateAppWithNoNameSpaceWhenRequired2(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path(guestbookWithNamespace).
		When().
		CreateWithNoNameSpace().
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			updatedApp, err := AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Get(context.Background(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
			assert.Equal(t, ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[1].Type)
		})
}

func TestNamespacedListResource(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: ptr.To(true)},
			SourceNamespaces:  []string{AppNamespace()},
		}).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		When().
		And(func() {
			FailOnErr(KubeClientset.CoreV1().ConfigMaps(DeploymentNamespace()).Create(context.Background(), &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "orphaned-configmap",
				},
			}, metav1.CreateOptions{}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName())
			require.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName(), "--orphaned=true")
			require.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.NotContains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.QualifiedName(), "--orphaned=false")
			require.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
			SourceNamespaces:  []string{AppNamespace()},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

// Given application is set with --sync-option CreateNamespace=true
//
//		application --dest-namespace does not exist
//
//	    Verify application --dest-namespace is created
//
//		application sync successful
//		when application is deleted, --dest-namespace is not deleted
func TestNamespacedNamespaceAutoCreation(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	updatedNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", updatedNamespace)
			require.NoError(t, err)
		}
	}()
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Timeout(30).
		Path("guestbook").
		When().
		CreateApp("--sync-option", "CreateNamespace=true").
		Then().
		Expect(NoNamespace(updatedNamespace)).
		When().
		AppSet("--dest-namespace", updatedNamespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced)).
		When().
		Delete(true).
		Then().
		Expect(Success("")).
		And(func(app *Application) {
			// Verify delete app does not delete the namespace auto created
			output, err := Run("", "kubectl", "get", "namespace", updatedNamespace)
			require.NoError(t, err)
			assert.Contains(t, output, updatedNamespace)
		})
}

// Given application is set with --sync-option CreateNamespace=true
//
//		application --dest-namespace does not exist
//
//	    Verify application --dest-namespace is created with managedNamespaceMetadata
func TestNamespacedNamespaceAutoCreationWithMetadata(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	updatedNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", updatedNamespace)
			require.NoError(t, err)
		}
	}()
	ctx := Given(t)
	ctx.
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Timeout(30).
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{"CreateNamespace=true"},
				ManagedNamespaceMetadata: &ManagedNamespaceMetadata{
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"bar": "bat"},
				},
			}
		}).
		Then().
		Expect(NoNamespace(updatedNamespace)).
		When().
		AppSet("--dest-namespace", updatedNamespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			assert.Empty(t, app.Status.Conditions)

			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			assert.Equal(t, map[string]string{"foo": "bar"}, ns.Labels)
			assert.Equal(t, map[string]string{"bar": "bat", "argocd.argoproj.io/sync-options": "ServerSideApply=true"}, ns.Annotations)
			assert.Equal(t, map[string]string{"foo": "bar"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Labels)
			assert.Equal(t, map[string]string{"bar": "bat"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Annotations)
		})).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced)).
		When().
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.GetName(), types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/syncPolicy/managedNamespaceMetadata/labels", "value": {"new":"label"} }]`), metav1.PatchOptions{}))
		}).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")

			assert.Equal(t, map[string]string{"new": "label"}, ns.Labels)
			assert.Equal(t, map[string]string{"bar": "bat", "argocd.argoproj.io/sync-options": "ServerSideApply=true"}, ns.Annotations)
			assert.Equal(t, map[string]string{"new": "label"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Labels)
			assert.Equal(t, map[string]string{"bar": "bat"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Annotations)
		})).
		When().
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.GetName(), types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/syncPolicy/managedNamespaceMetadata/annotations", "value": {"new":"custom-annotation"} }]`), metav1.PatchOptions{}))
		}).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			assert.Equal(t, map[string]string{"new": "label"}, ns.Labels)
			assert.Equal(t, map[string]string{"new": "custom-annotation", "argocd.argoproj.io/sync-options": "ServerSideApply=true"}, ns.Annotations)
			assert.Equal(t, map[string]string{"new": "label"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Labels)
			assert.Equal(t, map[string]string{"new": "custom-annotation"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Annotations)
		}))
}

// Given application is set with --sync-option CreateNamespace=true
//
//		application --dest-namespace does not exist
//
//	    Verify application namespace manifest takes precedence over managedNamespaceMetadata
func TestNamespacedNamespaceAutoCreationWithMetadataAndNsManifest(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	namespace := "guestbook-ui-with-namespace-manifest"
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", namespace)
			require.NoError(t, err)
		}
	}()

	ctx := Given(t)
	ctx.
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Timeout(30).
		Path("guestbook-with-namespace-manifest").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{"CreateNamespace=true"},
				ManagedNamespaceMetadata: &ManagedNamespaceMetadata{
					Labels:      map[string]string{"foo": "bar", "abc": "123"},
					Annotations: map[string]string{"bar": "bat"},
				},
			}
		}).
		Then().
		Expect(NoNamespace(namespace)).
		When().
		AppSet("--dest-namespace", namespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(namespace, func(app *Application, ns *v1.Namespace) {
			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Labels, "kubectl.kubernetes.io/last-applied-configuration")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			// The application namespace manifest takes precedence over what is in managedNamespaceMetadata
			assert.Equal(t, map[string]string{"test": "true"}, ns.Labels)
			assert.Equal(t, map[string]string{"foo": "bar", "something": "else"}, ns.Annotations)
		})).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", namespace, SyncStatusCodeSynced))
}

// Given application is set with --sync-option CreateNamespace=true
//
//		application --dest-namespace exists
//
//	    Verify application --dest-namespace is updated with managedNamespaceMetadata labels and annotations
func TestNamespacedNamespaceAutoCreationWithPreexistingNs(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	updatedNamespace := getNewNamespace(t)
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", updatedNamespace)
			require.NoError(t, err)
		}
	}()

	existingNs := `
apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    test: "true"
  annotations:
    something: "whatevs"		
`
	s := fmt.Sprintf(existingNs, updatedNamespace)

	tmpFile, err := os.CreateTemp("", "")
	errors.CheckError(err)
	_, err = tmpFile.Write([]byte(s))
	errors.CheckError(err)

	_, err = Run("", "kubectl", "apply", "-f", tmpFile.Name())
	require.NoError(t, err)

	ctx := Given(t)
	ctx.
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Timeout(30).
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{"CreateNamespace=true"},
				ManagedNamespaceMetadata: &ManagedNamespaceMetadata{
					Labels:      map[string]string{"foo": "bar"},
					Annotations: map[string]string{"bar": "bat"},
				},
			}
		}).
		Then().
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			assert.Empty(t, app.Status.Conditions)

			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			assert.Equal(t, map[string]string{"test": "true"}, ns.Labels)
			assert.Equal(t, map[string]string{"something": "whatevs"}, ns.Annotations)
		})).
		When().
		AppSet("--dest-namespace", updatedNamespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			assert.Empty(t, app.Status.Conditions)

			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			assert.Equal(t, map[string]string{"foo": "bar"}, ns.Labels)
			assert.Equal(t, map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true", "bar": "bat"}, ns.Annotations)
		})).
		When().
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.GetName(), types.JSONPatchType, []byte(`[{ "op": "add", "path": "/spec/syncPolicy/managedNamespaceMetadata/annotations/something", "value": "hmm" }]`), metav1.PatchOptions{}))
		}).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			assert.Empty(t, app.Status.Conditions)

			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")

			assert.Equal(t, map[string]string{"foo": "bar"}, ns.Labels)
			assert.Equal(t, map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true", "something": "hmm", "bar": "bat"}, ns.Annotations)
			assert.Equal(t, map[string]string{"something": "hmm", "bar": "bat"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Annotations)
		})).
		When().
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.GetName(), types.JSONPatchType, []byte(`[{ "op": "remove", "path": "/spec/syncPolicy/managedNamespaceMetadata/annotations/something" }]`), metav1.PatchOptions{}))
		}).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(updatedNamespace, func(app *Application, ns *v1.Namespace) {
			assert.Empty(t, app.Status.Conditions)

			delete(ns.Labels, "kubernetes.io/metadata.name")
			delete(ns.Labels, "argocd.argoproj.io/tracking-id")
			delete(ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
			delete(ns.Annotations, "argocd.argoproj.io/tracking-id")

			assert.Equal(t, map[string]string{"foo": "bar"}, ns.Labels)
			assert.Equal(t, map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true", "bar": "bat"}, ns.Annotations)
			assert.Equal(t, map[string]string{"bar": "bat"}, app.Spec.SyncPolicy.ManagedNamespaceMetadata.Annotations)
		})).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", updatedNamespace, SyncStatusCodeSynced))
}

func TestNamespacedFailedSyncWithRetry(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync"}}]`).
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command", "value": ["false"]}]`).
		CreateApp().
		IgnoreErrors().
		Sync("--retry-limit=1", "--retry-backoff-duration=1s").
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(OperationMessageContains("retried 1 times"))
}

func TestNamespacedCreateDisableValidation(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("baddir").
		When().
		CreateApp("--validate=false").
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "create", app.QualifiedName(), "--upsert", "--validate=false", "--repo", RepoURL(RepoURLTypeFile),
				"--path", "baddir2", "--project", app.Spec.Project, "--dest-server", KubernetesInternalAPIServerAddr, "--dest-namespace", DeploymentNamespace())
			require.NoError(t, err)
		}).
		When().
		AppSet("--path", "baddir3", "--validate=false")
}

func TestNamespacedCreateFromPartialFile(t *testing.T) {
	partialApp := `metadata:
  labels:
    labels.local/from-file: file
    labels.local/from-args: file
  annotations:
    annotations.local/from-file: file
  finalizers:
  - resources-finalizer.argocd.argoproj.io
spec:
  syncPolicy:
    automated:
      prune: true
`

	path := "helm-values"
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		When().
		// app should be auto-synced once created
		CreateFromPartialFile(partialApp, "--path", path, "-l", "labels.local/from-args=args", "--helm-set", "foo=foo").
		Then().
		Expect(Success("")).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			assert.Equal(t, map[string]string{"labels.local/from-file": "file", "labels.local/from-args": "args"}, app.ObjectMeta.Labels)
			assert.Equal(t, map[string]string{"annotations.local/from-file": "file"}, app.ObjectMeta.Annotations)
			assert.Equal(t, []string{"resources-finalizer.argocd.argoproj.io"}, app.ObjectMeta.Finalizers)
			assert.Equal(t, path, app.Spec.GetSource().Path)
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "foo"}}, app.Spec.GetSource().Helm.Parameters)
		})
}

// Ensure actions work when using a resource action that modifies status and/or spec
func TestNamespacedCRDStatusSubresourceAction(t *testing.T) {
	actions := `
discovery.lua: |
  actions = {}
  actions["update-spec"] = {["disabled"] = false}
  actions["update-status"] = {["disabled"] = false}
  actions["update-both"] = {["disabled"] = false}
  return actions
definitions:
- name: update-both
  action.lua: |
    obj.spec = {}
    obj.spec.foo = "update-both"
    obj.status = {}
    obj.status.bar = "update-both"
    return obj
- name: update-spec
  action.lua: |
    obj.spec = {}
    obj.spec.foo = "update-spec"
    return obj
- name: update-status
  action.lua: |
    obj.status = {}
    obj.status.bar = "update-status"
    return obj
`
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("crd-subresource").
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/StatusSubResource": {
					Actions: actions,
				},
				"argoproj.io/NonStatusSubResource": {
					Actions: actions,
				},
			})
		}).
		When().CreateApp().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		// tests resource actions on a CRD using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "StatusSubResource", "update-both")
			require.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "StatusSubResource", "update-spec")
			require.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "StatusSubResource", "update-status")
			require.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		}).
		// tests resource actions on a CRD *not* using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "NonStatusSubResource", "update-both")
			require.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "NonStatusSubResource", "update-spec")
			require.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.QualifiedName(), "--kind", "NonStatusSubResource", "update-status")
			require.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		})
}

func TestNamespacedAppLogs(t *testing.T) {
	t.SkipNow() // Too flaky. https://github.com/argoproj/argo-cd/issues/13834
	SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("guestbook-logs").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", app.QualifiedName(), "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", app.QualifiedName(), "--kind", "Pod")
			require.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCliWithRetry(5, "app", "logs", app.QualifiedName(), "--kind", "Service")
			require.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestNamespacedAppWaitOperationInProgress(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"batch/Job": {
					HealthLua: `return { status = 'Running' }`,
				},
				"apps/Deployment": {
					HealthLua: `return { status = 'Suspended' }`,
				},
			})
		}).
		Async(true).
		Path("hook-and-deployment").
		When().
		CreateApp().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "wait", app.QualifiedName(), "--suspended")
			errors.CheckError(err)
		})
}

func TestNamespacedSyncOptionReplace(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("config-map").
		When().
		PatchFile("config-map.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Replace=true"}}]`).
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "configmap/my-map created", app.Status.OperationState.SyncResult.Resources[0].Message)
		}).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "configmap/my-map replaced", app.Status.OperationState.SyncResult.Resources[0].Message)
		})
}

func TestNamespacedSyncOptionReplaceFromCLI(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("config-map").
		Replace().
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "configmap/my-map created", app.Status.OperationState.SyncResult.Resources[0].Message)
		}).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, "configmap/my-map replaced", app.Status.OperationState.SyncResult.Resources[0].Message)
		})
}

func TestNamespacedDiscoverNewCommit(t *testing.T) {
	var sha string
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("config-map").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			sha = app.Status.Sync.Revision
			assert.NotEmpty(t, sha)
		}).
		When().
		PatchFile("config-map.yaml", `[{"op": "replace", "path": "/data/foo", "value": "hello"}]`).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// make sure new commit is not discovered immediately after push
		And(func(app *Application) {
			assert.Equal(t, sha, app.Status.Sync.Revision)
		}).
		When().
		// make sure new commit is not discovered after refresh is requested
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.NotEqual(t, sha, app.Status.Sync.Revision)
		})
}

func TestNamespacedDisableManifestGeneration(t *testing.T) {
	Given(t).
		SetAppNamespace(AppNamespace()).
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		CreateApp().
		Refresh(RefreshTypeHard).
		Then().
		And(func(app *Application) {
			assert.Equal(t, ApplicationSourceTypeKustomize, app.Status.SourceType)
		}).
		When().
		And(func() {
			time.Sleep(3 * time.Second)
			SetEnableManifestGeneration(map[ApplicationSourceType]bool{
				ApplicationSourceTypeKustomize: false,
			})
		}).
		Refresh(RefreshTypeHard).
		Then().
		And(func(app *Application) {
			time.Sleep(1 * time.Second)
		}).
		And(func(app *Application) {
			assert.Equal(t, ApplicationSourceTypeDirectory, app.Status.SourceType)
		})
}

func TestCreateAppInNotAllowedNamespace(t *testing.T) {
	ctx := Given(t)
	ctx.
		ProjectSpec(AppProjectSpec{
			SourceRepos:      []string{"*"},
			SourceNamespaces: []string{"default"},
			Destinations: []ApplicationDestination{
				{Namespace: "*", Server: "*"},
			},
		}).
		Path(guestbookPath).
		SetTrackingMethod("annotation").
		SetAppNamespace("default").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(DoesNotExist()).
		Expect(Error("", "namespace 'default' is not permitted"))
}
