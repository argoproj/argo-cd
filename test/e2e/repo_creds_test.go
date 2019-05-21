package e2e

import (
	"fmt"
	"testing"

	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"

	. "github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

const repoUrl = "https://gitlab.com/argo-cd-test/test-apps.git"
const accessToken = "B5sBDeoqAVUouoHkrovy"
const appPath = "child-base"

// make sure you cannot access a private repo without set-up
func TestCannotAddAppFromPrivateRepoWithOutConfig(t *testing.T) {
	Given(t).
		Repo(repoUrl).
		Path(appPath).
		When().
		Create().
		Then().
		Expect(Error("No credentials available for source repository and repository is not publicly accessible"))
}

// make sure you can access a private repo, if the repo ise set-up in the CM
func TestCanAddAppFromPrivateRepoWithRepoConfig(t *testing.T) {
	Given(t).
		Repo(repoUrl).
		Path(appPath).
		And(func() {
			// I use CLI, but you could also modify the settings, we get a free test of the CLI here
			FailOnErr(fixture.RunCli("repo", "add", repoUrl, "--username", "blah", "--password", accessToken))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}

// make sure you can access a private repo, if the creds are set-up in the CM
func TestCanAddAppFromPrivateRepoWithCredConfig(t *testing.T) {

	Given(t).
		Repo(repoUrl).
		Path(appPath).
		And(func() {
			secretName := fixture.CreateSecret("blah", accessToken)
			FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm", "-p", fmt.Sprintf(`{"data": {"repository.credentials": "- passwordSecret:\n    key: password\n    name: %s\n  url: %s\n  usernameSecret:\n    key: username\n    name: %s\n"}}`, secretName, repoUrl, secretName)))
		}).
		When().
		Create().
		Then().
		Expect(Success(""))
}
