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
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/gpgkeys"
)

const (
	helmOCIProvChartPath = "testdata/helm-oci-provenance"
	helmProvPathSuffix   = "/provenance" // appended to RepoURLTypeHelmParent
	helmProvChart        = "helm-provenance"
	helmProvChartV       = "1.0.0"
	helmProvPassName     = "helm-prov-local-pass"
	helmProvFailName     = "helm-prov-local-fail"
	helmProvWrongKey     = "0000000000000000"
	helmOCIProvChart     = "demo-chart"
	helmOCIProvChartV    = "1.0.0"
	helmOCIProvPassName  = "helm-oci-prov-pass"
	helmOCIProvFailName  = "helm-oci-prov-fail"
	helmOCIProvWrongKey  = "0000000000000000"
)

func helmProvenanceLocalRepoURL() string {
	return strings.TrimSuffix(fixture.RepoURL(fixture.RepoURLTypeHelmParent), "/") + helmProvPathSuffix
}

func TestTraditionalHelmSourceIntegrityProvenancePassesWithAllowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
		HelmProvenanceRepoAdded("helm-provenance-local").
		Name(helmProvPassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, fixture.GpgGoodKeyID)).
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

func TestTraditionalHelmSourceIntegrityProvenanceFailsWithWrongKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
		HelmProvenanceRepoAdded("helm-provenance-local").
		Name(helmProvFailName).
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, helmProvWrongKey)).
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
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
		HelmOCIRepoAdded("helm-oci-provenance").
		Name(helmOCIProvPassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, fixture.GpgGoodKeyID)).
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
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
		HelmOCIRepoAdded("helm-oci-provenance").
		Name(helmOCIProvFailName).
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, helmOCIProvWrongKey)).
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

func TestHelmSourceIntegrityModeNonePasses(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		CustomCACertAdded().
		HelmRepoAdded("helm-mode-none").
		RepoURLType(fixture.RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		Name("helm-mode-none-pass").
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeNone)).
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

func TestHelmOCISourceIntegrityModeNonePasses(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	Given(t).
		PushChartToOCIRegistry(helmOCIProvChartPath, helmOCIProvChart, helmOCIProvChartV).
		HelmOCIRepoAdded("helm-oci-mode-none").
		Name("helm-oci-mode-none-pass").
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeNone)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        fixture.RepoURL(fixture.RepoURLTypeHelmOCI),
				Chart:          helmOCIProvChart,
				TargetRevision: helmOCIProvChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: "helm-oci-mode-none-pass"},
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
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
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
		And(func() { gpgkeys.AddGPGPublicKey(t) }).
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
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeNone))

	brokenApp := GivenWithSameState(workingApp).
		Name("helm-repo-clash-broken").
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

func appProjectWithMultipleHelmPolicies(repoURL string) AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:      []string{"*"},
		SourceNamespaces: []string{"*"},
		Destinations:     []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Helm: &SourceIntegrityHelm{
				Policies: []*SourceIntegrityHelmPolicy{
					{Repos: []SourceIntegrityHelmPolicyRepo{{URL: "*"}}, Provenance: &SourceIntegrityHelmPolicyProvenance{Mode: SourceIntegrityHelmPolicyProvenanceModeProvenance, Keys: []string{fixture.GpgGoodKeyID}}},
					{Repos: []SourceIntegrityHelmPolicyRepo{{URL: repoURL}}, Provenance: &SourceIntegrityHelmPolicyProvenance{Mode: SourceIntegrityHelmPolicyProvenanceModeProvenance, Keys: []string{fixture.GpgGoodKeyID}}},
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
					Provenance: &SourceIntegrityHelmPolicyProvenance{Mode: SourceIntegrityHelmPolicyProvenanceModeProvenance, Keys: []string{helmKeyID}},
				}},
			},
		},
	}
}
