package e2e

import (
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"

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
		Expect(Pod(func(p v1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
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
		Expect(Pod(func(p v1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
}
