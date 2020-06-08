package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

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
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestDeclarativeInvalidPath(t *testing.T) {
	Given(t).
		Path("garbage").
		When().
		Declarative("declarative-apps/app.yaml").
		Then().
		Expect(Success("")).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		Expect(Condition(ApplicationConditionComparisonError, "garbage: app path does not exist")).
		When().
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
		Expect(HealthIs(health.HealthStatusUnknown)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		Expect(Condition(ApplicationConditionInvalidSpecError, "Application referencing project garbage which does not exist")).
		When().
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}

func TestDeclarativeInvalidRepoURL(t *testing.T) {
	Given(t).
		Path("whatever").
		When().
		DeclarativeWithCustomRepo("declarative-apps/app.yaml", "http://github.com").
		Then().
		Expect(Success("")).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		Expect(Condition(ApplicationConditionComparisonError, "repository not found")).
		When().
		Delete(false).
		Then().
		Expect(Success("")).
		Expect(DoesNotExist())
}
