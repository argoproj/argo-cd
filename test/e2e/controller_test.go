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

func TestController(t *testing.T) {
	testApp := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "e2e-test"},
		Spec: v1alpha1.ApplicationSpec{Source: v1alpha1.ApplicationSource{
			RepoURL: "https://github.com/ksonnet/ksonnet.git", Path: ".", Environment: "default",
		}},
	}

	t.Run("TestComparisonErrorIfRepoDoesNotExist", func(t *testing.T) {
		ctrl := fixture.CreateController()
		ctx, cancel := context.WithCancel(context.Background())
		go ctrl.Run(ctx, 1)
		defer cancel()
		app := fixture.CreateApp(t, testApp)

		PollUntil(t, func() (done bool, err error) {
			app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(fmt.Sprintf("Unable to get app %v", err))
		}

		assert.Equal(t, app.Status.ComparisonResult.Status, v1alpha1.ComparisonStatusError)
	})

	t.Run("TestComparisonFailsIfClusterNotAdded", func(t *testing.T) {
		ctrl := fixture.CreateController()
		ctx, cancel := context.WithCancel(context.Background())
		go ctrl.Run(ctx, 1)
		defer cancel()
		_, err := fixture.ApiRepoService.Create(context.Background(), &v1alpha1.Repository{Repo: testApp.Spec.Source.RepoURL, Username: "", Password: ""})
		if err != nil {
			t.Fatal(fmt.Sprintf("Unable to create repo %v", err))
		}
		app := fixture.CreateApp(t, testApp)

		PollUntil(t, func() (done bool, err error) {
			app, err := fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
			return err == nil && app.Status.ComparisonResult.Status != v1alpha1.ComparisonStatusUnknown, err
		})

		app, err = fixture.AppClient.ArgoprojV1alpha1().Applications(fixture.Namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(fmt.Sprintf("Unable to get app %v", err))
		}

		assert.Equal(t, app.Status.ComparisonResult.Status, v1alpha1.ComparisonStatusError)
	})

}
