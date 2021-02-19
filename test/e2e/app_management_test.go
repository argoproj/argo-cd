package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	networkingv1beta "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/common"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/util/argo"
	. "github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	guestbookPath          = "guestbook"
	guestbookPathLocal     = "./testdata/guestbook_local"
	globalWithNoNameSpace  = "global-with-no-namesapce"
	guestbookWithNamespace = "guestbook-with-namespace"
)

func TestSyncToUnsignedCommit(t *testing.T) {
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestSyncToSignedCommitWithoutKnownKey(t *testing.T) {
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestSyncToSignedCommitKeyWithKnownKey(t *testing.T) {
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestAppCreation(t *testing.T) {
	ctx := Given(t)

	ctx.
		Path(guestbookPath).
		When().
		Create().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, Name(), app.Name)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.Source.RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.Source.Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, common.KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
			assert.Contains(t, output, Name())
		}).
		When().
		// ensure that create is idempotent
		Create().
		Then().
		Given().
		Revision("master").
		When().
		// ensure that update replaces spec and merge labels and annotations
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Patch(context.Background(),
				ctx.GetName(), types.MergePatchType, []byte(`{"metadata": {"labels": { "test": "label" }, "annotations": { "test": "annotation" }}}`), metav1.PatchOptions{}))
		}).
		Create("--upsert").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "label", app.Labels["test"])
			assert.Equal(t, "annotation", app.Annotations["test"])
			assert.Equal(t, "master", app.Spec.Source.TargetRevision)
		})
}

// demonstrate that we cannot use a standard sync when an immutable field is changed, we must use "force"
func TestImmutableChange(t *testing.T) {
	text := FailOnErr(Run(".", "kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o", "jsonpath={.spec.clusterIP}")).(string)
	parts := strings.Split(text, ".")
	n := rand.Intn(254)
	ip1 := fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], n)
	ip2 := fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], n+1)
	Given(t).
		Path("service").
		When().
		Create().
		PatchFile("service.yaml", fmt.Sprintf(`[{"op": "add", "path": "/spec/clusterIP", "value": "%s"}]`, ip1)).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		PatchFile("service.yaml", fmt.Sprintf(`[{"op": "add", "path": "/spec/clusterIP", "value": "%s"}]`, ip2)).
		IgnoreErrors().
		Sync().
		DoNotIgnoreErrors().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceResultMatches(ResourceResult{
			Kind:      "Service",
			Version:   "v1",
			Namespace: DeploymentNamespace(),
			Name:      "my-service",
			SyncPhase: "Sync",
			Status:    "SyncFailed",
			HookPhase: "Failed",
			Message:   `Service "my-service" is invalid`,
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

func TestInvalidAppProject(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		Project("does-not-exist").
		When().
		IgnoreErrors().
		Create().
		Then().
		Expect(Error("", "application references project does-not-exist which does not exist"))
}

func TestAppDeletion(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist()).
		Expect(Event(EventReasonResourceDeleted, "delete"))

	output, err := RunCli("app", "list")
	assert.NoError(t, err)
	assert.NotContains(t, output, Name())
}

func TestAppLabels(t *testing.T) {
	Given(t).
		Path("config-map").
		When().
		Create("-l", "foo=bar").
		Then().
		And(func(app *Application) {
			assert.Contains(t, FailOnErr(RunCli("app", "list")), Name())
			assert.Contains(t, FailOnErr(RunCli("app", "list", "-l", "foo=bar")), Name())
			assert.NotContains(t, FailOnErr(RunCli("app", "list", "-l", "foo=rubbish")), Name())
		}).
		Given().
		// remove both name and replace labels means nothing will sync
		Name("").
		When().
		IgnoreErrors().
		Sync("-l", "foo=rubbish").
		DoNotIgnoreErrors().
		Then().
		Expect(Error("", "no apps match selector foo=rubbish")).
		// check we can update the app and it is then sync'd
		Given().
		When().
		Sync("-l", "foo=bar")
}

func TestTrackAppStateAndSyncApp(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(Success(fmt.Sprintf("Service     %s  guestbook-ui  Synced ", DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("apps   Deployment  %s  guestbook-ui  Synced", DeploymentNamespace()))).
		Expect(Event(EventReasonResourceUpdated, "sync")).
		And(func(app *Application) {
			assert.NotNil(t, app.Status.OperationState.SyncResult)
		})
}

func TestAppRollbackSuccessful(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
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
				Source:     app.Spec.Source,
			}, {
				ID:         2,
				Revision:   "cdb",
				DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-2 * time.Minute)},
				Source:     app.Spec.Source,
			}}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			assert.NoError(t, err)

			app, err = AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			assert.NoError(t, err)

			// sync app and make sure it reaches InSync state
			_, err = RunCli("app", "rollback", app.Name, "1")
			assert.NoError(t, err)

		}).
		Expect(Event(EventReasonOperationStarted, "rollback")).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Equal(t, SyncStatusCodeSynced, app.Status.Sync.Status)
			assert.NotNil(t, app.Status.OperationState.SyncResult)
			assert.Equal(t, 2, len(app.Status.OperationState.SyncResult.Resources))
			assert.Equal(t, OperationSucceeded, app.Status.OperationState.Phase)
			assert.Equal(t, 3, len(app.Status.History))
		})
}

