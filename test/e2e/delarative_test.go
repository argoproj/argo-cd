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
		Expect(HealthIs(HealthStatusMissing)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDeclarativeInvalidPath(t *testing.T) {
	Given(t).
		Path("garbage").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		When().
		// TODO - should cascade work here or not?
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}

func TestDeclarativeInvalidProject(t *testing.T) {
	Given(t).
		Path("guestbook").
		Project("garbage").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(HealthIs(HealthStatusUnknown)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		When().
		// TODO - should cascade work here or not?
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}

func TestDeclarativeInvalidRepoURL(t *testing.T) {
	Given(t).
		Repo("http://foo").
		Path("guestbook").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		When().
		// TODO - should cascade work here or not?
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}
