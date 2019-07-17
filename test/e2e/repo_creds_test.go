package e2e

import (
	"fmt"
	"testing"

	. "github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// make sure you cannot create an app from a private repo without set-up
func TestCannotAddAppFromPrivateRepoWithoutCfg(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path(guestbookPath).
		When().
		IgnoreErrors().
		Create().
		Then().
		Expect(Error("", "repository not accessible"))
}

// make sure you can create an app from a private repo, if the repo is set-up in the CM
func TestCanAddAppFromPrivateRepoWithRepoCfg(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path("https-kustomize-base").
		And(func() {
			// I use CLI, but you could also modify the settings, we get a free test of the CLI here
			FailOnErr(fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPS), "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}

// make sure you can create an app from a private repo, if the creds are set-up in the CM
func TestCanAddAppFromPrivateRepoWithCredCfg(t *testing.T) {
	Given(t).
		HTTPSRepoURLAdded().
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path("https-kustomize-base").
		And(func() {
			secretName := fixture.CreateSecret(fixture.GitUsername, fixture.GitPassword)
			FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm",
				"-n", fixture.ArgoCDNamespace,
				"-p", fmt.Sprintf(
					`{"data": {"repository.credentials": "- passwordSecret:\n    key: password\n    name: %s\n  url: %s\n  insecure: true\n  usernameSecret:\n    key: username\n    name: %s\n"}}`,
					secretName,
					fixture.RepoURL(fixture.RepoURLTypeHTTPS),
					secretName,
				)))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}