func TestComparisonFailsIfClusterNotAdded(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		DestServer("https://not-registered-cluster/api").
		When().
		IgnoreErrors().
		Create().
		Then().
		Expect(DoesNotExist())
}

func TestCannotSetInvalidPath(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		IgnoreErrors().
		AppSet("--path", "garbage").
		Then().
		Expect(Error("", "app path does not exist"))
}

func TestManipulateApplicationResources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			manifests, err := RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML([]byte(manifests))
			assert.NoError(t, err)

			index := -1
			for i := range resources {
				if resources[i].GetKind() == kube.DeploymentKind {
					index = i
					break
				}
			}

			assert.True(t, index > -1)

			deployment := resources[index]

			closer, client, err := ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer io.Close(closer)

			_, err = client.DeleteResource(context.Background(), &applicationpkg.ApplicationResourceDeleteRequest{
				Name:         &app.Name,
				Group:        deployment.GroupVersionKind().Group,
				Kind:         deployment.GroupVersionKind().Kind,
				Version:      deployment.GroupVersionKind().Version,
				Namespace:    deployment.GetNamespace(),
				ResourceName: deployment.GetName(),
			})
			assert.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func assetSecretDataHidden(t *testing.T, manifest string) {
	secret, err := UnmarshalToUnstructured(manifest)
	assert.NoError(t, err)

	_, hasStringData, err := unstructured.NestedMap(secret.Object, "stringData")
	assert.NoError(t, err)
	assert.False(t, hasStringData)

	secretData, hasData, err := unstructured.NestedMap(secret.Object, "data")
	assert.NoError(t, err)
	assert.True(t, hasData)
	for _, v := range secretData {
		assert.Regexp(t, regexp.MustCompile(`[*]*`), v)
	}
	var lastAppliedConfigAnnotation string
	annotations := secret.GetAnnotations()
	if annotations != nil {
		lastAppliedConfigAnnotation = annotations[v1.LastAppliedConfigAnnotation]
	}
	if lastAppliedConfigAnnotation != "" {
		assetSecretDataHidden(t, lastAppliedConfigAnnotation)
	}
}

