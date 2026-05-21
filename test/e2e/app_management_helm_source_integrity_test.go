package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

const (
	helmOCIProvChartPath        = "testdata/helm-oci-provenance"
	helmProvPathSuffix          = "/provenance" // appended to RepoURLTypeHelmParent
	helmProvChart               = "helm-provenance"
	helmProvChartV              = "1.0.0"
	helmProvMirrorAllFailChartV = "1.0.1" // index entry with no .prov on any mirror URL
	helmProvPassName            = "helm-prov-local-pass"
	helmProvMirrorPassName      = "helm-prov-mirror-pass"
	helmProvMirrorFailName      = "helm-prov-mirror-fail"
	helmProvFailName            = "helm-prov-local-fail"
	helmProvWrongKey            = "0000000000000000"
	helmOCIProvChart            = "demo-chart"
	helmOCIProvChartV           = "1.0.0"
	helmOCIProvPassName         = "helm-oci-prov-pass"
	helmOCIProvFailName         = "helm-oci-prov-fail"
	helmOCIProvWrongKey         = "0000000000000000"
)

func helmProvenanceLocalRepoURL() string {
	return strings.TrimSuffix(fixture.RepoURL(fixture.RepoURLTypeHelmParent), "/") + helmProvPathSuffix
}

func TestTraditionalHelmSourceIntegrityProvenancePassesWithAllowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-provenance-local").
		Name(helmProvPassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(fixture.GpgGoodKeyID)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceLocalRepoURL(),
				Chart:          helmProvChart,
				TargetRevision: helmProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvPassName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

// TestTraditionalHelmSourceIntegrityProvenanceMirrorFallback verifies that Argo CD checks
// all chart URLs from the index for a .prov file and completes GPG provenance verification
func TestTraditionalHelmSourceIntegrityProvenanceMirrorFallback(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-provenance-mirror").
		Name(helmProvMirrorPassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(fixture.GpgGoodKeyID)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceLocalRepoURL(),
				Chart:          helmProvChart,
				TargetRevision: helmProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvMirrorPassName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

// TestTraditionalHelmSourceIntegrityProvenanceMirrorAllFail verifies that when every
// index mirror lacks a .prov file, sync fails with a provenance fetch error after all URLs are tried.
func TestTraditionalHelmSourceIntegrityProvenanceMirrorAllFail(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-provenance-mirror-fail").
		Name(helmProvMirrorFailName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(fixture.GpgGoodKeyID)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceLocalRepoURL(),
				Chart:          helmProvChart,
				TargetRevision: helmProvMirrorAllFailChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvMirrorFailName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "HELM/PROVENANCE")).
		Expect(Condition(ApplicationConditionComparisonError, "could not access chart for provenance verification")).
		Expect(Condition(ApplicationConditionComparisonError, "failed to fetch provenance")).
		Expect(Condition(ApplicationConditionComparisonError, "2 URL(s)"))
}

func TestTraditionalHelmSourceIntegrityProvenanceFailsWithWrongKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-provenance-local").
		Name(helmProvFailName).
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(helmProvWrongKey)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceLocalRepoURL(),
				Chart:          helmProvChart,
				TargetRevision: helmProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvFailName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "HELM/PROVENANCE")).
		Expect(Condition(ApplicationConditionComparisonError, "key_id="+fixture.GpgGoodKeyID))
}

func TestHelmOCISourceIntegrityProvenancePassesWithAllowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		PushChartWithProvenanceToOCIRegistry(helmOCIProvChartPath, helmOCIProvChart, helmOCIProvChartV).
		GPGPublicKeyAdded().
		Sleep(2).
		HelmOCIRepoAdded("helm-oci-provenance").
		Name(helmOCIProvPassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(fixture.GpgGoodKeyID)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
				Chart:          helmOCIProvChart,
				TargetRevision: helmOCIProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmOCIProvPassName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestHelmOCISourceIntegrityProvenanceFailsWithWrongKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		PushChartWithProvenanceToOCIRegistry(helmOCIProvChartPath, helmOCIProvChart, helmOCIProvChartV).
		GPGPublicKeyAdded().
		Sleep(2).
		HelmOCIRepoAdded("helm-oci-provenance").
		Name(helmOCIProvFailName).
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(helmOCIProvWrongKey)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
				Chart:          helmOCIProvChart,
				TargetRevision: helmOCIProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmOCIProvFailName},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "HELM/PROVENANCE")).
		Expect(Condition(ApplicationConditionComparisonError, "key_id="+fixture.GpgGoodKeyID))
}

func TestHelmSourceIntegrityNoVerificationPasses(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("helm-no-verification").
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		Name("helm-no-verification-pass").
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrityNoVerification()).
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

func TestHelmOCISourceIntegrityNoVerificationPasses(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		PushChartToOCIRegistry(helmOCIProvChartPath, helmOCIProvChart, helmOCIProvChartV).
		HelmOCIRepoAdded("helm-oci-no-verification").
		Name("helm-oci-no-verification-pass").
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrityNoVerification()).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
				Chart:          helmOCIProvChart,
				TargetRevision: helmOCIProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: "helm-oci-no-verification-pass"},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestHelmSourceIntegrityMultiplePoliciesFails(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	repoURL := helmProvenanceLocalRepoURL()
	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-multi-pol").
		Name("helm-multi-pol-fail").
		Project("default").
		ProjectSpec(appProjectWithMultipleHelmPolicies(repoURL)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        repoURL,
				Chart:          helmProvChart,
				TargetRevision: helmProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: "helm-multi-pol-fail"},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "multiple (2) Helm source integrity policies found for repo URL"))
}

