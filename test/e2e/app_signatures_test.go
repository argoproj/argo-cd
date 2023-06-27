package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// Test sync to an unsigned commit in legacy mode
func TestVerificationPolicyLegacyUnsigned(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-legacy").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionSourceVerificationError, "UNSIGNED"))
}

// Test sync to a commit signed by an unknown key in legacy mode
func TestVerificationPolicyLegacyUnknownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-legacy").
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
		Expect(Condition(ApplicationConditionSourceVerificationError, "UNKNOWN"))
}

func TestVerificationPolicyLegacy(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-legacy").
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
		Expect(NoConditions()).
		When().
		AddSignedFile("test2.yaml", "null").
		AddSignedFile("test3.yaml", "null").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestVerificationPolicyUnsignedCommitNoMatchingPolicy(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-nomatch").
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestVerificationPolicySignedCommitFull(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t, WithInitialSignedCommit(true)).
		Project("gpg-full").
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
		When().
		AddSignedFile("test2.yaml", "null").
		AddSignedFile("test3.yaml", "null").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestVerificationPolicySomeUnsignedCommitFull(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t, WithInitialSignedCommit(true)).
		Project("gpg-full").
		Path("guestbook-gpg").
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
		When().
		DeleteFile("guestbook-ui-deployment.yaml").
		AddSignedFile("test3.yaml", "null").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestVerificationPolicySignedBranchWithKnownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
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
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestVerificationPolicySignedBranchWithUnknownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
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
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestVerificationPolicyUnsignedBranch(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
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
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestVerificationPolicySignedTagWithKnownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
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
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestVerificationPolicySignedTagWithKnownKeyFullSigned(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t, WithInitialSignedCommit(true)).
		Project("gpg-full").
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
		Expect(HealthIs(health.HealthStatusHealthy))
}

func TestVerificationPolicySignedTagWithKnownKeyFullUnsigned(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-full").
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
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionSourceVerificationError, "UNSIGNED"))
}

func TestVerificationPolicySignedTagWithUnknownKey(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
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
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestVerificationPolicyUnsignedTag(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-head").
		Revision("unsigned-tag").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddTag("unsigned-tag").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing))
}

func TestVerificationPolicyProgressive(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-progressive").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("first.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		// HEAD is signed, but the initial commit is not, so we expect error
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Given().
		Project("gpg-head").
		When().
		IgnoreErrors().
		CreateApp("--upsert").
		// Refresh(RefreshTypeHard).
		Sync().
		Then().
		// We switched temporarily to head verification, so sync is expected to succeed now
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Given().
		Project("gpg-progressive").
		When().
		AddSignedFile("test2.yaml", "null").
		IgnoreErrors().
		CreateApp("--upsert").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Given().
		Project("gpg-full").
		When().
		IgnoreErrors().
		CreateApp("--upsert").
		// Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(Condition(ApplicationConditionSourceVerificationError, "UNSIGNED"))
}

func TestVerificationPolicyBootstrap(t *testing.T) {
	SkipOnEnv(t, "GPG")
	Given(t).
		Project("gpg-bootstrap").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("first.yaml", "null").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		// HEAD is signed, but the initial commit is not. We're within bootstrap period, so the sync should work.
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		AddSignedFile("second.yaml", "null").
		IgnoreErrors().
		CreateApp("--upsert").
		Sync().
		Then().
		// We switched temporarily to head verification, so sync is expected to succeed now
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Given().
		Project("gpg-full").
		When().
		IgnoreErrors().
		CreateApp("--upsert").
		// Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(Condition(ApplicationConditionSourceVerificationError, "UNSIGNED"))
}
