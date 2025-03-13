package e2e

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestGitSubmoduleSSHSupport(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeSSHSubmoduleParent).
		Path("submodule").
		Recurse().
		CustomSSHKnownHostsAdded().
		SubmoduleSSHRepoURLAdded(true).
		When().
		CreateFromFile(func(app *Application) {}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" }))
}

func TestGitSubmoduleHTTPSSupport(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPSSubmoduleParent).
		Path("submodule").
		Recurse().
		CustomCACertAdded().
		SubmoduleHTTPSRepoURLAdded(true).
		When().
		CreateFromFile(func(app *Application) {}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" }))
}

func TestGitSubmoduleRemovalSupport(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeSSHSubmoduleParent).
		Path("submodule").
		Recurse().
		CustomSSHKnownHostsAdded().
		SubmoduleSSHRepoURLAdded(true).
		When().
		CreateFromFile(func(app *Application) {}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" })).
		When().
		RemoveSubmodule().
		Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" }))
}
