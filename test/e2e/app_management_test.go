package e2e

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	argorepo "github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/diff"
	"github.com/argoproj/argo-cd/util/kube"
)

const (
	guestbookPath = "guestbook"
)

func assertAppHasEvent(t *testing.T, a *v1alpha1.Application, message string, reason string) {
	list, err := fixture.KubeClientset.CoreV1().Events(fixture.ArgoCDNamespace).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(map[string]string{
			"involvedObject.name":      a.Name,
			"involvedObject.uid":       string(a.UID),
			"involvedObject.namespace": fixture.ArgoCDNamespace,
		}).String(),
	})
	assert.NoError(t, err)
	for i := range list.Items {
		event := list.Items[i]
		if event.Reason == reason && strings.Contains(event.Message, message) {
			return
		}
	}
	t.Errorf("Unable to find event with reason=%s; message=%s", reason, message)
}

func getTestApp() *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-app",
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: v1alpha1.ApplicationSource{
				RepoURL: fixture.RepoURL(),
				Path:    guestbookPath,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    common.KubernetesInternalAPIServerAddr,
				Namespace: fixture.DeploymentNamespace,
			},
		},
	}
}

func createAndSync(t *testing.T, beforeCreate func(app *v1alpha1.Application)) *v1alpha1.Application {
	app := getTestApp()
	beforeCreate(app)

	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(app)
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "wait", app.Name, "--sync", "--timeout", "5")
	assert.NoError(t, err)

	app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	return app
}

func createAndSyncDefault(t *testing.T) *v1alpha1.Application {
	return createAndSync(t, func(app *v1alpha1.Application) {
		app.Spec.Source.Path = guestbookPath
	})
}

func TestAppCreation(t *testing.T) {
	fixture.EnsureCleanState()

	appName := "app-" + strconv.FormatInt(time.Now().Unix(), 10)
	_, err := fixture.RunCli("app", "create",
		"--name", appName,
		"--repo", fixture.RepoURL(),
		"--path", guestbookPath,
		"--dest-server", common.KubernetesInternalAPIServerAddr,
		"--dest-namespace", fixture.DeploymentNamespace)
	assert.NoError(t, err)

	var app *v1alpha1.Application
	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(appName, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeOutOfSync, err
	})

	assert.Equal(t, appName, app.Name)
	assert.Equal(t, fixture.RepoURL(), app.Spec.Source.RepoURL)
	assert.Equal(t, guestbookPath, app.Spec.Source.Path)
	assert.Equal(t, fixture.DeploymentNamespace, app.Spec.Destination.Namespace)
	assert.Equal(t, common.KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
	assertAppHasEvent(t, app, "create", argo.EventReasonResourceCreated)
}

func TestAppDeletion(t *testing.T) {
	fixture.EnsureCleanState()

	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(getTestApp())
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeOutOfSync, err
	})

	_, err = fixture.RunCli("app", "delete", app.Name)

	assert.NoError(t, err)

	WaitUntil(t, func() (bool, error) {
		_, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}
		return false, nil
	})

	assertAppHasEvent(t, app, "delete", argo.EventReasonResourceDeleted)
}

func TestTrackAppStateAndSyncApp(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSyncDefault(t)

	assertAppHasEvent(t, app, "sync", argo.EventReasonResourceUpdated)
	assert.Equal(t, v1alpha1.SyncStatusCodeSynced, app.Status.Sync.Status)
	assert.True(t, app.Status.OperationState.SyncResult != nil)
	assert.True(t, app.Status.OperationState.Phase == v1alpha1.OperationSucceeded)
}

func TestAppRollbackSuccessful(t *testing.T) {
	fixture.EnsureCleanState()

	// create app and ensure it's comparison status is not SyncStatusCodeUnknown
	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(getTestApp())
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Revision != "", nil
	})

	appWithHistory := app.DeepCopy()
	appWithHistory.Status.History = []v1alpha1.RevisionHistory{{
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
	patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &v1alpha1.Application{})
	assert.NoError(t, err)

	app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Patch(app.Name, types.MergePatchType, patch)
	assert.NoError(t, err)

	// sync app and make sure it reaches InSync state
	_, err = fixture.RunCli("app", "rollback", app.Name, "1")
	assert.NoError(t, err)

	assertAppHasEvent(t, app, "rollback", argo.EventReasonOperationStarted)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced, err
	})
	assert.Equal(t, v1alpha1.SyncStatusCodeSynced, app.Status.Sync.Status)
	assert.True(t, app.Status.OperationState.SyncResult != nil)
	assert.Equal(t, 2, len(app.Status.OperationState.SyncResult.Resources))
	assert.True(t, app.Status.OperationState.Phase == v1alpha1.OperationSucceeded)
	assert.Equal(t, 3, len(app.Status.History))
}

