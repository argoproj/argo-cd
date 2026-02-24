package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

var projectWithNoKeys = AppProjectSpec{
	SourceRepos:  []string{"*"},
	Destinations: []ApplicationDestination{{Namespace: "*", Server: "*"}},
	SourceIntegrity: &SourceIntegrity{
		Git: &SourceIntegrityGit{
			Policies: []*SourceIntegrityGitPolicy{{
				Repos: []SourceIntegrityGitPolicyRepo{{URL: "*"}},
				GPG: &SourceIntegrityGitPolicyGPG{
					Keys: []string{}, // Verifying but permitting no keys
					Mode: "head",
				},
			}},
		},
	},
}

func TestSyncToUnsignedCommit(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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
		Expect(Condition(ApplicationConditionComparisonError, "signed with key not in keyring (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestSyncToSignedCommitWithKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
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

func TestSyncToSignedCommitWithUnallowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
	Given(t).
		ProjectSpec(projectWithNoKeys).
		Path(guestbookPath).
		GPGPublicKeyAdded().
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
		Expect(Condition(ApplicationConditionComparisonError, "signed with unallowed key (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestSyncToSignedBranchWithKnownKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
	Given(t).
		Project("gpg").
		Revision("master").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddFile("test.yaml", "TestSyncToUnsignedBranch").
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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

func TestSyncToSignedAnnotatedTagWithUnallowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
	Given(t).
		ProjectSpec(projectWithNoKeys).
		Revision("v1.0").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddFile("test.yaml", "null").
		AddSignedTag("v1.0").
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(health.HealthStatusMissing)).
		Expect(Condition(ApplicationConditionComparisonError, "GIT/GPG: Failed verifying revision v1.0")).
		Expect(Condition(ApplicationConditionComparisonError, "signed with unallowed key (key_id="+fixture.GpgGoodKeyID+")"))
}

func TestSyncToTagBasedConstraint(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
	Given(t).
		Project("gpg").
		Revision("1.*").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		AddSignedTag("1.0").
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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
	fixture.EnsureCleanState(t)
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

func TestLocalManifestRejectedWithSourceIntegrity(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
	Given(t).
		Project("gpg").
		Path(guestbookPath).
		GPGPublicKeyAdded().
		Sleep(2).
		When().
		AddSignedFile("test.yaml", "null").
		CreateApp().
		Sync().
		Then().
		And(func(app *Application) {
			res, _ := fixture.RunCli("app", "manifests", app.Name)
			assert.Contains(t, res, "containerPort: 80")
			assert.Contains(t, res, "image: quay.io/argoprojlabs/argocd-e2e-container:0.2")
		}).
		Given().
		LocalPath(guestbookPathLocal).
		When().
		IgnoreErrors().
		Sync("--local-repo-root", ".").
		Then().
		Expect(ErrorRegex("", "Cannot use local manifests when source integrity is enforced"))
}

func TestOCISourceIgnoredWithSourceIntegrity(t *testing.T) {
	fixture.SkipOnEnv(t, "GPG")
	fixture.EnsureCleanState(t)
	// No keys in keyring, no keys in project, OCI is not git, yet source integrity is defined.
	// Expecting some of that would cause visible failure if the source integrity should be applied
	Given(t).
		Project("gpg").
		ProjectSpec(appProjectWithSourceIntegrity()).
		HTTPSInsecureRepoURLWithClientCertAdded().
		PushImageToOCIRegistry("testdata/guestbook", "1.0.0").
		OCIRepoAdded("my-oci-repo", "guestbook").
		OCIRegistry(fixture.OCIHostURL).
		OCIRegistryPath("guestbook").
		RepoURLType(fixture.RepoURLTypeOCI).
		Revision("1.0.0").
		Path(".").
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// Verify local manifests are permitted - source integrity criteria for git should not apply for oci
		Given().
		LocalPath(guestbookPathLocal).
		When().
		DoNotIgnoreErrors().
		Sync("--local-repo-root", ".", "--force", "--prune")
}

func TestHelmSourceIntegrityRepoClash(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	fixture.EnsureCleanState(t)

	workingApp := Given(t).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeNone))

	brokenApp := GivenWithSameState(workingApp).
		SetAppNamespace(fixture.ArgoCDAppNamespace).
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, "D56C4FCA57A46444"))

	expectHelmRepoClashWorkingAppState(workingApp.When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then())

	expectHelmRepoClashBrokenAppState(brokenApp.When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then())

	// Rerun to ensure app states remain independent across repeated syncs.
	expectHelmRepoClashWorkingAppState(workingApp.When().
		Sync().
		Then())

	expectHelmRepoClashBrokenAppState(brokenApp.When().
		IgnoreErrors().
		Sync().
		Then())
}

func expectHelmRepoClashWorkingAppState(cons *Consequences) {
	cons.
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func expectHelmRepoClashBrokenAppState(cons *Consequences) {
	cons.
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "provenance file (.prov) is required but missing"))
}

func appProjectWithHelmSourceIntegrity(mode SourceIntegrityHelmPolicyProvenanceMode, keys ...string) AppProjectSpec {
	if keys == nil {
		keys = []string{}
	}
	return AppProjectSpec{
		SourceRepos:      []string{"*"},
		SourceNamespaces: []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Helm: &SourceIntegrityHelm{
				Policies: []*SourceIntegrityHelmPolicy{{
					Repos: []SourceIntegrityHelmPolicyRepo{{URL: "*"}},
					Provenance: &SourceIntegrityHelmPolicyProvenance{
						Mode: mode,
						Keys: keys,
					},
				}},
			},
		},
	}
}
