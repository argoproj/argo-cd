package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestDeclarativeHappyApp(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestDeclarativeInvalidProject(t *testing.T) {
	Given(t).
		Path("guestbook").
		Project("garbage").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		When().
		// we should be able to delete the app
		// TODO - should cascade work here or not?
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}
