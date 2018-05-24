package e2e

import (
	"context"
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
		ObjectMeta: metav1.ObjectMeta{GenerateName: "e2e-test"},
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
		ctrl := fixture.CreateController()
		ctx, cancel := context.WithCancel(context.Background())
		go ctrl.Run(ctx, 1, 1)
		defer cancel()

		// create app and ensure it reaches OutOfSync state
		app := fixture.CreateApp(t, testApp)
		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		assert.Equal(t, v1alpha1.ComparisonStatusOutOfSync, app.Status.ComparisonResult.Status)

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
		appWithHistory.Status.History = []v1alpha1.DeploymentInfo{{
			ID:       1,
			Revision: "abc",
		}, {
			ID:       2,
			Revision: "cdb",
		}}

		ctrl := fixture.CreateController()
		ctx, cancel := context.WithCancel(context.Background())
		go ctrl.Run(ctx, 1, 1)
		defer cancel()

		// create app and ensure it reaches OutOfSync state
		app := fixture.CreateApp(t, appWithHistory)
		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		assert.Equal(t, v1alpha1.ComparisonStatusOutOfSync, app.Status.ComparisonResult.Status)

		// sync app and make sure it reaches InSync state
		_, err := fixture.RunCli("app", "rollback", app.Name, "1")
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

		ctrl := fixture.CreateController()
		ctx, cancel := context.WithCancel(context.Background())
		go ctrl.Run(ctx, 1, 1)
		defer cancel()

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
}
