package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
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
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
