package e2e

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestGitSubmoduleSSHSupport(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeSSHSubmoduleParent).
		Path("submodule").
		CustomSSHKnownHostsAdded().
		SubmoduleSSHRepoURLAdded(true).
		When().
		CreateFromFile(func(app *Application) { app.Spec.Source.Directory = &ApplicationSourceDirectory{Recurse: true} }).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" }))
}

func TestGitSubmoduleHTTPSSupport(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPSSubmoduleParent).
		Path("submodule").
		CustomCACertAdded().
		SubmoduleHTTPSRepoURLAdded(true).
		When().
		CreateFromFile(func(app *Application) { app.Spec.Source.Directory = &ApplicationSourceDirectory{Recurse: true} }).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "pod-in-submodule" }))
}