func TestComparisonFailsIfClusterNotAdded(t *testing.T) {
	fixture.EnsureCleanState()

	invalidApp := getTestApp()
	invalidApp.Spec.Destination.Server = "https://not-registered-cluster/api"

	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(invalidApp)
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeUnknown && len(app.Status.Conditions) > 0, err
	})

	app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, app.Status.Conditions[0].Type)

	_, err = fixture.RunCli("app", "delete", app.Name, "--cascade=false")
	assert.NoError(t, err)
}

func TestArgoCDWaitEnsureAppIsNotCrashing(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSyncDefault(t)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced && app.Status.Health.Status == v1alpha1.HealthStatusHealthy, err
	})

	_, err := fixture.RunCli("app", "set", app.Name, "--path", "crashing-guestbook")
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced && app.Status.Health.Status == v1alpha1.HealthStatusDegraded, err
	})
}

func TestArgoSyncWaves(t *testing.T) {
	fixture.EnsureCleanState()

	_, err := fixture.RunCli("app", "create", "sync-waves",
		"--path", "sync-waves",
		"--repo", fixture.RepoURL(),
		"--path", "sync-waves",
		"--dest-server", common.KubernetesInternalAPIServerAddr,
		"--dest-namespace", fixture.DeploymentNamespace,
		"--sync-policy", "automated")
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get("sync-waves", metav1.GetOptions{})
		return err == nil && getResource("pod-1", app.Status.Resources).Health.Status == v1alpha1.HealthStatusDegraded, err
	})
	assert.NoError(t, err)

	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get("sync-waves", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, v1alpha1.HealthStatusMissing, getResource("pod-2", app.Status.Resources).Health.Status)
}

func getResource(name string, resources []v1alpha1.ResourceStatus) v1alpha1.ResourceStatus {
	for _, candidate := range resources {
		if name == candidate.Name {
			return candidate
		}
	}
	panic(name)
}

func TestManipulateApplicationResources(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSyncDefault(t)

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

	_, err = client.DeleteResource(context.Background(), &application.ApplicationResourceDeleteRequest{
		Name:         &app.Name,
		Group:        deployment.GroupVersionKind().Group,
		Kind:         deployment.GroupVersionKind().Kind,
		Version:      deployment.GroupVersionKind().Version,
		Namespace:    deployment.GetNamespace(),
		ResourceName: deployment.GetName(),
	})
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeOutOfSync, err
	})
}

func TestAppWithSecrets(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSync(t, func(app *v1alpha1.Application) {
		app.Spec.Source.Path = "secrets"
	})

	diffOutput, err := fixture.RunCli("app", "diff", app.Name)
	assert.NoError(t, err)
	assert.Empty(t, diffOutput)

	// patch secret and make sure app is out of sync and diff detects the change
	_, err = fixture.KubeClientset.CoreV1().Secrets(fixture.DeploymentNamespace).Patch(
		"test-secret", types.JSONPatchType, []byte(`[{"op": "remove", "path": "/data/username"}]`))
	assert.NoError(t, err)

	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	refresh := string(v1alpha1.RefreshTypeNormal)
	app, err = client.Get(context.Background(), &application.ApplicationQuery{Name: &app.Name, Refresh: &refresh})
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeOutOfSync, err
	})

	diffOutput, err = fixture.RunCli("app", "diff", app.Name)
	assert.Error(t, err)
	assert.Contains(t, diffOutput, "username: '*********'")

	// local diff should ignore secrets
	diffOutput, err = fixture.RunCli("app", "diff", app.Name, "--local", "testdata/secrets")
	assert.NoError(t, err)
	assert.Empty(t, diffOutput)

	// ignore missing field and make sure diff shows no difference
	app.Spec.IgnoreDifferences = []v1alpha1.ResourceIgnoreDifferences{{
		Kind: kube.SecretKind, JSONPointers: []string{"/data/username"},
	}}
	_, err = client.UpdateSpec(context.Background(), &application.ApplicationUpdateSpecRequest{Name: &app.Name, Spec: app.Spec})

	assert.NoError(t, err)

	app, err = client.Get(context.Background(), &application.ApplicationQuery{Name: &app.Name, Refresh: &refresh})
	assert.NoError(t, err)
	assert.Equal(t, string(v1alpha1.SyncStatusCodeSynced), string(app.Status.Sync.Status))

	diffOutput, err = fixture.RunCli("app", "diff", app.Name)
	assert.NoError(t, err)
	assert.Empty(t, diffOutput)
}