func TestAppWithSecrets(t *testing.T) {
	closer, client, err := ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer io.Close(closer)

	Given(t).
		Path("secrets").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res := FailOnErr(client.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Namespace:    app.Spec.Destination.Namespace,
				Kind:         kube.SecretKind,
				Group:        "",
				Name:         &app.Name,
				Version:      "v1",
				ResourceName: "test-secret",
			})).(*applicationpkg.ApplicationResourceResponse)
			assetSecretDataHidden(t, res.Manifest)

			manifests, err := client.GetManifests(context.Background(), &applicationpkg.ApplicationManifestQuery{Name: &app.Name})
			errors.CheckError(err)

			for _, manifest := range manifests.Manifests {
				assetSecretDataHidden(t, manifest)
			}

			diffOutput := FailOnErr(RunCli("app", "diff", app.Name)).(string)
			assert.Empty(t, diffOutput)

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
			diffOutput, err := RunCli("app", "diff", app.Name)
			assert.Error(t, err)
			assert.Contains(t, diffOutput, "username: ++++++++")
			assert.Contains(t, diffOutput, "password: ++++++++++++")

			// local diff should ignore secrets
			diffOutput = FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)

			// ignore missing field and make sure diff shows no difference
			app.Spec.IgnoreDifferences = []ResourceIgnoreDifferences{{
				Kind: kube.SecretKind, JSONPointers: []string{"/data"},
			}}
			FailOnErr(client.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: app.Spec}))
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput := FailOnErr(RunCli("app", "diff", app.Name)).(string)
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
			diffOutput := FailOnErr(RunCli("app", "diff", app.Name, "--local", "testdata/secrets")).(string)
			assert.Empty(t, diffOutput)
		})
}

