package e2e

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/server/application"

	"github.com/argoproj/argo-cd/util"

	"github.com/argoproj/argo-cd/util/kube"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/diff"
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

	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(getTestApp())
	assert.NoError(t, err)

	// sync app and make sure it reaches InSync state
	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)
	assertAppHasEvent(t, app, "sync", argo.EventReasonResourceUpdated)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced, err
	})
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
		ID:       1,
		Revision: app.Status.Sync.Revision,
		Source:   app.Spec.Source,
	}, {
		ID:       2,
		Revision: "cdb",
		Source:   app.Spec.Source,
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

	app := getTestApp()

	// deploy app and make sure it is healthy
	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(app)
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced && app.Status.Health.Status == v1alpha1.HealthStatusHealthy, err
	})

	_, err = fixture.RunCli("app", "set", app.Name, "--path", "crashing-guestbook")
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	WaitUntil(t, func() (done bool, err error) {
		app, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		return err == nil && app.Status.Sync.Status == v1alpha1.SyncStatusCodeSynced && app.Status.Health.Status == v1alpha1.HealthStatusDegraded, err
	})
}

func TestManipulateApplicationResources(t *testing.T) {
	fixture.EnsureCleanState()

	app := getTestApp()

	// deploy app and make sure it is healthy
	app, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.ArgoCDNamespace).Create(app)
	assert.NoError(t, err)

	_, err = fixture.RunCli("app", "sync", app.Name)
	assert.NoError(t, err)

	manifests, err := fixture.RunCli("app", "manifests", app.Name, "--source", "live")
	assert.NoError(t, err)

	resources, err := kube.SplitYAML(manifests)
	assert.NoError(t, err)

	assert.Equal(t, 2, len(resources))
	index := sort.Search(len(resources), func(i int) bool {
		return resources[i].GetKind() == kube.DeploymentKind
	})
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
