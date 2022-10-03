package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestArgoCDWriteBackSingleImageValidApp(t *testing.T) {
	Given(t).
		Path("kustomize-app")
	When().
		// app should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicy{Automated{SelfHeal: false}}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Update(func(app *v1alpha1.Application) {
			app.ObjectMeta.Labels["argocd-image-updater.argoproj.io/image-list"] = "test=quay.io/jrao/test-image:1.X.X"
			app.ObjectMeta.Labels["argocd-image-updater.argoproj.io/test.update-strategy"] = "semver"
		}).Then().Expect()
}