func TestResourceDiffing(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Patch deployment
			_, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Patch(context.Background(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`), metav1.PatchOptions{})
			assert.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.Error(t, err)
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
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.NoError(t, err)
			assert.Empty(t, diffOutput)
		})
}

func TestCRDs(t *testing.T) {
	testEdgeCasesApplicationResources(t, "crd-creation", health.HealthStatusHealthy)
}

func TestKnownTypesInCRDDiffing(t *testing.T) {
	dummiesGVR := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "dummies"}

	Given(t).
		Path("crd-creation").
		When().Create().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		And(func() {
			dummyResIf := DynamicClientset.Resource(dummiesGVR).Namespace(DeploymentNamespace())
			patchData := []byte(`{"spec":{"requests": {"cpu": "2"}}}`)
			FailOnErr(dummyResIf.Patch(context.Background(), "dummy-crd-instance", types.MergePatchType, patchData, metav1.PatchOptions{}))
		}).Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"argoproj.io/Dummy": {
					KnownTypeFields: []KnownTypeField{{
						Field: "spec.requests",
						Type:  "core/v1/ResourceList",
					}},
				},
			})
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDuplicatedResources(t *testing.T) {
	testEdgeCasesApplicationResources(t, "duplicated-resources", health.HealthStatusHealthy)
}

func TestConfigMap(t *testing.T) {
	testEdgeCasesApplicationResources(t, "config-map", health.HealthStatusHealthy, "my-map  Synced                configmap/my-map created")
}

func TestFailedConversion(t *testing.T) {
	if os.Getenv("ARGOCD_E2E_K3S") == "true" {
		t.SkipNow()
	}
	defer func() {
		FailOnErr(Run("", "kubectl", "delete", "apiservice", "v1beta1.metrics.k8s.io"))
	}()

	testEdgeCasesApplicationResources(t, "failed-conversion", health.HealthStatusProgressing)
}

func testEdgeCasesApplicationResources(t *testing.T, appPath string, statusCode health.HealthStatusCode, message ...string) {
	expect := Given(t).
		Path(appPath).
		When().
		Create().
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
			diffOutput, err := RunCli("app", "diff", app.Name, "--local", path.Join("testdata", appPath))
			assert.Empty(t, diffOutput)
			assert.NoError(t, err)
		})
}

func TestKsonnetApp(t *testing.T) {
	Given(t).
		Path("ksonnet").
		Env("prod").
		// Null out dest server to verify that destination is inferred from ksonnet app
		Parameter("guestbook-ui=image=gcr.io/heptio-images/ks-guestbook-demo:0.1").
		DestServer("").
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {
			closer, client, err := ArgoCDClientset.NewRepoClient()
			assert.NoError(t, err)
			defer io.Close(closer)

			details, err := client.GetAppDetails(context.Background(), &repositorypkg.RepoAppDetailsQuery{
				Source: &app.Spec.Source,
			})
			assert.NoError(t, err)

			serviceType := ""
			for _, param := range details.Ksonnet.Parameters {
				if param.Name == "type" && param.Component == "guestbook-ui" {
					serviceType = param.Value
				}
			}
			assert.Equal(t, serviceType, "LoadBalancer")
		})
}

const actionsConfig = `discovery.lua: return { sample = {} }
definitions:
- name: sample
  action.lua: |
    obj.metadata.labels.sample = 'test'
    return obj`

func TestResourceAction(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {

			closer, client, err := ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer io.Close(closer)

			actions, err := client.ListResourceActions(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Name:         &app.Name,
				Group:        "apps",
				Kind:         "Deployment",
				Version:      "v1",
				Namespace:    DeploymentNamespace(),
				ResourceName: "guestbook-ui",
			})
			assert.NoError(t, err)
			assert.Equal(t, []ResourceAction{{Name: "sample", Disabled: false}}, actions.Actions)

			_, err = client.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{Name: &app.Name,
				Group:        "apps",
				Kind:         "Deployment",
				Version:      "v1",
				Namespace:    DeploymentNamespace(),
				ResourceName: "guestbook-ui",
				Action:       "sample",
			})
			assert.NoError(t, err)

			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)

			assert.Equal(t, "test", deployment.Labels["sample"])
		})
}

func TestSyncResourceByLabel(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {
			_, _ = RunCli("app", "sync", app.Name, "--label", fmt.Sprintf("app.kubernetes.io/instance=%s", app.Name))
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_, err := RunCli("app", "sync", app.Name, "--label", "this-label=does-not-exist")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "level=fatal")
		})
}

func TestLocalManifestSync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: gcr.io/heptio-images/ks-guestbook-demo:0.2")
		}).
		Given().
		LocalPath(guestbookPathLocal).
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 81")
			assert.Contains(t, res, "image: gcr.io/heptio-images/ks-guestbook-demo:0.3")
		}).
		Given().
		LocalPath("").
		When().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, _ := RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: gcr.io/heptio-images/ks-guestbook-demo:0.2")
		})
}

func TestLocalSync(t *testing.T) {
	Given(t).
		// we've got to use Helm as this uses kubeVersion
		Path("helm").
		When().
		Create().
		Then().
		And(func(app *Application) {
			FailOnErr(RunCli("app", "sync", app.Name, "--local", "testdata/helm"))
		})
}

func TestNoLocalSyncWithAutosyncEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.Name, "--sync-policy", "automated")
			assert.NoError(t, err)

			_, err = RunCli("app", "sync", app.Name, "--local", guestbookPathLocal)
			assert.Error(t, err)
		})
}

func TestLocalSyncDryRunWithAutosyncEnabled(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "set", app.Name, "--sync-policy", "automated")
			assert.NoError(t, err)

			appBefore := app.DeepCopy()
			_, err = RunCli("app", "sync", app.Name, "--dry-run", "--local", guestbookPathLocal)
			assert.NoError(t, err)

			appAfter := app.DeepCopy()
			assert.True(t, reflect.DeepEqual(appBefore, appAfter))
		})
}

func TestSyncAsync(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		Async(true).
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestPermissions(t *testing.T) {
	EnsureCleanState(t)
	appName := Name()
	_, err := RunCli("proj", "create", "test")
	assert.NoError(t, err)

	// make sure app cannot be created without permissions in project
	_, err = RunCli("app", "create", appName, "--repo", RepoURL(RepoURLTypeFile),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", DeploymentNamespace())
	assert.Error(t, err)
	sourceError := fmt.Sprintf("application repo %s is not permitted in project 'test'", RepoURL(RepoURLTypeFile))
	destinationError := fmt.Sprintf("application destination {%s %s} is not permitted in project 'test'", common.KubernetesInternalAPIServerAddr, DeploymentNamespace())

	assert.Contains(t, err.Error(), sourceError)
	assert.Contains(t, err.Error(), destinationError)

	proj, err := AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Get(context.Background(), "test", metav1.GetOptions{})
	assert.NoError(t, err)

	proj.Spec.Destinations = []ApplicationDestination{{Server: "*", Namespace: "*"}}
	proj.Spec.SourceRepos = []string{"*"}
	proj, err = AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Update(context.Background(), proj, metav1.UpdateOptions{})
	assert.NoError(t, err)

	// make sure controller report permissions issues in conditions
	_, err = RunCli("app", "create", appName, "--repo", RepoURL(RepoURLTypeFile),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", DeploymentNamespace())
	assert.NoError(t, err)
	defer func() {
		err = AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Delete(context.Background(), appName, metav1.DeleteOptions{})
		assert.NoError(t, err)
	}()

	proj.Spec.Destinations = []ApplicationDestination{}
	proj.Spec.SourceRepos = []string{}
	_, err = AppClientset.ArgoprojV1alpha1().AppProjects(ArgoCDNamespace).Update(context.Background(), proj, metav1.UpdateOptions{})
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	closer, client, err := ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer io.Close(closer)

	refresh := string(RefreshTypeNormal)
	app, err := client.Get(context.Background(), &applicationpkg.ApplicationQuery{Name: &appName, Refresh: &refresh})
	assert.NoError(t, err)

	destinationErrorExist := false
	sourceErrorExist := false
	for i := range app.Status.Conditions {
		if strings.Contains(app.Status.Conditions[i].Message, destinationError) {
			destinationErrorExist = true
		}
		if strings.Contains(app.Status.Conditions[i].Message, sourceError) {
			sourceErrorExist = true
		}
	}
	assert.True(t, destinationErrorExist)
	assert.True(t, sourceErrorExist)
}

// make sure that if we deleted a resource from the app, it is not pruned if annotated with Prune=false
func TestSyncOptionPruneFalse(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Prune=false"}}]`).
		Create().
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
func TestSyncOptionValidateFalse(t *testing.T) {

	// k3s does not validate at all, so this test does not work
	if os.Getenv("ARGOCD_E2E_K3S") == "true" {
		t.SkipNow()
	}

	Given(t).
		Path("crd-validation").
		When().
		Create().
		Then().
		Expect(Success("")).
		When().
		IgnoreErrors().
		Sync().
		Then().
		// client error
		Expect(Error("error validating data", "")).
		When().
		PatchFile("deployment.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/sync-options": "Validate=false"}}]`).
		Sync().
		Then().
		// server error
		Expect(Error("Error from server", ""))
}

// make sure that, if we have a resource that needs pruning, but we're ignoring it, the app is in-sync
func TestCompareOptionIgnoreExtraneous(t *testing.T) {
	Given(t).
		Prune(false).
		Path("two-nice-pods").
		When().
		PatchFile("pod-1.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/compare-options": "IgnoreExtraneous"}}]`).
		Create().
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

