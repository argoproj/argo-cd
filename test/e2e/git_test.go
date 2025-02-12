package e2e

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitSemverResolutionNotUsingConstraint(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("v0.1.0").
		When().
		AddTag("v0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestGitSemverResolutionNotUsingConstraintWithLeadingZero(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("0.1.0").
		When().
		AddTag("0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestGitSemverResolutionUsingConstraint(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("v0.1.*").
		When().
		AddTag("v0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		PatchFile("deployment.yaml", `[
	{"op": "replace", "path": "/metadata/name", "value": "new-app"},
	{"op": "replace", "path": "/spec/replicas", "value": 1}
]`).
		AddTag("v0.1.2").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p corev1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
}

func TestGitSemverResolutionUsingConstraintWithLeadingZero(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("0.1.*").
		When().
		AddTag("0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		PatchFile("deployment.yaml", `[
	{"op": "replace", "path": "/metadata/name", "value": "new-app"},
	{"op": "replace", "path": "/spec/replicas", "value": 1}
]`).
		AddTag("0.1.2").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p corev1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
}

func TestAnnotatedTagResolution(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		When().
		CreateApp().
		And(func() {
			// Create an annotated tag pointing to HEAD
			_, err := fixture.Run(".", "git", "tag", "-a", "v1.0.0", "-m", "Release v1.0.0")
			require.NoError(t, err)
		}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Verify the tag resolves to the commit
			revision := app.Status.Sync.Revision
			assert.True(t, git.IsCommitSHA(revision))
		})
}
