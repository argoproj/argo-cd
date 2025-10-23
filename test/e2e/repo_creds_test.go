package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// make sure you cannot create an app from a private repo without set-up
func TestCannotAddAppFromPrivateRepoWithoutCfg(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path(fixture.GuestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "repository not accessible"))
}

// make sure you cannot create an app from a private repo without set-up
func TestCannotAddAppFromClientCertRepoWithoutCfg(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPSClientCert).
		Path(fixture.GuestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "repository not accessible"))
}

// make sure you can create an app from a private repo, if the repo is set-up
func TestCanAddAppFromPrivateRepoWithRepoCfg(t *testing.T) {
	Given(t).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path(fixture.LocalOrRemotePath("https-kustomize-base")).
		And(func() {
			// I use CLI, but you could also modify the settings, we get a free test of the CLI here
			errors.NewHandler(t).FailOnErr(fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPS), "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))
		}).
		When().
		CreateApp().
		Then().
		Expect(Success(""))
}

// make sure we can create an app from a private repo, in a secure manner using
// a custom CA certificate bundle
func TestCanAddAppFromPrivateRepoWithCredCfg(t *testing.T) {
	Given(t).
		CustomCACertAdded().
		HTTPSCredentialsUserPassAdded().
		HTTPSRepoURLAdded(false).
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path(fixture.LocalOrRemotePath("https-kustomize-base")).
		When().
		CreateApp().
		Then().
		Expect(Success(""))
}
