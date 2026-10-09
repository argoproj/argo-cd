package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

const gpgInspectRepoProject = "gpg"

func appProjectWithGitPolicies(policies ...*SourceIntegrityGitPolicy) AppProjectSpec {
	return AppProjectSpec{
		SourceRepos:  []string{"*"},
		Destinations: []ApplicationDestination{{Namespace: "*", Server: "*"}},
		SourceIntegrity: &SourceIntegrity{
			Git: &SourceIntegrityGit{
				Policies: policies,
			},
		},
	}
}

func runGpgInspectRepo(t *testing.T, project, app string) (stdout string, err error) {
	t.Helper()
	return RunCli("proj", "source-integrity", "git", "gpg-inspect-repo", project, app)
}

func assertExitStatus(t *testing.T, err error, want int) {
	t.Helper()
	if want == 0 {
		require.NoError(t, err)
		return
	}
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("exit status %d", want))
}

func gitHeadRevision(t *testing.T) string {
	t.Helper()
	out, err := Run(LocalRepoRoot(), "git", "rev-parse", "HEAD")
	require.NoError(t, err)
	return strings.TrimSpace(out)
}

func TestGpgInspectRepo_NoSourceIntegrity(t *testing.T) {
	SkipOnEnv(t, "GPG")

	Given(t).
		Project("default").
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, "default", app.Name)
			assertExitStatus(t, err, 1)
			assert.Contains(t, out+err.Error(), "no git source integrity configured for project default")
		})
}

func TestGpgInspectRepo_HelmOnly(t *testing.T) {
	SkipOnEnv(t, "GPG", "HELM")

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		RepoURLType(RepoURLTypeHelm).
		Chart("helm").
		Revision("1.0.0").
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			_, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 3)
			assert.Contains(t, err.Error(), "not configured for any source")
		})
}

func TestGpgInspectRepo_SingleSourceValid(t *testing.T) {
	SkipOnEnv(t, "GPG")

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		GPGPublicKeyAdded().
		Sleep(2).
		Path(guestbookPath).
		When().
		AddSignedSealCommit().
		AddSignedFile("test.yaml", "null").
		CreateApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 0)
			assert.Contains(t, out, "Source passes strict Git/GPG source integrity checks")
		})
}

func TestGpgInspectRepo_SingleSourceProblematic(t *testing.T) {
	SkipOnEnv(t, "GPG")

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		GPGPublicKeyAdded().
		Sleep(2).
		Path(guestbookPath).
		When().
		AddSignedSealCommit().
		AddFile("test.yaml", "bad commit").
		CreateApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 2)
			assert.Contains(t, out, "PROBLEMATIC COMMITS:")
			assert.Contains(t, out, "unsigned")
		})
}

func TestGpgInspectRepo_MultiSourceMixed(t *testing.T) {
	SkipOnEnv(t, "GPG", "HELM")
	// Git history that is tested:
	//   initial (unsigned) → unsigned-marker (unsigned, pinned by "invalid") → seal (signed) → signed-marker (signed, HEAD / "valid")

	ctx := Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		GPGPublicKeyAdded().
		Sleep(2)

	ctx.When().AddFile("unsigned-marker.yaml", "unsigned")

	unsignedRevision := gitHeadRevision(t)

	GivenWithSameState(ctx).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		CustomCACertAdded().
		HelmRepoAdded("custom-repo").
		Sources([]ApplicationSource{
			{
				RepoURL:        RepoURL(RepoURLTypeFile),
				Path:           guestbookPath,
				TargetRevision: "HEAD",
				Name:           "valid",
			},
			{
				RepoURL:        RepoURL(RepoURLTypeHelm),
				Chart:          "helm",
				TargetRevision: "1.0.0",
				Name:           "helm-skip",
			},
			{
				RepoURL:        RepoURL(RepoURLTypeFile),
				Path:           "two-nice-pods",
				TargetRevision: unsignedRevision,
				Name:           "invalid",
			},
		}).
		When().
		AddSignedSealCommit().
		AddSignedFile("signed-marker.yaml", "signed").
		CreateMultiSourceApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 2)
			assert.Contains(t, out, "--------------------------------")
			assert.Contains(t, out, "Source passes strict Git/GPG source integrity checks")
			assert.Contains(t, out, "PROBLEMATIC COMMITS:")
			assert.Equal(t, 2, strings.Count(out, "Repo URL:"), "expected only the two git sources to be inspected, the Helm source should be skipped")
		})
}

func TestGpgInspectRepo_SealedHistory(t *testing.T) {
	SkipOnEnv(t, "GPG")

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		GPGPublicKeyAdded().
		Sleep(2).
		Path(guestbookPath).
		When().
		AddFile("a.yaml", "1").
		AddFile("b.yaml", "2").
		AddSignedSealCommit().
		AddSignedFile("c.yaml", "3").
		CreateApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 0)
			assert.Contains(t, out, "Source passes strict Git/GPG source integrity checks")
			assert.NotContains(t, out, "PROBLEMATIC COMMITS:")
		})
}

func TestGpgInspectRepo_NoMatchingPolicy(t *testing.T) {
	SkipOnEnv(t, "GPG")

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithGitPolicies(&SourceIntegrityGitPolicy{
			Repos: []SourceIntegrityGitPolicyRepo{{URL: "https://example.com/does-not-match/*"}},
			GPG: &SourceIntegrityGitPolicyGPG{
				Keys: []string{GpgGoodKeyID},
				Mode: SourceIntegrityGitPolicyGPGModeHead,
			},
		})).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 2)
			assert.Contains(t, out, "PROBLEMS: no matching git policy found for source")
		})
}

func TestGpgInspectRepo_MultipleMatchingPolicies(t *testing.T) {
	SkipOnEnv(t, "GPG")

	matchAllPolicy := func() *SourceIntegrityGitPolicy {
		return &SourceIntegrityGitPolicy{
			Repos: []SourceIntegrityGitPolicyRepo{{URL: "*"}},
			GPG: &SourceIntegrityGitPolicyGPG{
				Keys: []string{GpgGoodKeyID},
				Mode: SourceIntegrityGitPolicyGPGModeHead,
			},
		}
	}

	Given(t).
		Project(gpgInspectRepoProject).
		ProjectSpec(appProjectWithSourceIntegrity(GpgGoodKeyID)).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			// Multiple matching policies are rejected at app create time, so create with a
			// valid source integrity first, then introduce the invalid configuration.
			require.NoError(t, SetProjectSpec(gpgInspectRepoProject, appProjectWithGitPolicies(matchAllPolicy(), matchAllPolicy())))

			out, err := runGpgInspectRepo(t, gpgInspectRepoProject, app.Name)
			assertExitStatus(t, err, 2)
			assert.Contains(t, out, "PROBLEMS: multiple (2) git policies found for source")
			assert.Contains(t, out, "invalid configuration")
		})
}
