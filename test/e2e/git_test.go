package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestGitSemverResolution(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeSSH).
		Recurse().
		CustomSSHKnownHostsAdded().
		Revision("v0.1.*").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeUnknown)).
		When().
		AddTag("v0.1.0").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