func TestSelfManagedApps(t *testing.T) {

	Given(t).
		Path("self-managed-app").
		When().
		PatchFile("resources.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/spec/source/repoURL", "value": "%s"}]`, RepoURL(RepoURLTypeFile))).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(a *Application) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
			defer cancel()

			reconciledCount := 0
			var lastReconciledAt *metav1.Time
			for event := range ArgoCDClientset.WatchApplicationWithRetry(ctx, a.Name, a.ResourceVersion) {
				reconciledAt := event.Application.Status.ReconciledAt
				if reconciledAt == nil {
					reconciledAt = &metav1.Time{}
				}
				if lastReconciledAt != nil && !lastReconciledAt.Equal(reconciledAt) {
					reconciledCount = reconciledCount + 1
				}
				lastReconciledAt = reconciledAt
			}

			assert.True(t, reconciledCount < 3, "Application was reconciled too many times")
		})
}

func TestExcludedResource(t *testing.T) {
	Given(t).
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}).
		Path(guestbookPath).
		ResourceFilter(settings.ResourcesFilter{
			ResourceExclusions: []settings.FilteredResource{{Kinds: []string{kube.DeploymentKind}}},
		}).
		When().
		Create().
		Sync().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionExcludedResourceWarning, "Resource apps/Deployment guestbook-ui is excluded in the settings"))
}

func TestRevisionHistoryLimit(t *testing.T) {
	Given(t).
		Path("config-map").
		When().
		Create().
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

func TestOrphanedResource(t *testing.T) {
	Given(t).
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true)},
		}).
		Path(guestbookPath).
		When().
		Create().
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
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Group: "Test", Kind: "ConfigMap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true), Ignore: []OrphanedResourceKey{{Kind: "ConfigMap", Name: "orphaned-configmap"}}},
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions()).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

func TestNotPermittedResources(t *testing.T) {
	ctx := Given(t)

	ingress := &networkingv1beta.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-ingress",
			Labels: map[string]string{
				common.LabelKeyAppInstance: ctx.GetName(),
			},
		},
		Spec: networkingv1beta.IngressSpec{
			Rules: []networkingv1beta.IngressRule{{
				IngressRuleValue: networkingv1beta.IngressRuleValue{
					HTTP: &networkingv1beta.HTTPIngressRuleValue{
						Paths: []networkingv1beta.HTTPIngressPath{{
							Path: "/",
							Backend: networkingv1beta.IngressBackend{
								ServiceName: "guestbook-ui",
								ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
							},
						}},
					},
				},
			}},
		},
	}
	defer func() {
		log.Infof("Ingress 'sample-ingress' deleted from %s", ArgoCDNamespace)
		CheckError(KubeClientset.NetworkingV1beta1().Ingresses(ArgoCDNamespace).Delete(context.Background(), "sample-ingress", metav1.DeleteOptions{}))
	}()

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "guestbook-ui",
			Labels: map[string]string{
				common.LabelKeyAppInstance: ctx.GetName(),
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
		SourceRepos:  []string{"*"},
		Destinations: []ApplicationDestination{{Namespace: DeploymentNamespace(), Server: "*"}},
		NamespaceResourceBlacklist: []metav1.GroupKind{
			{Group: "", Kind: "Service"},
		}}).
		And(func() {
			FailOnErr(KubeClientset.NetworkingV1beta1().Ingresses(ArgoCDNamespace).Create(context.Background(), ingress, metav1.CreateOptions{}))
			FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Create(context.Background(), svc, metav1.CreateOptions{}))
		}).
		Path(guestbookPath).
		When().
		Create().
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
			assert.Equal(t, serviceStatus.Status, SyncStatusCodeUnknown, "Service is prohibited managed resource so should be set to Unknown")
			deploymentStatus := statusByKind[kube.DeploymentKind]
			assert.Equal(t, deploymentStatus.Status, SyncStatusCodeOutOfSync)
		}).
		When().
		Delete(true).
		Then().
		Expect(DoesNotExist())

	// Make sure prohibited resources are not deleted during application deletion
	FailOnErr(KubeClientset.NetworkingV1beta1().Ingresses(ArgoCDNamespace).Get(context.Background(), "sample-ingress", metav1.GetOptions{}))
	FailOnErr(KubeClientset.CoreV1().Services(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{}))
}

func TestSyncWithInfos(t *testing.T) {
	expectedInfo := make([]*Info, 2)
	expectedInfo[0] = &Info{Name: "name1", Value: "val1"}
	expectedInfo[1] = &Info{Name: "name2", Value: "val2"}

	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "sync", app.Name,
				"--info", fmt.Sprintf("%s=%s", expectedInfo[0].Name, expectedInfo[0].Value),
				"--info", fmt.Sprintf("%s=%s", expectedInfo[1].Name, expectedInfo[1].Value))
			assert.NoError(t, err)
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.ElementsMatch(t, app.Status.OperationState.Operation.Info, expectedInfo)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource console which does not require namespace
//Expect: no app.Status.Conditions
func TestCreateAppWithNoNameSpaceForGlobalResource(t *testing.T) {
	Given(t).
		Path(globalWithNoNameSpace).
		When().
		CreateWithNoNameSpace().
		Then().
		And(func(app *Application) {
			time.Sleep(500 * time.Millisecond)
			app, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Len(t, app.Status.Conditions, 0)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource deployment, and service which requires namespace
//       Deployment and service do not have namespace in manifest
//Expect: app.Status.Conditions for deployment ans service which does not have namespace in manifest
func TestCreateAppWithNoNameSpaceWhenRequired(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateWithNoNameSpace().
		Then().
		And(func(app *Application) {
			var updatedApp *Application
			for i := 0; i < 3; i++ {
				obj, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
				assert.NoError(t, err)
				updatedApp = obj
				if len(updatedApp.Status.Conditions) > 0 {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, updatedApp.Status.Conditions[0].Type, ApplicationConditionInvalidSpecError)
			assert.Equal(t, updatedApp.Status.Conditions[1].Type, ApplicationConditionInvalidSpecError)
		})
}

//Given: argocd app create does not provide --dest-namespace
//       Manifest contains resource deployment, and service which requires namespace
//       Some deployment and service has namespace in manifest
//       Some deployment and service does not have namespace in manifest
//Expect: app.Status.Conditions for deployment and service which does not have namespace in manifest
func TestCreateAppWithNoNameSpaceWhenRequired2(t *testing.T) {
	Given(t).
		Path(guestbookWithNamespace).
		When().
		CreateWithNoNameSpace().
		Then().
		And(func(app *Application) {
			var updatedApp *Application
			for i := 0; i < 3; i++ {
				obj, err := AppClientset.ArgoprojV1alpha1().Applications(ArgoCDNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
				assert.NoError(t, err)
				updatedApp = obj
				if len(updatedApp.Status.Conditions) > 0 {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			assert.Len(t, updatedApp.Status.Conditions, 2)
			assert.Equal(t, updatedApp.Status.Conditions[0].Type, ApplicationConditionInvalidSpecError)
			assert.Equal(t, updatedApp.Status.Conditions[1].Type, ApplicationConditionInvalidSpecError)
		})
}

func TestListResource(t *testing.T) {
	Given(t).
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: &OrphanedResourcesMonitorSettings{Warn: pointer.BoolPtr(true)},
		}).
		Path(guestbookPath).
		When().
		Create().
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
			output, err := RunCli("app", "resources", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name, "--orphaned=true")
			assert.NoError(t, err)
			assert.Contains(t, output, "orphaned-configmap")
			assert.NotContains(t, output, "guestbook-ui")
		}).
		And(func(app *Application) {
			output, err := RunCli("app", "resources", app.Name, "--orphaned=false")
			assert.NoError(t, err)
			assert.NotContains(t, output, "orphaned-configmap")
			assert.Contains(t, output, "guestbook-ui")
		}).
		Given().
		ProjectSpec(AppProjectSpec{
			SourceRepos:       []string{"*"},
			Destinations:      []ApplicationDestination{{Namespace: "*", Server: "*"}},
			OrphanedResources: nil,
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NoConditions())
}

// Given application is set with --sync-option CreateNamespace=true
//       application --dest-namespace does not exist
// Verity application --dest-namespace is created
//        application sync successful
//        when application is deleted, --dest-namespace is not deleted
func TestNamespaceAutoCreation(t *testing.T) {
	updatedNamespace := getNewNamespace(t)
	defer func() {
		_, err := Run("", "kubectl", "delete", "namespace", updatedNamespace)
		assert.NoError(t, err)
	}()
	Given(t).
		Timeout(30).
		Path("guestbook").
		When().
		Create("--sync-option", "CreateNamespace=true").
		Then().
		And(func(app *Application) {
			//Make sure the namespace we are about to update to does not exist
			_, err := Run("", "kubectl", "get", "namespace", updatedNamespace)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
		}).
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
			//Verify delete app does not delete the namespace auto created
			output, err := Run("", "kubectl", "get", "namespace", updatedNamespace)
			assert.NoError(t, err)
			assert.Contains(t, output, updatedNamespace)
		})
}

func TestFailedSyncWithRetry(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync"}}]`).
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command", "value": ["false"]}]`).
		Create().
		IgnoreErrors().
		Sync("--retry-limit=1", "--retry-backoff-duration=1s").
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(OperationMessageContains("retried 1 times"))
}

