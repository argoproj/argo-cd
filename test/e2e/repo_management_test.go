package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositorypkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/v3/util/errors"
	argoio "github.com/argoproj/argo-cd/v3/util/io"
)

func TestAddRemovePublicRepo(t *testing.T) {
	app.Given(t).And(func() {
		repoURL := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoURL)
		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(t.Context(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)
		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoURL {
				exists = true
				break
			}
		}
		assert.True(t, exists)

		_, err = fixture.RunCli("repo", "rm", repoURL)
		require.NoError(t, err)

		repo, err = repoClient.ListRepositories(t.Context(), &repositorypkg.RepoQuery{})
		require.NoError(t, err)
		exists = false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoURL {
				exists = true
				break
			}
		}
		assert.False(t, exists)
	})
}

func TestGetRepoWithInheritedCreds(t *testing.T) {
	app.Given(t).And(func() {
		// create repo credentials
		errors.NewHandler(t).FailOnErr(fixture.RunCli("repocreds", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPSOrg), "--github-app-id", fixture.GithubAppID, "--github-app-installation-id", fixture.GithubAppInstallationID, "--github-app-private-key-path", repos.CertKeyPath(t)))

		repoURL := fixture.RepoURL(fixture.RepoURLTypeHTTPS)

		// Hack: First we need to create repo with valid credentials
		errors.NewHandler(t).FailOnErr(fixture.RunCli("repo", "add", repoURL, "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))

		// Then, we remove username/password so that the repo inherits the credentials from our repocreds
		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		_, err = repoClient.UpdateRepository(t.Context(), &repositorypkg.RepoUpdateRequest{
			Repo: &v1alpha1.Repository{
				Repo: repoURL,
			},
		})
		require.NoError(t, err)

		// CLI output should indicate that repo has inherited credentials
		out, err := fixture.RunCli("repo", "get", repoURL)
		require.NoError(t, err)
		assert.Contains(t, out, "inherited")

		_, err = fixture.RunCli("repo", "rm", repoURL)
		require.NoError(t, err)
	})
}

func TestUpsertExistingRepo(t *testing.T) {
	app.Given(t).And(func() {
		repoURL := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoURL)
		require.NoError(t, err)

		_, err = fixture.RunCli("repo", "add", repoURL, "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		require.Error(t, err)

		_, err = fixture.RunCli("repo", "add", repoURL, "--upsert", "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		require.NoError(t, err)
	})
}

func TestAddRemoveHelmRepo(t *testing.T) {
	app.Given(t).CustomCACertAdded().And(func() {
		_, err := fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHelm),
			"--name", "testrepo",
			"--type", "helm",
			"--username", fixture.GitUsername,
			"--password", fixture.GitPassword,
			"--tls-client-cert-path", repos.CertPath(t),
			"--tls-client-cert-key-path", repos.CertKeyPath(t))
		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(t.Context(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)
		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
				exists = true
				break
			}
		}
		assert.True(t, exists)

		_, err = fixture.RunCli("repo", "rm", fixture.RepoURL(fixture.RepoURLTypeHelm))
		require.NoError(t, err)

		repo, err = repoClient.ListRepositories(t.Context(), &repositorypkg.RepoQuery{})
		require.NoError(t, err)
		exists = false
		for i := range repo.Items {
			if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
				exists = true
				break
			}
		}
		assert.False(t, exists)
	})
}

func TestAddHelmRepoInsecureSkipVerify(t *testing.T) {
	app.Given(t).And(func() {
		_, err := fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHelm),
			"--name", "testrepo",
			"--type", "helm",
			"--username", fixture.GitUsername,
			"--password", fixture.GitPassword,
			"--insecure-skip-server-verification",
			"--tls-client-cert-path", repos.CertPath(t),
			"--tls-client-cert-key-path", repos.CertKeyPath(t))

		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)

		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(t.Context(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)

		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
				exists = true
				break
			}
		}
		assert.True(t, exists)
	})
}

func TestFailOnPrivateRepoCreationWithPasswordAndBearerToken(t *testing.T) {
	app.Given(t).And(func() {
		repoURL := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoURL, "--password", fixture.GitPassword, "--bearer-token", fixture.GitBearerToken)
		require.ErrorContains(t, err, "only --bearer-token or --password is allowed, not both")
	})
}

func TestFailOnCreatePrivateNonHTTPSRepoWithBearerToken(t *testing.T) {
	app.Given(t).And(func() {
		repoURL := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoURL, "--bearer-token", fixture.GitBearerToken)
		require.ErrorContains(t, err, "--bearer-token is only supported for HTTPS repositories")
	})
}

func TestFailOnCreatePrivateNonGitRepoWithBearerToken(t *testing.T) {
	app.Given(t).And(func() {
		repoURL := fixture.RepoURL(fixture.RepoURLTypeHelm)
		_, err := fixture.RunCli("repo", "add", repoURL, "--bearer-token", fixture.GitBearerToken,
			"--insecure-skip-server-verification",
			"--tls-client-cert-path", repos.CertPath(t),
			"--tls-client-cert-key-path", repos.CertKeyPath(t),
			"--name", "testrepo",
			"--type", "helm")
		require.ErrorContains(t, err, "--bearer-token is only supported for Git repositories")
	})
}
