package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
)

func TestSyncToUnsignedCommit(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, " unsigned (key_id=)"))
}

func TestSyncToSignedCommitWithoutKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "signed with key not in keyring (key_id=D56C4FCA57A46444)"))
}

func TestSyncToSignedCommitWithKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestSyncToSignedBranchWithKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		Revision("master").
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestSyncToSignedBranchWithUnknownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		Revision("master").
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "signed with key not in keyring (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestSyncToUnsignedBranch(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Revision("master").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "unsigned (key_id=)"))
}

func TestSyncToSignedTagWithKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Revision("signed-tag").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedTag("signed-tag").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestSyncToSignedTagWithUnknownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Revision("signed-tag").
		Path(guestbookPath).
		Sleep(2).
		When().
		AddSignedTag("signed-tag").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision signed-tag by ")).
		Expect(Condition(ApplicationConditionComparisonError, "signed with key not in keyring (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestSyncToUnsignedAnnotatedTag(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Revision("unsigned-tag").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		// Signed commit with an unsigned annotated tag will validate the tag signature
		AddSignedFile("test.yaml", "null").
		AddAnnotatedTag("unsigned-tag", "message goes here").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision unsigned-tag by ")).
		Expect(Condition(ApplicationConditionComparisonError, "unsigned (key_id=)"))
}

func TestSyncToUnsignedSimpleTag(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg").
		Revision("unsigned-simple-tag").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		// Signed commit with an unsigned not-annotated tag will validate the commit, not the tag
		AddSignedFile("test.yaml", "null").
		AddTag("unsigned-simple-tag").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestNamespacedSyncToUnsignedCommit(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	GivenWithNamespace(t, fixture.AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision ")).
		Expect(Condition(ApplicationConditionComparisonError, "unsigned (key_id=)"))
}

func TestNamespacedSyncToSignedCommitWithUnknownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		SetAppNamespace(fixture.AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision ")).
		Expect(Condition(ApplicationConditionComparisonError, "signed with key not in keyring (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestNamespacedSyncToSignedCommit(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	Given(t).
		SetAppNamespace(fixture.AppNamespace()).
		SetTrackingMethod("annotation").
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}
