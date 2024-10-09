package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
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
		Expect(Condition(ApplicationConditionInvalidSpecError, "Application referencing project garbage which does not exist"))

	// TODO: you can`t delete application with invalid project due to enforcment that was recently added,
	// in https://github.com/argoproj/argo-cd/security/advisories/GHSA-2gvw-w6fj-7m3c
	// When().
	// Delete(false).
	// Then().
	// Expect(Success("")).
	// Expect(DoesNotExist())
}

func TestDeclarativeInvalidRepoURL(t *testing.T) {
	Given(t).
		Path("whatever").
		When().
		DeclarativeWithCustomRepo("declarative-apps/app.yaml", "https://github.com").
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