func TestHelmSourceIntegrityInvalidCRDFails(t *testing.T) {
	fixture.EnsureCleanState(t)
	projYAML := `apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: helm-invalid-crd-e2e
  namespace: ` + fixture.ArgoCDNamespace + `
spec:
  sourceRepos: ["*"]
  destinations: [{namespace: "*", server: "*"}]
  sourceIntegrity: {}
`
	tmpFile := filepath.Join(t.TempDir(), "project.yaml")
	require.NoError(t, os.WriteFile(tmpFile, []byte(projYAML), 0o600))
	out, err := fixture.Run("", "kubectl", "apply", "-f", tmpFile)
	require.Error(t, err)
	msg := out
	if err != nil {
		msg += " " + err.Error()
	}
	require.Contains(t, msg, "sourceIntegrity must specify at least one of git or helm")
}

func TestMultiSourceGitHelmOCIProvenanceAllPass(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	gitURL := fixture.RepoURL(fixture.RepoURLTypeFile)
	helmURL := helmProvenanceLocalRepoURL()
	ociURL := fixture.RepoURL(fixture.RepoURLTypeHelmOCI)

	sources := []ApplicationSource{{
		RepoURL:        gitURL,
		Path:           guestbookPath,
		Name:           "git",
		TargetRevision: "HEAD",
	}, {
		RepoURL:        helmURL,
		Chart:          helmProvChart,
		TargetRevision: helmProvChartV,
		Name:           "helm",
		Helm:           &ApplicationSourceHelm{ReleaseName: "multi-helm"},
	}, {
		RepoURL:        ociURL,
		Chart:          helmOCIProvChart,
		TargetRevision: helmOCIProvChartV,
		Name:           "helm-oci",
		Helm:           &ApplicationSourceHelm{ReleaseName: "multi-helm-oci"},
	}}

	Given(t).
		CustomCACertAdded().
		GPGPublicKeyAdded().
		Sleep(2).
		HelmProvenanceRepoAdded("helm-multi").
		PushChartWithProvenanceToOCIRegistry(helmOCIProvChartPath, helmOCIProvChart, helmOCIProvChartV).
		HelmOCIRepoAdded("helm-oci-multi").
		Sources(sources).
		Name("multi-git-helm-oci-pass").
		Project("gpg").
		ProjectSpec(appProjectWithGitAndHelmSourceIntegrity(fixture.GpgGoodKeyID)).
		When().
		AddSignedFile("multi-source-test.yaml", "test").
		IgnoreErrors().
		CreateMultiSourceAppFromFile(func(_ *Application) { /* no app modifications */ }).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
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
		ProjectSpec(appProjectWithHelmSourceIntegrityNoVerification())

	brokenApp := GivenWithSameState(workingApp).
		Name("helm-repo-clash-broken").
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity("D56C4FCA57A46444"))

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
		Expect(Condition(ApplicationConditionComparisonError, "could not access chart for provenance verification"))
}

func appProjectWithHelmSourceIntegrity(keys ...string) AppProjectSpec {
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
					Repos:      []SourceIntegrityHelmPolicyRepo{{URL: "*"}},
					Provenance: &SourceIntegrityHelmPolicyProvenance{Keys: keys},
				}},
			},
		},
	}
}

func appProjectWithHelmSourceIntegrityNoVerification() AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:      []string{"*"},
		SourceNamespaces: []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Helm: &SourceIntegrityHelm{
				Policies: []*SourceIntegrityHelmPolicy{{
					Repos:      []SourceIntegrityHelmPolicyRepo{{URL: "*"}},
					Provenance: &SourceIntegrityHelmPolicyProvenance{Keys: []string{}},
				}},
			},
		},
	}
}

func appProjectWithMultipleHelmPolicies(repoURL string) AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:      []string{"*"},
		SourceNamespaces: []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Helm: &SourceIntegrityHelm{
				Policies: []*SourceIntegrityHelmPolicy{
					{Repos: []SourceIntegrityHelmPolicyRepo{{URL: "*"}}, Provenance: &SourceIntegrityHelmPolicyProvenance{Keys: []string{fixture.GpgGoodKeyID}}},
					{Repos: []SourceIntegrityHelmPolicyRepo{{URL: repoURL}}, Provenance: &SourceIntegrityHelmPolicyProvenance{Keys: []string{fixture.GpgGoodKeyID}}},
				},
			},
		},
	}
}

func appProjectWithGitAndHelmSourceIntegrity(helmKeyID string) AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:      []string{"*"},
		SourceNamespaces: []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: []*SourceIntegrityGitPolicy{{
					Repos: []SourceIntegrityGitPolicyRepo{{URL: "*"}},
					GPG:   &SourceIntegrityGitPolicyGPG{Keys: []string{fixture.GpgGoodKeyID}, Mode: SourceIntegrityGitPolicyGPGModeHead},
				}},
			},
			Helm: &SourceIntegrityHelm{
				Policies: []*SourceIntegrityHelmPolicy{{
					Repos:      []SourceIntegrityHelmPolicyRepo{{URL: "*"}},
					Provenance: &SourceIntegrityHelmPolicyProvenance{Keys: []string{helmKeyID}},
				}},
			},
		},
	}
}