func TestCreateDisableValidation(t *testing.T) {
	Given(t).
		Path("baddir").
		When().
		Create("--validate=false").
		Then().
		And(func(app *Application) {
			_, err := RunCli("app", "create", app.Name, "--upsert", "--validate=false", "--repo", RepoURL(RepoURLTypeFile),
				"--path", "baddir2", "--project", app.Spec.Project, "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", DeploymentNamespace())
			assert.NoError(t, err)
		}).
		When().
		AppSet("--path", "baddir3", "--validate=false")

}

func TestCreateFromPartialFile(t *testing.T) {
	partialApp :=
		`metadata:
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
			assert.Equal(t, path, app.Spec.Source.Path)
			assert.Equal(t, []HelmParameter{{Name: "foo", Value: "foo"}}, app.Spec.Source.Helm.Parameters)
		})
}

// Ensure actions work when using a resource action that modifies status and/or spec
func TestCRDStatusSubresourceAction(t *testing.T) {
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
		When().Create().Sync().Then().
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		// tests resource actions on a CRD using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-both")
			assert.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-spec")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "StatusSubResource", "update-status")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "statussubresources", "status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		}).
		// tests resource actions on a CRD *not* using status subresource
		And(func(app *Application) {
			_, err := RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-both")
			assert.NoError(t, err)
			text := FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-both", text)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-both", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-spec")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.spec.foo}")).(string)
			assert.Equal(t, "update-spec", text)

			_, err = RunCli("app", "actions", "run", app.Name, "--kind", "NonStatusSubResource", "update-status")
			assert.NoError(t, err)
			text = FailOnErr(Run(".", "kubectl", "-n", app.Spec.Destination.Namespace, "get", "nonstatussubresources", "non-status-subresource", "-o", "jsonpath={.status.bar}")).(string)
			assert.Equal(t, "update-status", text)
		})
}

func TestAppLogs(t *testing.T) {
	Given(t).
		Path("guestbook-logs").
		When().
		Create().
		Sync().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Deployment", "--group", "", "--name", "guestbook-ui")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Pod")
			assert.NoError(t, err)
			assert.Contains(t, out, "Hi")
		}).
		And(func(app *Application) {
			out, err := RunCli("app", "logs", app.Name, "--kind", "Service")
			assert.NoError(t, err)
			assert.NotContains(t, out, "Hi")
		})
}

func TestAppWaitOperationInProgress(t *testing.T) {
	Given(t).
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
		Create().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		And(func() {
			_, err := RunCli("app", "wait", Name(), "--suspended")
			errors.CheckError(err)
		})
}
