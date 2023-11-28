package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/common"
	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestAppSkipReconcileTrue(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		// app should have no status
		CreateFromFile(func(app *Application) {
			app.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}
		}).
		Then().
		Expect(NoStatus())
}

func TestAppSkipReconcileFalse(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		// app should have status
		CreateFromFile(func(app *Application) {
			app.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "false"}
		}).
		Then().
		Expect(StatusExists())
}

func TestAppSkipReconcileNonBooleanValue(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		// app should have status
		CreateFromFile(func(app *Application) {
			app.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "not a boolean value"}
		}).
		Then().
		Expect(StatusExists())
}
