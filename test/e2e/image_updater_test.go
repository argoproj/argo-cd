package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/imageUpdater"
	// . "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

var desiredUpdatedImages = v1alpha1.KustomizeImages{v1alpha1.KustomizeImage("quay.io/jrao/test-image:1.1.0")}

func TestSimpleKustomizeAppImageUpdate(t *testing.T) {

	Given(t).
		SetAppNamespace(fixture.ArgoCDNamespace).
		Path("kustomize-app").
		NamePrefix("k2-").
		NameSuffix("-deploy1").
		When().
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{SelfHeal: true},
				Retry:     &v1alpha1.RetryStrategy{Limit: 0},
			}
		}).
		Then().
		Expect(SyncStatusIs(v1alpha1.SyncStatusCodeSynced)).
		When().
		Update(func(app *v1alpha1.Application) {
			app.ObjectMeta.Annotations = map[string]string{"argocd-image-updater.argoproj.io/image-list": "test=quay.io/jrao/test-image:1.X.X",
				"argocd-image-updater.argoproj.io/test.update-strategy": "semver"}
		}).
		Then().Expect(ApplicationImageUpdated(desiredUpdatedImages))
}
