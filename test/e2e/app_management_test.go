package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/util"
	. "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	guestbookPath      = "guestbook"
	guestbookPathLocal = "./testdata/guestbook_local"
)

func TestAppCreation(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, fixture.Name(), app.Name)
			assert.Equal(t, fixture.RepoURL(fixture.RepoURLTypeFile), app.Spec.Source.RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.Source.Path)
			assert.Equal(t, fixture.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, common.KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := fixture.RunCli("app", "list")
			assert.NoError(t, err)
			assert.Contains(t, output, fixture.Name())
		})
}

// demonstrate that we cannot use a standard sync when an immutable field is changed, we must use "force"
func TestImmutableChange(t *testing.T) {
	text := errors.FailOnErr(fixture.Run(".", "kubectl", "get", "service", "-n", "kube-system", "kube-dns", "-o", "jsonpath={.spec.clusterIP}")).(string)
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
		Expect(HealthIs(HealthStatusHealthy)).
		When().
		PatchFile("service.yaml", fmt.Sprintf(`[{"op": "add", "path": "/spec/clusterIP", "value": "%s"}]`, ip2)).
		IgnoreErrors().
		Sync().
		DoNotIgnoreErrors().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceResultIs(ResourceResult{
			Kind:      "Service",
			Version:   "v1",
			Namespace: fixture.DeploymentNamespace(),
			Name:      "my-service",
			SyncPhase: "Sync",
			Status:    "SyncFailed",
			HookPhase: "Failed",
			Message:   fmt.Sprintf(`kubectl failed exit status 1: The Service "my-service" is invalid: spec.clusterIP: Invalid value: "%s": field is immutable`, ip2),
		})).
		// now we can do this will a force
		Given().
		Force().
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy))
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

	output, err := fixture.RunCli("app", "list")
	assert.NoError(t, err)
	assert.NotContains(t, output, fixture.Name())
}

