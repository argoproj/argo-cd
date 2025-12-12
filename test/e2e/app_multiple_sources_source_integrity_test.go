package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func appProjectWithSourceIntegrity(keys ...string) AppProjectSpec {
	if keys == nil {
		keys = []string{}
	}
	return AppProjectSpec{
		SourceRepos:  []string{"*"},
		Destinations: []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{{
					Repos: []string{"*"},
					GPG: &SourceIntegrityGitPolicyGPG{
						Keys: keys,
						Mode: SourceIntegrityGitPolicyGPGModeHead,
					},
				}},
			},
		},
	}
}

func TestMultiSourceSourceIntegrityAllFailed(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "uno",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    "two-nice-pods",
	}}
	Given(t).
		Project("gpg").
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		GPGPublicKeyAdded().
		Sources(sources).
		When().
		IgnoreErrors().
		CreateMultiSourceAppFromFile().
		Sync("--retry-limit=0").
		Then().
		// TODO why is it syncing?
		// Expect(OperationPhaseIs(common.OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: source uno: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: source 2 of 2: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "unsigned (key_id=)")).
		// Should start passing after project update
		Given().
		When().
		TerminateOp().                        // TODO: Should not need to stop previous sync
		AddSignedFile("fake.yaml", "change"). // Needs a new commit to avoid using cached manifests
		IgnoreErrors().
		Sync("--retry-limit=0").
		Then().
		// Expect(OperationPhaseIs(common.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestMultiSourceSourceIntegritySomeFailed(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "guestbook",
	}, {
		RepoURL:        "https://github.com/argoproj/argocd-example-apps",
		Path:           "blue-green",
		TargetRevision: "53e28ff20cc530b9ada2173fbbd64d48338583ba", // picking a precise commit so tests have a known signature
		Name:           "blue-green",
	}}
	message := "GIT/GPG: source blue-green: Failed verifying revision 53e28ff20cc530b9ada2173fbbd64d48338583ba by 'May Zhang <may_zhang@intuit.com>': signed with key not in keyring (key_id=4AEE18F83AFDEB23)"
	Given(t).
		Project("gpg").
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		Sources(sources).
		GPGPublicKeyAdded().
		When().
		AddSignedFile("fake.yaml", "").
		IgnoreErrors().
		CreateMultiSourceAppFromFile().
		Then().
		// TODO why is it syncing?
		// Expect(OperationPhaseIs(common.OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, message))
}

func TestMultiSourceSourceIntegrityAllValid(t *testing.T) {
	sources := []ApplicationSource{{
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    guestbookPath,
		Name:    "valid",
	}, {
		RepoURL: RepoURL(RepoURLTypeFile),
		Path:    ".",
		Name:    "also-valid",
	}}
	Given(t).
		Project("gpg").
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		Sources(sources).
		GPGPublicKeyAdded().
		When().
		AddSignedFile("fake.yaml", "").
		IgnoreErrors().
		CreateMultiSourceAppFromFile().
		Sync().
		Then().
		Expect(OperationPhaseIs(common.OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions()).
		// Should start failing after key removal
		Given().
		GPGPublicKeyRemoved().
		When().
		AddSignedFile("fake.yaml", "change"). // Needs a new commit to avoid using cached manifests
		IgnoreErrors().
		Sync().
		Then().
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: source valid: Failed verifying revision")).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: source also-valid: Failed verifying revision"))
}
