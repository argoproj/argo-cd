package e2e

import (
	"fmt"
	"testing"

	. "github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/test/fixture/test_repos"
)

// make sure you cannot create an app from a private repo without set-up
func TestCannotAddAppFromPrivateRepoWithoutCfg(t *testing.T) {
	Given(t).
		Repo(HTTPSTestRepo.URL).
		Path("child-base").
		When().
		Create().
		Then().
		Expect(Error("repository not accessible: authentication required"))
}

// make sure you can create an app from a private repo, if the repo is set-up in the CM
func TestCanAddAppFromPrivateRepoWithRepoCfg(t *testing.T) {
	Given(t).
		Repo(HTTPSTestRepo.URL).
		Path("child-base").
		And(func() {
			// I use CLI, but you could also modify the settings, we get a free test of the CLI here
			FailOnErr(fixture.RunCli("repo", "add", HTTPSTestRepo.URL, "--username", HTTPSTestRepo.Username, "--password", HTTPSTestRepo.Password))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}

// make sure you can create an app from a private repo, if the creds are set-up in the CM
func TestCanAddAppFromPrivateRepoWithCredCfg(t *testing.T) {
	Given(t).
		Repo(HTTPSTestRepo.URL).
		Path("child-base").
		And(func() {
			secretName := fixture.CreateSecret(HTTPSTestRepo.Username, HTTPSTestRepo.Password)
			FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm",
				"-n", fixture.ArgoCDNamespace,
				"-p", fmt.Sprintf(`{"data": {"repository.credentials": "- passwordSecret:\n    key: password\n    name: %s\n  url: %s\n  usernameSecret:\n    key: username\n    name: %s\n"}}`, secretName, HTTPSTestRepo.URL, secretName)))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}