func TestTrackAppStateAndSyncApp(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		Create().
		Sync().
		Then().
		Expect(Success(fmt.Sprintf("apps  Deployment  %s          guestbook-ui  OutOfSync  Missing", fixture.DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("Service  %s          guestbook-ui  OutOfSync  Missing", fixture.DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("Service     %s  guestbook-ui  Synced  Healthy        service/guestbook-ui created", fixture.DeploymentNamespace()))).
		Expect(Success(fmt.Sprintf("apps   Deployment  %s  guestbook-ui  Synced  Healthy        deployment.apps/guestbook-ui created", fixture.DeploymentNamespace()))).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
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

			app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Patch(app.Name, types.MergePatchType, patch)
			assert.NoError(t, err)

			// sync app and make sure it reaches InSync state
			_, err = fixture.RunCli("app", "rollback", app.Name, "1")
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
			manifests, err := fixture.RunCli("app", "manifests", app.Name, "--source", "live")
			assert.NoError(t, err)
			resources, err := kube.SplitYAML(manifests)
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

			closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer util.Close(closer)

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
	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	Given(t).
		Path("secrets").
		When().
		Create().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			res, err := client.GetResource(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Namespace:    app.Spec.Destination.Namespace,
				Kind:         kube.SecretKind,
				Group:        "",
				Name:         &app.Name,
				Version:      "v1",
				ResourceName: "test-secret",
			})
			assert.NoError(t, err)

			assetSecretDataHidden(t, res.Manifest)

			diffOutput, err := fixture.RunCli("app", "diff", app.Name)
			assert.NoError(t, err)
			assert.Empty(t, diffOutput)

			// patch secret and make sure app is out of sync and diff detects the change
			_, err = fixture.KubeClientset.CoreV1().Secrets(fixture.DeploymentNamespace()).Patch(
				"test-secret", types.JSONPatchType, []byte(`[{"op": "remove", "path": "/data/username"}]`))
			assert.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {

			diffOutput, err := fixture.RunCli("app", "diff", app.Name)
			assert.Error(t, err)
			assert.Contains(t, diffOutput, "username: +++++++++")

			// local diff should ignore secrets
			diffOutput, err = fixture.RunCli("app", "diff", app.Name, "--local", "testdata/secrets")
			assert.NoError(t, err)
			assert.Empty(t, diffOutput)

			// ignore missing field and make sure diff shows no difference
			app.Spec.IgnoreDifferences = []ResourceIgnoreDifferences{{
				Kind: kube.SecretKind, JSONPointers: []string{"/data/username"},
			}}
			_, err = client.UpdateSpec(context.Background(), &applicationpkg.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: app.Spec})

			assert.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name)
			assert.NoError(t, err)
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
			_, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`))
			assert.NoError(t, err)
		}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.Error(t, err)
			assert.Contains(t, diffOutput, fmt.Sprintf("===== apps/Deployment %s/guestbook-ui ======", fixture.DeploymentNamespace()))
		}).
		Given().
		ResourceOverrides(map[string]ResourceOverride{"apps/Deployment": {IgnoreDifferences: ` jsonPointers: ["/spec/template/spec/containers/0/image"]`}}).
		When().
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
			assert.NoError(t, err)
			assert.Empty(t, diffOutput)
		})
}

func TestDeprecatedExtensions(t *testing.T) {
	testEdgeCasesApplicationResources(t, "deprecated-extensions", HealthStatusProgressing)
}

func TestCRDs(t *testing.T) {
	testEdgeCasesApplicationResources(t, "crd-creation", HealthStatusHealthy)
}

func TestDuplicatedResources(t *testing.T) {
	testEdgeCasesApplicationResources(t, "duplicated-resources", HealthStatusHealthy)
}

func TestConfigMap(t *testing.T) {
	testEdgeCasesApplicationResources(t, "config-map", HealthStatusHealthy)
}

func TestFailedConversion(t *testing.T) {

	defer func() {
		errors.FailOnErr(fixture.Run("", "kubectl", "delete", "apiservice", "v1beta1.metrics.k8s.io"))
	}()

	testEdgeCasesApplicationResources(t, "failed-conversion", HealthStatusProgressing)
}

func testEdgeCasesApplicationResources(t *testing.T, appPath string, statusCode HealthStatusCode) {
	Given(t).
		Path(appPath).
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(statusCode)).
		And(func(app *Application) {
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", path.Join("testdata", appPath))
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
			closer, client, err := fixture.ArgoCDClientset.NewRepoClient()
			assert.NoError(t, err)
			defer util.Close(closer)

			details, err := client.GetAppDetails(context.Background(), &repositorypkg.RepoAppDetailsQuery{
				Path:     app.Spec.Source.Path,
				Repo:     app.Spec.Source.RepoURL,
				Revision: app.Spec.Source.TargetRevision,
				Ksonnet:  &apiclient.KsonnetAppDetailsQuery{Environment: "prod"},
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

			closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer util.Close(closer)

			actions, err := client.ListResourceActions(context.Background(), &applicationpkg.ApplicationResourceRequest{
				Name:         &app.Name,
				Group:        "apps",
				Kind:         "Deployment",
				Version:      "v1",
				Namespace:    fixture.DeploymentNamespace(),
				ResourceName: "guestbook-ui",
			})
			assert.NoError(t, err)
			assert.Equal(t, []ResourceAction{{Name: "sample"}}, actions.Actions)

			_, err = client.RunResourceAction(context.Background(), &applicationpkg.ResourceActionRunRequest{Name: &app.Name,
				Group:        "apps",
				Kind:         "Deployment",
				Version:      "v1",
				Namespace:    fixture.DeploymentNamespace(),
				ResourceName: "guestbook-ui",
				Action:       "sample",
			})
			assert.NoError(t, err)

			deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Get("guestbook-ui", metav1.GetOptions{})
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
			_, _ = fixture.RunCli("app", "sync", app.Name, "--label", fmt.Sprintf("app.kubernetes.io/instance=%s", app.Name))
		}).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			_, err := fixture.RunCli("app", "sync", app.Name, "--label", "this-label=does-not-exist")
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
			res, _ := fixture.RunCli("app", "manifests", app.Name)
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
			res, _ := fixture.RunCli("app", "manifests", app.Name)
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
			res, _ := fixture.RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: gcr.io/heptio-images/ks-guestbook-demo:0.2")
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
			_, err := fixture.RunCli("app", "set", app.Name, "--sync-policy", "automated")
			assert.NoError(t, err)

			_, err = fixture.RunCli("app", "sync", app.Name, "--local", guestbookPathLocal)
			assert.Error(t, err)
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
	fixture.EnsureCleanState(t)
	appName := fixture.Name()
	_, err := fixture.RunCli("proj", "create", "test")
	assert.NoError(t, err)

	// make sure app cannot be created without permissions in project
	_, err = fixture.RunCli("app", "create", appName, "--repo", fixture.RepoURL(fixture.RepoURLTypeFile),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace())
	assert.Error(t, err)
	sourceError := fmt.Sprintf("application repo %s is not permitted in project 'test'", fixture.RepoURL(fixture.RepoURLTypeFile))
	destinationError := fmt.Sprintf("application destination {%s %s} is not permitted in project 'test'", common.KubernetesInternalAPIServerAddr, fixture.DeploymentNamespace())

	assert.Contains(t, err.Error(), sourceError)
	assert.Contains(t, err.Error(), destinationError)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get("test", metav1.GetOptions{})
	assert.NoError(t, err)

	proj.Spec.Destinations = []ApplicationDestination{{Server: "*", Namespace: "*"}}
	proj.Spec.SourceRepos = []string{"*"}
	proj, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(proj)
	assert.NoError(t, err)

	// make sure controller report permissions issues in conditions
	_, err = fixture.RunCli("app", "create", appName, "--repo", fixture.RepoURL(fixture.RepoURLTypeFile),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace())
	assert.NoError(t, err)
	defer func() {
		err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Delete(appName, &metav1.DeleteOptions{})
		assert.NoError(t, err)
	}()

	proj.Spec.Destinations = []ApplicationDestination{}
	proj.Spec.SourceRepos = []string{}
	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(proj)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

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
			assert.Equal(t, SyncStatusCodeOutOfSync, app.Status.Resources[1].Status)
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
		PatchFile("resources.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/spec/source/repoURL", "value": "%s"}]`, fixture.RepoURL(fixture.RepoURLTypeFile))).
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
			for event := range fixture.ArgoCDClientset.WatchApplicationWithRetry(ctx, a.Name) {
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
		Then().
		Expect(Condition(ApplicationConditionExcludedResourceWarning, "Resource apps/Deployment guestbook-ui is excluded in the settings"))
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
			errors.FailOnErr(fixture.KubeClientset.CoreV1().ConfigMaps(fixture.DeploymentNamespace()).Create(&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "orphaned-configmap",
				},
			}))
		}).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(Condition(ApplicationConditionOrphanedResourceWarning, "Application has 1 orphaned resources")).
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
