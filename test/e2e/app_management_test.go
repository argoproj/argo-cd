package e2e

import (
	"context"
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// load the gcp plugin (required to authenticate against GKE clusters).
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// load the oidc plugin (required to authenticate with OpenID Connect).

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
			t.Fatal(fmt.Sprintf("Unable to sync app %v", err))
		}

		WaitUntil(t, func() (done bool, err error) {
			app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusSynced, err
		})
		assert.Equal(t, v1alpha1.ComparisonStatusSynced, app.Status.ComparisonResult.Status)
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
			t.Fatal(fmt.Sprintf("Unable to get app %v", err))
		}

		assert.Equal(t, v1alpha1.ComparisonStatusError, app.Status.ComparisonResult.Status)
	})
}