func TestResourceDiffing(t *testing.T) {
	fixture.EnsureCleanState()

	app := getTestApp()

	// deploy app and make sure it is healthy
	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(app)
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced, err
	})

	// Patch deployment
	_, err = fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace).Patch(
		"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/spec/template/spec/containers/0/image", "value": "test" }]`))
	assert.NoError(t, err)

	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	refresh := string(v1alpha1.RefreshTypeNormal)
	app, err = client.Get(context.Background(), &application.ApplicationQuery{Name: &app.Name, Refresh: &refresh})
	assert.NoError(t, err)

	// Make sure application is out of sync due to deployment image difference
	assert.Equal(t, string(v1alpha1.SyncStatusCodeOutOfSync), string(app.Status.Sync.Status))
	diffOutput, _ := fixture.RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
	assert.Contains(t, diffOutput, fmt.Sprintf("===== apps/Deployment %s/guestbook-ui ======", fixture.DeploymentNamespace))

	// Update settings to ignore image difference
	settings, err := fixture.SettingsManager.GetSettings()
	assert.NoError(t, err)
	settings.ResourceOverrides = map[string]v1alpha1.ResourceOverride{
		"apps/Deployment": {IgnoreDifferences: ` jsonPointers: ["/spec/template/spec/containers/0/image"]`},
	}
	err = fixture.SettingsManager.SaveSettings(settings)
	assert.NoError(t, err)

	app, err = client.Get(context.Background(), &application.ApplicationQuery{Name: &app.Name, Refresh: &refresh})
	assert.NoError(t, err)

	// Make sure application is in synced state and CLI show no difference
	assert.Equal(t, string(v1alpha1.SyncStatusCodeSynced), string(app.Status.Sync.Status))

	diffOutput, err = fixture.RunCli("app", "diff", app.Name, "--local", "testdata/guestbook")
	assert.Empty(t, diffOutput)
	assert.NoError(t, err)
}

func TestEdgeCasesApplicationResources(t *testing.T) {

	apps := map[string]string{
		"DeprecatedExtensions": "deprecated-extensions",
		"CRDs":                 "crd-creation",
		"DuplicatedResources":  "duplicated-resources",
		"FailedConversion":     "failed-conversion",
	}

	for name, appPath := range apps {
		t.Run(fmt.Sprintf("Test%s", name), func(t *testing.T) {
			fixture.EnsureCleanState()

			app := createAndSync(t, func(app *v1alpha1.Application) {
				app.Spec.Source.Path = appPath
			})

			closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
			assert.NoError(t, err)
			defer util.Close(closer)

			refresh := string(v1alpha1.RefreshTypeNormal)
			app, err = client.Get(context.Background(), &application.ApplicationQuery{Name: &app.Name, Refresh: &refresh})
			assert.NoError(t, err)

			assert.Equal(t, string(v1alpha1.SyncStatusCodeSynced), string(app.Status.Sync.Status))
			diffOutput, err := fixture.RunCli("app", "diff", app.Name, "--local", path.Join("testdata", appPath))
			assert.Empty(t, diffOutput)
			assert.NoError(t, err)
		})
	}
}

func TestKsonnetApp(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSync(t, func(app *v1alpha1.Application) {
		app.Spec.Source.Path = "ksonnet"
		app.Spec.Source.Ksonnet = &v1alpha1.ApplicationSourceKsonnet{
			Environment: "prod",
			Parameters: []v1alpha1.KsonnetParameter{{
				Component: "guestbook-ui",
				Name:      "image",
				Value:     "gcr.io/heptio-images/ks-guestbook-demo:0.1",
			}},
		}
	})
	closer, client, err := fixture.ArgoCDClientset.NewRepoClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	details, err := client.GetAppDetails(context.Background(), &repository.RepoAppDetailsQuery{
		Path:     app.Spec.Source.Path,
		Repo:     app.Spec.Source.RepoURL,
		Revision: app.Spec.Source.TargetRevision,
		Ksonnet:  &argorepo.KsonnetAppDetailsQuery{Environment: "prod"},
	})
	assert.NoError(t, err)

	serviceType := ""
	for _, param := range details.Ksonnet.Parameters {
		if param.Name == "type" && param.Component == "guestbook-ui" {
			serviceType = param.Value
		}
	}
	assert.Equal(t, serviceType, "LoadBalancer")
}

const actionsConfig = `discovery.lua: return { sample = {} }
definitions:
- name: sample
  action.lua: |
    obj.metadata.labels.sample = 'test'
    return obj`

func TestResourceAction(t *testing.T) {
	fixture.EnsureCleanState()

	settings, err := fixture.SettingsManager.GetSettings()
	assert.NoError(t, err)

	settings.ResourceOverrides = map[string]v1alpha1.ResourceOverride{"apps/Deployment": {Actions: actionsConfig}}
	err = fixture.SettingsManager.SaveSettings(settings)
	assert.NoError(t, err)

	app := createAndSyncDefault(t)

	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	actions, err := client.ListResourceActions(context.Background(), &application.ApplicationResourceRequest{
		Name:         &app.Name,
		Group:        "apps",
		Kind:         "Deployment",
		Version:      "v1",
		Namespace:    fixture.DeploymentNamespace,
		ResourceName: "guestbook-ui",
	})
	assert.NoError(t, err)
	assert.Equal(t, []v1alpha1.ResourceAction{{Name: "sample"}}, actions.Actions)

	_, err = client.RunResourceAction(context.Background(), &application.ResourceActionRunRequest{Name: &app.Name,
		Group:        "apps",
		Kind:         "Deployment",
		Version:      "v1",
		Namespace:    fixture.DeploymentNamespace,
		ResourceName: "guestbook-ui",
		Action:       "sample",
	})
	assert.NoError(t, err)

	deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace).Get("guestbook-ui", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, "test", deployment.Labels["sample"])
}

func TestSyncResourceByLabel(t *testing.T) {
	fixture.EnsureCleanState()

	app := createAndSyncDefault(t)

	res, _ := fixture.RunCli("app", "sync", app.Name, "--label",
		fmt.Sprintf("app.kubernetes.io/instance=test-%s", strings.Split(app.Name, "-")[1]))
	assert.Contains(t, res, "guestbook-ui  Synced  Healthy")

	res, _ = fixture.RunCli("app", "sync", app.Name, "--label", "this-label=does-not-exist")
	assert.Contains(t, res, "level=fatal")
}

func TestPermissions(t *testing.T) {
	fixture.EnsureCleanState()
	appName := "test-app"
	_, err := fixture.RunCli("proj", "create", "test")
	assert.NoError(t, err)

	// make sure app cannot be created without permissions in project
	output, err := fixture.RunCli("app", "create", appName, "--repo", fixture.RepoURL(),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace)
	assert.Error(t, err)
	sourceError := fmt.Sprintf("application repo %s is not permitted in project 'test'", fixture.RepoURL())
	destinationError := fmt.Sprintf("application destination {%s %s} is not permitted in project 'test'", common.KubernetesInternalAPIServerAddr, fixture.DeploymentNamespace)

	assert.Contains(t, output, sourceError)
	assert.Contains(t, output, destinationError)

	proj, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Get("test", metav1.GetOptions{})
	assert.NoError(t, err)

	proj.Spec.Destinations = []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}}
	proj.Spec.SourceRepos = []string{"*"}
	proj, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(proj)
	assert.NoError(t, err)

	// make sure controller report permissions issues in conditions
	_, err = fixture.RunCli("app", "create", "test-app", "--repo", fixture.RepoURL(),
		"--path", guestbookPath, "--project", "test", "--dest-server", common.KubernetesInternalAPIServerAddr, "--dest-namespace", fixture.DeploymentNamespace)
	assert.NoError(t, err)
	defer func() {
		err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Delete(appName, &metav1.DeleteOptions{})
		assert.NoError(t, err)
	}()

	proj.Spec.Destinations = []v1alpha1.ApplicationDestination{}
	proj.Spec.SourceRepos = []string{}
	_, err = fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.ArgoCDNamespace).Update(proj)
	assert.NoError(t, err)
	closer, client, err := fixture.ArgoCDClientset.NewApplicationClient()
	assert.NoError(t, err)
	defer util.Close(closer)

	refresh := string(v1alpha1.RefreshTypeNormal)
	app, err := client.Get(context.Background(), &application.ApplicationQuery{Name: &appName, Refresh: &refresh})
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
