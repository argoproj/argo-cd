package e2e

import (
	"strconv"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).
	"k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func TestAppManagement(t *testing.T) {
	testApp := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			Source: v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/argoproj/argo-cd.git", Path: ".", Environment: "minikube",
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    fixture.Config.Host,
				Namespace: fixture.Namespace,
			},
		},
	}

	t.Run("TestAppCreation", func(t *testing.T) {
		appName := "app-" + strconv.FormatInt(time.Now().Unix(), 10)
		_, err := fixture.RunCli("app", "create",
			"--name", appName,
			"--repo", "https://github.com/argoproj/argo-cd.git",
			"--env", "minikube",
			"--path", ".",
			"--dest-server", fixture.Config.Host,
			"--dest-namespace", fixture.Namespace)
		if err != nil {
			t.Fatalf("Unable to create app %v", err)
		}

		app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(appName, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get app %v", err)
		}
		assert.Equal(t, appName, app.Name)
		assert.Equal(t, "https://github.com/argoproj/argo-cd.git", app.Spec.Source.RepoURL)
		assert.Equal(t, "minikube", app.Spec.Source.Environment)
		assert.Equal(t, ".", app.Spec.Source.Path)
		assert.Equal(t, fixture.Namespace, app.Spec.Destination.Namespace)
		assert.Equal(t, fixture.Config.Host, app.Spec.Destination.Server)
	})

	t.Run("TestAppDeletion", func(t *testing.T) {
		app := fixture.CreateApp(t, testApp)
		_, err := fixture.RunCli("app", "delete", app.Name)

		if err != nil {
			t.Fatalf("Unable to delete app %v", err)
		}

		_, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.Name, metav1.GetOptions{})

		assert.NotNil(t, err)
		assert.True(t, errors.IsNotFound(err))
	})

	t.Run("TestTrackAppStateAndSyncApp", func(t *testing.T) {
		app := fixture.CreateApp(t, testApp)
		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		// sync app and make sure it reaches InSync state
		_, err := fixture.RunCli("app", "sync", app.Name)
		if err != nil {
			t.Fatalf("Unable to sync app %v", err)
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusSynced, err
		})
		assert.Equal(t, v1alpha1.ComparisonStatusSynced, app.Status.ComparisonResult.Status)
		assert.True(t, app.Status.OperationState.SyncResult != nil)
		assert.True(t, app.Status.OperationState.Phase == v1alpha1.OperationSucceeded)
	})

	t.Run("TestAppRollbackSuccessful", func(t *testing.T) {
		appWithHistory := testApp.DeepCopy()

		// create app and ensure it's comparion status is not ComparisonStatusUnknown
		app := fixture.CreateApp(t, appWithHistory)
		app.Status.History = []v1alpha1.DeploymentInfo{{
			ID:                          1,
			Revision:                    "abc",
			ComponentParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
		}, {
			ID:                          2,
			Revision:                    "cdb",
			ComponentParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
		}}
		app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Update(app)
		if err != nil {
			t.Fatalf("Unable to update app %v", err)
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		// sync app and make sure it reaches InSync state
		_, err = fixture.RunCli("app", "rollback", app.Name, "1")
		if err != nil {
			t.Fatalf("Unable to sync app %v", err)
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusSynced, err
		})
		assert.Equal(t, v1alpha1.ComparisonStatusSynced, app.Status.ComparisonResult.Status)
		assert.True(t, app.Status.OperationState.RollbackResult != nil)
		assert.Equal(t, 2, len(app.Status.OperationState.RollbackResult.Resources))
		assert.True(t, app.Status.OperationState.Phase == v1alpha1.OperationSucceeded)
		assert.Equal(t, 3, len(app.Status.History))
	})

	t.Run("TestComparisonFailsIfClusterNotAdded", func(t *testing.T) {
		invalidApp := testApp.DeepCopy()
		invalidApp.Spec.Destination.Server = "https://not-registered-cluster/api"

		app := fixture.CreateApp(t, invalidApp)

		WaitUntil(t, func() (done bool, err error) {
			app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unable to get app %v", err)
		}

		assert.Equal(t, v1alpha1.ComparisonStatusError, app.Status.ComparisonResult.Status)
	})

	t.Run("TestArgoCDWaitEnsureAppIsNotCrashing", func(t *testing.T) {
		updatedApp := testApp.DeepCopy()

		// deploy app and make sure it is healthy
		app := fixture.CreateApp(t, updatedApp)
		_, err := fixture.RunCli("app", "sync", app.Name)
		if err != nil {
			t.Fatalf("Unable to sync app %v", err)
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusSynced && app.Status.Health.Status == v1alpha1.HealthStatusHealthy, err
		})

		// deploy app which fails and make sure it became unhealthy
		app.Spec.Source.ComponentParameterOverrides = append(
			app.Spec.Source.ComponentParameterOverrides,
			v1alpha1.ComponentParameter{Name: "command", Value: "wrong-command", Component: "guestbook-ui"})
		_, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Update(app)
		if err != nil {
			t.Fatalf("Unable to set app parameter %v", err)
		}

		_, err = fixture.RunCli("app", "sync", app.Name)
		if err != nil {
			t.Fatalf("Unable to sync app %v", err)
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusSynced && app.Status.Health.Status == v1alpha1.HealthStatusDegraded, err
		})
	})
}
